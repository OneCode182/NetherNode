// Package rcon implements a minimal Source RCON protocol client, used by the
// nethernode CLI to run Minecraft server console commands (save-all flush,
// save-off/save-on, op/deop, and ad-hoc admin commands) over TCP.
//
// Wire format (all integers little-endian int32):
//
//	Size(4) | ID(4) | Type(4) | Body(N bytes) | 0x00 | 0x00
//
// Size counts every field after itself (ID + Type + Body + the two trailing
// NUL bytes). Packet types used here:
//
//	3 SERVERDATA_AUTH          client -> server, login with the RCON password.
//	2 SERVERDATA_AUTH_RESPONSE server -> client, reply to auth (id == -1 on
//	                           failure, id == request id on success).
//	2 SERVERDATA_EXECCOMMAND   client -> server, run a console command.
//	0 SERVERDATA_RESPONSE_VALUE server -> client, command output.
package rcon

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	typeAuth         int32 = 3
	typeAuthResp     int32 = 2
	typeCommand      int32 = 2
	typeResponse     int32 = 0
	authFailedID     int32 = -1
	packetHeaderLen        = 4 + 4 // ID + Type
	packetTrailerLen       = 2     // two NUL bytes
	// maxPacketSize guards against a malicious or broken peer claiming an
	// unreasonably large Size field and forcing an unbounded allocation.
	maxPacketSize = 1 << 21 // 2 MiB
)

// ErrAuthFailed is returned by Dial/DialTimeout when the server rejects the
// RCON password (auth response packet id == -1).
var ErrAuthFailed = errors.New("rcon: authentication failed")

// Client is a connected, authenticated RCON session. It is not safe for
// concurrent use by multiple goroutines; callers issuing overlapping
// Command calls must serialize them (Client itself does so with an internal
// mutex to avoid interleaving packets on the wire).
type Client struct {
	conn    net.Conn
	timeout time.Duration
	mu      sync.Mutex
	nextID  int32
}

// Dial connects to addr (e.g. "127.0.0.1:25575") and authenticates with
// password. It blocks without a deadline; use DialTimeout to bound it.
func Dial(addr, password string) (*Client, error) {
	return DialTimeout(addr, password, 0)
}

// DialTimeout connects to addr and authenticates with password. If d is
// greater than zero it bounds the TCP dial and every subsequent network
// operation (auth handshake and each Command call) with that duration; a
// zero d blocks indefinitely.
func DialTimeout(addr, password string, d time.Duration) (*Client, error) {
	dialer := net.Dialer{Timeout: d}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("rcon: dial %s: %w", addr, err)
	}

	c := &Client{conn: conn, timeout: d, nextID: 1}
	if err := c.authenticate(password); err != nil {
		conn.Close()
		return nil, err
	}
	return c, nil
}

// authenticate performs the SERVERDATA_AUTH handshake. Some server
// implementations send an empty SERVERDATA_RESPONSE_VALUE packet before the
// real SERVERDATA_AUTH_RESPONSE packet; those are skipped.
func (c *Client) authenticate(password string) error {
	c.setDeadline()
	defer c.clearDeadline()

	id := c.newID()
	if err := writePacket(c.conn, id, typeAuth, password); err != nil {
		return fmt.Errorf("rcon: send auth packet: %w", err)
	}

	for {
		pkt, err := readPacket(c.conn)
		if err != nil {
			return fmt.Errorf("rcon: read auth response: %w", err)
		}
		if pkt.id == authFailedID {
			return ErrAuthFailed
		}
		if pkt.typ == typeAuthResp {
			return nil
		}
		// Leading empty SERVERDATA_RESPONSE_VALUE quirk: keep reading.
	}
}

// Command runs cmd on the server and returns its combined output. Large or
// multi-part responses (either split across several RESPONSE_VALUE packets,
// or a single packet whose bytes arrive over multiple TCP reads) are
// reassembled transparently.
//
// End-of-response detection uses the standard RCON trailer trick: a second,
// empty SERVERDATA_EXECCOMMAND packet is sent right after cmd. Its id is
// distinct from the main request, and because the server processes and
// answers requests in order, every packet belonging to cmd's response is
// guaranteed to arrive before the (empty) response to the trailer.
func (c *Client) Command(cmd string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.setDeadline()
	defer c.clearDeadline()

	id := c.newID()
	if err := writePacket(c.conn, id, typeCommand, cmd); err != nil {
		return "", fmt.Errorf("rcon: send command packet: %w", err)
	}
	trailerID := c.newID()
	if err := writePacket(c.conn, trailerID, typeCommand, ""); err != nil {
		return "", fmt.Errorf("rcon: send trailer packet: %w", err)
	}

	var body strings.Builder
	for {
		pkt, err := readPacket(c.conn)
		if err != nil {
			return "", fmt.Errorf("rcon: read command response: %w", err)
		}
		switch pkt.id {
		case trailerID:
			return body.String(), nil
		case id:
			body.WriteString(pkt.body)
		default:
			// Ignore anything unrelated to this exchange.
		}
	}
}

// Close closes the underlying TCP connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// newID returns the next request id, skipping the reserved auth-failure
// sentinel (-1) and zero.
func (c *Client) newID() int32 {
	id := atomic.AddInt32(&c.nextID, 1)
	for id == authFailedID || id == 0 {
		id = atomic.AddInt32(&c.nextID, 1)
	}
	return id
}

func (c *Client) setDeadline() {
	if c.timeout > 0 {
		c.conn.SetDeadline(time.Now().Add(c.timeout))
	}
}

func (c *Client) clearDeadline() {
	if c.timeout > 0 {
		c.conn.SetDeadline(time.Time{})
	}
}

// packet is a decoded RCON packet.
type packet struct {
	id   int32
	typ  int32
	body string
}

// writePacket encodes and writes a single RCON packet to w.
func writePacket(w io.Writer, id, typ int32, body string) error {
	size := int32(packetHeaderLen + len(body) + packetTrailerLen)
	buf := make([]byte, 4+size)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(size))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(id))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(typ))
	copy(buf[12:], body)
	// buf[12+len(body):] already zero-valued (NUL, NUL) from make().
	_, err := w.Write(buf)
	return err
}

// readPacket reads and decodes a single RCON packet from r. It uses
// io.ReadFull throughout, so it correctly reassembles a packet whose bytes
// are delivered across multiple underlying TCP reads (partial reads are not
// mistaken for a short/malformed packet).
func readPacket(r io.Reader) (packet, error) {
	var sizeBuf [4]byte
	if _, err := io.ReadFull(r, sizeBuf[:]); err != nil {
		return packet{}, err
	}
	size := int32(binary.LittleEndian.Uint32(sizeBuf[:]))
	if size < packetHeaderLen+packetTrailerLen {
		return packet{}, fmt.Errorf("rcon: packet size %d too small", size)
	}
	if size > maxPacketSize {
		return packet{}, fmt.Errorf("rcon: packet size %d exceeds max %d", size, maxPacketSize)
	}

	rest := make([]byte, size)
	if _, err := io.ReadFull(r, rest); err != nil {
		return packet{}, fmt.Errorf("rcon: read packet body: %w", err)
	}

	id := int32(binary.LittleEndian.Uint32(rest[0:4]))
	typ := int32(binary.LittleEndian.Uint32(rest[4:8]))
	body := rest[8:]
	// Trim the mandatory trailing NUL(s); be lenient about exact count.
	body = trimTrailingNULs(body)

	return packet{id: id, typ: typ, body: string(body)}, nil
}

func trimTrailingNULs(b []byte) []byte {
	end := len(b)
	for end > 0 && b[end-1] == 0 {
		end--
	}
	return b[:end]
}

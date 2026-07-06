package rcon

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

const testPassword = "s3cr3t"

// startFakeServer listens on 127.0.0.1:0, accepts exactly one connection,
// and runs handle on it in a background goroutine. It returns the listener
// address to dial and registers cleanup to close the listener.
func startFakeServer(t *testing.T, handle func(conn net.Conn)) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		handle(conn)
	}()

	return ln.Addr().String()
}

// serveAuth reads one SERVERDATA_AUTH packet and replies according to
// password match. It returns whether auth succeeded, so callers can decide
// whether to keep serving commands.
func serveAuth(t *testing.T, conn net.Conn, password string) bool {
	t.Helper()

	pkt, err := readPacket(conn)
	if err != nil {
		t.Errorf("fake server: read auth packet: %v", err)
		return false
	}
	if pkt.typ != typeAuth {
		t.Errorf("fake server: expected auth packet type %d, got %d", typeAuth, pkt.typ)
		return false
	}
	if pkt.body != password {
		if err := writePacket(conn, authFailedID, typeAuthResp, ""); err != nil {
			t.Errorf("fake server: write auth-fail response: %v", err)
		}
		return false
	}
	if err := writePacket(conn, pkt.id, typeAuthResp, ""); err != nil {
		t.Errorf("fake server: write auth-ok response: %v", err)
	}
	return true
}

// serveCommands loops reading SERVERDATA_EXECCOMMAND packets. For a
// non-empty command body it calls onCommand to get the response chunks to
// send back (as separate RESPONSE_VALUE packets, each echoing the request
// id, exactly mirroring what a fragmented multi-packet reply looks like).
// For the empty-body trailer packet used by Client.Command to mark the end
// of a response, it echoes an empty RESPONSE_VALUE and keeps looping.
func serveCommands(t *testing.T, conn net.Conn, onCommand func(cmd string) []string) {
	t.Helper()

	for {
		pkt, err := readPacket(conn)
		if err != nil {
			return
		}
		if pkt.body == "" {
			// Trailer packet: echo empty response, ready for next command.
			if err := writePacket(conn, pkt.id, typeResponse, ""); err != nil {
				return
			}
			continue
		}
		for _, chunk := range onCommand(pkt.body) {
			if err := writePacket(conn, pkt.id, typeResponse, chunk); err != nil {
				return
			}
		}
	}
}

func TestDialAuthOK(t *testing.T) {
	addr := startFakeServer(t, func(conn net.Conn) {
		serveAuth(t, conn, testPassword)
	})

	c, err := Dial(addr, testPassword)
	if err != nil {
		t.Fatalf("Dial: unexpected error: %v", err)
	}
	defer c.Close()
}

func TestDialAuthFail(t *testing.T) {
	addr := startFakeServer(t, func(conn net.Conn) {
		serveAuth(t, conn, testPassword)
	})

	_, err := Dial(addr, "wrong-password")
	if err == nil {
		t.Fatal("Dial: expected auth error, got nil")
	}
	if !errors.Is(err, ErrAuthFailed) {
		t.Fatalf("Dial: expected ErrAuthFailed, got %v", err)
	}
}

func TestDialTimeoutUnreachable(t *testing.T) {
	// 192.0.2.0/24 is TEST-NET-1 (RFC 5737): reserved, non-routable, so the
	// dial blocks until the deadline instead of failing fast.
	_, err := DialTimeout("192.0.2.1:25575", testPassword, 50*time.Millisecond)
	if err == nil {
		t.Fatal("DialTimeout: expected error dialing unreachable address")
	}
}

func TestCommandRoundtrip(t *testing.T) {
	addr := startFakeServer(t, func(conn net.Conn) {
		if !serveAuth(t, conn, testPassword) {
			return
		}
		serveCommands(t, conn, func(cmd string) []string {
			if cmd == "say hi" {
				return []string{"hi there"}
			}
			return []string{""}
		})
	})

	c, err := Dial(addr, testPassword)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	got, err := c.Command("say hi")
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	if want := "hi there"; got != want {
		t.Fatalf("Command: got %q, want %q", got, want)
	}

	// A second command on the same connection must also round-trip
	// correctly (id counter keeps advancing, trailer detection still works).
	got2, err := c.Command("noop")
	if err != nil {
		t.Fatalf("second Command: %v", err)
	}
	if got2 != "" {
		t.Fatalf("second Command: got %q, want empty", got2)
	}
}

func TestCommandFragmentedResponse(t *testing.T) {
	addr := startFakeServer(t, func(conn net.Conn) {
		if !serveAuth(t, conn, testPassword) {
			return
		}
		serveCommands(t, conn, func(cmd string) []string {
			if cmd == "big" {
				// Split the logical response across three RESPONSE_VALUE
				// packets sharing the request id, as a real Source/Minecraft
				// server would for output exceeding one packet.
				return []string{"Hello, ", "fragmented ", "World!"}
			}
			return []string{""}
		})
	})

	c, err := Dial(addr, testPassword)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	got, err := c.Command("big")
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	if want := "Hello, fragmented World!"; got != want {
		t.Fatalf("Command: got %q, want %q", got, want)
	}
}

// dribbleReader returns at most one byte per Read call, regardless of the
// caller's buffer size. It simulates a TCP peer whose packet bytes arrive
// split across many short reads, which readPacket must reassemble via
// io.ReadFull rather than assuming one Read returns a whole packet.
type dribbleReader struct {
	r io.Reader
}

func (d dribbleReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return d.r.Read(p[:1])
}

func TestReadPacketHandlesFragmentedTransport(t *testing.T) {
	var buf bytes.Buffer
	const wantID, wantType = int32(7), typeResponse
	const wantBody = "fragmented over many tiny reads"
	if err := writePacket(&buf, wantID, wantType, wantBody); err != nil {
		t.Fatalf("writePacket: %v", err)
	}

	pkt, err := readPacket(dribbleReader{r: &buf})
	if err != nil {
		t.Fatalf("readPacket: %v", err)
	}
	if pkt.id != wantID || pkt.typ != wantType || pkt.body != wantBody {
		t.Fatalf("readPacket: got %+v, want id=%d type=%d body=%q", pkt, wantID, wantType, wantBody)
	}
}

func TestReadPacketRejectsOversizedPacket(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xff, 0xff, 0xff, 0x7f}) // size = 0x7fffffff, way over max
	if _, err := readPacket(&buf); err == nil {
		t.Fatal("readPacket: expected error for oversized packet, got nil")
	}
}

func TestClose(t *testing.T) {
	addr := startFakeServer(t, func(conn net.Conn) {
		serveAuth(t, conn, testPassword)
	})

	c, err := Dial(addr, testPassword)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

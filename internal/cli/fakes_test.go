package cli

import (
	"context"
	"errors"
	"time"

	"github.com/onecode182/nethernode/internal/mcstatus"
)

// execCall records a single fake compose.Runner.Exec invocation.
type execCall struct {
	name string
	args []string
}

// execRecorder is a scriptable stand-in for compose.Runner's Exec hook: it
// logs every call and replays a single scripted (out, err) result for all
// of them, which is enough for the CLI-level tests (they assert on argv,
// not on varying outputs per call).
type execRecorder struct {
	calls []execCall
	out   string
	err   error
}

func (r *execRecorder) exec(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, execCall{name: name, args: append([]string(nil), args...)})
	return r.out, r.err
}

// fakeRCON is a scriptable rconClient: it records every command issued and
// looks up canned (output, error) pairs by exact command string, defaulting
// to ("", nil) for anything not scripted.
type fakeRCON struct {
	responses map[string]string
	errs      map[string]error
	calls     []string
	closed    bool
	closeErr  error
}

func newFakeRCON() *fakeRCON {
	return &fakeRCON{responses: map[string]string{}, errs: map[string]error{}}
}

func (f *fakeRCON) Command(cmd string) (string, error) {
	f.calls = append(f.calls, cmd)
	if err, ok := f.errs[cmd]; ok {
		return "", err
	}
	return f.responses[cmd], nil
}

func (f *fakeRCON) Close() error {
	f.closed = true
	return f.closeErr
}

// errDialUnreachable is the canned dial error used to simulate an
// unreachable/not-running server.
var errDialUnreachable = errors.New("rcon: dial 127.0.0.1:25575: connection refused")

// dialerFor builds an rconDialer that returns client (and records that it
// was invoked via dialed), or dialErr if set.
func dialerFor(client *fakeRCON, dialErr error, dialed *int) rconDialer {
	return func(addr, password string, timeout time.Duration) (rconClient, error) {
		if dialed != nil {
			*dialed++
		}
		if dialErr != nil {
			return nil, dialErr
		}
		return client, nil
	}
}

// fakeMCStatus is a scriptable mcstatusClient.
type fakeMCStatus struct {
	java    *mcstatus.JavaStatus
	javaErr error

	bedrock    *mcstatus.BedrockStatus
	bedrockErr error

	javaAddrs    []string
	bedrockAddrs []string
}

func (f *fakeMCStatus) Java(_ context.Context, address string) (*mcstatus.JavaStatus, error) {
	f.javaAddrs = append(f.javaAddrs, address)
	if f.javaErr != nil {
		return nil, f.javaErr
	}
	return f.java, nil
}

func (f *fakeMCStatus) Bedrock(_ context.Context, address string) (*mcstatus.BedrockStatus, error) {
	f.bedrockAddrs = append(f.bedrockAddrs, address)
	if f.bedrockErr != nil {
		return nil, f.bedrockErr
	}
	return f.bedrock, nil
}

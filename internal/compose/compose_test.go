package compose

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// call records a single Exec invocation.
type call struct {
	name string
	args []string
}

// recorder builds a fake ExecFunc that logs calls and replays scripted results.
type recorder struct {
	calls []call
	out   string
	err   error
}

func (r *recorder) exec(_ context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, call{name: name, args: append([]string(nil), args...)})
	return r.out, r.err
}

func TestRunner_Up(t *testing.T) {
	cases := []struct {
		name    string
		runner  Runner
		wantCmd []string
	}{
		{
			name:    "compose file and project dir",
			runner:  Runner{ComposeFile: "compose.yaml", ProjectDir: "/srv/nethernode"},
			wantCmd: []string{"compose", "-f", "compose.yaml", "--project-directory", "/srv/nethernode", "up", "-d"},
		},
		{
			name:    "no compose file or project dir",
			runner:  Runner{},
			wantCmd: []string{"compose", "up", "-d"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{out: "created"}
			r := tc.runner
			r.Exec = rec.exec

			out, err := r.Up(context.Background())
			if err != nil {
				t.Fatalf("Up() error = %v", err)
			}
			if out != "created" {
				t.Fatalf("Up() output = %q, want %q", out, "created")
			}
			assertSingleCall(t, rec, "docker", tc.wantCmd)
		})
	}
}

func TestRunner_Down(t *testing.T) {
	rec := &recorder{out: "stopped"}
	r := Runner{ComposeFile: "compose.yaml", ProjectDir: "/srv/nethernode", Exec: rec.exec}

	out, err := r.Down(context.Background())
	if err != nil {
		t.Fatalf("Down() error = %v", err)
	}
	if out != "stopped" {
		t.Fatalf("Down() output = %q, want %q", out, "stopped")
	}
	assertSingleCall(t, rec, "docker", []string{"compose", "-f", "compose.yaml", "--project-directory", "/srv/nethernode", "down"})
}

func TestRunner_PS(t *testing.T) {
	rec := &recorder{out: "NAME  STATUS"}
	r := Runner{ComposeFile: "compose.yaml", ProjectDir: "/srv/nethernode", Exec: rec.exec}

	out, err := r.PS(context.Background())
	if err != nil {
		t.Fatalf("PS() error = %v", err)
	}
	if out != "NAME  STATUS" {
		t.Fatalf("PS() output = %q, want %q", out, "NAME  STATUS")
	}
	assertSingleCall(t, rec, "docker", []string{"compose", "-f", "compose.yaml", "--project-directory", "/srv/nethernode", "ps"})
}

func TestRunner_DryRun(t *testing.T) {
	cases := []struct {
		name    string
		call    func(r *Runner) (string, error)
		wantOut string
	}{
		{
			name:    "up",
			call:    func(r *Runner) (string, error) { return r.Up(context.Background()) },
			wantOut: "docker compose -f compose.yaml --project-directory /srv/nethernode up -d",
		},
		{
			name:    "down",
			call:    func(r *Runner) (string, error) { return r.Down(context.Background()) },
			wantOut: "docker compose -f compose.yaml --project-directory /srv/nethernode down",
		},
		{
			name:    "ps",
			call:    func(r *Runner) (string, error) { return r.PS(context.Background()) },
			wantOut: "docker compose -f compose.yaml --project-directory /srv/nethernode ps",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			r := Runner{ComposeFile: "compose.yaml", ProjectDir: "/srv/nethernode", DryRun: true, Exec: rec.exec}

			out, err := tc.call(&r)
			if err != nil {
				t.Fatalf("call error = %v", err)
			}
			if out != tc.wantOut {
				t.Fatalf("dry-run output = %q, want %q", out, tc.wantOut)
			}
			if len(rec.calls) != 0 {
				t.Fatalf("DryRun must not execute, got calls = %+v", rec.calls)
			}
		})
	}
}

func TestRunner_ContainerRunning(t *testing.T) {
	cases := []struct {
		name    string
		out     string
		execErr error
		want    bool
		wantErr bool
	}{
		{name: "running", out: "true\n", want: true},
		{name: "stopped", out: "false\n", want: false},
		{name: "unparseable output", out: "unknown", wantErr: true},
		{name: "exec error", execErr: errors.New("no such object"), wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{out: tc.out, err: tc.execErr}
			r := Runner{Exec: rec.exec}

			got, err := r.ContainerRunning(context.Background(), "nethernode-mc")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ContainerRunning() error = nil, want error")
				}
			} else if err != nil {
				t.Fatalf("ContainerRunning() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("ContainerRunning() = %v, want %v", got, tc.want)
			}
			assertSingleCall(t, rec, "docker", []string{"inspect", "-f", "{{.State.Running}}", "nethernode-mc"})
		})
	}
}

func TestRunner_ContainerRunning_DryRun(t *testing.T) {
	rec := &recorder{out: "true\n"}
	r := Runner{DryRun: true, Exec: rec.exec}

	got, err := r.ContainerRunning(context.Background(), "nethernode-mc")
	if err != nil {
		t.Fatalf("ContainerRunning() error = %v", err)
	}
	if got != false {
		t.Fatalf("ContainerRunning() in DryRun = %v, want false", got)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("DryRun must not execute, got calls = %+v", rec.calls)
	}
}

func TestRunner_ContainerCommand(t *testing.T) {
	rec := &recorder{out: "There are 0 players"}
	r := Runner{Exec: rec.exec}
	out, err := r.ContainerCommand(context.Background(), "nethernode-minecraft", "rcon-cli", "list")
	if err != nil || out != "There are 0 players" {
		t.Fatalf("ContainerCommand() = %q, %v", out, err)
	}
	assertSingleCall(t, rec, "docker", []string{"exec", "nethernode-minecraft", "rcon-cli", "list"})
}

func TestRunner_DefaultExec(t *testing.T) {
	// No Exec injected: falls back to os/exec via the unexported default.
	r := Runner{Exec: nil}
	out, err := r.exec(context.Background(), "echo", "hi")
	if err != nil {
		t.Fatalf("default exec error = %v", err)
	}
	if !strings.Contains(out, "hi") {
		t.Fatalf("default exec output = %q, want it to contain %q", out, "hi")
	}
}

func assertSingleCall(t *testing.T, rec *recorder, wantName string, wantArgs []string) {
	t.Helper()
	if len(rec.calls) != 1 {
		t.Fatalf("got %d calls, want 1: %+v", len(rec.calls), rec.calls)
	}
	got := rec.calls[0]
	if got.name != wantName {
		t.Fatalf("call name = %q, want %q", got.name, wantName)
	}
	if len(got.args) != len(wantArgs) {
		t.Fatalf("call args = %v, want %v", got.args, wantArgs)
	}
	for i := range wantArgs {
		if got.args[i] != wantArgs[i] {
			t.Fatalf("call args = %v, want %v", got.args, wantArgs)
		}
	}
}

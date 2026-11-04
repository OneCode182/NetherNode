package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func TestCmdStart(t *testing.T) {
	t.Run("real", func(t *testing.T) {
		ta := newTestApp(t, false)
		if err := CmdStart(ta.App, nil); err != nil {
			t.Fatalf("CmdStart() error = %v", err)
		}
		if len(ta.Exec.calls) != 1 {
			t.Fatalf("Exec calls = %d, want 1: %+v", len(ta.Exec.calls), ta.Exec.calls)
		}
		if !hasArg(ta.Exec.calls[0].args, "up") || !hasArg(ta.Exec.calls[0].args, "-d") {
			t.Fatalf("Exec call args = %v, want up -d", ta.Exec.calls[0].args)
		}
		if !strings.Contains(ta.Stdout.String(), "ok") {
			t.Fatalf("stdout = %q, want it to contain compose output", ta.Stdout.String())
		}
	})

	t.Run("dry-run", func(t *testing.T) {
		ta := newTestApp(t, true)
		if err := CmdStart(ta.App, nil); err != nil {
			t.Fatalf("CmdStart() error = %v", err)
		}
		if len(ta.Exec.calls) != 0 {
			t.Fatalf("dry-run must not exec, got %+v", ta.Exec.calls)
		}
		out := ta.Stdout.String()
		if !strings.Contains(out, "[dry-run]") || !strings.Contains(out, "up") {
			t.Fatalf("stdout = %q, want a [dry-run] plan mentioning up", out)
		}
	})

	t.Run("unknown flag", func(t *testing.T) {
		ta := newTestApp(t, false)
		if err := CmdStart(ta.App, []string{"--bogus"}); err == nil {
			t.Fatal("CmdStart() with unknown flag: error = nil, want error")
		}
	})
}

func TestCmdStop(t *testing.T) {
	t.Run("real default backs up and stops", func(t *testing.T) {
		ta := newTestApp(t, false)
		client := newFakeRCON()
		client.responses["save-all flush"] = "Saved the game"
		dialed := 0
		ta.App.DialRCON = dialerFor(client, nil, &dialed)

		if err := CmdStop(ta.App, nil); err != nil {
			t.Fatalf("CmdStop() error = %v", err)
		}
		if dialed != 1 {
			t.Fatalf("dialed = %d, want 1", dialed)
		}
		if len(client.calls) != 1 || client.calls[0] != "save-all flush" {
			t.Fatalf("rcon calls = %v, want [save-all flush]", client.calls)
		}
		if !client.closed {
			t.Fatal("rcon client was not closed")
		}
		if got := countEntries(t, ta.App.Config.BackupDest); got != 1 {
			t.Fatalf("backup archives = %d, want 1", got)
		}
		if len(ta.Exec.calls) != 1 || !hasArg(ta.Exec.calls[0].args, "down") {
			t.Fatalf("Exec calls = %+v, want a single down call", ta.Exec.calls)
		}
	})

	t.Run("--no-backup skips archive but still saves and stops", func(t *testing.T) {
		ta := newTestApp(t, false)
		client := newFakeRCON()
		dialed := 0
		ta.App.DialRCON = dialerFor(client, nil, &dialed)

		if err := CmdStop(ta.App, []string{"--no-backup"}); err != nil {
			t.Fatalf("CmdStop() error = %v", err)
		}
		if len(client.calls) != 1 || client.calls[0] != "save-all flush" {
			t.Fatalf("rcon calls = %v, want [save-all flush]", client.calls)
		}
		if got := countEntries(t, ta.App.Config.BackupDest); got != 0 {
			t.Fatalf("backup archives = %d, want 0 with --no-backup", got)
		}
		if len(ta.Exec.calls) != 1 || !hasArg(ta.Exec.calls[0].args, "down") {
			t.Fatalf("Exec calls = %+v, want a single down call", ta.Exec.calls)
		}
	})

	t.Run("RCON unreachable degrades gracefully", func(t *testing.T) {
		ta := newTestApp(t, false)
		dialed := 0
		ta.App.DialRCON = dialerFor(nil, errDialUnreachable, &dialed)

		if err := CmdStop(ta.App, nil); err != nil {
			t.Fatalf("CmdStop() error = %v, want nil (save is best-effort)", err)
		}
		if dialed != 1 {
			t.Fatalf("dialed = %d, want 1", dialed)
		}
		if !strings.Contains(ta.Stderr.String(), "warning") {
			t.Fatalf("stderr = %q, want a warning about the skipped save", ta.Stderr.String())
		}
		if got := countEntries(t, ta.App.Config.BackupDest); got != 1 {
			t.Fatalf("backup archives = %d, want 1 (backup still runs from disk)", got)
		}
	})

	t.Run("dry-run touches nothing", func(t *testing.T) {
		ta := newTestApp(t, true)
		dialed := 0
		ta.App.DialRCON = dialerFor(newFakeRCON(), nil, &dialed)

		if err := CmdStop(ta.App, nil); err != nil {
			t.Fatalf("CmdStop() error = %v", err)
		}
		if dialed != 0 {
			t.Fatalf("dry-run dialed RCON %d times, want 0", dialed)
		}
		if len(ta.Exec.calls) != 0 {
			t.Fatalf("dry-run must not exec, got %+v", ta.Exec.calls)
		}
		if got := countEntries(t, ta.App.Config.BackupDest); got != 0 {
			t.Fatalf("dry-run must not write backups, found %d", got)
		}
		out := ta.Stdout.String()
		for _, want := range []string{"[dry-run]", "save-all flush", "down"} {
			if !strings.Contains(out, want) {
				t.Fatalf("stdout = %q, want it to mention %q", out, want)
			}
		}
	})
}

func TestCmdRestart(t *testing.T) {
	t.Run("real saves, backs up, downs, and ups", func(t *testing.T) {
		ta := newTestApp(t, false)
		client := newFakeRCON()
		dialed := 0
		ta.App.DialRCON = dialerFor(client, nil, &dialed)

		if err := CmdRestart(ta.App, nil); err != nil {
			t.Fatalf("CmdRestart() error = %v", err)
		}
		if got := countEntries(t, ta.App.Config.BackupDest); got != 1 {
			t.Fatalf("backup archives = %d, want 1", got)
		}
		if len(ta.Exec.calls) != 2 {
			t.Fatalf("Exec calls = %d, want 2 (down, up): %+v", len(ta.Exec.calls), ta.Exec.calls)
		}
		if !hasArg(ta.Exec.calls[0].args, "down") {
			t.Fatalf("first Exec call = %v, want down", ta.Exec.calls[0].args)
		}
		if !hasArg(ta.Exec.calls[1].args, "up") || !hasArg(ta.Exec.calls[1].args, "-d") {
			t.Fatalf("second Exec call = %v, want up -d", ta.Exec.calls[1].args)
		}
	})

	t.Run("--no-backup", func(t *testing.T) {
		ta := newTestApp(t, false)
		dialed := 0
		ta.App.DialRCON = dialerFor(newFakeRCON(), nil, &dialed)

		if err := CmdRestart(ta.App, []string{"--no-backup"}); err != nil {
			t.Fatalf("CmdRestart() error = %v", err)
		}
		if got := countEntries(t, ta.App.Config.BackupDest); got != 0 {
			t.Fatalf("backup archives = %d, want 0 with --no-backup", got)
		}
	})

	t.Run("dry-run touches nothing", func(t *testing.T) {
		ta := newTestApp(t, true)
		dialed := 0
		ta.App.DialRCON = dialerFor(newFakeRCON(), nil, &dialed)

		if err := CmdRestart(ta.App, nil); err != nil {
			t.Fatalf("CmdRestart() error = %v", err)
		}
		if dialed != 0 {
			t.Fatalf("dry-run dialed RCON %d times, want 0", dialed)
		}
		if len(ta.Exec.calls) != 0 {
			t.Fatalf("dry-run must not exec, got %+v", ta.Exec.calls)
		}
		out := ta.Stdout.String()
		if !strings.Contains(out, "down") || !strings.Contains(out, "up") {
			t.Fatalf("stdout = %q, want plan mentioning down and up", out)
		}
	})
}

func TestCmdSaveServer(t *testing.T) {
	t.Run("real success", func(t *testing.T) {
		ta := newTestApp(t, false)
		client := newFakeRCON()
		client.responses["save-all flush"] = "Saved the game"
		dialed := 0
		ta.App.DialRCON = dialerFor(client, nil, &dialed)

		if err := CmdSaveServer(ta.App, nil); err != nil {
			t.Fatalf("CmdSaveServer() error = %v", err)
		}
		if len(client.calls) != 1 || client.calls[0] != "save-all flush" {
			t.Fatalf("rcon calls = %v, want [save-all flush]", client.calls)
		}
		if !strings.Contains(ta.Stdout.String(), "complete") {
			t.Fatalf("stdout = %q, want a completion message", ta.Stdout.String())
		}
	})

	t.Run("RCON unreachable is a real error", func(t *testing.T) {
		ta := newTestApp(t, false)
		dialed := 0
		ta.App.DialRCON = dialerFor(nil, errDialUnreachable, &dialed)

		if err := CmdSaveServer(ta.App, nil); err == nil {
			t.Fatal("CmdSaveServer() error = nil, want error when RCON is unreachable")
		}
	})

	t.Run("dry-run touches nothing", func(t *testing.T) {
		ta := newTestApp(t, true)
		dialed := 0
		ta.App.DialRCON = dialerFor(newFakeRCON(), nil, &dialed)

		if err := CmdSaveServer(ta.App, nil); err != nil {
			t.Fatalf("CmdSaveServer() error = %v", err)
		}
		if dialed != 0 {
			t.Fatalf("dry-run dialed RCON %d times, want 0", dialed)
		}
		if !strings.Contains(ta.Stdout.String(), "[dry-run]") {
			t.Fatalf("stdout = %q, want a [dry-run] line", ta.Stdout.String())
		}
	})
}

func TestCmdBackupServer(t *testing.T) {
	t.Run("real success toggles autosave in order", func(t *testing.T) {
		ta := newTestApp(t, false)
		client := newFakeRCON()
		dialed := 0
		ta.App.DialRCON = dialerFor(client, nil, &dialed)

		if err := CmdBackupServer(ta.App, nil); err != nil {
			t.Fatalf("CmdBackupServer() error = %v", err)
		}
		want := []string{"save-all flush", "save-off", "save-all flush", "save-on"}
		if len(client.calls) != len(want) {
			t.Fatalf("rcon calls = %v, want %v", client.calls, want)
		}
		for i, c := range want {
			if client.calls[i] != c {
				t.Fatalf("rcon calls[%d] = %q, want %q (full: %v)", i, client.calls[i], c, client.calls)
			}
		}
		if !client.closed {
			t.Fatal("rcon client was not closed")
		}
		if got := countEntries(t, ta.App.Config.BackupDest); got != 1 {
			t.Fatalf("backup archives = %d, want 1", got)
		}
	})

	t.Run("retention override prunes older archives", func(t *testing.T) {
		ta := newTestApp(t, false)
		dialed := 0
		ta.App.DialRCON = dialerFor(newFakeRCON(), nil, &dialed)

		dest := ta.App.Config.BackupDest
		now := time.Now()
		seed := []struct {
			name string
			age  time.Duration
		}{
			{"minecraft-old1.tar.gz", 3 * time.Hour},
			{"minecraft-old2.tar.gz", 2 * time.Hour},
			{"minecraft-old3.tar.gz", 1 * time.Hour},
		}
		for _, s := range seed {
			p := filepath.Join(dest, s.name)
			if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
				t.Fatalf("seed archive %s: %v", s.name, err)
			}
			mtime := now.Add(-s.age)
			if err := os.Chtimes(p, mtime, mtime); err != nil {
				t.Fatalf("chtimes %s: %v", s.name, err)
			}
		}

		if err := CmdBackupServer(ta.App, []string{"--retention", "2"}); err != nil {
			t.Fatalf("CmdBackupServer() error = %v", err)
		}
		if got := countEntries(t, dest); got != 2 {
			t.Fatalf("backup archives after prune = %d, want 2", got)
		}
	})

	t.Run("RCON unreachable still backs up", func(t *testing.T) {
		ta := newTestApp(t, false)
		dialed := 0
		ta.App.DialRCON = dialerFor(nil, errDialUnreachable, &dialed)

		if err := CmdBackupServer(ta.App, nil); err != nil {
			t.Fatalf("CmdBackupServer() error = %v, want nil (backup still runs)", err)
		}
		if !strings.Contains(ta.Stderr.String(), "warning") {
			t.Fatalf("stderr = %q, want a warning about unreachable RCON", ta.Stderr.String())
		}
		if got := countEntries(t, ta.App.Config.BackupDest); got != 1 {
			t.Fatalf("backup archives = %d, want 1", got)
		}
	})

	t.Run("dry-run touches nothing", func(t *testing.T) {
		ta := newTestApp(t, true)
		dialed := 0
		ta.App.DialRCON = dialerFor(newFakeRCON(), nil, &dialed)

		if err := CmdBackupServer(ta.App, []string{"--retention", "2"}); err != nil {
			t.Fatalf("CmdBackupServer() error = %v", err)
		}
		if dialed != 0 {
			t.Fatalf("dry-run dialed RCON %d times, want 0", dialed)
		}
		if got := countEntries(t, ta.App.Config.BackupDest); got != 0 {
			t.Fatalf("dry-run must not write backups, found %d", got)
		}
		out := ta.Stdout.String()
		for _, want := range []string{"[dry-run]", "save-off", "save-on", "retention=2"} {
			if !strings.Contains(out, want) {
				t.Fatalf("stdout = %q, want it to mention %q", out, want)
			}
		}
	})

	t.Run("invalid retention flag", func(t *testing.T) {
		ta := newTestApp(t, false)
		if err := CmdBackupServer(ta.App, []string{"--retention", "not-a-number"}); err == nil {
			t.Fatal("CmdBackupServer() with invalid --retention: error = nil, want error")
		}
	})
}

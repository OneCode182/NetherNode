package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onecode182/nethernode/internal/opsjson"
)

func TestCmdAdmin_MissingSubcommand(t *testing.T) {
	ta := newTestApp(t, false)
	if err := CmdAdmin(ta.App, nil); err == nil {
		t.Fatal("CmdAdmin(nil) error = nil, want error")
	}
}

func TestCmdAdmin_UnknownSubcommand(t *testing.T) {
	ta := newTestApp(t, false)
	if err := CmdAdmin(ta.App, []string{"frobnicate"}); err == nil {
		t.Fatal("CmdAdmin(frobnicate) error = nil, want error")
	}
}

func TestCmdAdminList(t *testing.T) {
	t.Run("no ops.json prints empty, no error", func(t *testing.T) {
		ta := newTestApp(t, false)
		if err := CmdAdmin(ta.App, []string{"list"}); err != nil {
			t.Fatalf("CmdAdmin(list) error = %v", err)
		}
		if !strings.Contains(ta.Stdout.String(), "no ops configured") {
			t.Fatalf("stdout = %q, want a no-ops message", ta.Stdout.String())
		}
	})

	t.Run("lists existing entries", func(t *testing.T) {
		ta := newTestApp(t, false)
		entries := []opsjson.Entry{
			{UUID: opsjson.OfflineUUID("Steve"), Name: "Steve", Level: 4, BypassesPlayerLimit: false},
			{UUID: opsjson.OfflineUUID("Alex"), Name: "Alex", Level: 1, BypassesPlayerLimit: true},
		}
		if err := opsjson.WriteAtomic(ta.App.opsPath(), entries); err != nil {
			t.Fatalf("seed ops.json: %v", err)
		}

		if err := CmdAdmin(ta.App, []string{"list"}); err != nil {
			t.Fatalf("CmdAdmin(list) error = %v", err)
		}
		out := ta.Stdout.String()
		for _, want := range []string{"Steve", "Alex", "level=4", "level=1", "bypassesPlayerLimit=true"} {
			if !strings.Contains(out, want) {
				t.Fatalf("stdout = %q, want it to mention %q", out, want)
			}
		}
	})

	t.Run("dry-run reads nothing", func(t *testing.T) {
		ta := newTestApp(t, true)
		// No ops.json seeded at all; dry-run must still exit 0.
		if err := CmdAdmin(ta.App, []string{"list"}); err != nil {
			t.Fatalf("CmdAdmin(list) dry-run error = %v", err)
		}
		if !strings.Contains(ta.Stdout.String(), "[dry-run]") {
			t.Fatalf("stdout = %q, want a [dry-run] line", ta.Stdout.String())
		}
	})
}

func TestCmdAdminAdd(t *testing.T) {
	t.Run("default level only runs rcon op, does not touch ops.json", func(t *testing.T) {
		ta := newTestApp(t, false)
		client := newFakeRCON()
		dialed := 0
		ta.App.DialRCON = dialerFor(client, nil, &dialed)

		if err := CmdAdmin(ta.App, []string{"add", "Steve"}); err != nil {
			t.Fatalf("CmdAdmin(add) error = %v", err)
		}
		if len(client.calls) != 1 || client.calls[0] != "op Steve" {
			t.Fatalf("rcon calls = %v, want [op Steve]", client.calls)
		}
		if !client.closed {
			t.Fatal("rcon client was not closed")
		}
		if _, err := os.Stat(ta.App.opsPath()); !os.IsNotExist(err) {
			t.Fatalf("ops.json should not be created for the default level, stat err = %v", err)
		}
	})

	t.Run("non-default level patches ops.json after rcon op", func(t *testing.T) {
		ta := newTestApp(t, false)
		client := newFakeRCON()
		dialed := 0
		ta.App.DialRCON = dialerFor(client, nil, &dialed)

		if err := CmdAdmin(ta.App, []string{"add", "Steve", "--level", "1"}); err != nil {
			t.Fatalf("CmdAdmin(add) error = %v", err)
		}
		if len(client.calls) != 1 || client.calls[0] != "op Steve" {
			t.Fatalf("rcon calls = %v, want [op Steve]", client.calls)
		}

		entries, err := opsjson.Read(ta.App.opsPath())
		if err != nil {
			t.Fatalf("read ops.json: %v", err)
		}
		if len(entries) != 1 || entries[0].Name != "Steve" || entries[0].Level != 1 {
			t.Fatalf("ops.json entries = %+v, want [Steve level=1]", entries)
		}
		if entries[0].UUID != opsjson.OfflineUUID("Steve") {
			t.Fatalf("ops.json UUID = %q, want offline UUID", entries[0].UUID)
		}
	})

	t.Run("non-default level flag before player name", func(t *testing.T) {
		ta := newTestApp(t, false)
		ta.App.DialRCON = dialerFor(newFakeRCON(), nil, nil)

		if err := CmdAdmin(ta.App, []string{"add", "--level", "2", "Steve"}); err != nil {
			t.Fatalf("CmdAdmin(add) error = %v", err)
		}
		entries, err := opsjson.Read(ta.App.opsPath())
		if err != nil {
			t.Fatalf("read ops.json: %v", err)
		}
		if len(entries) != 1 || entries[0].Level != 2 {
			t.Fatalf("ops.json entries = %+v, want level=2", entries)
		}
	})

	t.Run("rejects invalid level", func(t *testing.T) {
		ta := newTestApp(t, false)
		for _, level := range []string{"0", "5"} {
			t.Run(level, func(t *testing.T) {
				if err := CmdAdmin(ta.App, []string{"add", "Steve", "--level", level}); err == nil {
					t.Fatal("CmdAdmin(add) error = nil, want invalid level error")
				}
			})
		}
	})

	t.Run("rcon failure falls back to atomic ops.json patch", func(t *testing.T) {
		ta := newTestApp(t, false)
		ta.App.DialRCON = dialerFor(nil, errDialUnreachable, nil)

		if err := CmdAdmin(ta.App, []string{"add", "Steve", "--level", "3"}); err != nil {
			t.Fatalf("CmdAdmin(add) fallback error = %v", err)
		}
		entries, err := opsjson.Read(ta.App.opsPath())
		if err != nil {
			t.Fatalf("read ops.json: %v", err)
		}
		if len(entries) != 1 || entries[0].Name != "Steve" || entries[0].Level != 3 {
			t.Fatalf("ops.json entries = %+v, want Steve level=3", entries)
		}
		if !strings.Contains(ta.Stdout.String(), "server restart needed") {
			t.Fatalf("stdout = %q, want restart-needed note", ta.Stdout.String())
		}
	})

	t.Run("rcon failure is a real error", func(t *testing.T) {
		ta := newTestApp(t, false)
		ta.App.DialRCON = dialerFor(nil, errDialUnreachable, nil)

		// A write error still surfaces even though the RCON failure itself can
		// fall back to an offline ops.json patch.
		badDataDir := filepath.Join(filepath.Dir(ta.App.Config.DataDir), "file-as-dir")
		if err := os.WriteFile(badDataDir, []byte("not a dir"), 0o644); err != nil {
			t.Fatalf("seed blocking file: %v", err)
		}
		ta.App.Config.DataDir = badDataDir
		if err := CmdAdmin(ta.App, []string{"add", "Steve"}); err == nil {
			t.Fatal("CmdAdmin(add) error = nil, want error when RCON and offline patch both fail")
		}
	})

	t.Run("wrong argument count errors", func(t *testing.T) {
		ta := newTestApp(t, false)
		if err := CmdAdmin(ta.App, []string{"add"}); err == nil {
			t.Fatal("CmdAdmin(add) with no player: error = nil, want error")
		}
		if err := CmdAdmin(ta.App, []string{"add", "Steve", "Alex"}); err == nil {
			t.Fatal("CmdAdmin(add) with two players: error = nil, want error")
		}
	})

	t.Run("dry-run touches nothing", func(t *testing.T) {
		ta := newTestApp(t, true)
		dialed := 0
		ta.App.DialRCON = dialerFor(newFakeRCON(), nil, &dialed)

		if err := CmdAdmin(ta.App, []string{"add", "Steve", "--level", "1"}); err != nil {
			t.Fatalf("CmdAdmin(add) dry-run error = %v", err)
		}
		if dialed != 0 {
			t.Fatalf("dry-run dialed RCON %d times, want 0", dialed)
		}
		if _, err := os.Stat(ta.App.opsPath()); !os.IsNotExist(err) {
			t.Fatalf("dry-run must not write ops.json, stat err = %v", err)
		}
		out := ta.Stdout.String()
		for _, want := range []string{"[dry-run]", "op Steve", "level=1"} {
			if !strings.Contains(out, want) {
				t.Fatalf("stdout = %q, want it to mention %q", out, want)
			}
		}
	})

	t.Run("dry-run at default level does not mention a patch", func(t *testing.T) {
		ta := newTestApp(t, true)
		ta.App.DialRCON = dialerFor(newFakeRCON(), nil, nil)

		if err := CmdAdmin(ta.App, []string{"add", "Steve"}); err != nil {
			t.Fatalf("CmdAdmin(add) dry-run error = %v", err)
		}
		if strings.Contains(ta.Stdout.String(), "patch") {
			t.Fatalf("stdout = %q, must not mention a patch at the default level", ta.Stdout.String())
		}
	})
}

func TestCmdAdminRemove(t *testing.T) {
	t.Run("runs rcon deop", func(t *testing.T) {
		ta := newTestApp(t, false)
		if err := opsjson.WriteAtomic(ta.App.opsPath(), []opsjson.Entry{{UUID: opsjson.OfflineUUID("Steve"), Name: "Steve", Level: 4}}); err != nil {
			t.Fatalf("seed ops.json: %v", err)
		}
		client := newFakeRCON()
		dialed := 0
		ta.App.DialRCON = dialerFor(client, nil, &dialed)

		if err := CmdAdmin(ta.App, []string{"remove", "Steve"}); err != nil {
			t.Fatalf("CmdAdmin(remove) error = %v", err)
		}
		if len(client.calls) != 1 || client.calls[0] != "deop Steve" {
			t.Fatalf("rcon calls = %v, want [deop Steve]", client.calls)
		}
		if !client.closed {
			t.Fatal("rcon client was not closed")
		}
		entries, err := opsjson.Read(ta.App.opsPath())
		if err != nil {
			t.Fatalf("read ops.json: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("ops.json entries = %+v, want empty after remove", entries)
		}
	})

	t.Run("rcon failure falls back to atomic ops.json remove", func(t *testing.T) {
		ta := newTestApp(t, false)
		if err := opsjson.WriteAtomic(ta.App.opsPath(), []opsjson.Entry{{UUID: opsjson.OfflineUUID("Steve"), Name: "Steve", Level: 4}}); err != nil {
			t.Fatalf("seed ops.json: %v", err)
		}
		ta.App.DialRCON = dialerFor(nil, errDialUnreachable, nil)

		if err := CmdAdmin(ta.App, []string{"remove", "Steve"}); err != nil {
			t.Fatalf("CmdAdmin(remove) fallback error = %v", err)
		}
		entries, err := opsjson.Read(ta.App.opsPath())
		if err != nil {
			t.Fatalf("read ops.json: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("ops.json entries = %+v, want empty after offline remove", entries)
		}
		if !strings.Contains(ta.Stderr.String(), "patching ops.json offline") {
			t.Fatalf("stderr = %q, want offline patch warning", ta.Stderr.String())
		}
	})

	t.Run("wrong argument count errors", func(t *testing.T) {
		ta := newTestApp(t, false)
		if err := CmdAdmin(ta.App, []string{"remove"}); err == nil {
			t.Fatal("CmdAdmin(remove) with no player: error = nil, want error")
		}
	})

	t.Run("dry-run touches nothing", func(t *testing.T) {
		ta := newTestApp(t, true)
		dialed := 0
		ta.App.DialRCON = dialerFor(newFakeRCON(), nil, &dialed)

		if err := CmdAdmin(ta.App, []string{"remove", "Steve"}); err != nil {
			t.Fatalf("CmdAdmin(remove) dry-run error = %v", err)
		}
		if dialed != 0 {
			t.Fatalf("dry-run dialed RCON %d times, want 0", dialed)
		}
		if !strings.Contains(ta.Stdout.String(), "[dry-run]") {
			t.Fatalf("stdout = %q, want a [dry-run] line", ta.Stdout.String())
		}
		if !strings.Contains(ta.Stdout.String(), "remove Steve") {
			t.Fatalf("stdout = %q, want ops.json remove plan", ta.Stdout.String())
		}
	})
}

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onecode182/nethernode/internal/serverprops"
)

const settingsSample = "#Minecraft server properties\ndifficulty=easy\nmax-players=20\nwhite-list=false\n"

func seedServerProperties(t *testing.T, ta *testApp, body string) {
	t.Helper()
	if err := os.WriteFile(ta.App.serverPropertiesPath(), []byte(body), 0o644); err != nil {
		t.Fatalf("seed server.properties: %v", err)
	}
}

func TestCmdSettings_MissingSubcommand(t *testing.T) {
	ta := newTestApp(t, false)
	if err := CmdSettings(ta.App, nil); err == nil {
		t.Fatal("CmdSettings(nil) error = nil, want error")
	}
}

func TestCmdSettings_UnknownSubcommand(t *testing.T) {
	ta := newTestApp(t, false)
	if err := CmdSettings(ta.App, []string{"frobnicate"}); err == nil {
		t.Fatal("CmdSettings(frobnicate) error = nil, want error")
	}
}

func TestCmdSettingsGet(t *testing.T) {
	t.Run("known key prints value", func(t *testing.T) {
		ta := newTestApp(t, false)
		seedServerProperties(t, ta, settingsSample)

		if err := CmdSettings(ta.App, []string{"get", "difficulty"}); err != nil {
			t.Fatalf("CmdSettings(get) error = %v", err)
		}
		if got := strings.TrimSpace(ta.Stdout.String()); got != "easy" {
			t.Fatalf("stdout = %q, want %q", got, "easy")
		}
	})

	t.Run("unknown key errors", func(t *testing.T) {
		ta := newTestApp(t, false)
		seedServerProperties(t, ta, settingsSample)

		if err := CmdSettings(ta.App, []string{"get", "nonexistent-key"}); err == nil {
			t.Fatal("CmdSettings(get, nonexistent-key) error = nil, want error")
		}
	})

	t.Run("missing file errors", func(t *testing.T) {
		ta := newTestApp(t, false)
		if err := CmdSettings(ta.App, []string{"get", "difficulty"}); err == nil {
			t.Fatal("CmdSettings(get) with no server.properties: error = nil, want error")
		}
	})

	t.Run("wrong argument count errors", func(t *testing.T) {
		ta := newTestApp(t, false)
		if err := CmdSettings(ta.App, []string{"get"}); err == nil {
			t.Fatal("CmdSettings(get) with no key: error = nil, want error")
		}
	})

	t.Run("dry-run reads nothing", func(t *testing.T) {
		ta := newTestApp(t, true)
		// No server.properties seeded; dry-run must still exit 0.
		if err := CmdSettings(ta.App, []string{"get", "difficulty"}); err != nil {
			t.Fatalf("CmdSettings(get) dry-run error = %v", err)
		}
		if !strings.Contains(ta.Stdout.String(), "[dry-run]") {
			t.Fatalf("stdout = %q, want a [dry-run] line", ta.Stdout.String())
		}
	})
}

func TestCmdSettingsSet(t *testing.T) {
	t.Run("patches key atomically, preserving order and comments", func(t *testing.T) {
		ta := newTestApp(t, false)
		seedServerProperties(t, ta, settingsSample)

		if err := CmdSettings(ta.App, []string{"set", "difficulty", "hard"}); err != nil {
			t.Fatalf("CmdSettings(set) error = %v", err)
		}

		body, err := os.ReadFile(ta.App.serverPropertiesPath())
		if err != nil {
			t.Fatalf("read server.properties: %v", err)
		}
		want := "#Minecraft server properties\ndifficulty=hard\nmax-players=20\nwhite-list=false\n"
		if string(body) != want {
			t.Fatalf("server.properties =\n%q\nwant\n%q", string(body), want)
		}
	})

	t.Run("joins free-form values with spaces", func(t *testing.T) {
		ta := newTestApp(t, false)
		seedServerProperties(t, ta, settingsSample)

		if err := CmdSettings(ta.App, []string{"set", "motd", "Nether Node Night"}); err != nil {
			t.Fatalf("CmdSettings(set motd) error = %v", err)
		}
		lines, err := serverprops.ReadFile(ta.App.serverPropertiesPath())
		if err != nil {
			t.Fatalf("read server.properties: %v", err)
		}
		if v, ok := serverprops.Get(lines, "motd"); !ok || v != "Nether Node Night" {
			t.Fatalf("motd = (%q, %v), want (\"Nether Node Night\", true)", v, ok)
		}
	})

	t.Run("no leftover temp file", func(t *testing.T) {
		ta := newTestApp(t, false)
		seedServerProperties(t, ta, settingsSample)

		if err := CmdSettings(ta.App, []string{"set", "difficulty", "hard"}); err != nil {
			t.Fatalf("CmdSettings(set) error = %v", err)
		}
		tmp := filepath.Join(ta.App.Config.DataDir, ".server.properties.tmp")
		if _, err := os.Stat(tmp); !os.IsNotExist(err) {
			t.Fatalf("temp file left behind: err = %v", err)
		}
	})

	t.Run("new key is appended", func(t *testing.T) {
		ta := newTestApp(t, false)
		seedServerProperties(t, ta, settingsSample)

		if err := CmdSettings(ta.App, []string{"set", "pvp", "false"}); err != nil {
			t.Fatalf("CmdSettings(set) error = %v", err)
		}
		lines, err := serverprops.ReadFile(ta.App.serverPropertiesPath())
		if err != nil {
			t.Fatalf("read server.properties: %v", err)
		}
		if v, ok := serverprops.Get(lines, "pvp"); !ok || v != "false" {
			t.Fatalf("pvp = (%q, %v), want (\"false\", true)", v, ok)
		}
	})

	t.Run("--apply sends the matching rcon command", func(t *testing.T) {
		ta := newTestApp(t, false)
		seedServerProperties(t, ta, settingsSample)
		client := newFakeRCON()
		dialed := 0
		ta.App.DialRCON = dialerFor(client, nil, &dialed)

		if err := CmdSettings(ta.App, []string{"set", "difficulty", "hard", "--apply"}); err != nil {
			t.Fatalf("CmdSettings(set) error = %v", err)
		}
		if len(client.calls) != 1 || client.calls[0] != "difficulty hard" {
			t.Fatalf("rcon calls = %v, want [difficulty hard]", client.calls)
		}
		if !strings.Contains(ta.Stdout.String(), "applied via rcon") {
			t.Fatalf("stdout = %q, want an applied-via-rcon message", ta.Stdout.String())
		}
	})

	t.Run("whitelist alias writes canonical white-list property", func(t *testing.T) {
		ta := newTestApp(t, false)
		seedServerProperties(t, ta, settingsSample)
		client := newFakeRCON()
		ta.App.DialRCON = dialerFor(client, nil, nil)

		if err := CmdSettings(ta.App, []string{"set", "whitelist", "true", "--apply"}); err != nil {
			t.Fatalf("CmdSettings(set whitelist) error = %v", err)
		}
		lines, err := serverprops.ReadFile(ta.App.serverPropertiesPath())
		if err != nil {
			t.Fatalf("read server.properties: %v", err)
		}
		if v, ok := serverprops.Get(lines, "white-list"); !ok || v != "true" {
			t.Fatalf("white-list = (%q, %v), want (\"true\", true)", v, ok)
		}
		if _, ok := serverprops.Get(lines, "whitelist"); ok {
			t.Fatal("whitelist alias must not be persisted as a separate key")
		}
		if len(client.calls) != 1 || client.calls[0] != "whitelist on" {
			t.Fatalf("rcon calls = %v, want [whitelist on]", client.calls)
		}
	})

	t.Run("setting keys are trimmed lower-case canonical", func(t *testing.T) {
		ta := newTestApp(t, false)
		seedServerProperties(t, ta, settingsSample)

		if err := CmdSettings(ta.App, []string{"set", "  DIFFICULTY  ", "hard"}); err != nil {
			t.Fatalf("CmdSettings(set DIFFICULTY) error = %v", err)
		}
		lines, err := serverprops.ReadFile(ta.App.serverPropertiesPath())
		if err != nil {
			t.Fatalf("read server.properties: %v", err)
		}
		if v, ok := serverprops.Get(lines, "difficulty"); !ok || v != "hard" {
			t.Fatalf("difficulty = (%q, %v), want (\"hard\", true)", v, ok)
		}
	})

	t.Run("whitelist alias get reads canonical white-list property", func(t *testing.T) {
		ta := newTestApp(t, false)
		seedServerProperties(t, ta, settingsSample)

		if err := CmdSettings(ta.App, []string{"get", "whitelist"}); err != nil {
			t.Fatalf("CmdSettings(get whitelist) error = %v", err)
		}
		if got := strings.TrimSpace(ta.Stdout.String()); got != "false" {
			t.Fatalf("stdout = %q, want false", got)
		}
	})

	t.Run("--apply flag works before positional args too", func(t *testing.T) {
		ta := newTestApp(t, false)
		seedServerProperties(t, ta, settingsSample)
		client := newFakeRCON()
		ta.App.DialRCON = dialerFor(client, nil, nil)

		if err := CmdSettings(ta.App, []string{"set", "--apply", "difficulty", "hard"}); err != nil {
			t.Fatalf("CmdSettings(set) error = %v", err)
		}
		if len(client.calls) != 1 || client.calls[0] != "difficulty hard" {
			t.Fatalf("rcon calls = %v, want [difficulty hard]", client.calls)
		}
	})

	t.Run("--apply on a key with no live rcon equivalent notes restart needed", func(t *testing.T) {
		ta := newTestApp(t, false)
		seedServerProperties(t, ta, settingsSample)
		dialed := 0
		ta.App.DialRCON = dialerFor(newFakeRCON(), nil, &dialed)

		if err := CmdSettings(ta.App, []string{"set", "max-players", "30", "--apply"}); err != nil {
			t.Fatalf("CmdSettings(set) error = %v", err)
		}
		if dialed != 0 {
			t.Fatalf("dialed RCON %d times for a key with no live equivalent, want 0", dialed)
		}
		if !strings.Contains(ta.Stdout.String(), "restart needed") {
			t.Fatalf("stdout = %q, want a restart-needed note", ta.Stdout.String())
		}
	})

	t.Run("without --apply, rcon is never dialed", func(t *testing.T) {
		ta := newTestApp(t, false)
		seedServerProperties(t, ta, settingsSample)
		dialed := 0
		ta.App.DialRCON = dialerFor(newFakeRCON(), nil, &dialed)

		if err := CmdSettings(ta.App, []string{"set", "difficulty", "hard"}); err != nil {
			t.Fatalf("CmdSettings(set) error = %v", err)
		}
		if dialed != 0 {
			t.Fatalf("dialed RCON %d times without --apply, want 0", dialed)
		}
	})

	t.Run("wrong argument count errors", func(t *testing.T) {
		ta := newTestApp(t, false)
		if err := CmdSettings(ta.App, []string{"set", "difficulty"}); err == nil {
			t.Fatal("CmdSettings(set) with one argument: error = nil, want error")
		}
	})

	t.Run("dry-run touches nothing", func(t *testing.T) {
		ta := newTestApp(t, true)
		seedServerProperties(t, ta, settingsSample)
		dialed := 0
		ta.App.DialRCON = dialerFor(newFakeRCON(), nil, &dialed)

		if err := CmdSettings(ta.App, []string{"set", "difficulty", "hard", "--apply"}); err != nil {
			t.Fatalf("CmdSettings(set) dry-run error = %v", err)
		}
		if dialed != 0 {
			t.Fatalf("dry-run dialed RCON %d times, want 0", dialed)
		}
		body, err := os.ReadFile(ta.App.serverPropertiesPath())
		if err != nil {
			t.Fatalf("read server.properties: %v", err)
		}
		if string(body) != settingsSample {
			t.Fatalf("dry-run must not modify server.properties, got %q", string(body))
		}
		out := ta.Stdout.String()
		for _, want := range []string{"[dry-run]", "difficulty=hard", "rcon difficulty hard"} {
			if !strings.Contains(out, want) {
				t.Fatalf("stdout = %q, want it to mention %q", out, want)
			}
		}
	})

	t.Run("dry-run on a missing file still exits cleanly (no data dir yet)", func(t *testing.T) {
		ta := newTestApp(t, true)
		// server.properties intentionally not seeded.
		if err := CmdSettings(ta.App, []string{"set", "difficulty", "hard", "--apply"}); err != nil {
			t.Fatalf("CmdSettings(set) dry-run error = %v", err)
		}
	})
}

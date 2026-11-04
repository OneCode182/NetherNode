package cli

import (
	"strings"
	"testing"
)

func TestCmdPlugins(t *testing.T) {
	t.Run("sync runs plugins-sync script", func(t *testing.T) {
		ta := newTestApp(t, false)
		ta.App.Config.ScriptDir = "/opt/nethernode/scripts"

		if err := CmdPlugins(ta.App, []string{"sync"}); err != nil {
			t.Fatalf("CmdPlugins(sync) error = %v", err)
		}
		if len(ta.Exec.calls) != 1 {
			t.Fatalf("exec calls = %d, want 1", len(ta.Exec.calls))
		}
		call := ta.Exec.calls[0]
		if call.name != "/opt/nethernode/scripts/plugins-sync.sh" || len(call.args) != 0 {
			t.Fatalf("exec call = %+v, want script with no args", call)
		}
	})

	t.Run("list maps to --list", func(t *testing.T) {
		ta := newTestApp(t, false)
		ta.App.Config.ScriptDir = "/opt/nethernode/scripts"

		if err := CmdPlugins(ta.App, []string{"list"}); err != nil {
			t.Fatalf("CmdPlugins(list) error = %v", err)
		}
		call := ta.Exec.calls[0]
		if call.name != "/opt/nethernode/scripts/plugins-sync.sh" || len(call.args) != 1 || call.args[0] != "--list" {
			t.Fatalf("exec call = %+v, want --list", call)
		}
	})

	t.Run("dry-run does not execute script", func(t *testing.T) {
		ta := newTestApp(t, true)
		ta.App.Config.ScriptDir = "/opt/nethernode/scripts"

		if err := CmdPlugins(ta.App, []string{"sync", "--dry-run"}); err != nil {
			t.Fatalf("CmdPlugins(sync --dry-run) error = %v", err)
		}
		if len(ta.Exec.calls) != 0 {
			t.Fatalf("dry-run exec calls = %d, want 0", len(ta.Exec.calls))
		}
		if !strings.Contains(ta.Stdout.String(), "[dry-run] /opt/nethernode/scripts/plugins-sync.sh [--dry-run]") {
			t.Fatalf("stdout = %q, want dry-run script plan", ta.Stdout.String())
		}
	})

	t.Run("unknown subcommand errors", func(t *testing.T) {
		ta := newTestApp(t, false)
		if err := CmdPlugins(ta.App, []string{"frobnicate"}); err == nil {
			t.Fatal("CmdPlugins(frobnicate) error = nil, want error")
		}
	})
}

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onecode182/nethernode/internal/mcstatus"
)

func TestCmdStatus_DryRunTouchesNothing(t *testing.T) {
	ta := newTestApp(t, true)
	dialed := 0
	ta.App.DialRCON = dialerFor(newFakeRCON(), nil, &dialed)
	mc := &fakeMCStatus{}
	ta.App.MCStatus = mc

	if err := CmdStatus(ta.App, []string{"--json"}); err != nil {
		t.Fatalf("CmdStatus() error = %v", err)
	}
	if dialed != 0 {
		t.Fatalf("dry-run dialed RCON %d times, want 0", dialed)
	}
	if len(ta.Exec.calls) != 0 {
		t.Fatalf("dry-run must not exec, got %+v", ta.Exec.calls)
	}
	if len(mc.javaAddrs) != 0 || len(mc.bedrockAddrs) != 0 {
		t.Fatalf("dry-run must not query mcstatus.io, got java=%v bedrock=%v", mc.javaAddrs, mc.bedrockAddrs)
	}
	if !strings.Contains(ta.Stdout.String(), "[dry-run]") {
		t.Fatalf("stdout = %q, want a [dry-run] plan", ta.Stdout.String())
	}
}

func TestCmdStatus_JSONAggregatesAllSources(t *testing.T) {
	ta := newTestApp(t, false)
	// docker inspect: container running.
	ta.Exec.out = "true\n"

	client := newFakeRCON()
	client.responses["list"] = "There are 2 of a max of 5 players online: Steve, Alex"
	dialed := 0
	ta.App.DialRCON = dialerFor(client, nil, &dialed)

	mc := &fakeMCStatus{
		java:    &mcstatus.JavaStatus{Online: true, Version: "Paper 26.2", PlayersOnline: 2, PlayersMax: 5},
		bedrock: &mcstatus.BedrockStatus{Online: true, Version: "1.21.50", PlayersOnline: 1, PlayersMax: 5},
	}
	ta.App.MCStatus = mc

	// Seed one backup archive so Backups.Count/Newest are non-zero.
	if err := os.WriteFile(filepath.Join(ta.App.Config.BackupDest, "minecraft-20260706T000000Z.tar.gz"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed backup: %v", err)
	}

	if err := CmdStatus(ta.App, []string{"--json", "--host", "play.example.com"}); err != nil {
		t.Fatalf("CmdStatus() error = %v", err)
	}

	var report StatusReport
	if err := json.Unmarshal(ta.Stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode status JSON: %v\noutput: %s", err, ta.Stdout.String())
	}

	if !report.Container.Running || report.Container.Error != "" {
		t.Errorf("Container = %+v, want running=true no error", report.Container)
	}
	if !report.RCON.Reachable || report.RCON.Error != "" || !strings.Contains(report.RCON.Raw, "Steve") {
		t.Errorf("RCON = %+v, want reachable with player list", report.RCON)
	}
	if !report.Java.Online || report.Java.Version != "Paper 26.2" || report.Java.PlayersOnline != 2 {
		t.Errorf("Java = %+v, want online Paper 26.2 2 players", report.Java)
	}
	if !report.Bedrock.Online || report.Bedrock.Version != "1.21.50" {
		t.Errorf("Bedrock = %+v, want online 1.21.50", report.Bedrock)
	}
	if report.Backups.Count != 1 || report.Backups.Newest == "" {
		t.Errorf("Backups = %+v, want count=1 with a newest entry", report.Backups)
	}
	if report.Disk.Raw == "" || report.Disk.Error != "" {
		t.Errorf("Disk = %+v, want a raw df line", report.Disk)
	}

	// --host must override the mcstatus.io lookup host, not the RCON host.
	if len(mc.javaAddrs) != 1 || mc.javaAddrs[0] != "play.example.com:25565" {
		t.Errorf("java lookup addr = %v, want [play.example.com:25565]", mc.javaAddrs)
	}
	if len(mc.bedrockAddrs) != 1 || mc.bedrockAddrs[0] != "play.example.com:19132" {
		t.Errorf("bedrock lookup addr = %v, want [play.example.com:19132]", mc.bedrockAddrs)
	}
}

func TestCmdStatus_DegradesGracefullyOnEveryFailingSource(t *testing.T) {
	ta := newTestApp(t, false)
	ta.Exec.err = errDialUnreachable // both docker inspect and df fail

	ta.App.DialRCON = dialerFor(nil, errDialUnreachable, new(int))
	ta.App.MCStatus = &fakeMCStatus{javaErr: errDialUnreachable, bedrockErr: errDialUnreachable}
	// Point BackupDest at a path that does not exist, to fail that source too.
	ta.App.Config.BackupDest = filepath.Join(ta.App.Config.BackupDest, "does-not-exist")

	if err := CmdStatus(ta.App, []string{"--json"}); err != nil {
		t.Fatalf("CmdStatus() error = %v, want nil (every source degrades independently)", err)
	}

	var report StatusReport
	if err := json.Unmarshal(ta.Stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode status JSON: %v\noutput: %s", err, ta.Stdout.String())
	}
	if report.Container.Error == "" {
		t.Error("Container.Error = \"\", want a docker-inspect failure recorded")
	}
	if report.RCON.Error == "" {
		t.Error("RCON.Error = \"\", want a dial failure recorded")
	}
	if report.Java.Error == "" || report.Bedrock.Error == "" {
		t.Error("Java/Bedrock errors not recorded")
	}
	if report.Backups.Error == "" {
		t.Error("Backups.Error = \"\", want a missing-dir failure recorded")
	}
	if report.Disk.Error == "" {
		t.Error("Disk.Error = \"\", want a df failure recorded")
	}
}

func TestCmdStatus_HumanReadableOutput(t *testing.T) {
	ta := newTestApp(t, false)
	ta.Exec.out = "false\n"
	dialed := 0
	ta.App.DialRCON = dialerFor(nil, errDialUnreachable, &dialed)
	ta.App.MCStatus = &fakeMCStatus{javaErr: errDialUnreachable, bedrockErr: errDialUnreachable}

	if err := CmdStatus(ta.App, nil); err != nil {
		t.Fatalf("CmdStatus() error = %v", err)
	}
	out := ta.Stdout.String()
	for _, want := range []string{"container", "rcon", "java", "bedrock", "backups", "disk"} {
		if !strings.Contains(out, want) {
			t.Errorf("human output missing section %q, got:\n%s", want, out)
		}
	}
}

func TestCmdStatus_UnknownFlag(t *testing.T) {
	ta := newTestApp(t, false)
	if err := CmdStatus(ta.App, []string{"--bogus"}); err == nil {
		t.Fatal("CmdStatus() with unknown flag: error = nil, want error")
	}
}

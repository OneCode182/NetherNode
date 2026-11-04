package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/onecode182/nethernode/internal/backup"
)

func makeWorld(t *testing.T, dataDir, name string) string {
	t.Helper()
	path := filepath.Join(dataDir, name)
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir world %s: %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(path, "level.dat"), []byte("level data"), 0o644); err != nil {
		t.Fatalf("write level.dat for %s: %v", name, err)
	}
	return path
}

func writeServerProperties(t *testing.T, app *App, body string, mode os.FileMode) string {
	t.Helper()
	path := app.serverPropertiesPath()
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatalf("write server.properties: %v", err)
	}
	return path
}

func TestCmdWorldList_SortedFiltersDimensionsAndSymlinks(t *testing.T) {
	ta := newTestApp(t, false)
	for _, name := range []string{"zeta", "world", "world_nether", "world_the_end", "orphan_nether", "orphan_the_end"} {
		makeWorld(t, ta.App.Config.DataDir, name)
	}
	if err := os.WriteFile(filepath.Join(ta.App.Config.DataDir, "not-a-world"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write non-world entry: %v", err)
	}
	if err := os.Mkdir(filepath.Join(ta.App.Config.DataDir, "missing-level"), 0o755); err != nil {
		t.Fatalf("mkdir missing-level: %v", err)
	}
	target := makeWorld(t, t.TempDir(), "linked")
	if err := os.Symlink(target, filepath.Join(ta.App.Config.DataDir, "linked")); err != nil {
		t.Fatalf("symlink linked world: %v", err)
	}
	levelTarget := filepath.Join(t.TempDir(), "level.dat")
	if err := os.WriteFile(levelTarget, []byte("level data"), 0o644); err != nil {
		t.Fatalf("write level target: %v", err)
	}
	if err := os.Mkdir(filepath.Join(ta.App.Config.DataDir, "symlink-level"), 0o755); err != nil {
		t.Fatalf("mkdir symlink-level: %v", err)
	}
	if err := os.Symlink(levelTarget, filepath.Join(ta.App.Config.DataDir, "symlink-level", "level.dat")); err != nil {
		t.Fatalf("symlink level.dat: %v", err)
	}

	if err := CmdWorld(ta.App, []string{"list"}); err != nil {
		t.Fatalf("CmdWorld list: %v", err)
	}
	want := "orphan_nether\norphan_the_end\nworld\nzeta\n"
	if got := ta.Stdout.String(); got != want {
		t.Fatalf("list output = %q, want %q", got, want)
	}
}

func TestCmdWorldList_EmptyAndUnreadable(t *testing.T) {
	ta := newTestApp(t, false)
	if err := os.Remove(filepath.Join(ta.App.Config.DataDir, "level.dat")); err != nil {
		t.Fatalf("remove seed level.dat: %v", err)
	}
	if err := CmdWorld(ta.App, []string{"list"}); err != nil {
		t.Fatalf("empty list error = %v, want nil", err)
	}
	if ta.Stdout.Len() != 0 {
		t.Fatalf("empty list output = %q, want empty", ta.Stdout.String())
	}

	dataFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(dataFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write unreadable data path: %v", err)
	}
	ta.App.Config.DataDir = dataFile
	if err := CmdWorld(ta.App, []string{"list"}); err == nil {
		t.Fatal("list on non-directory DataDir error = nil, want error")
	}
}

func TestCmdWorldGet_RequiresExactlyOneNonemptyLevelName(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantValue string
		wantErr   bool
	}{
		{name: "one", body: "# comment\nlevel-name=survival\n", wantValue: "survival"},
		{name: "empty plus one", body: "level-name=\nlevel-name=creative\n", wantValue: "creative"},
		{name: "missing", body: "motd=hello\n", wantErr: true},
		{name: "empty", body: "level-name=\n", wantErr: true},
		{name: "duplicate nonempty", body: "level-name=one\nlevel-name=two\n", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ta := newTestApp(t, false)
			writeServerProperties(t, ta.App, tc.body, 0o644)
			err := CmdWorld(ta.App, []string{"get"})
			if (err != nil) != tc.wantErr {
				t.Fatalf("get error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if got := ta.Stdout.String(); got != tc.wantValue+"\n" {
				t.Fatalf("get output = %q, want %q", got, tc.wantValue+"\n")
			}
		})
	}
}

func TestCmdWorldSet_PatchesOnlyLevelNameAndPreservesMode(t *testing.T) {
	ta := newTestApp(t, false)
	makeWorld(t, ta.App.Config.DataDir, "creative")
	path := writeServerProperties(t, ta.App, "# header\nlevel-name=old\nmotd=Keep me\n\n", 0o640)

	if err := CmdWorld(ta.App, []string{"set", "creative"}); err != nil {
		t.Fatalf("world set: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read patched properties: %v", err)
	}
	want := "# header\nlevel-name=creative\nmotd=Keep me\n\n"
	if string(got) != want {
		t.Fatalf("patched properties = %q, want %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat patched properties: %v", err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("patched mode = %o, want 640", info.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), ".server.properties.tmp")); !os.IsNotExist(err) {
		t.Fatalf("atomic temp file status = %v, want nonexistent", err)
	}
	if !strings.Contains(ta.Stdout.String(), "restart needed") {
		t.Fatalf("set output = %q, want restart-needed note", ta.Stdout.String())
	}
}

func TestCmdWorldSet_ApplyPatchesBeforeSafeRestart(t *testing.T) {
	ta := newTestApp(t, false)
	makeWorld(t, ta.App.Config.DataDir, "new-world")
	path := writeServerProperties(t, ta.App, "# keep\nlevel-name=old\nmotd=unchanged\n", 0o644)

	client := newFakeRCON()
	ta.App.DialRCON = dialerFor(client, nil, nil)
	var events []string
	ta.Exec.reply = func(_ context.Context, _ string, args ...string) (string, error) {
		switch {
		case hasArg(args, "down"):
			events = append(events, "down")
		case hasArg(args, "up"):
			events = append(events, "up")
		}
		return "compose ok", nil
	}
	ta.App.Backup = func(_ backup.Options, _ time.Time) (*backup.Result, error) {
		events = append(events, "backup")
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if !strings.Contains(string(body), "level-name=new-world\n") {
			return nil, errors.New("backup observed unpatched server.properties")
		}
		return &backup.Result{ArchivePath: "fake-backup.tar.gz"}, nil
	}

	if err := CmdWorld(ta.App, []string{"set", "--apply", "new-world"}); err != nil {
		t.Fatalf("world set --apply: %v", err)
	}
	if got, want := strings.Join(events, ","), "backup,down,up"; got != want {
		t.Fatalf("restart events = %q, want %q", got, want)
	}
	if len(client.calls) != 1 || client.calls[0] != "save-all flush" {
		t.Fatalf("RCON calls = %v, want save-all flush", client.calls)
	}
}

func TestCmdWorldSet_RejectsInvalidNamesAndDoesNotMutate(t *testing.T) {
	invalid := []string{"", " leading", "trailing ", ".", "..", "/absolute", "one/two", `one\two`, "nul\x00name", "line\nname"}
	for _, name := range invalid {
		t.Run(strings.ReplaceAll(name, "\x00", "NUL"), func(t *testing.T) {
			ta := newTestApp(t, false)
			path := writeServerProperties(t, ta.App, "level-name=old\n", 0o644)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read before: %v", err)
			}
			if err := CmdWorld(ta.App, []string{"set", name}); err == nil {
				t.Fatalf("set name %q error = nil, want error", name)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read after: %v", err)
			}
			if !bytes.Equal(after, before) {
				t.Fatalf("invalid name %q mutated properties: before %q after %q", name, before, after)
			}
		})
	}
}

func TestCmdWorldSet_RejectsMissingWorldAndDuplicateProperty(t *testing.T) {
	t.Run("missing world", func(t *testing.T) {
		ta := newTestApp(t, false)
		path := writeServerProperties(t, ta.App, "level-name=old\n", 0o644)
		before, _ := os.ReadFile(path)
		if err := CmdWorld(ta.App, []string{"set", "missing"}); err == nil {
			t.Fatal("missing world error = nil, want error")
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(after, before) {
			t.Fatalf("missing world mutated properties: before %q after %q", before, after)
		}
	})

	t.Run("duplicate property", func(t *testing.T) {
		ta := newTestApp(t, false)
		makeWorld(t, ta.App.Config.DataDir, "new-world")
		path := writeServerProperties(t, ta.App, "level-name=one\nlevel-name=two\n", 0o644)
		before, _ := os.ReadFile(path)
		if err := CmdWorld(ta.App, []string{"set", "new-world"}); err == nil {
			t.Fatal("duplicate level-name error = nil, want error")
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(after, before) {
			t.Fatalf("duplicate property mutated: before %q after %q", before, after)
		}
	})
}

func TestCmdWorldSet_UsesFilteredSelectableWorlds(t *testing.T) {
	ta := newTestApp(t, false)
	makeWorld(t, ta.App.Config.DataDir, "world")
	makeWorld(t, ta.App.Config.DataDir, "world_nether")
	path := writeServerProperties(t, ta.App, "# preserve\nlevel-name=old\nmotd=unchanged\n", 0o644)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read properties before hidden-world set: %v", err)
	}

	if err := CmdWorld(ta.App, []string{"set", "world_nether"}); err == nil {
		t.Fatal("set world_nether error = nil, want hidden dimension rejection")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read properties after hidden-world set: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("hidden dimension mutated properties: before %q after %q", before, after)
	}

	makeWorld(t, ta.App.Config.DataDir, "orphan_nether")
	if err := CmdWorld(ta.App, []string{"set", "orphan_nether"}); err != nil {
		t.Fatalf("set orphan_nether: %v", err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read properties after orphan set: %v", err)
	}
	if !bytes.Contains(updated, []byte("level-name=orphan_nether\n")) {
		t.Fatalf("orphan selection did not patch level-name: %q", updated)
	}
}

func TestCmdWorld_ArgumentsDispatchAndHelp(t *testing.T) {
	if _, ok := lifecycleCommands["world"]; !ok {
		t.Fatal("lifecycleCommands does not register world")
	}
	if !strings.Contains(usage, "world set <name> [--apply]") {
		t.Fatal("usage does not document world set")
	}

	ta := newTestApp(t, false)
	cases := [][]string{
		{},
		{"unknown"},
		{"list", "extra"},
		{"get", "extra"},
		{"set"},
		{"set", "one", "two"},
		{"set", "one", "--apply", "--apply"},
	}
	for _, args := range cases {
		if err := CmdWorld(ta.App, args); err == nil {
			t.Fatalf("CmdWorld(%v) error = nil, want error", args)
		}
	}
}

func TestCmdWorld_DryRunDoesNotReadWriteOrCallExternalDependencies(t *testing.T) {
	ta := newTestApp(t, true)
	dataDir := filepath.Join(t.TempDir(), "missing-data")
	ta.App.Config.DataDir = dataDir
	ta.App.Backup = func(backup.Options, time.Time) (*backup.Result, error) {
		t.Fatal("dry-run called backup")
		return nil, nil
	}
	dialed := 0
	ta.App.DialRCON = dialerFor(newFakeRCON(), nil, &dialed)

	for _, args := range [][]string{{"list"}, {"get"}, {"set", "new-world"}, {"set", "--apply", "new-world"}} {
		if err := CmdWorld(ta.App, args); err != nil {
			t.Fatalf("CmdWorld(%v) dry-run error = %v", args, err)
		}
	}
	if dialed != 0 {
		t.Fatalf("dry-run dialed RCON %d times, want 0", dialed)
	}
	if len(ta.Exec.calls) != 0 {
		t.Fatalf("dry-run invoked compose: %+v", ta.Exec.calls)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("dry-run created DataDir: stat error = %v", err)
	}
	if out := ta.Stdout.String(); strings.Count(out, "[dry-run]") < 6 {
		t.Fatalf("dry-run output = %q, want all commands to emit plans", out)
	}
}

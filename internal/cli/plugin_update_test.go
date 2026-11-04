package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/onecode182/nethernode/internal/pluginupdate"
)

func pluginJarFixture(t *testing.T, name, version string) []byte {
	t.Helper()
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	file, err := archive.Create("plugin.yml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(file, "name: %s\nversion: '%s'\nmain: example.Main\n", name, version); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func writeFixture(t *testing.T, path string, body []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, body, mode); err != nil {
		t.Fatal(err)
	}
}

type pluginFixture struct {
	*testApp
	plugins         string
	running         bool
	players         int
	failNewHealth   bool
	failSave        bool
	failSaveOn      bool
	checkingVersion bool
	failedStart     bool
	commands        []string
	old             installedPlugins
	new             installedPlugins
	bundle          pluginupdate.Bundle
}

type pluginRCONFixture struct{ fixture *pluginFixture }

func (client pluginRCONFixture) Close() error { return nil }
func (client pluginRCONFixture) Command(command string) (string, error) {
	f := client.fixture
	f.commands = append(f.commands, command)
	switch command {
	case "version":
		if f.checkingVersion {
			f.checkingVersion = false
			return "Checking version, please wait...", nil
		}
		return "This server is running Paper version 26.1.2-74-ver/26.1.2@d4c8ab9 (2026-10-02T13:00:00Z) (Implementing API version 26.1.2.build.74-stable)", nil
	case "list":
		return fmt.Sprintf("There are %d of a max of 5 players online:", f.players), nil
	case "plugins":
		return "Plugins (3): §aGeyser-Spigot§r, §aViaVersion§r, §aViaBackwards§r", nil
	case "save-all flush":
		if f.failSave {
			return "Unable to save", nil
		}
		return "Saved the game", nil
	case "save-off":
		return "Automatic saving is now disabled", nil
	case "save-on":
		if f.failSaveOn {
			return "", errors.New("save-on failed")
		}
		return "Automatic saving is now enabled", nil
	case "geyser version":
		return "Geyser version 2.12.0-b1258\nSupported Bedrock versions: 26.40 - 26.52", nil
	}
	if strings.HasPrefix(command, "version ") {
		installed, err := inspectInstalledPlugins(f.plugins)
		if err != nil {
			return "", err
		}
		names := []string{"Geyser-Spigot", "ViaVersion", "ViaBackwards"}
		for index, name := range names {
			if command == "version "+name {
				if f.failNewHealth && installed == f.new {
					return name + " version broken", nil
				}
				return name + " version " + installed[index].Version, nil
			}
		}
	}
	return "", fmt.Errorf("unexpected RCON command %q", command)
}

func newPluginFixture(t *testing.T) *pluginFixture {
	t.Helper()
	f := &pluginFixture{testApp: newTestApp(t, false), running: true}
	f.App.Config.MinecraftVersion = "26.2" // Live aux runtime intentionally differs.
	f.App.Config.PluginUpdateDir = t.TempDir()
	if err := os.Chmod(f.App.Config.PluginUpdateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	f.plugins = filepath.Join(f.App.Config.DataDir, "plugins")
	if err := os.Mkdir(f.plugins, 0o755); err != nil {
		t.Fatal(err)
	}
	names := []string{"Geyser-Spigot", "ViaVersion", "ViaBackwards"}
	oldVersions := []string{"2.11.0-SNAPSHOT", "5.11.0", "5.11.0"}
	newVersions := []string{"2.12.0-SNAPSHOT", "5.12.1", "5.12.1"}
	oldFiles := []string{"Geyser-Spigot.jar", "ViaVersion-5.11.0.jar", "ViaBackwards-5.11.0.jar"}
	newFiles := []string{"Geyser-Spigot.jar", "ViaVersion-5.12.1.jar", "ViaBackwards-5.12.1.jar"}
	staged := filepath.Join(f.App.Config.PluginUpdateDir, "staged")
	if err := os.Mkdir(staged, 0o700); err != nil {
		t.Fatal(err)
	}
	f.bundle = pluginupdate.Bundle{ServerVersion: "26.1.2", CreatedAt: fixedNow}
	for index, name := range names {
		writeFixture(t, filepath.Join(f.plugins, oldFiles[index]), pluginJarFixture(t, name, oldVersions[index]), 0o644)
		body := pluginJarFixture(t, name, newVersions[index])
		writeFixture(t, filepath.Join(staged, newFiles[index]), body, 0o600)
		artifact := pluginupdate.Artifact{Name: pluginNames[index], Version: newVersions[index], Filename: newFiles[index], HashAlgorithm: "sha512", URL: fmt.Sprintf("https://cdn.modrinth.com/data/ABCD/versions/EFGH/%s", newFiles[index])}
		if index == 0 {
			artifact.Version = "2.12.0"
			artifact.HashAlgorithm = "sha256"
			artifact.URL = "https://download.geysermc.org/v2/projects/geyser/versions/2.12.0/builds/1258/downloads/spigot"
			hash := sha256.Sum256(body)
			artifact.Hash = hex.EncodeToString(hash[:])
		} else {
			hash := sha512.Sum512(body)
			artifact.Hash = hex.EncodeToString(hash[:])
		}
		f.bundle.Artifacts[index] = artifact
	}
	var err error
	f.old, err = inspectInstalledPlugins(f.plugins)
	if err != nil {
		t.Fatal(err)
	}
	f.new, err = inspectInstalledPlugins(staged)
	if err != nil {
		t.Fatal(err)
	}
	if err := writePluginJSON(filepath.Join(staged, "bundle.json"), f.bundle); err != nil {
		t.Fatal(err)
	}
	if err := writePluginJSON(filepath.Join(staged, "baseline.json"), f.old); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(f.plugins, ".nethernode-plugins.state"), []byte("floodgate|floodgate-spigot.jar\ngeyser|Geyser-Spigot.jar\nviaversion|ViaVersion-5.11.0.jar\nviabackwards|ViaBackwards-5.11.0.jar\nskinsrestorer|SkinsRestorer.jar\n"), 0o644)
	if err := os.Mkdir(filepath.Join(f.plugins, "floodgate"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(f.plugins, "floodgate", "key.pem"), []byte("test-key-preserve"), 0o600)
	f.App.DialRCON = func(string, string, time.Duration) (rconClient, error) { return pluginRCONFixture{f}, nil }
	f.Exec.reply = func(_ context.Context, name string, args ...string) (string, error) {
		joined := strings.Join(args, " ")
		f.commands = append(f.commands, name+" "+joined)
		if name != "docker" {
			return "", fmt.Errorf("unexpected exec %s", name)
		}
		if len(args) > 0 && args[0] == "inspect" {
			if strings.Contains(joined, ".Mounts") {
				return f.App.Config.DataDir, nil
			}
			return fmt.Sprint(f.running), nil
		}
		if strings.Contains(joined, "stop --timeout 120 minecraft") {
			f.running = false
			return "stopped", nil
		}
		if strings.HasSuffix(joined, "start minecraft") {
			if f.failedStart {
				f.failedStart = false
				return "", errors.New("start failed")
			}
			f.running = true
			return "started", nil
		}
		return "", fmt.Errorf("unexpected docker args %v", args)
	}
	f.App.PluginHTTPClient = &http.Client{Transport: pluginHTTPTransport(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network must not be used during apply")
	})}
	return f
}

type pluginHTTPTransport func(*http.Request) (*http.Response, error)

func (transport pluginHTTPTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestPluginApplyBundleAndRollback(t *testing.T) {
	f := newPluginFixture(t)
	if err := CmdPlugins(f.App, []string{"update", "geyser", "--apply"}); err != nil {
		t.Fatal(err)
	}
	got, err := inspectInstalledPlugins(f.plugins)
	if err != nil {
		t.Fatal(err)
	}
	if got != f.new {
		t.Fatalf("installed=%v,want %v", got, f.new)
	}
	state, err := os.ReadFile(filepath.Join(f.plugins, ".nethernode-plugins.state"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"floodgate|floodgate-spigot.jar", "skinsrestorer|SkinsRestorer.jar", "viaversion|ViaVersion-5.12.1.jar"} {
		if !strings.Contains(string(state), expected) {
			t.Fatalf("state missing %s: %s", expected, state)
		}
	}
	key, err := os.ReadFile(filepath.Join(f.plugins, "floodgate", "key.pem"))
	if err != nil || string(key) != "test-key-preserve" {
		t.Fatalf("Floodgate key changed: %s %v", key, err)
	}
	commands := strings.Join(f.commands, "\n")
	if strings.Contains(commands, "pull") || strings.Contains(commands, "compose -f compose.yaml down") || strings.Contains(commands, "up -d") {
		t.Fatalf("unexpected lifecycle mutation: %s", commands)
	}
	if !strings.Contains(commands, "save-all flush\nsave-off\nsave-all flush\nsave-on") {
		t.Fatalf("save ordering: %s", commands)
	}
	if err := CmdPlugins(f.App, []string{"rollback", "geyser", "--apply"}); err != nil {
		t.Fatal(err)
	}
	got, err = inspectInstalledPlugins(f.plugins)
	if err != nil {
		t.Fatal(err)
	}
	if got != f.old {
		t.Fatalf("rollback=%v,want %v", got, f.old)
	}
	if err := CmdPlugins(f.App, []string{"rollback", "geyser", "--apply"}); err == nil {
		t.Fatal("repeated rollback succeeded")
	}
}

func TestPluginApplySafetyGates(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *pluginFixture)
		want   string
	}{
		{"players", func(_ *testing.T, f *pluginFixture) { f.players = 1 }, "zero players"},
		{"stopped", func(_ *testing.T, f *pluginFixture) { f.running = false }, "already be running"},
		{"save failure", func(_ *testing.T, f *pluginFixture) { f.failSave = true }, "successful save-all flush"},
		{"save-on failure", func(_ *testing.T, f *pluginFixture) { f.failSaveOn = true }, "restore autosave"},
		{"JAR drift", func(t *testing.T, f *pluginFixture) {
			writeFixture(t, filepath.Join(f.plugins, f.old[0].Filename), pluginJarFixture(t, "Geyser-Spigot", "2.10.0"), 0o644)
		}, "drift"},
		{"staged hash drift", func(t *testing.T, f *pluginFixture) {
			writeFixture(t, filepath.Join(f.App.Config.PluginUpdateDir, "staged", f.new[0].Filename), []byte("corrupt"), 0o600)
		}, "checksum mismatch"},
		{"duplicate", func(t *testing.T, f *pluginFixture) {
			writeFixture(t, filepath.Join(f.plugins, "duplicate.jar"), pluginJarFixture(t, "Geyser-Spigot", "2.11.0"), 0o644)
		}, "duplicate"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newPluginFixture(t)
			test.mutate(t, f)
			err := CmdPlugins(f.App, []string{"update", "geyser", "--apply"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v,want %s", err, test.want)
			}
			if strings.Contains(strings.Join(f.commands, "\n"), "stop --timeout") {
				t.Fatalf("unsafe stop after %s: %v", test.name, f.commands)
			}
		})
	}
}

func TestPluginActivationFailureRestoresPreviousJARs(t *testing.T) {
	for _, failure := range []string{"health", "start"} {
		t.Run(failure, func(t *testing.T) {
			f := newPluginFixture(t)
			f.failNewHealth = failure == "health"
			f.failedStart = failure == "start"
			err := CmdPlugins(f.App, []string{"update", "geyser", "--apply"})
			if err == nil || !strings.Contains(err.Error(), "previous JARs restored and healthy") {
				t.Fatalf("error=%v", err)
			}
			got, readErr := inspectInstalledPlugins(f.plugins)
			if readErr != nil || got != f.old {
				t.Fatalf("rollback=%v %v", got, readErr)
			}
			if !f.running {
				t.Fatal("server was not restarted after rollback")
			}
		})
	}
}

func TestPluginUpdateDryRunAndArguments(t *testing.T) {
	for _, args := range [][]string{{"update", "geyser"}, {"update", "geyser", "--apply"}, {"check", "geyser"}, {"rollback", "geyser", "--apply"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			f := newPluginFixture(t)
			f.App.DryRun = true
			f.App.Config.DataDir = "/does-not-exist"
			if err := CmdPlugins(f.App, args); err != nil {
				t.Fatal(err)
			}
			if len(f.commands) != 0 {
				t.Fatalf("dry-run executed %v", f.commands)
			}
		})
	}
	f := newPluginFixture(t)
	for _, args := range [][]string{{"update"}, {"update", "floodgate"}, {"update", "geyser", "--restart"}, {"check", "geyser", "--apply"}, {"rollback", "geyser"}, {"update", "geyser", "--apply", "--apply"}} {
		if err := CmdPlugins(f.App, args); err == nil {
			t.Fatalf("invalid arguments accepted: %v", args)
		}
	}
}

func TestEnabledPluginParsingAndBedrockRange(t *testing.T) {
	if err := enabledPluginOutput("Plugins: §aGeyser-Spigot§r, §aViaVersion§r, §aViaBackwards§r"); err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{"Plugins: Geyser-Spigot, ViaVersion, ViaBackwards", "Plugins: §cGeyser-Spigot§r, §aViaVersion§r, §aViaBackwards§r", "Unknown command"} {
		if enabledPluginOutput(out) == nil {
			t.Fatalf("unsafe enabled state accepted: %q", out)
		}
	}
	for _, test := range []struct {
		output string
		want   bool
	}{{"Supported Bedrock versions: 26.40 - 26.52", true}, {"Supported Bedrock versions: 26.52", true}, {"Supported Bedrock versions: 26.50", false}, {"Geyser 26.52", false}, {"Bedrock: 26.52 - 26.53", true}} {
		if got := geyserSupportsTargetBedrock(test.output); got != test.want {
			t.Fatalf("support(%q)=%t,want %t", test.output, got, test.want)
		}
	}
}

func TestPluginStateLocationAndLock(t *testing.T) {
	f := newPluginFixture(t)
	for _, path := range []string{f.App.Config.DataDir, filepath.Join(f.App.Config.DataDir, "nested"), filepath.Dir(f.App.Config.DataDir), f.App.Config.BackupDest} {
		if err := validatePluginStateLocation(path, f.App.Config.DataDir, f.App.Config.BackupDest); err == nil {
			t.Fatalf("unsafe state location accepted: %s", path)
		}
	}
	unlock, err := lockPluginUpdates(filepath.Join(f.plugins, ".nethernode-plugin-update.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if err := CmdPlugins(f.App, []string{"update", "geyser", "--apply"}); err == nil || !strings.Contains(err.Error(), "holds the lock") {
		t.Fatalf("lock error=%v", err)
	}
}

func TestLivePaperVersionFormatsAndAsyncRetry(t *testing.T) {
	for _, test := range []struct{ output, want string }{
		{"This server is running Paper version 26.1.2-74-ver/26.1.2@abca987 (2026-10-02T13:00:00Z) (Implementing API version 26.1.2.build.74-stable)", "26.1.2"},
		{"This server is running Paper version git-Paper-74 (MC: 1.21.4)", "1.21.4"},
		{"§aThis server is running Paper version 26.2-44-ver/26.2@abc", "26.2"},
	} {
		got, err := parseLivePluginMinecraftVersion(test.output)
		if err != nil || got != test.want {
			t.Fatalf("parse(%q)=%q,%v,want %q", test.output, got, err, test.want)
		}
	}
	if _, err := parseLivePluginMinecraftVersion("Checking version, please wait..."); err == nil {
		t.Fatal("async status accepted as version")
	}
	f := newPluginFixture(t)
	f.checkingVersion = true
	got, err := f.App.livePluginMinecraftVersion()
	if err != nil || got != "26.1.2" {
		t.Fatalf("async retry=%s,%v", got, err)
	}
	if strings.Count(strings.Join(f.commands, "\n"), "version") != 2 {
		t.Fatalf("version retries=%v", f.commands)
	}
}

func TestPluginBackupFailureRestoresAutosaveWithoutStopping(t *testing.T) {
	f := newPluginFixture(t)
	f.App.Config.BackupDest = filepath.Join(t.TempDir(), "not-a-directory")
	writeFixture(t, f.App.Config.BackupDest, []byte("existing"), 0o600)
	err := CmdPlugins(f.App, []string{"update", "geyser", "--apply"})
	if err == nil {
		t.Fatal("backup destination error ignored")
	}
	commands := strings.Join(f.commands, "\n")
	if !strings.Contains(commands, "save-on") {
		t.Fatal("autosave not restored after archive failure")
	}
	if strings.Contains(commands, "stop --timeout") {
		t.Fatalf("stopped after backup failed: %s", commands)
	}
}

func TestPluginStageUsesLiveVersionWithoutApplying(t *testing.T) {
	f := newPluginFixture(t)
	staged := filepath.Join(f.App.Config.PluginUpdateDir, "staged")
	payloads := make(map[string][]byte)
	for _, artifact := range f.bundle.Artifacts {
		body, err := os.ReadFile(filepath.Join(staged, artifact.Filename))
		if err != nil {
			t.Fatal(err)
		}
		payloads[artifact.URL] = body
	}
	queriedVersions := []string{}
	f.App.PluginHTTPClient = &http.Client{Transport: pluginHTTPTransport(func(request *http.Request) (*http.Response, error) {
		var body []byte
		if value, ok := payloads[request.URL.String()]; ok {
			body = value
		} else if strings.Contains(request.URL.Path, "/projects/geyser/") {
			body, _ = json.Marshal(map[string]any{"project_id": "geyser", "version": "2.12.0", "build": 1258, "channel": "default", "downloads": map[string]any{"spigot": map[string]string{"name": f.bundle.Artifacts[0].Filename, "sha256": f.bundle.Artifacts[0].Hash}}})
		} else {
			queriedVersions = append(queriedVersions, request.URL.Query().Get("game_versions"))
			index := 1
			if strings.Contains(request.URL.Path, "viabackwards") {
				index = 2
			}
			artifact := f.bundle.Artifacts[index]
			body, _ = json.Marshal([]any{map[string]any{"id": "EFGH", "project_id": "ABCD", "version_number": artifact.Version, "version_type": "release", "date_published": "2026-10-14T12:00:00Z", "loaders": []string{"paper"}, "game_versions": []string{"26.1.2"}, "files": []any{map[string]any{"filename": artifact.Filename, "url": artifact.URL, "primary": true, "hashes": map[string]string{"sha512": artifact.Hash}}}}})
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})}
	if err := CmdPlugins(f.App, []string{"update", "geyser"}); err != nil {
		t.Fatal(err)
	}
	for _, version := range queriedVersions {
		if version != `["26.1.2"]` {
			t.Fatalf("resolved configured version instead of live: %s", version)
		}
	}
	got, err := inspectInstalledPlugins(f.plugins)
	if err != nil || got != f.old {
		t.Fatalf("stage mutated installed JARs: %v %v", got, err)
	}
	if strings.Contains(strings.Join(f.commands, "\n"), "stop --timeout") {
		t.Fatal("stage stopped server")
	}
	if _, err := pluginupdate.ReadStaged(staged); err != nil {
		t.Fatal(err)
	}
}

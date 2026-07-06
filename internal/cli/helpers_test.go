package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/onecode182/nethernode/internal/backup"
	"github.com/onecode182/nethernode/internal/compose"
)

// fixedNow is used wherever tests need a deterministic clock.
var fixedNow = time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

// testApp bundles an App wired to fakes/temp dirs with the pieces tests
// need to assert on: the compose exec recorder and the stdout/stderr
// buffers.
type testApp struct {
	App    *App
	Exec   *execRecorder
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
}

// newTestApp builds an App backed entirely by fakes and t.TempDir()
// directories: no real docker, RCON socket, or network call is reachable
// from it, so tests can run fully offline.
func newTestApp(t *testing.T, dryRun bool) *testApp {
	t.Helper()

	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "level.dat"), []byte("world save data"), 0o644); err != nil {
		t.Fatalf("seed data dir: %v", err)
	}
	backupDir := t.TempDir()

	rec := &execRecorder{out: "ok"}
	var stdout, stderr bytes.Buffer

	app := &App{
		Config: Config{
			ContainerName:   "nethernode-minecraft",
			ComposeFile:     "compose.yaml",
			DataDir:         dataDir,
			BackupDest:      backupDir,
			BackupRetention: 5,
			BackupLabel:     "minecraft",
			RCONHost:        "127.0.0.1",
			RCONPort:        "25575",
			RCONPassword:    "s3cr3t",
			RCONTimeout:     time.Second,
			PublicHost:      "localhost",
			JavaPort:        "25565",
			BedrockPort:     "19132",
		},
		Stdout: &stdout,
		Stderr: &stderr,
		DryRun: dryRun,
		Now:    func() time.Time { return fixedNow },
		Compose: &compose.Runner{
			ComposeFile: "compose.yaml",
			DryRun:      dryRun,
			Exec:        rec.exec,
		},
		Backup: backup.Run,
	}

	return &testApp{App: app, Exec: rec, Stdout: &stdout, Stderr: &stderr}
}

// countBackupArchives counts files under dir; useful for asserting whether
// CmdStop/CmdRestart/CmdBackupServer actually wrote (or, for --no-backup /
// dry-run, did not write) an archive.
func countEntries(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	return len(entries)
}

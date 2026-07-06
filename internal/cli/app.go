package cli

import (
	"context"
	"io"
	"time"

	"github.com/onecode182/nethernode/internal/backup"
	"github.com/onecode182/nethernode/internal/compose"
	"github.com/onecode182/nethernode/internal/mcstatus"
	"github.com/onecode182/nethernode/internal/rcon"
)

// rconClient is the subset of *rcon.Client the CLI needs, narrowed to an
// interface so tests can inject a fake instead of a live TCP connection.
type rconClient interface {
	Command(cmd string) (string, error)
	Close() error
}

// rconDialer opens an authenticated RCON session. The default wraps
// rcon.DialTimeout; tests override it to avoid real sockets.
type rconDialer func(addr, password string, timeout time.Duration) (rconClient, error)

func defaultDialRCON(addr, password string, timeout time.Duration) (rconClient, error) {
	return rcon.DialTimeout(addr, password, timeout)
}

// mcstatusClient is the subset of *mcstatus.Client the CLI needs.
type mcstatusClient interface {
	Java(ctx context.Context, address string) (*mcstatus.JavaStatus, error)
	Bedrock(ctx context.Context, address string) (*mcstatus.BedrockStatus, error)
}

// backupRunner runs a backup (including its own retention prune); the
// default is backup.Run.
type backupRunner func(opts backup.Options, now time.Time) (*backup.Result, error)

// App wires together the CLI's dependencies. Every side-effecting
// dependency (compose, RCON, mcstatus.io, backups) is an interface/func
// field so unit tests can substitute fakes and run fully offline.
type App struct {
	Config Config
	Stdout io.Writer
	Stderr io.Writer
	DryRun bool

	Now      func() time.Time
	Compose  *compose.Runner
	DialRCON rconDialer
	MCStatus mcstatusClient
	Backup   backupRunner
}

// NewApp builds an App wired to the real environment (docker compose,
// live RCON dial, mcstatus.io, filesystem tar/gzip backups).
func NewApp(cfg Config, stdout, stderr io.Writer, dryRun bool) *App {
	return &App{
		Config: cfg,
		Stdout: stdout,
		Stderr: stderr,
		DryRun: dryRun,
		Now:    time.Now,
		Compose: &compose.Runner{
			ComposeFile: cfg.ComposeFile,
			DryRun:      dryRun,
		},
		DialRCON: defaultDialRCON,
		MCStatus: mcstatus.New(),
		Backup:   backup.Run,
	}
}

// backupOptions builds backup.Options from Config, applying an optional
// retention override (<=0 means "use Config.BackupRetention").
func (a *App) backupOptions(retentionOverride int) backup.Options {
	retention := a.Config.BackupRetention
	if retentionOverride > 0 {
		retention = retentionOverride
	}
	return backup.Options{
		SourceDir: a.Config.DataDir,
		DestDir:   a.Config.BackupDest,
		Label:     a.Config.BackupLabel,
		Retention: retention,
	}
}

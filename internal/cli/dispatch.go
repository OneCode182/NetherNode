package cli

import (
	"fmt"
	"io"
)

// Version is the CLI version string; overridden at build time via
// -ldflags "-X github.com/onecode182/nethernode/internal/cli.Version=...".
var Version = "dev"

const usage = `NetherNode server CLI

Usage:
  nethernode [--dry-run] <command> [flags]

Commands:
  help                          Show this help.
  version                       Print CLI version.
  start                         docker compose up -d.
  stop [--no-backup]            Save + backup, then docker compose down.
  restart [--no-backup]         Save + backup, then down and up -d.
  status [--host H] [--json]    Docker/RCON/mcstatus/backups/disk summary.
  save-server                   RCON save-all flush.
  backup-server [--retention N] Save + archive + prune old backups.
  admin list                    List ops.json entries.
  admin add <player> [--level N] RCON op; fallback ops.json patch offline.
  admin remove <player>          RCON deop; fallback ops.json removal offline.
  settings get <key>            Print a server.properties value.
  settings set <key> <value>    Atomically patch server.properties.
    [--apply]                     Also apply live via RCON when possible.

Global flags:
  --dry-run   Print planned actions/commands and exit 0 without touching
              docker, RCON, the network, or files.

plugins management arrives in a later step.
`

// Run parses args, dispatches to the matching command, and returns the
// process exit code. Errors are written to stderr.
func Run(args []string, stdout, stderr io.Writer) int {
	dryRun, rest := extractDryRun(args)

	command := "help"
	if len(rest) > 0 {
		command = rest[0]
		rest = rest[1:]
	} else {
		rest = nil
	}

	switch command {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "version", "--version":
		fmt.Fprintf(stdout, "nethernode %s\n", Version)
		return 0
	}

	handler, ok := lifecycleCommands[command]
	if !ok {
		fmt.Fprintf(stderr, "unknown command: %s\n\n%s", command, usage)
		return 1
	}

	app := NewApp(LoadConfig(), stdout, stderr, dryRun)
	if err := handler(app, rest); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// lifecycleCommands maps command names to their implementation. Kept as a
// map (rather than a longer switch) so dispatch and the command table stay
// in one place as admin/settings/plugins commands land in later steps.
var lifecycleCommands = map[string]func(*App, []string) error{
	"start":         CmdStart,
	"stop":          CmdStop,
	"restart":       CmdRestart,
	"status":        CmdStatus,
	"save-server":   CmdSaveServer,
	"backup-server": CmdBackupServer,
	"admin":         CmdAdmin,
	"settings":      CmdSettings,
}

// extractDryRun removes every "--dry-run" occurrence from args (regardless
// of position, so it works as both a leading global flag and a trailing
// per-command one) and reports whether it was present.
func extractDryRun(args []string) (bool, []string) {
	found := false
	rest := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--dry-run" {
			found = true
			continue
		}
		rest = append(rest, a)
	}
	return found, rest
}

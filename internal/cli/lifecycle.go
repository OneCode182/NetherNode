package cli

import (
	"context"
	"flag"
	"fmt"
)

// printf writes to a.Stdout, matching fmt.Fprintf's signature. Output errors
// are ignored: Stdout is a terminal or test buffer, never a resource whose
// write failures the CLI needs to react to.
func (a *App) printf(format string, args ...any) {
	fmt.Fprintf(a.Stdout, format, args...)
}

func (a *App) warnf(format string, args ...any) {
	fmt.Fprintf(a.Stderr, format, args...)
}

// rconRun dials RCON once, runs each command in cmds in order, and closes
// the session. It stops at the first command error but still returns the
// output collected so far.
func (a *App) rconRun(cmds ...string) ([]string, error) {
	client, err := a.DialRCON(a.Config.RCONAddr(), a.Config.RCONPassword, a.Config.RCONTimeout)
	if err != nil {
		return nil, fmt.Errorf("rcon dial %s: %w", a.Config.RCONAddr(), err)
	}
	defer client.Close()

	outs := make([]string, 0, len(cmds))
	for _, c := range cmds {
		out, err := client.Command(c)
		if err != nil {
			return outs, fmt.Errorf("rcon command %q: %w", c, err)
		}
		outs = append(outs, out)
	}
	return outs, nil
}

// saveBestEffort runs "save-all flush" over RCON, logging (not failing) on
// error: a server that is not running, or not RCON-reachable, simply has
// nothing to flush.
func (a *App) saveBestEffort() {
	if _, err := a.rconRun("save-all flush"); err != nil {
		a.warnf("warning: save-all flush skipped: %v\n", err)
	}
}

// runBackup runs a backup with an optional retention override (<=0 keeps
// Config.BackupRetention) and prints a one-line summary.
func (a *App) runBackup(retentionOverride int) (string, error) {
	res, err := a.Backup(a.backupOptions(retentionOverride), a.Now())
	if err != nil {
		return "", err
	}
	msg := fmt.Sprintf("backup created: %s (%d bytes), pruned %d old archive(s)\n", res.ArchivePath, res.Size, len(res.Removed))
	a.printf("%s", msg)
	return msg, nil
}

// CmdStart runs `docker compose up -d`.
func CmdStart(a *App, args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Compose.DryRun (set from a.DryRun at App construction) makes Up return
	// the command line it would have run instead of executing it, so this
	// call is safe to make unconditionally in dry-run mode too.
	out, err := a.Compose.Up(context.Background())
	if a.DryRun {
		a.printf("[dry-run] %s\n", out)
		return nil
	}
	a.printf("%s\n", out)
	if err != nil {
		return fmt.Errorf("start: docker compose up: %w", err)
	}
	return nil
}

// CmdStop saves and backs up (unless --no-backup), then runs
// `docker compose down`.
func CmdStop(a *App, args []string) error {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	noBackup := fs.Bool("no-backup", false, "skip save+backup before stopping")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if a.DryRun {
		a.printPreShutdownPlan(*noBackup)
		out, _ := a.Compose.Down(context.Background())
		a.printf("[dry-run] %s\n", out)
		return nil
	}

	a.saveBestEffort()
	if !*noBackup {
		if _, err := a.runBackup(0); err != nil {
			return fmt.Errorf("stop: backup before shutdown: %w", err)
		}
	}

	out, err := a.Compose.Down(context.Background())
	a.printf("%s\n", out)
	if err != nil {
		return fmt.Errorf("stop: docker compose down: %w", err)
	}
	return nil
}

// CmdRestart saves and backs up (unless --no-backup), then runs
// `docker compose down` followed by `docker compose up -d`.
func CmdRestart(a *App, args []string) error {
	fs := flag.NewFlagSet("restart", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	noBackup := fs.Bool("no-backup", false, "skip save+backup before restarting")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if a.DryRun {
		a.printPreShutdownPlan(*noBackup)
		downOut, _ := a.Compose.Down(context.Background())
		a.printf("[dry-run] %s\n", downOut)
		upOut, _ := a.Compose.Up(context.Background())
		a.printf("[dry-run] %s\n", upOut)
		return nil
	}

	a.saveBestEffort()
	if !*noBackup {
		if _, err := a.runBackup(0); err != nil {
			return fmt.Errorf("restart: backup before restart: %w", err)
		}
	}

	ctx := context.Background()
	downOut, err := a.Compose.Down(ctx)
	a.printf("%s\n", downOut)
	if err != nil {
		return fmt.Errorf("restart: docker compose down: %w", err)
	}

	upOut, err := a.Compose.Up(ctx)
	a.printf("%s\n", upOut)
	if err != nil {
		return fmt.Errorf("restart: docker compose up: %w", err)
	}
	return nil
}

// CmdSaveServer runs "save-all flush" over RCON. Unlike stop/restart's
// best-effort save, this command's whole point is the save, so an RCON
// failure is a real error.
func CmdSaveServer(a *App, args []string) error {
	fs := flag.NewFlagSet("save-server", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if a.DryRun {
		a.printf("[dry-run] rcon save-all flush @ %s\n", a.Config.RCONAddr())
		return nil
	}

	if _, err := a.rconRun("save-all flush"); err != nil {
		return fmt.Errorf("save-server: %w", err)
	}
	a.printf("save-all flush complete\n")
	return nil
}

// CmdBackupServer force-saves, disables autosave, flushes again for a
// consistent on-disk snapshot, archives Config.DataDir, re-enables autosave
// (even on archive failure), and prunes old archives beyond retention.
func CmdBackupServer(a *App, args []string) error {
	fs := flag.NewFlagSet("backup-server", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	retention := fs.Int("retention", 0, "backups to keep (default: BACKUP_RETENTION)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if a.DryRun {
		a.printf("[dry-run] rcon save-all flush @ %s\n", a.Config.RCONAddr())
		a.printf("[dry-run] rcon save-off @ %s\n", a.Config.RCONAddr())
		a.printf("[dry-run] rcon save-all flush @ %s\n", a.Config.RCONAddr())
		opts := a.backupOptions(*retention)
		a.printf("[dry-run] archive %s -> %s (label=%s, retention=%d)\n", opts.SourceDir, opts.DestDir, opts.Label, opts.Retention)
		a.printf("[dry-run] rcon save-on @ %s\n", a.Config.RCONAddr())
		return nil
	}

	client, dialErr := a.DialRCON(a.Config.RCONAddr(), a.Config.RCONPassword, a.Config.RCONTimeout)
	if dialErr != nil {
		a.warnf("warning: RCON unreachable, backing up without pausing autosave: %v\n", dialErr)
	} else {
		if _, err := client.Command("save-all flush"); err != nil {
			a.warnf("warning: save-all flush failed: %v\n", err)
		}
		if _, err := client.Command("save-off"); err != nil {
			a.warnf("warning: save-off failed: %v\n", err)
		}
		if _, err := client.Command("save-all flush"); err != nil {
			a.warnf("warning: save-all flush (post save-off) failed: %v\n", err)
		}
		defer func() {
			if _, err := client.Command("save-on"); err != nil {
				a.warnf("warning: save-on failed: %v\n", err)
			}
			client.Close()
		}()
	}

	if _, err := a.runBackup(*retention); err != nil {
		return fmt.Errorf("backup-server: %w", err)
	}
	return nil
}

func (a *App) printPreShutdownPlan(noBackup bool) {
	a.printf("[dry-run] rcon save-all flush @ %s (best-effort)\n", a.Config.RCONAddr())
	if noBackup {
		a.printf("[dry-run] skip backup (--no-backup)\n")
		return
	}
	opts := a.backupOptions(0)
	a.printf("[dry-run] archive %s -> %s (label=%s, retention=%d)\n", opts.SourceDir, opts.DestDir, opts.Label, opts.Retention)
}

package cli

import (
	"context"
	"fmt"
	"path/filepath"
)

// CmdPlugins routes legacy full sync/list commands to the shell helper and
// fixed Geyser bundle operations to the transactional Go updater.
func CmdPlugins(a *App, args []string) error {
	sub := "list"
	rest := args
	if len(args) > 0 {
		sub, rest = args[0], args[1:]
	}

	switch sub {
	case "sync":
		return a.runPluginScript(rest...)
	case "list":
		return a.runPluginScript(append([]string{"--list"}, rest...)...)
	case "check", "update", "rollback":
		return a.runPluginUpdate(sub, rest)
	default:
		return fmt.Errorf("plugins: unknown subcommand %q (want sync|list|check|update|rollback)", sub)
	}
}

func (a *App) runPluginScript(args ...string) error {
	script := filepath.Join(a.Config.ScriptDir, "plugins-sync.sh")
	if a.DryRun {
		a.printf("[dry-run] %s %v\n", script, args)
		return nil
	}
	out, err := a.Compose.Run(context.Background(), script, args...)
	if out != "" {
		a.printf("%s", out)
	}
	if err != nil {
		return fmt.Errorf("plugins: run %s: %w", script, err)
	}
	return nil
}

package cli

import (
	"context"
	"fmt"
	"path/filepath"
)

// CmdPlugins keeps plugin management reachable from the Go CLI while the
// resolver/installer remains the battle-tested shell helper. The Go binary is
// the user-facing root; the script is the bounded system integration point.
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
	default:
		return fmt.Errorf("plugins: unknown subcommand %q (want sync|list)", sub)
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

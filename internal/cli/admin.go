package cli

import (
	"fmt"
	"path/filepath"

	"github.com/onecode182/nethernode/internal/opsjson"
)

// opsPath returns Config.DataDir/ops.json.
func (a *App) opsPath() string {
	return filepath.Join(a.Config.DataDir, opsjson.FileName)
}

// CmdAdmin dispatches "admin list|add|remove" to its subcommand.
func CmdAdmin(a *App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("admin: missing subcommand (want list|add|remove)")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return cmdAdminList(a, rest)
	case "add":
		return cmdAdminAdd(a, rest)
	case "remove":
		return cmdAdminRemove(a, rest)
	default:
		return fmt.Errorf("admin: unknown subcommand %q (want list|add|remove)", sub)
	}
}

// cmdAdminList prints every ops.json entry (name, uuid, level,
// bypassesPlayerLimit). It never touches RCON: ops.json is the source of
// truth for "who is currently an op", regardless of whether the server is
// running.
func cmdAdminList(a *App, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("admin list: unexpected argument(s): %v", args)
	}

	path := a.opsPath()
	if a.DryRun {
		a.printf("[dry-run] read %s\n", path)
		return nil
	}

	entries, err := opsjson.Read(path)
	if err != nil {
		return fmt.Errorf("admin list: %w", err)
	}
	if len(entries) == 0 {
		a.printf("no ops configured in %s\n", path)
		return nil
	}
	for _, e := range entries {
		a.printf("%s\tuuid=%s\tlevel=%d\tbypassesPlayerLimit=%v\n", e.Name, e.UUID, e.Level, e.BypassesPlayerLimit)
	}
	return nil
}

// cmdAdminAdd runs RCON `op <player>`, which grants the server's default
// op-permission-level (opsjson.DefaultLevel, normally 4) and is what
// actually lets the player use op commands right away. When --level asks
// for a different level, RCON has no command for that (op-permission-level
// is a server-wide setting, not a per-player argument), so nethernode
// hand-patches ops.json's level field for that player; the running server
// only picks that change up on its own restart or a /reload.
func cmdAdminAdd(a *App, args []string) error {
	level, rest, err := extractIntFlag(args, "level", opsjson.DefaultLevel)
	if err != nil {
		return fmt.Errorf("admin add: %w", err)
	}
	if level < 1 || level > 4 {
		return fmt.Errorf("admin add: --level must be between 1 and 4, got %d", level)
	}
	if len(rest) != 1 {
		return fmt.Errorf("admin add: expected exactly one <player> argument, got %d: %v", len(rest), rest)
	}
	player := rest[0]
	path := a.opsPath()

	if a.DryRun {
		a.printf("[dry-run] rcon op %s @ %s\n", player, a.Config.RCONAddr())
		if level != opsjson.DefaultLevel {
			a.printf("[dry-run] patch %s: set %s level=%d (server restart or /reload needed to apply)\n", path, player, level)
		}
		return nil
	}

	if _, err := a.rconRun("op " + player); err != nil {
		if patchErr := patchOpLevel(path, player, level); patchErr != nil {
			return fmt.Errorf("admin add: rcon failed (%w); offline ops.json patch also failed: %v", err, patchErr)
		}
		a.printf("op %s: rcon unavailable; patched %s level=%d (server restart needed to apply)\n", player, path, level)
		return nil
	}
	a.printf("op %s: rcon op complete\n", player)

	if level == opsjson.DefaultLevel {
		return nil
	}

	if err := patchOpLevel(path, player, level); err != nil {
		return fmt.Errorf("admin add: %w", err)
	}
	a.printf("op %s: patched level=%d in %s (server restart or /reload needed to apply)\n", player, level, path)
	return nil
}

// cmdAdminRemove runs RCON `deop <player>` and then atomically removes the
// entry from ops.json. If RCON is offline, nethernode still removes the file
// entry so the deop takes effect on the next start.
func cmdAdminRemove(a *App, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("admin remove: expected exactly one <player> argument, got %d: %v", len(args), args)
	}
	player := args[0]
	path := a.opsPath()

	if a.DryRun {
		a.printf("[dry-run] rcon deop %s @ %s\n", player, a.Config.RCONAddr())
		a.printf("[dry-run] remove %s from %s if present\n", player, path)
		return nil
	}

	rconOK := true
	if _, err := a.rconRun("deop " + player); err != nil {
		rconOK = false
		a.warnf("warning: admin remove rcon failed; patching ops.json offline: %v\n", err)
	}

	removed, err := removeOp(path, player)
	if err != nil {
		return fmt.Errorf("admin remove: patch %s: %w", path, err)
	}
	switch {
	case rconOK && removed:
		a.printf("deop %s: rcon deop complete; removed from %s\n", player, path)
	case rconOK:
		a.printf("deop %s: rcon deop complete; no %s entry found in %s\n", player, player, path)
	case removed:
		a.printf("deop %s: removed from %s (server restart needed to apply)\n", player, path)
	default:
		a.printf("deop %s: no %s entry found in %s (rcon unavailable)\n", player, player, path)
	}
	return nil
}

func patchOpLevel(path, player string, level int) error {
	entries, err := opsjson.Read(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	entries = opsjson.Upsert(entries, player, level)
	if err := opsjson.WriteAtomic(path, entries); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func removeOp(path, player string) (bool, error) {
	entries, err := opsjson.Read(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	entries, removed := opsjson.Remove(entries, player)
	if !removed {
		return false, nil
	}
	if err := opsjson.WriteAtomic(path, entries); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return removed, nil
}

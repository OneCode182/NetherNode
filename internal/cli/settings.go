package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/onecode182/nethernode/internal/serverprops"
)

// serverPropertiesPath returns Config.DataDir/server.properties.
func (a *App) serverPropertiesPath() string {
	return filepath.Join(a.Config.DataDir, "server.properties")
}

// settingsRCONCommand maps a server.properties key/value pair to the RCON
// command that applies it on a running server immediately, if one exists.
// Most keys have no live equivalent and only take effect on the next
// server start; callers must fall back to a "restart needed" note.
func settingsRCONCommand(key, value string) (string, bool) {
	key = canonicalSettingKey(key)
	switch key {
	case "difficulty":
		return "difficulty " + value, true
	case "white-list":
		switch value {
		case "true":
			return "whitelist on", true
		case "false":
			return "whitelist off", true
		default:
			return "", false
		}
	default:
		return "", false
	}
}

func canonicalSettingKey(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "whitelist":
		return "white-list"
	default:
		return strings.ToLower(strings.TrimSpace(key))
	}
}

// CmdSettings dispatches "settings get|set" to its subcommand.
func CmdSettings(a *App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("settings: missing subcommand (want get|set)")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "get":
		return cmdSettingsGet(a, rest)
	case "set":
		return cmdSettingsSet(a, rest)
	default:
		return fmt.Errorf("settings: unknown subcommand %q (want get|set)", sub)
	}
}

// cmdSettingsGet prints the value of a single server.properties key.
func cmdSettingsGet(a *App, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("settings get: expected exactly one <key> argument, got %d: %v", len(args), args)
	}
	key := canonicalSettingKey(args[0])
	path := a.serverPropertiesPath()

	if a.DryRun {
		a.printf("[dry-run] read %s key=%s\n", path, key)
		return nil
	}

	lines, err := serverprops.ReadFile(path)
	if err != nil {
		return fmt.Errorf("settings get: %w", err)
	}
	value, ok := serverprops.Get(lines, key)
	if !ok {
		return fmt.Errorf("settings get: key %q not set in %s", key, path)
	}
	a.printf("%s\n", value)
	return nil
}

// cmdSettingsSet atomically patches a single server.properties key,
// preserving every other line's order/comments exactly, appending the key
// at the end when it was not already present. With --apply, it also sends
// the matching RCON command when one exists (settingsRCONCommand), else it
// prints a note that a server restart is needed.
func cmdSettingsSet(a *App, args []string) error {
	apply, rest := extractBoolFlag(args, "apply")
	if len(rest) < 2 {
		return fmt.Errorf("settings set: expected <key> <value> arguments, got %d: %v", len(rest), rest)
	}
	key, value := canonicalSettingKey(rest[0]), strings.Join(rest[1:], " ")
	path := a.serverPropertiesPath()

	if a.DryRun {
		a.printf("[dry-run] patch %s: set %s=%s\n", path, key, value)
		if apply {
			if cmd, ok := settingsRCONCommand(key, value); ok {
				a.printf("[dry-run] rcon %s @ %s\n", cmd, a.Config.RCONAddr())
			} else {
				a.printf("[dry-run] no live rcon equivalent for %q; server restart needed to apply\n", key)
			}
		}
		return nil
	}

	lines, err := serverprops.ReadFile(path)
	if err != nil {
		return fmt.Errorf("settings set: %w", err)
	}
	lines = serverprops.Set(lines, key, value)
	if err := serverprops.WriteAtomicFile(path, lines); err != nil {
		return fmt.Errorf("settings set: %w", err)
	}
	a.printf("%s: set %s=%s\n", path, key, value)

	if !apply {
		return nil
	}
	cmd, ok := settingsRCONCommand(key, value)
	if !ok {
		a.printf("note: no live rcon equivalent for %q; server restart needed to apply\n", key)
		return nil
	}
	if _, err := a.rconRun(cmd); err != nil {
		return fmt.Errorf("settings set: apply via rcon: %w", err)
	}
	a.printf("applied via rcon: %s\n", cmd)
	return nil
}

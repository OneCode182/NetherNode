package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/onecode182/nethernode/internal/serverprops"
)

// CmdWorld dispatches "world list|get|set" to its subcommand.
func CmdWorld(a *App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("world: missing subcommand (want list|get|set)")
	}

	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return cmdWorldList(a, rest)
	case "get":
		return cmdWorldGet(a, rest)
	case "set":
		return cmdWorldSet(a, rest)
	default:
		return fmt.Errorf("world: unknown subcommand %q (want list|get|set)", sub)
	}
}

// cmdWorldList prints selectable world directory names in sorted order.
func cmdWorldList(a *App, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("world list: expected no arguments, got %d: %v", len(args), args)
	}

	if a.DryRun {
		a.printf("[dry-run] scan immediate world directories under %s\n", a.Config.DataDir)
		return nil
	}

	worlds, err := scanWorlds(a.Config.DataDir)
	if err != nil {
		return fmt.Errorf("world list: scan %s: %w", a.Config.DataDir, err)
	}
	for _, world := range worlds {
		a.printf("%s\n", world)
	}
	return nil
}

// scanWorlds returns direct, non-symlink child directories that contain a
// regular level.dat. Nether and End dimensions are hidden when their base
// world is also selectable.
func scanWorlds(dataDir string) ([]string, error) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, err
	}

	worlds := make([]string, 0, len(entries))
	for _, entry := range entries {
		worldPath := filepath.Join(dataDir, entry.Name())
		info, err := os.Lstat(worldPath)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			continue
		}

		levelInfo, err := os.Lstat(filepath.Join(worldPath, "level.dat"))
		if err != nil || levelInfo.Mode()&os.ModeSymlink != 0 || !levelInfo.Mode().IsRegular() {
			continue
		}
		worlds = append(worlds, entry.Name())
	}

	sort.Strings(worlds)
	selectable := make(map[string]struct{}, len(worlds))
	for _, world := range worlds {
		selectable[world] = struct{}{}
	}

	filtered := worlds[:0]
	for _, world := range worlds {
		base, dimension := dimensionBase(world)
		if dimension != "" {
			if _, ok := selectable[base]; ok {
				continue
			}
		}
		filtered = append(filtered, world)
	}
	return filtered, nil
}

func dimensionBase(name string) (base, dimension string) {
	for _, suffix := range []string{"_nether", "_the_end"} {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix), suffix
		}
	}
	return name, ""
}

// cmdWorldGet prints the one nonempty level-name setting from
// server.properties.
func cmdWorldGet(a *App, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("world get: expected no arguments, got %d: %v", len(args), args)
	}

	path := a.serverPropertiesPath()
	if a.DryRun {
		a.printf("[dry-run] read %s key=level-name\n", path)
		return nil
	}

	lines, err := serverprops.ReadFile(path)
	if err != nil {
		return fmt.Errorf("world get: %w", err)
	}
	values := nonemptyLevelNames(lines)
	if len(values) != 1 {
		return fmt.Errorf("world get: expected exactly one nonempty level-name in %s, found %d", path, len(values))
	}
	a.printf("%s\n", values[0])
	return nil
}

// cmdWorldSet atomically changes only level-name. --apply is deliberately
// handled before positional validation so it is accepted anywhere in args.
func cmdWorldSet(a *App, args []string) error {
	name, apply, err := parseWorldSetArgs(args)
	if err != nil {
		return err
	}

	path := a.serverPropertiesPath()
	if a.DryRun {
		a.printf("[dry-run] atomic patch %s: set level-name=%s\n", path, name)
		if apply {
			return CmdRestart(a, nil)
		}
		return nil
	}

	if err := requireSelectableWorld(a.Config.DataDir, name); err != nil {
		return fmt.Errorf("world set: %w", err)
	}

	lines, err := serverprops.ReadFile(path)
	if err != nil {
		return fmt.Errorf("world set: %w", err)
	}
	if countLevelNames(lines) > 1 {
		return fmt.Errorf("world set: duplicate level-name entries in %s", path)
	}
	lines = serverprops.Set(lines, "level-name", name)
	if err := serverprops.WriteAtomicFile(path, lines); err != nil {
		return fmt.Errorf("world set: %w", err)
	}
	a.printf("%s: set level-name=%s\n", path, name)

	if !apply {
		a.printf("note: server restart needed to apply level-name\n")
		return nil
	}
	if err := CmdRestart(a, nil); err != nil {
		return fmt.Errorf("world set apply restart: %w", err)
	}
	return nil
}

func parseWorldSetArgs(args []string) (string, bool, error) {
	apply := false
	positional := make([]string, 0, 1)
	for _, arg := range args {
		if arg == "--apply" {
			if apply {
				return "", false, fmt.Errorf("world set: duplicate --apply flag")
			}
			apply = true
			continue
		}
		if strings.HasPrefix(arg, "--") {
			return "", false, fmt.Errorf("world set: unknown flag %q (want --apply)", arg)
		}
		positional = append(positional, arg)
	}
	if len(positional) != 1 {
		return "", false, fmt.Errorf("world set: expected exactly one <name> argument plus optional --apply, got %d: %v", len(positional), positional)
	}
	if err := validateWorldName(positional[0]); err != nil {
		return "", false, fmt.Errorf("world set: %w", err)
	}
	return positional[0], apply, nil
}

func validateWorldName(name string) error {
	if name == "" {
		return fmt.Errorf("world name must be nonempty")
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("world name must not change after TrimSpace: %q", name)
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("invalid world name %q: absolute paths are not allowed", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid world name %q: dot paths are not allowed", name)
	}
	if strings.ContainsAny(name, `/\\`) {
		return fmt.Errorf("invalid world name %q: must be one path component", name)
	}
	for _, r := range name {
		if r == 0 || unicode.IsControl(r) {
			return fmt.Errorf("invalid world name %q: control characters are not allowed", name)
		}
	}
	return nil
}

func requireSelectableWorld(dataDir, name string) error {
	worlds, err := scanWorlds(dataDir)
	if err != nil {
		return fmt.Errorf("scan %s: %w", dataDir, err)
	}
	selectable := false
	for _, world := range worlds {
		if world == name {
			selectable = true
			break
		}
	}
	if !selectable {
		return fmt.Errorf("world %q is not selectable", name)
	}

	worldPath := filepath.Join(dataDir, name)
	info, err := os.Lstat(worldPath)
	if err != nil {
		return fmt.Errorf("selectable world %q: %w", filepath.Base(worldPath), err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("selectable world %q: path is not a real directory", filepath.Base(worldPath))
	}

	levelInfo, err := os.Lstat(filepath.Join(worldPath, "level.dat"))
	if err != nil {
		return fmt.Errorf("selectable world %q: level.dat: %w", filepath.Base(worldPath), err)
	}
	if levelInfo.Mode()&os.ModeSymlink != 0 || !levelInfo.Mode().IsRegular() {
		return fmt.Errorf("selectable world %q: level.dat is not a regular file", filepath.Base(worldPath))
	}
	return nil
}

func countLevelNames(lines []serverprops.Line) int {
	count := 0
	for _, line := range lines {
		if line.Key == "level-name" {
			count++
		}
	}
	return count
}

func nonemptyLevelNames(lines []serverprops.Line) []string {
	values := make([]string, 0, 1)
	for _, line := range lines {
		if line.Key != "level-name" {
			continue
		}
		raw := strings.TrimRight(line.Raw, "\r")
		_, value, ok := strings.Cut(raw, "=")
		if ok {
			value = strings.TrimSpace(value)
			if value != "" {
				values = append(values, value)
			}
		}
	}
	return values
}

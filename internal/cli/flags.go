package cli

import (
	"fmt"
	"strconv"
	"strings"
)

// extractBoolFlag scans args for "--name" anywhere in the list (like
// extractDryRun, and unlike the stdlib flag package, which stops parsing
// flags at the first positional argument), removes it, and reports whether
// it was present. It exists because admin/settings subcommands mix a
// leading positional argument (player name, key) with a trailing flag
// (--level, --apply), a shape flag.FlagSet cannot parse directly.
func extractBoolFlag(args []string, name string) (bool, []string) {
	found := false
	rest := make([]string, 0, len(args))
	needle := "--" + name
	for _, a := range args {
		if a == needle {
			found = true
			continue
		}
		rest = append(rest, a)
	}
	return found, rest
}

// extractIntFlag scans args for "--name value" or "--name=value" anywhere
// in the list, removes it, and returns its parsed value (or def if absent).
func extractIntFlag(args []string, name string, def int) (int, []string, error) {
	val := def
	rest := make([]string, 0, len(args))
	prefix := "--" + name + "="
	needle := "--" + name

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == needle:
			if i+1 >= len(args) {
				return 0, nil, fmt.Errorf("flag needs an argument: %s", needle)
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return 0, nil, fmt.Errorf("invalid value %q for %s: %w", args[i+1], needle, err)
			}
			val = n
			i++
		case strings.HasPrefix(a, prefix):
			raw := strings.TrimPrefix(a, prefix)
			n, err := strconv.Atoi(raw)
			if err != nil {
				return 0, nil, fmt.Errorf("invalid value %q for %s: %w", raw, needle, err)
			}
			val = n
		default:
			rest = append(rest, a)
		}
	}
	return val, rest, nil
}

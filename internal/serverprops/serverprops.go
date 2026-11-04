// Package serverprops reads and atomically writes a Minecraft
// server.properties file, preserving comments, blank lines, and key order
// exactly, changing (or appending) only the one key a caller sets.
//
// It implements enough of the Java Properties format for server.properties
// in practice: "key=value" lines, "#"-prefixed comments, and blank lines.
// It does not implement escape sequences or line continuations, which
// server.properties does not use.
package serverprops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Line is one line of a server.properties file. Key is non-empty only for
// a "key=value" setting line; comments and blank lines carry an empty Key
// and are rendered back out via Raw exactly as read.
type Line struct {
	Raw string
	Key string
}

// Parse splits body into Lines, one per input line (split on "\n"; any
// trailing "\r" is kept as part of Raw so CRLF files round-trip exactly).
// Render(Parse(body)) reproduces body byte-for-byte when no Set call is
// applied in between.
func Parse(body []byte) []Line {
	raw := strings.Split(string(body), "\n")
	lines := make([]Line, len(raw))
	for i, r := range raw {
		key, _, ok := splitKV(r)
		if ok {
			lines[i] = Line{Raw: r, Key: key}
		} else {
			lines[i] = Line{Raw: r}
		}
	}
	return lines
}

// splitKV extracts (key, value, ok) from a raw properties line. ok is
// false for comments ("#" prefix, after trimming) and blank lines.
func splitKV(raw string) (key, value string, ok bool) {
	trimmed := strings.TrimRight(raw, "\r")
	t := strings.TrimSpace(trimmed)
	if t == "" || strings.HasPrefix(t, "#") {
		return "", "", false
	}
	k, v, found := strings.Cut(trimmed, "=")
	if !found {
		return "", "", false
	}
	return strings.TrimSpace(k), strings.TrimSpace(v), true
}

// Get returns the value of key and whether it was present.
func Get(lines []Line, key string) (string, bool) {
	for _, l := range lines {
		if l.Key == key {
			_, v, _ := splitKV(l.Raw)
			return v, true
		}
	}
	return "", false
}

// Set returns a new slice with key's line rewritten to "key=value" in
// place (preserving every other line's text and the overall order
// exactly), or, if key is not already present, appended as a new
// "key=value" line at the end. If the last line is the empty "phantom"
// entry produced by the source file's own trailing newline, the new line
// is inserted before it, so appending a brand-new key never introduces a
// stray blank line ahead of it or drops the file's trailing newline.
func Set(lines []Line, key, value string) []Line {
	out := make([]Line, len(lines))
	copy(out, lines)
	newLine := Line{Raw: key + "=" + value, Key: key}

	for i, l := range out {
		if l.Key == key {
			out[i] = newLine
			return out
		}
	}

	if n := len(out); n > 0 && out[n-1].Raw == "" && out[n-1].Key == "" {
		result := make([]Line, 0, n+1)
		result = append(result, out[:n-1]...)
		result = append(result, newLine, out[n-1])
		return result
	}
	return append(out, newLine)
}

// Render joins lines back into a byte slice, one line per "\n".
func Render(lines []Line) []byte {
	raws := make([]string, len(lines))
	for i, l := range lines {
		raws[i] = l.Raw
	}
	return []byte(strings.Join(raws, "\n"))
}

// ReadFile reads and parses path.
func ReadFile(path string) ([]Line, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("serverprops: read %s: %w", path, err)
	}
	return Parse(body), nil
}

// WriteAtomicFile renders lines and writes them to path via a temp file
// plus rename, so a crash or concurrent read mid-write never observes a
// truncated/corrupt server.properties. The written file's permission bits
// mirror the existing file's, if any, defaulting to 0644 for a new file.
func WriteAtomicFile(path string, lines []Line) error {
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("serverprops: create dir %s: %w", dir, err)
	}

	tmp := filepath.Join(dir, "."+filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmp, Render(lines), mode); err != nil {
		return fmt.Errorf("serverprops: write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("serverprops: rename into place: %w", err)
	}
	return nil
}

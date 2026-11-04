// Package opsjson reads and atomically writes a Paper/vanilla Minecraft
// server's ops.json (the file the running server's own console /op and
// /deop commands maintain), and computes the deterministic "offline
// player" UUID nethernode needs when hand-creating a new entry.
//
// This repo runs with online-mode=false (see
// .agents/tasks/active/nethernode-v2-paper-crossplay-go-cli.task.md), so
// every player UUID is the offline-mode one vanilla/Paper itself would
// assign: UUID.nameUUIDFromBytes(("OfflinePlayer:" + name).getBytes(UTF-8)),
// a name-based (version 3) UUID derived from an MD5 hash. See OfflineUUID.
package opsjson

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileName is the ops.json file name inside the Minecraft data directory.
const FileName = "ops.json"

// DefaultLevel is the permission level a vanilla/Paper server's own console
// `op <player>` command grants (its op-permission-level default). Only a
// request for a different level needs nethernode to hand-patch ops.json.
const DefaultLevel = 4

// Entry is one ops.json record.
type Entry struct {
	UUID                string `json:"uuid"`
	Name                string `json:"name"`
	Level               int    `json:"level"`
	BypassesPlayerLimit bool   `json:"bypassesPlayerLimit"`
}

// Read loads entries from path. A missing or empty file is not an error:
// it reads as an empty ops list, matching a fresh server with no ops yet.
func Read(path string) ([]Entry, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("opsjson: read %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil, nil
	}
	var entries []Entry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("opsjson: parse %s: %w", path, err)
	}
	return entries, nil
}

// Find returns the index of the entry matching name (case-insensitive,
// matching Mojang usernames being unique case-insensitively), or -1.
func Find(entries []Entry, name string) int {
	for i, e := range entries {
		if strings.EqualFold(e.Name, name) {
			return i
		}
	}
	return -1
}

// Upsert sets the Level for name, creating a new entry (with
// OfflineUUID(name) and BypassesPlayerLimit=false) if none exists yet, or
// updating the Level in place (keeping the existing UUID/
// BypassesPlayerLimit) otherwise. It returns the updated slice.
func Upsert(entries []Entry, name string, level int) []Entry {
	if i := Find(entries, name); i >= 0 {
		entries[i].Level = level
		return entries
	}
	return append(entries, Entry{
		UUID:  OfflineUUID(name),
		Name:  name,
		Level: level,
	})
}

// Remove deletes the entry matching name, if present, returning the
// updated slice and whether an entry was actually removed.
func Remove(entries []Entry, name string) ([]Entry, bool) {
	i := Find(entries, name)
	if i < 0 {
		return entries, false
	}
	return append(entries[:i], entries[i+1:]...), true
}

// WriteAtomic marshals entries as indented JSON (matching vanilla/Paper's
// own ops.json formatting) and writes them to path via a temp file plus
// rename, so a crash or concurrent read mid-write never observes a
// truncated/corrupt ops.json.
func WriteAtomic(path string, entries []Entry) error {
	if entries == nil {
		entries = []Entry{}
	}
	body, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("opsjson: encode: %w", err)
	}
	body = append(body, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("opsjson: create dir %s: %w", dir, err)
	}

	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}

	tmp := filepath.Join(dir, "."+filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmp, body, mode); err != nil {
		return fmt.Errorf("opsjson: write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("opsjson: rename into place: %w", err)
	}
	return nil
}

// OfflineUUID computes the deterministic UUID a vanilla/Paper server
// assigns to name when online-mode=false:
// UUID.nameUUIDFromBytes(("OfflinePlayer:" + name).getBytes(UTF-8)), i.e. an
// MD5-derived, name-based (RFC 4122 version 3) UUID.
func OfflineUUID(name string) string {
	sum := md5.Sum([]byte("OfflinePlayer:" + name))
	sum[6] = (sum[6] & 0x0f) | 0x30 // version 3 (name-based, MD5)
	sum[8] = (sum[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

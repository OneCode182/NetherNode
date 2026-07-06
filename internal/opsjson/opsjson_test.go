package opsjson

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOfflineUUID(t *testing.T) {
	// Cross-checked against Java's
	// UUID.nameUUIDFromBytes(("OfflinePlayer:" + name).getBytes(UTF_8)),
	// the exact algorithm vanilla/Paper use when online-mode=false.
	cases := map[string]string{
		"Notch": "b50ad385-829d-3141-a216-7e7d7539ba7f",
		"Steve": "5627dd98-e6be-3c21-b8a8-e92344183641",
		"Alex":  "36532b5e-c442-3dbb-a24c-c7e55d0f979a",
	}
	for name, want := range cases {
		if got := OfflineUUID(name); got != want {
			t.Errorf("OfflineUUID(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestOfflineUUID_Deterministic(t *testing.T) {
	if OfflineUUID("Steve") != OfflineUUID("Steve") {
		t.Fatal("OfflineUUID must be deterministic for the same name")
	}
	if OfflineUUID("Steve") == OfflineUUID("Alex") {
		t.Fatal("OfflineUUID must differ for different names")
	}
}

func TestRead_MissingFileIsEmptyNotError(t *testing.T) {
	dir := t.TempDir()
	entries, err := Read(filepath.Join(dir, "ops.json"))
	if err != nil {
		t.Fatalf("Read() error = %v, want nil for a missing file", err)
	}
	if len(entries) != 0 {
		t.Fatalf("Read() = %v, want empty", entries)
	}
}

func TestRead_EmptyFileIsEmptyNotError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ops.json")
	if err := os.WriteFile(path, []byte("  \n"), 0o644); err != nil {
		t.Fatalf("seed empty file: %v", err)
	}
	entries, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error = %v, want nil for a blank file", err)
	}
	if len(entries) != 0 {
		t.Fatalf("Read() = %v, want empty", entries)
	}
}

func TestRead_MalformedFileErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ops.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("seed malformed file: %v", err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("Read() error = nil, want an error for malformed JSON")
	}
}

func TestWriteAtomic_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ops.json")

	entries := []Entry{
		{UUID: OfflineUUID("Steve"), Name: "Steve", Level: 4, BypassesPlayerLimit: false},
	}
	if err := WriteAtomic(path, entries); err != nil {
		t.Fatalf("WriteAtomic() error = %v", err)
	}

	// No leftover temp file.
	if _, err := os.Stat(filepath.Join(dir, ".ops.json.tmp")); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind: err = %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read() after WriteAtomic() error = %v", err)
	}
	if len(got) != 1 || got[0] != entries[0] {
		t.Fatalf("Read() = %+v, want %+v", got, entries)
	}
}

func TestWriteAtomic_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "ops.json")
	if err := WriteAtomic(path, []Entry{{Name: "Steve"}}); err != nil {
		t.Fatalf("WriteAtomic() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func TestWriteAtomic_NilEntriesWritesEmptyArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ops.json")
	if err := WriteAtomic(path, nil); err != nil {
		t.Fatalf("WriteAtomic() error = %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != "[]\n" {
		t.Fatalf("body = %q, want %q", string(body), "[]\n")
	}
}

func TestFindUpsertRemove(t *testing.T) {
	var entries []Entry

	entries = Upsert(entries, "Steve", DefaultLevel)
	if len(entries) != 1 || entries[0].Name != "Steve" || entries[0].Level != DefaultLevel {
		t.Fatalf("Upsert(new) = %+v", entries)
	}
	if entries[0].UUID != OfflineUUID("Steve") {
		t.Fatalf("Upsert(new) UUID = %q, want %q", entries[0].UUID, OfflineUUID("Steve"))
	}

	entries = Upsert(entries, "steve", 1) // case-insensitive match
	if len(entries) != 1 {
		t.Fatalf("Upsert(existing, case-insensitive) grew slice: %+v", entries)
	}
	if entries[0].Level != 1 {
		t.Fatalf("Upsert(existing) level = %d, want 1", entries[0].Level)
	}
	if entries[0].Name != "Steve" {
		t.Fatalf("Upsert(existing) must not rewrite Name, got %q", entries[0].Name)
	}

	entries = Upsert(entries, "Alex", DefaultLevel)
	if len(entries) != 2 {
		t.Fatalf("Upsert(second new) = %+v, want 2 entries", entries)
	}

	if i := Find(entries, "ALEX"); i != 1 {
		t.Fatalf("Find(ALEX) = %d, want 1", i)
	}
	if i := Find(entries, "Herobrine"); i != -1 {
		t.Fatalf("Find(missing) = %d, want -1", i)
	}

	entries, removed := Remove(entries, "Steve")
	if !removed {
		t.Fatal("Remove(Steve) removed = false, want true")
	}
	if len(entries) != 1 || entries[0].Name != "Alex" {
		t.Fatalf("Remove(Steve) left = %+v, want only Alex", entries)
	}

	entries, removed = Remove(entries, "Herobrine")
	if removed {
		t.Fatal("Remove(missing) removed = true, want false")
	}
	if len(entries) != 1 {
		t.Fatalf("Remove(missing) must not change length, got %+v", entries)
	}
}

package serverprops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `#Minecraft server properties
#Sun Jul 06 00:00:00 UTC 2026
difficulty=easy
max-players=20
motd=A Minecraft Server

online-mode=false
white-list=false
`

func TestParseRender_RoundTrip(t *testing.T) {
	lines := Parse([]byte(sample))
	if got := string(Render(lines)); got != sample {
		t.Fatalf("Render(Parse(sample)) =\n%q\nwant\n%q", got, sample)
	}
}

func TestParseRender_RoundTrip_NoTrailingNewline(t *testing.T) {
	body := "difficulty=easy\nmax-players=20"
	lines := Parse([]byte(body))
	if got := string(Render(lines)); got != body {
		t.Fatalf("Render(Parse(body)) = %q, want %q", got, body)
	}
}

func TestGet(t *testing.T) {
	lines := Parse([]byte(sample))

	cases := []struct {
		key       string
		wantValue string
		wantOK    bool
	}{
		{"difficulty", "easy", true},
		{"max-players", "20", true},
		{"motd", "A Minecraft Server", true},
		{"online-mode", "false", true},
		{"nonexistent-key", "", false},
	}
	for _, tc := range cases {
		v, ok := Get(lines, tc.key)
		if ok != tc.wantOK || v != tc.wantValue {
			t.Errorf("Get(%q) = (%q, %v), want (%q, %v)", tc.key, v, ok, tc.wantValue, tc.wantOK)
		}
	}
}

func TestGet_IgnoresCommentsAndBlankLines(t *testing.T) {
	lines := Parse([]byte(sample))
	if _, ok := Get(lines, "Minecraft server properties"); ok {
		t.Fatal("Get() must not treat a comment line as a key")
	}
}

func TestSet_ExistingKeyPreservesOrderAndComments(t *testing.T) {
	lines := Parse([]byte(sample))
	lines = Set(lines, "difficulty", "hard")

	got := string(Render(lines))
	want := `#Minecraft server properties
#Sun Jul 06 00:00:00 UTC 2026
difficulty=hard
max-players=20
motd=A Minecraft Server

online-mode=false
white-list=false
`
	if got != want {
		t.Fatalf("Render() after Set() =\n%q\nwant\n%q", got, want)
	}
}

func TestSet_NewKeyAppendsAtEnd(t *testing.T) {
	lines := Parse([]byte(sample))
	lines = Set(lines, "pvp", "false")

	v, ok := Get(lines, "pvp")
	if !ok || v != "false" {
		t.Fatalf("Get(pvp) after Set() = (%q, %v), want (\"false\", true)", v, ok)
	}
	rendered := string(Render(lines))
	const wantTail = "white-list=false\npvp=false\n"
	if !strings.HasSuffix(rendered, wantTail) {
		t.Fatalf("Set() of a new key must append right before the trailing newline, got tail %q, want suffix %q", rendered[max(0, len(rendered)-40):], wantTail)
	}
	// Every prior line must still be present, untouched.
	if v, _ := Get(lines, "difficulty"); v != "easy" {
		t.Fatalf("Set(pvp) must not disturb difficulty, got %q", v)
	}
}

func TestSet_NewKeyOnFileWithNoTrailingNewline(t *testing.T) {
	lines := Parse([]byte("difficulty=easy"))
	lines = Set(lines, "pvp", "false")

	want := "difficulty=easy\npvp=false"
	if got := string(Render(lines)); got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
}

func TestSet_DoesNotMutateInputSlice(t *testing.T) {
	original := Parse([]byte(sample))
	snapshot := make([]Line, len(original))
	copy(snapshot, original)

	_ = Set(original, "difficulty", "hard")

	for i := range original {
		if original[i] != snapshot[i] {
			t.Fatalf("Set() mutated its input slice at index %d: %+v vs %+v", i, original[i], snapshot[i])
		}
	}
}

func TestReadFile_WriteAtomicFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.properties")
	if err := os.WriteFile(path, []byte(sample), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	lines, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines = Set(lines, "difficulty", "peaceful")
	lines = Set(lines, "level-seed", "12345")

	if err := WriteAtomicFile(path, lines); err != nil {
		t.Fatalf("WriteAtomicFile() error = %v", err)
	}

	// No leftover temp file.
	if _, err := os.Stat(filepath.Join(dir, ".server.properties.tmp")); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind: err = %v", err)
	}

	reloaded, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() after write error = %v", err)
	}
	if v, _ := Get(reloaded, "difficulty"); v != "peaceful" {
		t.Fatalf("difficulty after round-trip = %q, want peaceful", v)
	}
	if v, _ := Get(reloaded, "level-seed"); v != "12345" {
		t.Fatalf("level-seed after round-trip = %q, want 12345", v)
	}
	if v, _ := Get(reloaded, "max-players"); v != "20" {
		t.Fatalf("max-players after round-trip = %q, want unchanged 20", v)
	}
}

func TestWriteAtomicFile_PreservesFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.properties")
	if err := os.WriteFile(path, []byte(sample), 0o640); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	lines, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines = Set(lines, "difficulty", "hard")
	if err := WriteAtomicFile(path, lines); err != nil {
		t.Fatalf("WriteAtomicFile() error = %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode after WriteAtomicFile() = %v, want 0640", fi.Mode().Perm())
	}
}

func TestWriteAtomicFile_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "server.properties")
	if err := WriteAtomicFile(path, Parse([]byte("difficulty=hard\n"))); err != nil {
		t.Fatalf("WriteAtomicFile() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}

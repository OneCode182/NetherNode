package backup

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

// writeFixtureTree creates a small world-like directory tree under root and
// returns a map of slash-separated relative paths to their file contents.
func writeFixtureTree(t *testing.T, root string) map[string]string {
	t.Helper()

	files := map[string]string{
		"world/level.dat":        "level-data",
		"world/region/r.0.0.mca": "region-data",
		"ops.json":               "[]",
		"server.properties":      "difficulty=hard\n",
		"logs/latest.log":        "starting server\n",
	}

	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir fixture dir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture file: %v", err)
		}
	}

	return files
}

// readArchive opens a gzip tar archive and returns a map of entry name to
// content for regular files only.
func readArchive(t *testing.T, path string) map[string]string {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	got := map[string]string{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("tar read content for %s: %v", header.Name, err)
		}
		got[header.Name] = string(data)
	}
	return got
}

func TestRunValidatesOptions(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	destDir := filepath.Join(tmp, "dest")

	cases := []struct {
		name string
		opts Options
	}{
		{"missing source dir", Options{DestDir: destDir, Label: "mc", Retention: 5}},
		{"missing dest dir", Options{SourceDir: sourceDir, Label: "mc", Retention: 5}},
		{"missing label", Options{SourceDir: sourceDir, DestDir: destDir, Retention: 5}},
		{"zero retention", Options{SourceDir: sourceDir, DestDir: destDir, Label: "mc", Retention: 0}},
		{"negative retention", Options{SourceDir: sourceDir, DestDir: destDir, Label: "mc", Retention: -1}},
		{"nonexistent source dir", Options{SourceDir: filepath.Join(tmp, "missing"), DestDir: destDir, Label: "mc", Retention: 5}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Run(tc.opts, time.Now()); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestRunSourceMustBeDirectory(t *testing.T) {
	tmp := t.TempDir()
	sourceFile := filepath.Join(tmp, "source-file")
	if err := os.WriteFile(sourceFile, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	destDir := filepath.Join(tmp, "dest")

	_, err := Run(Options{SourceDir: sourceFile, DestDir: destDir, Label: "mc", Retention: 5}, time.Now())
	if err == nil {
		t.Fatalf("expected error for non-directory source, got nil")
	}
}

func TestRunCreatesReadableArchive(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "source")
	destDir := filepath.Join(tmp, "dest")
	want := writeFixtureTree(t, sourceDir)

	now := time.Date(2026, 7, 6, 12, 30, 45, 0, time.UTC)

	result, err := Run(Options{
		SourceDir: sourceDir,
		DestDir:   destDir,
		Label:     "minecraft",
		Retention: 5,
	}, now)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	wantName := "minecraft-20260706T123045Z.tar.gz"
	if filepath.Base(result.ArchivePath) != wantName {
		t.Fatalf("archive name = %q, want %q", filepath.Base(result.ArchivePath), wantName)
	}
	if filepath.Dir(result.ArchivePath) != destDir {
		t.Fatalf("archive dir = %q, want %q", filepath.Dir(result.ArchivePath), destDir)
	}

	nameRe := regexp.MustCompile(`^minecraft-\d{8}T\d{6}Z\.tar\.gz$`)
	if !nameRe.MatchString(filepath.Base(result.ArchivePath)) {
		t.Fatalf("archive name %q does not match expected pattern", filepath.Base(result.ArchivePath))
	}

	info, err := os.Stat(result.ArchivePath)
	if err != nil {
		t.Fatalf("stat archive: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("archive is empty")
	}
	if result.Size != info.Size() {
		t.Fatalf("result.Size = %d, want %d (actual file size)", result.Size, info.Size())
	}

	// No temp file should linger.
	tmpPath := filepath.Join(destDir, "."+wantName+".tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatalf("temp archive file still present: %s", tmpPath)
	}

	if len(result.Removed) != 0 {
		t.Fatalf("expected no removals on first backup, got %v", result.Removed)
	}

	got := readArchive(t, result.ArchivePath)
	if len(got) != len(want) {
		t.Fatalf("archive has %d entries, want %d (%v)", len(got), len(want), got)
	}
	for rel, content := range want {
		gotContent, ok := got[rel]
		if !ok {
			t.Fatalf("archive missing entry %q; got entries: %v", rel, got)
		}
		if gotContent != content {
			t.Fatalf("archive entry %q content = %q, want %q", rel, gotContent, content)
		}
	}
}

func TestRunPrunesOldBackups(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "source")
	destDir := filepath.Join(tmp, "dest")
	writeFixtureTree(t, sourceDir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	label := "minecraft"
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	// Pre-create 4 older archives with distinct mtimes, oldest first.
	oldNames := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		ts := base.Add(time.Duration(i) * time.Hour)
		name := label + "-" + ts.Format(timeLayout) + ".tar.gz"
		path := filepath.Join(destDir, name)
		if err := os.WriteFile(path, []byte("old-archive"), 0o644); err != nil {
			t.Fatalf("write old archive %s: %v", name, err)
		}
		if err := os.Chtimes(path, ts, ts); err != nil {
			t.Fatalf("chtimes %s: %v", name, err)
		}
		oldNames = append(oldNames, path)
	}

	now := base.Add(10 * time.Hour) // newer than all pre-created archives.
	result, err := Run(Options{
		SourceDir: sourceDir,
		DestDir:   destDir,
		Label:     label,
		Retention: 2,
	}, now)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// With retention 2 and 5 total archives (4 old + 1 new), 3 should be removed:
	// the oldest 3 of the pre-created ones. Newest kept = new archive + oldNames[3].
	if len(result.Removed) != 3 {
		t.Fatalf("removed count = %d, want 3 (%v)", len(result.Removed), result.Removed)
	}
	wantRemoved := map[string]bool{
		oldNames[0]: true,
		oldNames[1]: true,
		oldNames[2]: true,
	}
	for _, r := range result.Removed {
		if !wantRemoved[r] {
			t.Fatalf("unexpected removal: %s", r)
		}
	}

	if _, err := os.Stat(result.ArchivePath); err != nil {
		t.Fatalf("new archive should still exist: %v", err)
	}
	if _, err := os.Stat(oldNames[3]); err != nil {
		t.Fatalf("newest old archive should be kept: %v", err)
	}
	for _, removedPath := range result.Removed {
		if _, err := os.Stat(removedPath); !os.IsNotExist(err) {
			t.Fatalf("removed archive still present: %s", removedPath)
		}
	}
}

func TestPruneKeepsNewestByModTime(t *testing.T) {
	tmp := t.TempDir()
	label := "minecraft"
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	type entry struct {
		name    string
		modTime time.Time
	}

	// Deliberately create files out of chronological order to ensure Prune
	// sorts by mtime rather than name or creation order.
	entries := []entry{
		{name: label + "-c.tar.gz", modTime: base.Add(2 * time.Hour)},
		{name: label + "-a.tar.gz", modTime: base},
		{name: label + "-e.tar.gz", modTime: base.Add(4 * time.Hour)},
		{name: label + "-b.tar.gz", modTime: base.Add(1 * time.Hour)},
		{name: label + "-d.tar.gz", modTime: base.Add(3 * time.Hour)},
		{name: "other-label-x.tar.gz", modTime: base.Add(5 * time.Hour)},
	}

	for _, e := range entries {
		path := filepath.Join(tmp, e.name)
		if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			t.Fatalf("write %s: %v", e.name, err)
		}
		if err := os.Chtimes(path, e.modTime, e.modTime); err != nil {
			t.Fatalf("chtimes %s: %v", e.name, err)
		}
	}

	removed, err := Prune(tmp, label, 2)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}

	wantRemoved := map[string]bool{
		filepath.Join(tmp, label+"-a.tar.gz"): true,
		filepath.Join(tmp, label+"-b.tar.gz"): true,
		filepath.Join(tmp, label+"-c.tar.gz"): true,
	}
	if len(removed) != len(wantRemoved) {
		t.Fatalf("removed = %v, want 3 entries matching %v", removed, wantRemoved)
	}
	for _, r := range removed {
		if !wantRemoved[r] {
			t.Fatalf("unexpected removal: %s", r)
		}
	}

	// Newest two of this label plus the other-label archive must remain.
	for _, keep := range []string{
		filepath.Join(tmp, label+"-d.tar.gz"),
		filepath.Join(tmp, label+"-e.tar.gz"),
		filepath.Join(tmp, "other-label-x.tar.gz"),
	} {
		if _, err := os.Stat(keep); err != nil {
			t.Fatalf("expected %s to remain: %v", keep, err)
		}
	}
}

func TestPruneRejectsNonPositiveRetention(t *testing.T) {
	tmp := t.TempDir()
	if _, err := Prune(tmp, "minecraft", 0); err == nil {
		t.Fatalf("expected error for zero retention")
	}
	if _, err := Prune(tmp, "minecraft", -3); err == nil {
		t.Fatalf("expected error for negative retention")
	}
}

func TestPruneNoRemovalWithinRetention(t *testing.T) {
	tmp := t.TempDir()
	label := "minecraft"
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		ts := base.Add(time.Duration(i) * time.Hour)
		name := label + "-" + ts.Format(timeLayout) + ".tar.gz"
		path := filepath.Join(tmp, name)
		if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if err := os.Chtimes(path, ts, ts); err != nil {
			t.Fatalf("chtimes %s: %v", name, err)
		}
	}

	removed, err := Prune(tmp, label, 5)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("expected no removals when under retention, got %v", removed)
	}
}

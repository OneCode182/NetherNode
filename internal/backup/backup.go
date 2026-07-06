// Package backup implements tar.gz archiving and retention pruning for
// NetherNode Minecraft world backups, mirroring the semantics of
// ops/backup-server.sh: archive the source directory tree into
// Label-YYYYMMDDTHHMMSSZ.tar.gz (UTC), write to a temp file and rename into
// place atomically, then prune older archives beyond the retention count.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// timeLayout matches the shell script's `date -u +%Y%m%dT%H%M%SZ` format.
const timeLayout = "20060102T150405Z"

// Options configures a single backup run.
type Options struct {
	SourceDir string
	DestDir   string
	Label     string
	Retention int
}

// Result reports the outcome of a backup run.
type Result struct {
	ArchivePath string
	Size        int64
	Removed     []string
}

// Run archives SourceDir into DestDir as a gzip-compressed tar named
// Label-YYYYMMDDTHHMMSSZ.tar.gz (timestamp from now, in UTC), then prunes
// old archives for Label beyond Retention. The archive is written to a
// temporary file first and moved into place with os.Rename for atomicity.
func Run(opts Options, now time.Time) (*Result, error) {
	if opts.SourceDir == "" {
		return nil, fmt.Errorf("backup: source dir is required")
	}
	if opts.DestDir == "" {
		return nil, fmt.Errorf("backup: dest dir is required")
	}
	if opts.Label == "" {
		return nil, fmt.Errorf("backup: label is required")
	}
	if opts.Retention < 1 {
		return nil, fmt.Errorf("backup: retention must be a positive integer, got %d", opts.Retention)
	}

	info, err := os.Stat(opts.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("backup: source dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("backup: source dir is not a directory: %s", opts.SourceDir)
	}

	if err := os.MkdirAll(opts.DestDir, 0o755); err != nil {
		return nil, fmt.Errorf("backup: create dest dir: %w", err)
	}

	timestamp := now.UTC().Format(timeLayout)
	archiveName := fmt.Sprintf("%s-%s.tar.gz", opts.Label, timestamp)
	archivePath := filepath.Join(opts.DestDir, archiveName)
	tmpPath := filepath.Join(opts.DestDir, "."+archiveName+".tmp")

	// Clear any leftover temp file from a previous failed run.
	_ = os.Remove(tmpPath)

	size, err := writeArchive(tmpPath, opts.SourceDir)
	if err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}

	if err := os.Rename(tmpPath, archivePath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("backup: rename archive into place: %w", err)
	}

	removed, err := Prune(opts.DestDir, opts.Label, opts.Retention)
	if err != nil {
		return nil, fmt.Errorf("backup: prune after archive: %w", err)
	}

	return &Result{
		ArchivePath: archivePath,
		Size:        size,
		Removed:     removed,
	}, nil
}

// writeArchive walks sourceDir and writes a gzip-compressed tar of its
// contents (relative to sourceDir, mirroring `tar -C sourceDir -czf dest .`)
// to dest. It returns the resulting file size.
func writeArchive(dest, sourceDir string) (int64, error) {
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("backup: create tmp archive: %w", err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	walkErr := filepath.Walk(sourceDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		var link string
		if fi.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(fi, link)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		if fi.IsDir() {
			header.Name += "/"
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if fi.Mode().IsRegular() {
			file, ferr := os.Open(path)
			if ferr != nil {
				return ferr
			}
			defer file.Close()
			if _, cerr := io.Copy(tw, file); cerr != nil {
				return cerr
			}
		}
		return nil
	})

	if walkErr != nil {
		_ = tw.Close()
		_ = gz.Close()
		return 0, fmt.Errorf("backup: build archive: %w", walkErr)
	}

	if err := tw.Close(); err != nil {
		_ = gz.Close()
		return 0, fmt.Errorf("backup: close tar writer: %w", err)
	}
	if err := gz.Close(); err != nil {
		return 0, fmt.Errorf("backup: close gzip writer: %w", err)
	}

	stat, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("backup: stat archive: %w", err)
	}

	return stat.Size(), nil
}

// Prune removes archives in destDir matching the "Label-*.tar.gz" pattern
// beyond the newest retention entries, ordered by modification time. It
// returns the paths removed.
func Prune(destDir, label string, retention int) ([]string, error) {
	if retention < 1 {
		return nil, fmt.Errorf("backup: retention must be a positive integer, got %d", retention)
	}

	pattern := fmt.Sprintf("%s-*.tar.gz", label)
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return nil, fmt.Errorf("backup: read dest dir: %w", err)
	}

	type candidate struct {
		path    string
		modTime time.Time
	}

	var candidates []candidate
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matched, err := filepath.Match(pattern, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("backup: match pattern: %w", err)
		}
		if !matched {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("backup: stat entry %s: %w", entry.Name(), err)
		}
		candidates = append(candidates, candidate{
			path:    filepath.Join(destDir, entry.Name()),
			modTime: info.ModTime(),
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	var removed []string
	for i := retention; i < len(candidates); i++ {
		if err := os.Remove(candidates[i].path); err != nil {
			return nil, fmt.Errorf("backup: remove old backup %s: %w", candidates[i].path, err)
		}
		removed = append(removed, candidates[i].path)
	}

	return removed, nil
}

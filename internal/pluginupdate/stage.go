package pluginupdate

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maxPluginYAMLBytes = 1 << 20

type stagedDownload struct {
	artifact Artifact
	tempPath string
}

// StageBundle downloads each resolved artifact into dir, verifies upstream
// checksums and plugin.yml identity, then atomically publishes the artifacts
// and manifest. dir must be a dedicated private staging directory.
func StageBundle(ctx context.Context, dir string, bundle Bundle, client *http.Client) (Bundle, error) {
	if ctx == nil {
		return Bundle{}, errors.New("plugin update: context is nil")
	}
	if client == nil {
		return Bundle{}, errors.New("plugin update: HTTP client is nil")
	}
	if err := validateBundle(bundle); err != nil {
		return Bundle{}, err
	}
	stageDir, err := ensureStageDir(dir)
	if err != nil {
		return Bundle{}, err
	}

	downloads := make([]stagedDownload, 0, len(bundle.Artifacts))
	defer func() {
		for _, download := range downloads {
			if download.tempPath != "" {
				_ = os.Remove(download.tempPath)
			}
		}
	}()
	for _, artifact := range bundle.Artifacts {
		if err := ctx.Err(); err != nil {
			return Bundle{}, fmt.Errorf("plugin update: staging canceled: %w", err)
		}
		tempPath, err := downloadArtifact(ctx, stageDir, artifact, client)
		if err != nil {
			return Bundle{}, err
		}
		downloads = append(downloads, stagedDownload{artifact: artifact, tempPath: tempPath})
	}

	for i := range downloads {
		if err := ctx.Err(); err != nil {
			return Bundle{}, fmt.Errorf("plugin update: staging canceled: %w", err)
		}
		download := &downloads[i]
		finalPath := filepath.Join(stageDir, download.artifact.Filename)
		if err := rejectExistingSymlink(finalPath); err != nil {
			return Bundle{}, err
		}
		if err := os.Rename(download.tempPath, finalPath); err != nil {
			return Bundle{}, fmt.Errorf("plugin update: publish staged %s: %w", download.artifact.Name, err)
		}
		download.tempPath = ""
	}
	if err := syncDirectory(stageDir); err != nil {
		return Bundle{}, err
	}
	if err := publishManifest(stageDir, bundle); err != nil {
		return Bundle{}, err
	}
	return ReadStaged(stageDir)
}

// ReadStaged loads bundle.json and verifies every staged JAR before returning
// metadata safe to pass to a later apply step.
func ReadStaged(dir string) (Bundle, error) {
	stageDir, err := existingStageDir(dir)
	if err != nil {
		return Bundle{}, err
	}
	manifestPath := filepath.Join(stageDir, manifestFilename)
	manifest, err := openRegularFile(manifestPath, maxMetadataBytes)
	if err != nil {
		return Bundle{}, fmt.Errorf("plugin update: read staged manifest: %w", err)
	}
	defer manifest.Close()
	body, err := io.ReadAll(io.LimitReader(manifest, maxMetadataBytes+1))
	if err != nil {
		return Bundle{}, fmt.Errorf("plugin update: read staged manifest: %w", err)
	}
	if len(body) == 0 || len(body) > maxMetadataBytes {
		return Bundle{}, errors.New("plugin update: staged manifest is empty or too large")
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	var bundle Bundle
	if err := decoder.Decode(&bundle); err != nil {
		return Bundle{}, fmt.Errorf("plugin update: decode staged manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Bundle{}, errors.New("plugin update: staged manifest has trailing data")
	}
	if err := validateBundle(bundle); err != nil {
		return Bundle{}, err
	}
	for _, artifact := range bundle.Artifacts {
		filePath := filepath.Join(stageDir, artifact.Filename)
		if err := verifyArtifactFile(filePath, artifact); err != nil {
			return Bundle{}, err
		}
	}
	return bundle, nil
}

func ensureStageDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", errors.New("plugin update: staging directory is empty")
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("plugin update: resolve staging directory: %w", err)
	}
	info, err := os.Lstat(absDir)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(absDir), 0755); err != nil {
			return "", fmt.Errorf("plugin update: create staging parent: %w", err)
		}
		if err := os.Mkdir(absDir, 0700); err != nil {
			return "", fmt.Errorf("plugin update: create staging directory: %w", err)
		}
		info, err = os.Lstat(absDir)
	}
	if err != nil {
		return "", fmt.Errorf("plugin update: inspect staging directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("plugin update: staging path must be a real directory")
	}
	if info.Mode().Perm()&0077 != 0 {
		return "", errors.New("plugin update: staging directory permissions must be private (0700)")
	}
	return absDir, nil
}

func existingStageDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", errors.New("plugin update: staging directory is empty")
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("plugin update: resolve staging directory: %w", err)
	}
	info, err := os.Lstat(absDir)
	if err != nil {
		return "", fmt.Errorf("plugin update: inspect staging directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return "", errors.New("plugin update: staging path must be a private real directory")
	}
	return absDir, nil
}

func downloadArtifact(ctx context.Context, stageDir string, artifact Artifact, client *http.Client) (string, error) {
	allowedHosts := []string{"download.geysermc.org"}
	if artifact.Name == ArtifactViaVersion || artifact.Name == ArtifactViaBackwards {
		allowedHosts = []string{"cdn.modrinth.com"}
	}
	response, err := openGET(ctx, client, artifact.URL, allowedHosts)
	if err != nil {
		return "", fmt.Errorf("plugin update: download %s: %w", artifact.Name, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("plugin update: download %s returned HTTP %d", artifact.Name, response.StatusCode)
	}

	temp, err := os.CreateTemp(stageDir, ".nethernode-"+string(artifact.Name)+"-*.tmp")
	if err != nil {
		return "", fmt.Errorf("plugin update: create staged %s temp file: %w", artifact.Name, err)
	}
	tempPath := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("plugin update: secure staged %s temp file: %w", artifact.Name, err)
	}

	hasher, expected, err := checksumFor(artifact)
	if err != nil {
		_ = temp.Close()
		return "", err
	}
	n, err := io.Copy(io.MultiWriter(temp, hasher), io.LimitReader(response.Body, maxArtifactBytes+1))
	if err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("plugin update: read %s download: %w", artifact.Name, err)
	}
	if n == 0 {
		_ = temp.Close()
		return "", fmt.Errorf("plugin update: %s download is empty", artifact.Name)
	}
	if n > maxArtifactBytes {
		_ = temp.Close()
		return "", fmt.Errorf("plugin update: %s download exceeds %d bytes", artifact.Name, maxArtifactBytes)
	}
	if subtle.ConstantTimeCompare(hasher.Sum(nil), expected) != 1 {
		_ = temp.Close()
		return "", fmt.Errorf("plugin update: %s checksum mismatch", artifact.Name)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("plugin update: sync staged %s: %w", artifact.Name, err)
	}
	if err := temp.Close(); err != nil {
		return "", fmt.Errorf("plugin update: close staged %s: %w", artifact.Name, err)
	}
	if err := verifyJarPath(tempPath, artifact); err != nil {
		return "", err
	}
	cleanup = false
	return tempPath, nil
}

func checksumFor(artifact Artifact) (hash.Hash, []byte, error) {
	var hasher hash.Hash
	switch artifact.HashAlgorithm {
	case "sha256":
		hasher = sha256.New()
	case "sha512":
		hasher = sha512.New()
	default:
		return nil, nil, fmt.Errorf("plugin update: unsupported checksum algorithm %q", artifact.HashAlgorithm)
	}
	expected, err := hex.DecodeString(artifact.Hash)
	if err != nil {
		return nil, nil, fmt.Errorf("plugin update: invalid %s checksum", artifact.Name)
	}
	return hasher, expected, nil
}

func verifyArtifactFile(filePath string, artifact Artifact) error {
	file, err := openRegularFile(filePath, maxArtifactBytes)
	if err != nil {
		return fmt.Errorf("plugin update: verify staged %s: %w", artifact.Name, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("plugin update: stat staged %s: %w", artifact.Name, err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("plugin update: staged %s is empty", artifact.Name)
	}
	if info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("plugin update: staged %s permissions must be private (0600)", artifact.Name)
	}
	hasher, expected, err := checksumFor(artifact)
	if err != nil {
		return err
	}
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("plugin update: hash staged %s: %w", artifact.Name, err)
	}
	if subtle.ConstantTimeCompare(hasher.Sum(nil), expected) != 1 {
		return fmt.Errorf("plugin update: staged %s checksum mismatch", artifact.Name)
	}
	if err := verifyJarReaderAt(file, info.Size(), artifact); err != nil {
		return err
	}
	return nil
}

func verifyJarPath(filePath string, artifact Artifact) error {
	file, err := openRegularFile(filePath, maxArtifactBytes)
	if err != nil {
		return fmt.Errorf("plugin update: verify downloaded %s: %w", artifact.Name, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("plugin update: stat downloaded %s: %w", artifact.Name, err)
	}
	return verifyJarReaderAt(file, info.Size(), artifact)
}

func verifyJarReaderAt(readerAt io.ReaderAt, size int64, artifact Artifact) error {
	archive, err := zip.NewReader(readerAt, size)
	if err != nil {
		return fmt.Errorf("plugin update: %s payload is not a valid JAR: %w", artifact.Name, err)
	}
	var descriptor *zip.File
	for _, file := range archive.File {
		if file.Name != "plugin.yml" || file.FileInfo().IsDir() {
			continue
		}
		if descriptor != nil {
			return fmt.Errorf("plugin update: %s JAR has multiple plugin.yml files", artifact.Name)
		}
		descriptor = file
	}
	if descriptor == nil {
		return fmt.Errorf("plugin update: %s JAR has no root plugin.yml", artifact.Name)
	}
	if descriptor.UncompressedSize64 == 0 || descriptor.UncompressedSize64 > maxPluginYAMLBytes {
		return fmt.Errorf("plugin update: %s plugin.yml is empty or too large", artifact.Name)
	}
	input, err := descriptor.Open()
	if err != nil {
		return fmt.Errorf("plugin update: open %s plugin.yml: %w", artifact.Name, err)
	}
	defer input.Close()
	body, err := io.ReadAll(io.LimitReader(input, maxPluginYAMLBytes+1))
	if err != nil {
		return fmt.Errorf("plugin update: read %s plugin.yml: %w", artifact.Name, err)
	}
	if len(body) == 0 || len(body) > maxPluginYAMLBytes || !utf8.Valid(body) {
		return fmt.Errorf("plugin update: %s plugin.yml is empty, too large, or invalid UTF-8", artifact.Name)
	}
	name, err := pluginYAMLName(body)
	if err != nil {
		return fmt.Errorf("plugin update: %s plugin.yml: %w", artifact.Name, err)
	}
	if !pluginNameMatches(artifact.Name, name) {
		return fmt.Errorf("plugin update: %s JAR identifies as plugin %q", artifact.Name, name)
	}
	return nil
}

func pluginYAMLName(body []byte) (string, error) {
	text := strings.TrimPrefix(string(body), "\uFEFF")
	var name string
	for _, line := range strings.Split(text, "\n") {
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(key) != "name" {
			continue
		}
		if name != "" {
			return "", errors.New("duplicate top-level name")
		}
		value = strings.TrimSpace(value)
		if before, _, hasComment := strings.Cut(value, " #"); hasComment {
			value = strings.TrimSpace(before)
		}
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if value == "" || strings.ContainsAny(value, "\r\n\x00") {
			return "", errors.New("invalid top-level name")
		}
		name = value
	}
	if name == "" {
		return "", errors.New("missing top-level name")
	}
	return name, nil
}

func pluginNameMatches(artifactName ArtifactName, name string) bool {
	switch artifactName {
	case ArtifactGeyser:
		return strings.EqualFold(name, "Geyser-Spigot") || strings.EqualFold(name, "Geyser")
	case ArtifactViaVersion:
		return strings.EqualFold(name, "ViaVersion")
	case ArtifactViaBackwards:
		return strings.EqualFold(name, "ViaBackwards")
	default:
		return false
	}
}

func openRegularFile(filePath string, maxBytes int64) (*os.File, error) {
	before, err := os.Lstat(filePath)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, errors.New("path is not a regular file")
	}
	if before.Size() > maxBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", maxBytes)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !os.SameFile(before, after) || !after.Mode().IsRegular() {
		_ = file.Close()
		return nil, errors.New("file changed while opening")
	}
	return file, nil
}

func publishManifest(stageDir string, bundle Bundle) error {
	manifestPath := filepath.Join(stageDir, manifestFilename)
	if err := rejectExistingSymlink(manifestPath); err != nil {
		return err
	}
	temp, err := os.CreateTemp(stageDir, ".nethernode-manifest-*.tmp")
	if err != nil {
		return fmt.Errorf("plugin update: create manifest temp file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("plugin update: secure manifest temp file: %w", err)
	}
	body, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		_ = temp.Close()
		return fmt.Errorf("plugin update: encode staged manifest: %w", err)
	}
	if _, err := temp.Write(append(body, '\n')); err != nil {
		_ = temp.Close()
		return fmt.Errorf("plugin update: write staged manifest: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("plugin update: sync staged manifest: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("plugin update: close staged manifest: %w", err)
	}
	if err := os.Rename(tempPath, manifestPath); err != nil {
		return fmt.Errorf("plugin update: publish staged manifest: %w", err)
	}
	if err := syncDirectory(stageDir); err != nil {
		return err
	}
	return nil
}

func rejectExistingSymlink(filePath string) error {
	info, err := os.Lstat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("plugin update: inspect staged path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("plugin update: staged path %q is not a regular file", filepath.Base(filePath))
	}
	return nil
}

func syncDirectory(dir string) error {
	entry, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("plugin update: open staging directory for sync: %w", err)
	}
	defer entry.Close()
	if err := entry.Sync(); err != nil {
		return fmt.Errorf("plugin update: sync staging directory: %w", err)
	}
	return nil
}

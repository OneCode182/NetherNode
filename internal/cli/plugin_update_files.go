package cli

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/onecode182/nethernode/internal/pluginupdate"
)

type installedPlugin struct {
	Name     pluginupdate.ArtifactName `json:"name"`
	Filename string                    `json:"filename"`
	SHA256   string                    `json:"sha256"`
	Version  string                    `json:"version"`
}

type installedPlugins [3]installedPlugin

type pluginTransaction struct {
	Before        installedPlugins `json:"before"`
	After         installedPlugins `json:"after"`
	Backup        string           `json:"backup"`
	ManagedBefore [3]string        `json:"managed_before"`
	ServerVersion string           `json:"server_version"`
}

var pluginNames = [...]pluginupdate.ArtifactName{pluginupdate.ArtifactGeyser, pluginupdate.ArtifactViaVersion, pluginupdate.ArtifactViaBackwards}

func pluginYAMLValue(body, key string) string {
	pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:\s*([^\r\n]+)`)
	match := pattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(match[1]), `"'`)
}

func inspectPluginJAR(path string) (string, string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", "", err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != "plugin.yml" {
			continue
		}
		if file.UncompressedSize64 > 1<<20 {
			return "", "", errors.New("plugin.yml exceeds size limit")
		}
		stream, err := file.Open()
		if err != nil {
			return "", "", err
		}
		body, readErr := io.ReadAll(io.LimitReader(stream, (1<<20)+1))
		closeErr := stream.Close()
		if readErr != nil {
			return "", "", readErr
		}
		if closeErr != nil {
			return "", "", closeErr
		}
		if len(body) > 1<<20 {
			return "", "", errors.New("plugin.yml exceeds size limit")
		}
		return pluginYAMLValue(string(body), "name"), pluginYAMLValue(string(body), "version"), nil
	}
	return "", "", nil
}

func fileSHA256(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("refusing non-regular file %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func inspectInstalledPlugins(dir string) (installedPlugins, error) {
	var result installedPlugins
	entries, err := os.ReadDir(dir)
	if err != nil {
		return result, fmt.Errorf("plugin update: read plugins dir: %w", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			return result, fmt.Errorf("plugin update: refusing symlinked JAR %s", entry.Name())
		}
		name, version, err := inspectPluginJAR(path)
		if err != nil {
			return result, fmt.Errorf("plugin update: inspect %s: %w", entry.Name(), err)
		}
		name = strings.ToLower(name)
		if name == "geyser-spigot" {
			name = "geyser"
		}
		for index, wanted := range pluginNames {
			if name != string(wanted) {
				continue
			}
			if result[index].Filename != "" {
				return result, fmt.Errorf("plugin update: duplicate installed %s JARs", wanted)
			}
			hash, err := fileSHA256(path)
			if err != nil {
				return result, err
			}
			if version == "" {
				return result, fmt.Errorf("plugin update: %s has no plugin version", entry.Name())
			}
			result[index] = installedPlugin{Name: wanted, Filename: entry.Name(), SHA256: hash, Version: version}
		}
	}
	for index, wanted := range pluginNames {
		if result[index].Filename == "" {
			return result, fmt.Errorf("plugin update: installed %s JAR is missing", wanted)
		}
	}
	return result, nil
}

func writePluginJSON(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(append(body, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func readPluginJSON(path string, target any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("plugin update: refusing non-regular state %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("plugin update: invalid trailing state")
	}
	return nil
}

func copyPluginFile(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("plugin update: refusing non-regular file %s", source)
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.CreateTemp(filepath.Dir(target), ".plugin-*")
	if err != nil {
		return err
	}
	defer os.Remove(output.Name())
	if err := output.Chmod(0o644); err != nil {
		output.Close()
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	if err := output.Sync(); err != nil {
		output.Close()
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	return os.Rename(output.Name(), target)
}

func validateInstalledSet(set installedPlugins) error {
	for index, plugin := range set {
		if plugin.Name != pluginNames[index] || plugin.Filename != filepath.Base(plugin.Filename) || !strings.HasSuffix(strings.ToLower(plugin.Filename), ".jar") || strings.ContainsAny(plugin.Filename, `/\\`) || len(plugin.SHA256) != 64 || plugin.Version == "" {
			return errors.New("plugin update: invalid installed-artifact state")
		}
		if _, err := hex.DecodeString(plugin.SHA256); err != nil {
			return errors.New("plugin update: invalid installed SHA256")
		}
	}
	return nil
}

func verifyInstalledSet(dir string, set installedPlugins) error {
	if err := validateInstalledSet(set); err != nil {
		return err
	}
	for _, plugin := range set {
		hash, err := fileSHA256(filepath.Join(dir, plugin.Filename))
		if err != nil {
			return err
		}
		if hash != plugin.SHA256 {
			return fmt.Errorf("plugin update: hash drift for %s", plugin.Filename)
		}
		name, version, err := inspectPluginJAR(filepath.Join(dir, plugin.Filename))
		if err != nil {
			return err
		}
		wantName := string(plugin.Name)
		if plugin.Name == pluginupdate.ArtifactGeyser {
			wantName = "geyser-spigot"
		}
		if strings.ToLower(name) != wantName || version != plugin.Version {
			return fmt.Errorf("plugin update: identity drift for %s", plugin.Filename)
		}
	}
	return nil
}

func swapPluginSet(pluginsDir, sourceDir string, from, to installedPlugins) error {
	if err := validateInstalledSet(from); err != nil {
		return err
	}
	if err := verifyInstalledSet(sourceDir, to); err != nil {
		return err
	}
	for _, plugin := range from {
		if err := os.Remove(filepath.Join(pluginsDir, plugin.Filename)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	for _, plugin := range to {
		if err := copyPluginFile(filepath.Join(sourceDir, plugin.Filename), filepath.Join(pluginsDir, plugin.Filename)); err != nil {
			return err
		}
	}
	return nil
}

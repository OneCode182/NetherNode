package pluginupdate

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

// ArtifactName is the fixed set of plugin artifacts this package can update.
type ArtifactName string

const (
	ArtifactGeyser       ArtifactName = "geyser"
	ArtifactViaVersion   ArtifactName = "viaversion"
	ArtifactViaBackwards ArtifactName = "viabackwards"

	manifestFilename = "bundle.json"
	maxArtifactBytes = 128 << 20
	maxMetadataBytes = 8 << 20
)

// Artifact describes one verified plugin JAR selected from an official source.
type Artifact struct {
	Name          ArtifactName `json:"name"`
	Version       string       `json:"version"`
	Filename      string       `json:"filename"`
	URL           string       `json:"url"`
	HashAlgorithm string       `json:"hash_algorithm"`
	Hash          string       `json:"hash"`
}

// Bundle fixes the updater to the Geyser/ViaVersion/ViaBackwards trio.
type Bundle struct {
	ServerVersion string      `json:"server_version"`
	CreatedAt     time.Time   `json:"created_at"`
	Artifacts     [3]Artifact `json:"artifacts"`
}

var (
	serverVersionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+){1,3}$`)
	versionPattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+~-]{0,127}$`)
	filenamePattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,180}\.jar$`)
	hex256Pattern        = regexp.MustCompile(`^[A-Fa-f0-9]{64}$`)
	hex512Pattern        = regexp.MustCompile(`^[A-Fa-f0-9]{128}$`)
	modrinthIDPattern    = regexp.MustCompile(`^[A-Za-z0-9]+$`)
)

func validateBundle(bundle Bundle) error {
	if !serverVersionPattern.MatchString(bundle.ServerVersion) {
		return fmt.Errorf("plugin update: invalid server version %q", bundle.ServerVersion)
	}
	if bundle.CreatedAt.IsZero() {
		return errors.New("plugin update: bundle has no creation time")
	}

	want := [...]ArtifactName{ArtifactGeyser, ArtifactViaVersion, ArtifactViaBackwards}
	seen := make(map[string]struct{}, len(want))
	for i, artifact := range bundle.Artifacts {
		if artifact.Name != want[i] {
			return fmt.Errorf("plugin update: artifact %d has name %q, want %q", i, artifact.Name, want[i])
		}
		if err := validateArtifact(artifact); err != nil {
			return err
		}
		key := strings.ToLower(artifact.Filename)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("plugin update: duplicate artifact filename %q", artifact.Filename)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateArtifact(artifact Artifact) error {
	if !versionPattern.MatchString(artifact.Version) {
		return fmt.Errorf("plugin update: %s has invalid version %q", artifact.Name, artifact.Version)
	}
	if !filenamePattern.MatchString(artifact.Filename) || path.Base(artifact.Filename) != artifact.Filename || strings.ContainsAny(artifact.Filename, `/\\`) {
		return fmt.Errorf("plugin update: %s has unsafe filename %q", artifact.Name, artifact.Filename)
	}
	if err := validateArtifactFilename(artifact.Name, artifact.Filename); err != nil {
		return err
	}

	switch artifact.Name {
	case ArtifactGeyser:
		if artifact.HashAlgorithm != "sha256" || !hex256Pattern.MatchString(artifact.Hash) {
			return errors.New("plugin update: Geyser requires a valid SHA256 checksum")
		}
		if !validGeyserDownloadURL(artifact.URL, artifact.Version) {
			return fmt.Errorf("plugin update: Geyser URL is outside the official download endpoint")
		}
	case ArtifactViaVersion, ArtifactViaBackwards:
		if artifact.HashAlgorithm != "sha512" || !hex512Pattern.MatchString(artifact.Hash) {
			return fmt.Errorf("plugin update: %s requires a valid SHA512 checksum", artifact.Name)
		}
		if !validModrinthDownloadURL(artifact.URL, artifact.Filename) {
			return fmt.Errorf("plugin update: %s URL is outside the official Modrinth CDN", artifact.Name)
		}
	default:
		return fmt.Errorf("plugin update: unsupported artifact name %q", artifact.Name)
	}
	return nil
}

func validateArtifactFilename(name ArtifactName, filename string) error {
	lower := strings.ToLower(filename)
	var prefix string
	switch name {
	case ArtifactGeyser:
		prefix = "geyser-spigot"
	case ArtifactViaVersion:
		prefix = "viaversion-"
	case ArtifactViaBackwards:
		prefix = "viabackwards-"
	default:
		return fmt.Errorf("plugin update: unsupported artifact name %q", name)
	}
	if !strings.HasPrefix(lower, prefix) {
		return fmt.Errorf("plugin update: %s has unexpected filename %q", name, filename)
	}
	return nil
}

func validGeyserDownloadURL(rawURL, version string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Hostname() != "download.geysermc.org" || !validHTTPSAuthority(u) || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 9 || parts[0] != "v2" || parts[1] != "projects" || parts[2] != "geyser" || parts[3] != "versions" || parts[5] != "builds" || parts[7] != "downloads" || parts[8] != "spigot" {
		return false
	}
	if parts[4] != version || !versionPattern.MatchString(parts[4]) {
		return false
	}
	if len(parts[6]) == 0 {
		return false
	}
	for _, r := range parts[6] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validModrinthDownloadURL(rawURL, filename string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Hostname() != "cdn.modrinth.com" || !validHTTPSAuthority(u) || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 5 || parts[0] != "data" || parts[2] != "versions" || parts[4] != filename {
		return false
	}
	return modrinthIDPattern.MatchString(parts[1]) && modrinthIDPattern.MatchString(parts[3])
}

func validHTTPSAuthority(u *url.URL) bool {
	return u.User == nil && (u.Port() == "" || u.Port() == "443")
}

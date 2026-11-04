package pluginupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	geyserMetadataURL = "https://download.geysermc.org/v2/projects/geyser/versions/latest/builds/latest"
	modrinthAPIURL    = "https://api.modrinth.com/v2/project"
)

// ResolveBundle reads official metadata and selects the latest compatible
// release bundle for the supplied Paper game version. It does not download or
// write plugin artifacts.
func ResolveBundle(ctx context.Context, serverVersion string, client *http.Client) (Bundle, error) {
	if ctx == nil {
		return Bundle{}, errors.New("plugin update: context is nil")
	}
	if !serverVersionPattern.MatchString(serverVersion) {
		return Bundle{}, fmt.Errorf("plugin update: invalid server version %q", serverVersion)
	}
	if client == nil {
		return Bundle{}, errors.New("plugin update: HTTP client is nil")
	}

	geyser, err := resolveGeyser(ctx, client)
	if err != nil {
		return Bundle{}, err
	}
	viaVersion, viaBackwards, err := resolveViaPair(ctx, serverVersion, client)
	if err != nil {
		return Bundle{}, err
	}

	bundle := Bundle{
		ServerVersion: serverVersion,
		CreatedAt:     time.Now().UTC(),
		Artifacts: [3]Artifact{
			geyser,
			viaVersion,
			viaBackwards,
		},
	}
	if err := validateBundle(bundle); err != nil {
		return Bundle{}, err
	}
	return bundle, nil
}

type geyserBuildMetadata struct {
	ProjectID string `json:"project_id"`
	Version   string `json:"version"`
	Build     int    `json:"build"`
	Channel   string `json:"channel"`
	Downloads map[string]struct {
		Name   string `json:"name"`
		SHA256 string `json:"sha256"`
	} `json:"downloads"`
}

func resolveGeyser(ctx context.Context, client *http.Client) (Artifact, error) {
	var metadata geyserBuildMetadata
	if err := fetchJSON(ctx, client, geyserMetadataURL, []string{"download.geysermc.org"}, &metadata); err != nil {
		return Artifact{}, fmt.Errorf("plugin update: resolve Geyser metadata: %w", err)
	}
	if metadata.ProjectID != "geyser" {
		return Artifact{}, fmt.Errorf("plugin update: unexpected Geyser project_id %q", metadata.ProjectID)
	}
	if metadata.Channel != "" && metadata.Channel != "default" {
		return Artifact{}, fmt.Errorf("plugin update: Geyser channel is %q, want default", metadata.Channel)
	}
	if !versionPattern.MatchString(metadata.Version) || metadata.Build < 1 {
		return Artifact{}, errors.New("plugin update: invalid Geyser version/build metadata")
	}
	download, ok := metadata.Downloads["spigot"]
	if !ok {
		return Artifact{}, errors.New("plugin update: Geyser metadata has no Spigot download")
	}
	artifact := Artifact{
		Name:          ArtifactGeyser,
		Version:       metadata.Version,
		Filename:      download.Name,
		URL:           fmt.Sprintf("https://download.geysermc.org/v2/projects/geyser/versions/%s/builds/%d/downloads/spigot", metadata.Version, metadata.Build),
		HashAlgorithm: "sha256",
		Hash:          strings.ToLower(download.SHA256),
	}
	if err := validateArtifact(artifact); err != nil {
		return Artifact{}, fmt.Errorf("plugin update: invalid Geyser metadata: %w", err)
	}
	return artifact, nil
}

type modrinthVersion struct {
	ID            string         `json:"id"`
	ProjectID     string         `json:"project_id"`
	VersionNumber string         `json:"version_number"`
	VersionType   string         `json:"version_type"`
	DatePublished time.Time      `json:"date_published"`
	Loaders       []string       `json:"loaders"`
	GameVersions  []string       `json:"game_versions"`
	Files         []modrinthFile `json:"files"`
}

type modrinthFile struct {
	Filename string            `json:"filename"`
	URL      string            `json:"url"`
	Primary  bool              `json:"primary"`
	Hashes   map[string]string `json:"hashes"`
}

type viaCandidate struct {
	artifact  Artifact
	published time.Time
}

func resolveViaPair(ctx context.Context, serverVersion string, client *http.Client) (Artifact, Artifact, error) {
	viaVersions, err := resolveModrinthVersions(ctx, client, "viaversion", ArtifactViaVersion, serverVersion)
	if err != nil {
		return Artifact{}, Artifact{}, err
	}
	backwardsVersions, err := resolveModrinthVersions(ctx, client, "viabackwards", ArtifactViaBackwards, serverVersion)
	if err != nil {
		return Artifact{}, Artifact{}, err
	}

	backwardsByVersion := make(map[string]viaCandidate, len(backwardsVersions))
	for _, candidate := range backwardsVersions {
		current, ok := backwardsByVersion[candidate.artifact.Version]
		if !ok || candidate.published.After(current.published) {
			backwardsByVersion[candidate.artifact.Version] = candidate
		}
	}
	for _, candidate := range viaVersions {
		if backwards, ok := backwardsByVersion[candidate.artifact.Version]; ok {
			return candidate.artifact, backwards.artifact, nil
		}
	}
	return Artifact{}, Artifact{}, fmt.Errorf("plugin update: no matching ViaVersion/ViaBackwards release pair supports Paper %s", serverVersion)
}

func resolveModrinthVersions(ctx context.Context, client *http.Client, projectSlug string, artifactName ArtifactName, serverVersion string) ([]viaCandidate, error) {
	if projectSlug != "viaversion" && projectSlug != "viabackwards" {
		return nil, fmt.Errorf("plugin update: unsupported Modrinth project %q", projectSlug)
	}
	query := url.Values{}
	gameVersions, _ := json.Marshal([]string{serverVersion})
	loaders, _ := json.Marshal([]string{"paper"})
	query.Set("game_versions", string(gameVersions))
	query.Set("loaders", string(loaders))
	query.Set("version_type", "release")
	endpoint := modrinthAPIURL + "/" + projectSlug + "/version?" + query.Encode()
	var versions []modrinthVersion
	if err := fetchJSON(ctx, client, endpoint, []string{"api.modrinth.com"}, &versions); err != nil {
		return nil, fmt.Errorf("plugin update: resolve %s metadata: %w", artifactName, err)
	}

	candidates := make([]viaCandidate, 0, len(versions))
	for index, version := range versions {
		if version.VersionType != "release" || !containsExact(version.Loaders, "paper") || !containsExact(version.GameVersions, serverVersion) {
			continue
		}
		if !modrinthIDPattern.MatchString(version.ID) || !modrinthIDPattern.MatchString(version.ProjectID) || !versionPattern.MatchString(version.VersionNumber) || version.DatePublished.IsZero() {
			return nil, fmt.Errorf("plugin update: malformed %s release metadata at index %d", artifactName, index)
		}
		file, err := selectModrinthFile(version.Files, artifactName)
		if err != nil {
			return nil, fmt.Errorf("plugin update: invalid %s release %q: %w", artifactName, version.VersionNumber, err)
		}
		artifact := Artifact{
			Name:          artifactName,
			Version:       version.VersionNumber,
			Filename:      file.Filename,
			URL:           file.URL,
			HashAlgorithm: "sha512",
			Hash:          strings.ToLower(file.Hashes["sha512"]),
		}
		if err := validateArtifact(artifact); err != nil {
			return nil, fmt.Errorf("plugin update: invalid %s release %q: %w", artifactName, version.VersionNumber, err)
		}
		candidates = append(candidates, viaCandidate{artifact: artifact, published: version.DatePublished})
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("plugin update: Modrinth has no Paper release for %s on %s", artifactName, serverVersion)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].published.After(candidates[j].published)
	})
	return candidates, nil
}

func selectModrinthFile(files []modrinthFile, artifactName ArtifactName) (modrinthFile, error) {
	var primary *modrinthFile
	jarFiles := make([]modrinthFile, 0, len(files))
	for i := range files {
		file := &files[i]
		if !strings.HasSuffix(strings.ToLower(file.Filename), ".jar") {
			continue
		}
		jarFiles = append(jarFiles, *file)
		if file.Primary {
			if primary != nil {
				return modrinthFile{}, fmt.Errorf("multiple primary JAR files")
			}
			primary = file
		}
	}
	if primary != nil {
		return *primary, nil
	}
	if len(jarFiles) == 1 {
		return jarFiles[0], nil
	}
	if len(jarFiles) == 0 {
		return modrinthFile{}, fmt.Errorf("no JAR file")
	}
	return modrinthFile{}, fmt.Errorf("multiple JAR files without a primary file for %s", artifactName)
}

func containsExact(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func fetchJSON(ctx context.Context, client *http.Client, rawURL string, allowedHosts []string, target any) error {
	response, err := openGET(ctx, client, rawURL, allowedHosts)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s returned HTTP %d", endpointLabel(rawURL), response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxMetadataBytes+1))
	if err != nil {
		return fmt.Errorf("read %s response: %w", endpointLabel(rawURL), err)
	}
	if len(body) == 0 {
		return fmt.Errorf("%s returned an empty response", endpointLabel(rawURL))
	}
	if len(body) > maxMetadataBytes {
		return fmt.Errorf("%s response exceeds %d bytes", endpointLabel(rawURL), maxMetadataBytes)
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s response: %w", endpointLabel(rawURL), err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%s response has trailing JSON", endpointLabel(rawURL))
		}
		return fmt.Errorf("decode trailing %s response: %w", endpointLabel(rawURL), err)
	}
	return nil
}

func endpointLabel(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "upstream"
	}
	return u.Host + u.Path
}

func openGET(ctx context.Context, client *http.Client, rawURL string, allowedHosts []string) (*http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil || !isAllowedHTTPSURL(u, allowedHosts) {
		return nil, fmt.Errorf("plugin update: refusing unsafe upstream URL %q", rawURL)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("plugin update: build upstream request: %w", err)
	}
	request.Header.Set("User-Agent", "NetherNode-PluginUpdater/1.0")

	clientCopy := *client
	originalCheckRedirect := client.CheckRedirect
	clientCopy.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if !isAllowedHTTPSURL(request.URL, allowedHosts) {
			return fmt.Errorf("plugin update: refusing redirect to unsafe host %q", request.URL.Host)
		}
		if originalCheckRedirect != nil {
			return originalCheckRedirect(request, via)
		}
		if len(via) >= 10 {
			return errors.New("plugin update: too many redirects")
		}
		return nil
	}
	response, err := clientCopy.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", endpointLabel(rawURL), err)
	}
	return response, nil
}

func isAllowedHTTPSURL(u *url.URL, allowedHosts []string) bool {
	if u == nil || u.Scheme != "https" || !validHTTPSAuthority(u) {
		return false
	}
	for _, host := range allowedHosts {
		if strings.EqualFold(u.Hostname(), host) {
			return true
		}
	}
	return false
}

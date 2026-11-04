package pluginupdate

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testServerVersion = "26.2"

type testUpstream struct {
	geyserMetadata []byte
	viaMetadata    map[string][]byte
	responses      map[string]testResponse
	seenQueries    map[string]url.Values
}

type testResponse struct {
	status   int
	body     []byte
	redirect string
}

type rewriteTransport struct {
	serverURL *url.URL
	base      http.RoundTripper
}

func (transport rewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copyRequest := request.Clone(request.Context())
	localURL := *transport.serverURL
	localURL.Path = request.URL.Path
	localURL.RawPath = ""
	localURL.RawQuery = request.URL.RawQuery
	copyRequest.URL = &localURL
	copyRequest.Host = localURL.Host
	response, err := transport.base.RoundTrip(copyRequest)
	if response != nil {
		response.Request = request
	}
	return response, err
}

func (upstream *testUpstream) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	if strings.HasPrefix(path, "/v2/project/") && strings.HasSuffix(path, "/version") {
		project := strings.TrimSuffix(strings.TrimPrefix(path, "/v2/project/"), "/version")
		upstream.seenQueries[project] = request.URL.Query()
		writeTestResponse(w, testResponse{status: http.StatusOK, body: upstream.viaMetadata[project]})
		return
	}
	if path == "/v2/projects/geyser/versions/latest/builds/latest" {
		writeTestResponse(w, testResponse{status: http.StatusOK, body: upstream.geyserMetadata})
		return
	}
	if response, ok := upstream.responses[path]; ok {
		writeTestResponse(w, response)
		return
	}
	writeTestResponse(w, testResponse{status: http.StatusNotFound, body: []byte("missing")})
}

func writeTestResponse(w http.ResponseWriter, response testResponse) {
	status := response.status
	if status == 0 {
		status = http.StatusOK
	}
	if response.redirect != "" {
		w.Header().Set("Location", response.redirect)
		w.WriteHeader(http.StatusFound)
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(response.body)
}

func newTestUpstream(t *testing.T) (*testUpstream, *http.Client) {
	t.Helper()
	geyserJar := makeTestJar(t, "Geyser-Spigot")
	viaJar := makeTestJar(t, "ViaVersion")
	backwardsJar := makeTestJar(t, "ViaBackwards")
	geyserPath := "/v2/projects/geyser/versions/2.12.0/builds/1258/downloads/spigot"
	viaPath := "/data/AAAA/versions/VV5121/ViaVersion-5.12.1.jar"
	backwardsPath := "/data/BBBB/versions/VB5121/ViaBackwards-5.12.1.jar"
	upstream := &testUpstream{
		geyserMetadata: mustJSON(t, map[string]any{
			"project_id": "geyser",
			"version":    "2.12.0",
			"build":      1258,
			"channel":    "default",
			"downloads": map[string]any{
				"spigot": map[string]any{"name": "Geyser-Spigot.jar", "sha256": sha256Hex(geyserJar)},
			},
		}),
		viaMetadata: map[string][]byte{
			"viaversion": mustJSON(t, []any{
				newModrinthVersion("5.12.1", "VVersion12", "ViaVersion-5.12.1.jar", viaPath, sha512Hex(viaJar), "2026-10-14T12:00:00Z"),
				newModrinthVersion("5.11.1", "VVersion11", "ViaVersion-5.11.1.jar", "/data/AAAA/versions/VV5111/ViaVersion-5.11.1.jar", sha512Hex(viaJar), "2026-09-01T12:00:00Z"),
			}),
			"viabackwards": mustJSON(t, []any{
				newModrinthVersion("5.12.1", "VBack12", "ViaBackwards-5.12.1.jar", backwardsPath, sha512Hex(backwardsJar), "2026-10-14T11:00:00Z"),
				newModrinthVersion("5.10.0", "VBack10", "ViaBackwards-5.10.0.jar", "/data/BBBB/versions/VB5100/ViaBackwards-5.10.0.jar", sha512Hex(backwardsJar), "2026-08-01T12:00:00Z"),
			}),
		},
		responses: map[string]testResponse{
			geyserPath:    {status: http.StatusOK, body: geyserJar},
			viaPath:       {status: http.StatusOK, body: viaJar},
			backwardsPath: {status: http.StatusOK, body: backwardsJar},
		},
		seenQueries: make(map[string]url.Values),
	}
	server := httptest.NewServer(upstream)
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	client := &http.Client{Transport: rewriteTransport{serverURL: serverURL, base: http.DefaultTransport}}
	return upstream, client
}

func newModrinthVersion(version, versionID, filename, fileURL, sha512, published string) map[string]any {
	return map[string]any{
		"id":             versionID,
		"project_id":     "StaticProjectId",
		"version_number": version,
		"version_type":   "release",
		"date_published": published,
		"loaders":        []string{"paper"},
		"game_versions":  []string{testServerVersion},
		"files": []any{map[string]any{
			"filename": filename,
			"url":      "https://cdn.modrinth.com" + fileURL,
			"primary":  true,
			"hashes":   map[string]string{"sha512": sha512},
		}},
	}
}

func makeTestJar(t *testing.T, pluginName string) []byte {
	t.Helper()
	var body bytes.Buffer
	archive := zip.NewWriter(&body)
	entry, err := archive.Create("plugin.yml")
	if err != nil {
		t.Fatalf("create plugin.yml: %v", err)
	}
	if _, err := io.WriteString(entry, "name: "+pluginName+"\nmain: test.Plugin\nversion: 1.0.0\n"); err != nil {
		t.Fatalf("write plugin.yml: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close test JAR: %v", err)
	}
	return body.Bytes()
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return body
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func sha512Hex(body []byte) string {
	sum := sha512.Sum512(body)
	return hex.EncodeToString(sum[:])
}

func TestResolveBundleSelectsLatestCompatiblePair(t *testing.T) {
	upstream, client := newTestUpstream(t)
	bundle, err := ResolveBundle(context.Background(), testServerVersion, client)
	if err != nil {
		t.Fatalf("ResolveBundle() error = %v", err)
	}
	if bundle.ServerVersion != testServerVersion || bundle.CreatedAt.IsZero() {
		t.Fatalf("bundle header = %+v", bundle)
	}
	wantNames := [...]ArtifactName{ArtifactGeyser, ArtifactViaVersion, ArtifactViaBackwards}
	for index, want := range wantNames {
		if bundle.Artifacts[index].Name != want {
			t.Fatalf("artifact[%d].Name = %q, want %q", index, bundle.Artifacts[index].Name, want)
		}
	}
	if bundle.Artifacts[0].Version != "2.12.0" || bundle.Artifacts[0].HashAlgorithm != "sha256" || bundle.Artifacts[0].Filename != "Geyser-Spigot.jar" {
		t.Fatalf("Geyser artifact = %+v", bundle.Artifacts[0])
	}
	if bundle.Artifacts[1].Version != "5.12.1" || bundle.Artifacts[2].Version != "5.12.1" {
		t.Fatalf("Via pair = %s / %s, want shared latest 5.12.1", bundle.Artifacts[1].Version, bundle.Artifacts[2].Version)
	}
	for _, name := range []string{"viaversion", "viabackwards"} {
		query := upstream.seenQueries[name]
		if query.Get("version_type") != "release" || query.Get("game_versions") != `["26.2"]` || query.Get("loaders") != `["paper"]` {
			t.Fatalf("%s query = %v", name, query)
		}
	}
	if bundle.Artifacts[1].HashAlgorithm != "sha512" || bundle.Artifacts[2].HashAlgorithm != "sha512" {
		t.Fatalf("Via checksum algorithms = %s / %s", bundle.Artifacts[1].HashAlgorithm, bundle.Artifacts[2].HashAlgorithm)
	}
}

func TestResolveBundleRejectsBadMetadata(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*testing.T, *testUpstream)
		wantErr string
	}{
		{
			name: "Geyser HTTP status",
			mutate: func(t *testing.T, upstream *testUpstream) {
				upstream.geyserMetadata = []byte(`{"project_id":"geyser"}`)
			},
			wantErr: "invalid Geyser version/build metadata",
		},
		{
			name: "legacy project field rejected",
			mutate: func(t *testing.T, upstream *testUpstream) {
				var metadata map[string]any
				if err := json.Unmarshal(upstream.geyserMetadata, &metadata); err != nil {
					t.Fatal(err)
				}
				metadata["project"] = "geyser"
				delete(metadata, "project_id")
				upstream.geyserMetadata = mustJSON(t, metadata)
			},
			wantErr: "unexpected Geyser project_id",
		},
		{
			name: "Geyser malformed JSON",
			mutate: func(t *testing.T, upstream *testUpstream) {
				upstream.geyserMetadata = []byte("{")
			},
			wantErr: "decode",
		},
		{
			name: "missing Geyser checksum",
			mutate: func(t *testing.T, upstream *testUpstream) {
				var metadata map[string]any
				if err := json.Unmarshal(upstream.geyserMetadata, &metadata); err != nil {
					t.Fatal(err)
				}
				metadata["downloads"].(map[string]any)["spigot"].(map[string]any)["sha256"] = ""
				upstream.geyserMetadata = mustJSON(t, metadata)
			},
			wantErr: "valid SHA256",
		},
		{
			name: "missing Via checksum",
			mutate: func(t *testing.T, upstream *testUpstream) {
				var versions []map[string]any
				if err := json.Unmarshal(upstream.viaMetadata["viaversion"], &versions); err != nil {
					t.Fatal(err)
				}
				files := versions[0]["files"].([]any)
				files[0].(map[string]any)["hashes"].(map[string]any)["sha512"] = ""
				upstream.viaMetadata["viaversion"] = mustJSON(t, versions)
			},
			wantErr: "valid SHA512",
		},
		{
			name: "oversized metadata",
			mutate: func(t *testing.T, upstream *testUpstream) {
				upstream.geyserMetadata = bytes.Repeat([]byte("x"), maxMetadataBytes+1)
			},
			wantErr: "exceeds 8388608 bytes",
		},
		{
			name: "non-default Geyser channel",
			mutate: func(t *testing.T, upstream *testUpstream) {
				var metadata map[string]any
				if err := json.Unmarshal(upstream.geyserMetadata, &metadata); err != nil {
					t.Fatal(err)
				}
				metadata["channel"] = "dev"
				upstream.geyserMetadata = mustJSON(t, metadata)
			},
			wantErr: "channel is \"dev\"",
		},
		{
			name: "no common Via version",
			mutate: func(t *testing.T, upstream *testUpstream) {
				upstream.viaMetadata["viabackwards"] = mustJSON(t, []any{
					newModrinthVersion("5.10.0", "VBack10", "ViaBackwards-5.10.0.jar", "/data/BBBB/versions/VB5100/ViaBackwards-5.10.0.jar", strings.Repeat("a", 128), "2026-08-01T12:00:00Z"),
				})
			},
			wantErr: "no matching ViaVersion/ViaBackwards",
		},
		{
			name: "unsafe Modrinth URL",
			mutate: func(t *testing.T, upstream *testUpstream) {
				var versions []map[string]any
				if err := json.Unmarshal(upstream.viaMetadata["viaversion"], &versions); err != nil {
					t.Fatal(err)
				}
				files := versions[0]["files"].([]any)
				files[0].(map[string]any)["url"] = "https://evil.invalid/data/AAAA/versions/VV5121/ViaVersion-5.12.1.jar"
				upstream.viaMetadata["viaversion"] = mustJSON(t, versions)
			},
			wantErr: "outside the official Modrinth CDN",
		},
		{
			name: "unsafe filename",
			mutate: func(t *testing.T, upstream *testUpstream) {
				var versions []map[string]any
				if err := json.Unmarshal(upstream.viaMetadata["viaversion"], &versions); err != nil {
					t.Fatal(err)
				}
				files := versions[0]["files"].([]any)
				files[0].(map[string]any)["filename"] = "../ViaVersion-5.12.1.jar"
				upstream.viaMetadata["viaversion"] = mustJSON(t, versions)
			},
			wantErr: "unsafe filename",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream, client := newTestUpstream(t)
			test.mutate(t, upstream)
			_, err := ResolveBundle(context.Background(), testServerVersion, client)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ResolveBundle() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestResolveBundleRejectsInvalidInputsAndHTTPStatus(t *testing.T) {
	if _, err := ResolveBundle(context.Background(), "26.2&evil=true", http.DefaultClient); err == nil {
		t.Fatal("ResolveBundle() accepted unsafe server version")
	}
	if _, err := ResolveBundle(context.Background(), testServerVersion, nil); err == nil {
		t.Fatal("ResolveBundle() accepted nil HTTP client")
	}
	if _, err := ResolveBundle(nil, testServerVersion, http.DefaultClient); err == nil {
		t.Fatal("ResolveBundle() accepted nil context")
	}

	upstream, client := newTestUpstream(t)
	upstream.geyserMetadata = []byte(`{"project_id":"geyser","version":"2.12.0","build":1258,"downloads":{}}`)
	upstream.responses["/"] = testResponse{}
	// The custom upstream responds 200 for metadata; use a transport-level
	// handler to make the selected metadata path return an HTTP failure.
	client.Transport = statusTransport{base: client.Transport, statusPath: "/v2/projects/geyser/versions/latest/builds/latest", status: http.StatusServiceUnavailable}
	_, err := ResolveBundle(context.Background(), testServerVersion, client)
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("ResolveBundle() error = %v, want HTTP 503", err)
	}
}

type statusTransport struct {
	base       http.RoundTripper
	statusPath string
	status     int
}

func (transport statusTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport.base.RoundTrip(request)
	if err == nil && response != nil && request.URL.Path == transport.statusPath {
		response.StatusCode = transport.status
	}
	return response, err
}

func TestStageBundleAndReadStagedVerifyPrivateArtifacts(t *testing.T) {
	_, client := newTestUpstream(t)
	bundle, err := ResolveBundle(context.Background(), testServerVersion, client)
	if err != nil {
		t.Fatalf("ResolveBundle() error = %v", err)
	}
	root := t.TempDir()
	stageDir := filepath.Join(root, "updates")
	staged, err := StageBundle(context.Background(), stageDir, bundle, client)
	if err != nil {
		t.Fatalf("StageBundle() error = %v", err)
	}
	if staged != bundle {
		t.Fatalf("StageBundle() bundle = %+v, want %+v", staged, bundle)
	}
	info, err := os.Stat(stageDir)
	if err != nil {
		t.Fatalf("Stat(stage dir): %v", err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("staging dir mode = %04o, want private", info.Mode().Perm())
	}
	for _, artifact := range bundle.Artifacts {
		info, err := os.Stat(filepath.Join(stageDir, artifact.Filename))
		if err != nil {
			t.Fatalf("Stat(%s): %v", artifact.Filename, err)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Fatalf("%s mode = %04o, want private", artifact.Filename, info.Mode().Perm())
		}
	}
	if _, err := os.Stat(filepath.Join(stageDir, manifestFilename)); err != nil {
		t.Fatalf("staged manifest missing: %v", err)
	}
	loaded, err := ReadStaged(stageDir)
	if err != nil {
		t.Fatalf("ReadStaged() error = %v", err)
	}
	if loaded != bundle {
		t.Fatalf("ReadStaged() bundle = %+v, want %+v", loaded, bundle)
	}
}

func TestStageBundleAllowsOfficialHostRedirect(t *testing.T) {
	upstream, client := newTestUpstream(t)
	artifactPath := "/v2/projects/geyser/versions/2.12.0/builds/1258/downloads/spigot"
	redirectPath := artifactPath + "/artifact"
	upstream.responses[artifactPath] = testResponse{status: http.StatusFound, redirect: "https://download.geysermc.org" + redirectPath}
	upstream.responses[redirectPath] = upstream.responses[artifactPath]
	upstream.responses[redirectPath] = testResponse{status: http.StatusOK, body: makeTestJar(t, "Geyser-Spigot")}
	bundle, err := ResolveBundle(context.Background(), testServerVersion, client)
	if err != nil {
		t.Fatalf("ResolveBundle() error = %v", err)
	}
	if _, err := StageBundle(context.Background(), filepath.Join(t.TempDir(), "updates"), bundle, client); err != nil {
		t.Fatalf("StageBundle() official redirect error = %v", err)
	}
}

func TestStageBundleRejectsUnsafeBundleBeforeNetworkOrWrite(t *testing.T) {
	_, client := newTestUpstream(t)
	bundle, err := ResolveBundle(context.Background(), testServerVersion, client)
	if err != nil {
		t.Fatalf("ResolveBundle() error = %v", err)
	}
	bundle.Artifacts[0].Filename = "../Geyser-Spigot.jar"
	stageDir := filepath.Join(t.TempDir(), "updates")
	if _, err := StageBundle(context.Background(), stageDir, bundle, client); err == nil {
		t.Fatal("StageBundle() accepted unsafe filename")
	}
	if _, err := os.Stat(stageDir); !os.IsNotExist(err) {
		t.Fatalf("stage directory exists after invalid bundle: err=%v", err)
	}
}

func TestStageBundleRejectsDownloadFailuresAndInvalidJars(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*testing.T, *testUpstream)
		wantError string
	}{
		{
			name: "checksum mismatch",
			mutate: func(t *testing.T, upstream *testUpstream) {
				path := "/v2/projects/geyser/versions/2.12.0/builds/1258/downloads/spigot"
				upstream.responses[path] = testResponse{status: http.StatusOK, body: []byte("tampered")}
			},
			wantError: "checksum mismatch",
		},
		{
			name: "wrong plugin identity",
			mutate: func(t *testing.T, upstream *testUpstream) {
				body := makeTestJar(t, "ViaVersion")
				path := "/v2/projects/geyser/versions/2.12.0/builds/1258/downloads/spigot"
				upstream.responses[path] = testResponse{status: http.StatusOK, body: body}
				var metadata map[string]any
				if err := json.Unmarshal(upstream.geyserMetadata, &metadata); err != nil {
					t.Fatal(err)
				}
				metadata["downloads"].(map[string]any)["spigot"].(map[string]any)["sha256"] = sha256Hex(body)
				upstream.geyserMetadata = mustJSON(t, metadata)
			},
			wantError: "identifies as plugin",
		},
		{
			name: "malformed JAR",
			mutate: func(t *testing.T, upstream *testUpstream) {
				body := []byte("not a zip JAR")
				path := "/v2/projects/geyser/versions/2.12.0/builds/1258/downloads/spigot"
				upstream.responses[path] = testResponse{status: http.StatusOK, body: body}
				var metadata map[string]any
				if err := json.Unmarshal(upstream.geyserMetadata, &metadata); err != nil {
					t.Fatal(err)
				}
				metadata["downloads"].(map[string]any)["spigot"].(map[string]any)["sha256"] = sha256Hex(body)
				upstream.geyserMetadata = mustJSON(t, metadata)
			},
			wantError: "not a valid JAR",
		},
		{
			name: "malformed plugin.yml",
			mutate: func(t *testing.T, upstream *testUpstream) {
				body := makeTestJarBody(t, "main: test.Plugin\nversion: 1.0.0\n")
				path := "/v2/projects/geyser/versions/2.12.0/builds/1258/downloads/spigot"
				upstream.responses[path] = testResponse{status: http.StatusOK, body: body}
				var metadata map[string]any
				if err := json.Unmarshal(upstream.geyserMetadata, &metadata); err != nil {
					t.Fatal(err)
				}
				metadata["downloads"].(map[string]any)["spigot"].(map[string]any)["sha256"] = sha256Hex(body)
				upstream.geyserMetadata = mustJSON(t, metadata)
			},
			wantError: "missing top-level name",
		},
		{
			name: "oversized plugin descriptor",
			mutate: func(t *testing.T, upstream *testUpstream) {
				body := makeTestJarBody(t, "name: Geyser-Spigot\n"+strings.Repeat("#", maxPluginYAMLBytes))
				path := "/v2/projects/geyser/versions/2.12.0/builds/1258/downloads/spigot"
				upstream.responses[path] = testResponse{status: http.StatusOK, body: body}
				var metadata map[string]any
				if err := json.Unmarshal(upstream.geyserMetadata, &metadata); err != nil {
					t.Fatal(err)
				}
				metadata["downloads"].(map[string]any)["spigot"].(map[string]any)["sha256"] = sha256Hex(body)
				upstream.geyserMetadata = mustJSON(t, metadata)
			},
			wantError: "plugin.yml is empty or too large",
		},
		{
			name: "download HTTP status",
			mutate: func(t *testing.T, upstream *testUpstream) {
				path := "/v2/projects/geyser/versions/2.12.0/builds/1258/downloads/spigot"
				upstream.responses[path] = testResponse{status: http.StatusServiceUnavailable, body: []byte("busy")}
			},
			wantError: "HTTP 503",
		},
		{
			name: "empty download",
			mutate: func(t *testing.T, upstream *testUpstream) {
				path := "/v2/projects/geyser/versions/2.12.0/builds/1258/downloads/spigot"
				upstream.responses[path] = testResponse{status: http.StatusOK, body: nil}
			},
			wantError: "download is empty",
		},
		{
			name: "unsafe redirect",
			mutate: func(t *testing.T, upstream *testUpstream) {
				path := "/v2/projects/geyser/versions/2.12.0/builds/1258/downloads/spigot"
				upstream.responses[path] = testResponse{status: http.StatusFound, redirect: "https://evil.invalid/payload.jar"}
			},
			wantError: "unsafe host",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream, client := newTestUpstream(t)
			test.mutate(t, upstream)
			bundle, err := ResolveBundle(context.Background(), testServerVersion, client)
			if err != nil {
				t.Fatalf("ResolveBundle() error = %v", err)
			}
			stageDir := filepath.Join(t.TempDir(), "updates")
			_, err = StageBundle(context.Background(), stageDir, bundle, client)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("StageBundle() error = %v, want substring %q", err, test.wantError)
			}
			if _, statErr := os.Stat(filepath.Join(stageDir, manifestFilename)); !os.IsNotExist(statErr) {
				t.Fatalf("manifest exists after failed stage: stat err=%v", statErr)
			}
		})
	}
}

func TestReadStagedDetectsTamperingAndSymlinks(t *testing.T) {
	_, client := newTestUpstream(t)
	bundle, err := ResolveBundle(context.Background(), testServerVersion, client)
	if err != nil {
		t.Fatalf("ResolveBundle() error = %v", err)
	}
	stageDir := filepath.Join(t.TempDir(), "updates")
	if _, err := StageBundle(context.Background(), stageDir, bundle, client); err != nil {
		t.Fatalf("StageBundle() error = %v", err)
	}
	artifactPath := filepath.Join(stageDir, bundle.Artifacts[1].Filename)
	manifestPath := filepath.Join(stageDir, manifestFilename)
	manifestBody, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read staged manifest: %v", err)
	}
	oldHash := bundle.Artifacts[0].Hash
	newHash := strings.Repeat("0", len(oldHash))
	if oldHash == newHash {
		newHash = strings.Repeat("1", len(oldHash))
	}
	tamperedManifest := bytes.Replace(manifestBody, []byte(oldHash), []byte(newHash), 1)
	if bytes.Equal(tamperedManifest, manifestBody) {
		t.Fatal("could not locate checksum in staged manifest")
	}
	if err := os.WriteFile(manifestPath, tamperedManifest, 0600); err != nil {
		t.Fatalf("tamper staged manifest: %v", err)
	}
	if _, err := ReadStaged(stageDir); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("ReadStaged() error = %v, want manifest checksum mismatch", err)
	}
	if err := os.WriteFile(manifestPath, manifestBody, 0600); err != nil {
		t.Fatalf("restore staged manifest: %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("tampered"), 0600); err != nil {
		t.Fatalf("tamper staged artifact: %v", err)
	}
	if _, err := ReadStaged(stageDir); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("ReadStaged() error = %v, want checksum mismatch", err)
	}

	if err := os.Remove(artifactPath); err != nil {
		t.Fatalf("remove staged artifact: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.jar")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.Symlink(outside, artifactPath); err != nil {
		t.Fatalf("create staged symlink: %v", err)
	}
	if _, err := ReadStaged(stageDir); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("ReadStaged() error = %v, want symlink refusal", err)
	}
}

func TestResolveBundleMetadataFailuresIncludeUpstreamStatus(t *testing.T) {
	upstream, client := newTestUpstream(t)
	upstream.responses["/v2/project/viaversion/version"] = testResponse{status: http.StatusUnauthorized, body: []byte("denied")}
	client.Transport = statusTransport{base: client.Transport, statusPath: "/v2/project/viaversion/version", status: http.StatusBadGateway}
	_, err := ResolveBundle(context.Background(), testServerVersion, client)
	if err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("ResolveBundle() error = %v, want HTTP 502", err)
	}
}

func TestReadStagedRejectsManifestTraversal(t *testing.T) {
	stageDir := t.TempDir()
	if err := os.Chmod(stageDir, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf(`{"server_version":%q,"created_at":%q,"artifacts":[{"name":"geyser","version":"2.12.0","filename":"../outside.jar","url":"https://download.geysermc.org/v2/projects/geyser/versions/2.12.0/builds/1258/downloads/spigot","hash_algorithm":"sha256","hash":"%s"}]}`,
		testServerVersion, time.Now().UTC().Format(time.RFC3339Nano), strings.Repeat("a", 64))
	if err := os.WriteFile(filepath.Join(stageDir, manifestFilename), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadStaged(stageDir); err == nil || !strings.Contains(err.Error(), "unsafe filename") {
		t.Fatalf("ReadStaged() error = %v, want malformed fixed bundle", err)
	}
}

func TestOpenRegularFileEnforcesArtifactPayloadLimit(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "oversized.jar")
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxArtifactBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := openRegularFile(filePath, maxArtifactBytes); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("openRegularFile() error = %v, want size limit", err)
	}
}

func makeTestJarBody(t *testing.T, descriptor string) []byte {
	t.Helper()
	var body bytes.Buffer
	archive := zip.NewWriter(&body)
	entry, err := archive.Create("plugin.yml")
	if err != nil {
		t.Fatalf("create plugin.yml: %v", err)
	}
	if _, err := io.WriteString(entry, descriptor); err != nil {
		t.Fatalf("write plugin.yml: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close test JAR: %v", err)
	}
	return body.Bytes()
}

package mcstatus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// javaOnlineFixture mirrors a real GET /v2/status/java/<address> response
// for an online server, including fields the client intentionally ignores
// (icon, players.list, eula_blocked, mods, plugins, software) to prove
// parsing is defensive rather than strict.
const javaOnlineFixture = `{
  "online": true,
  "host": "play.example.com",
  "port": 25565,
  "ip_address": "203.0.113.10",
  "eula_blocked": false,
  "retrieved_at": 1700000000000,
  "expires_at": 1700000030000,
  "version": {
    "name_raw": "Paper 1.21.4",
    "name_clean": "Paper 1.21.4",
    "name_html": "<span>Paper 1.21.4</span>",
    "protocol": 769
  },
  "players": {
    "online": 7,
    "max": 20,
    "list": [{"name_raw": "Steve", "name_clean": "Steve", "name_html": "Steve", "uuid": "00000000-0000-0000-0000-000000000000"}]
  },
  "motd": {
    "raw": "&aWelcome",
    "clean": "Welcome",
    "html": "<span>Welcome</span>"
  },
  "icon": "data:image/png;base64,AAAA",
  "mods": [],
  "software": "Paper",
  "plugins": []
}`

// javaOfflineFixture mirrors a real offline response: no version, players,
// or motd object at all.
const javaOfflineFixture = `{
  "online": false,
  "host": "play.example.com",
  "port": 25565,
  "eula_blocked": false,
  "retrieved_at": 1700000000000,
  "expires_at": 1700000030000
}`

// bedrockOnlineFixture mirrors a real GET /v2/status/bedrock/<address>
// response for an online server.
const bedrockOnlineFixture = `{
  "online": true,
  "host": "play.example.com",
  "port": 19132,
  "ip_address": "203.0.113.20",
  "eula_blocked": false,
  "retrieved_at": 1700000000000,
  "expires_at": 1700000030000,
  "version": {
    "name_raw": "1.21.50",
    "name_clean": "1.21.50",
    "name_html": "<span>1.21.50</span>",
    "protocol": 622
  },
  "players": {
    "online": 3,
    "max": 10
  },
  "motd": {
    "raw": "&bCrossplay",
    "clean": "Crossplay",
    "html": "<span>Crossplay</span>"
  },
  "gamemode": "Survival",
  "server_id": "1234567890"
}`

func newTestServer(t *testing.T, wantPath string, status int, body string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wantPath != "" && r.URL.Path != wantPath {
			t.Errorf("unexpected request path: got %q, want %q", r.URL.Path, wantPath)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestClient_Java(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
		want    *JavaStatus
	}{
		{
			name:   "online",
			status: http.StatusOK,
			body:   javaOnlineFixture,
			want: &JavaStatus{
				Online:        true,
				Host:          "play.example.com",
				Port:          25565,
				Version:       "Paper 1.21.4",
				PlayersOnline: 7,
				PlayersMax:    20,
				MOTD:          "Welcome",
			},
		},
		{
			name:   "offline",
			status: http.StatusOK,
			body:   javaOfflineFixture,
			want: &JavaStatus{
				Online: false,
				Host:   "play.example.com",
				Port:   25565,
			},
		},
		{
			name:    "server error",
			status:  http.StatusInternalServerError,
			body:    `{"message":"internal error"}`,
			wantErr: true,
		},
		{
			name:    "malformed json",
			status:  http.StatusOK,
			body:    `{"online": true,`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, "/status/java/play.example.com", tt.status, tt.body)
			client := &Client{BaseURL: ts.URL}

			got, err := client.Java(context.Background(), "play.example.com")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Java() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Java() unexpected error: %v", err)
			}
			if *got != *tt.want {
				t.Fatalf("Java() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestClient_Bedrock(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
		want    *BedrockStatus
	}{
		{
			name:   "online",
			status: http.StatusOK,
			body:   bedrockOnlineFixture,
			want: &BedrockStatus{
				Online:        true,
				Host:          "play.example.com",
				Port:          19132,
				Version:       "1.21.50",
				PlayersOnline: 3,
				PlayersMax:    10,
				MOTD:          "Crossplay",
			},
		},
		{
			name:   "offline",
			status: http.StatusOK,
			body:   `{"online": false, "host": "play.example.com", "port": 19132}`,
			want: &BedrockStatus{
				Online: false,
				Host:   "play.example.com",
				Port:   19132,
			},
		},
		{
			name:    "not found",
			status:  http.StatusNotFound,
			body:    `{"message":"not found"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, "/status/bedrock/play.example.com", tt.status, tt.body)
			client := &Client{BaseURL: ts.URL}

			got, err := client.Bedrock(context.Background(), "play.example.com")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Bedrock() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Bedrock() unexpected error: %v", err)
			}
			if *got != *tt.want {
				t.Fatalf("Bedrock() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestClient_EmptyAddressDoesNotHitNetwork(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &Client{BaseURL: ts.URL}

	if _, err := client.Java(context.Background(), ""); err == nil {
		t.Fatal("Java(\"\") error = nil, want error")
	}
	if _, err := client.Bedrock(context.Background(), ""); err == nil {
		t.Fatal("Bedrock(\"\") error = nil, want error")
	}
	if calls != 0 {
		t.Fatalf("expected no network calls for empty address, got %d", calls)
	}
}

func TestNew(t *testing.T) {
	c := New()
	if c.BaseURL != defaultBaseURL {
		t.Fatalf("New().BaseURL = %q, want %q", c.BaseURL, defaultBaseURL)
	}
	if c.HTTPClient == nil {
		t.Fatal("New().HTTPClient = nil, want non-nil")
	}
}

func TestClient_ZeroValueFallsBackToDefaults(t *testing.T) {
	var c Client
	if got := c.baseURL(); got != defaultBaseURL {
		t.Fatalf("baseURL() = %q, want %q", got, defaultBaseURL)
	}
	if got := c.httpClient(); got != http.DefaultClient {
		t.Fatalf("httpClient() = %v, want http.DefaultClient", got)
	}
}

func TestClient_RequestTimeout(t *testing.T) {
	// Sanity check that a canceled context surfaces as an error rather than
	// hanging or panicking.
	ts := newTestServer(t, "", http.StatusOK, javaOnlineFixture)
	client := &Client{BaseURL: ts.URL}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Java(ctx, "play.example.com:25565")
	if err == nil {
		t.Fatal("Java() with canceled context error = nil, want error")
	}
	if !strings.Contains(err.Error(), "mcstatus:") {
		t.Fatalf("error = %v, want mcstatus-prefixed error", err)
	}
}

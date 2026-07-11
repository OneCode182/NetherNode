// Package mcstatus is a minimal client for the mcstatus.io v2 API
// (https://mcstatus.io/docs), used to summarize Java and Bedrock Minecraft
// server status without depending on the local RCON/Docker state.
package mcstatus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// defaultBaseURL is the production mcstatus.io v2 API root.
const defaultBaseURL = "https://api.mcstatus.io/v2"

// Client is a small HTTP client for the mcstatus.io v2 API. The zero value
// is usable: BaseURL falls back to the production API and HTTPClient falls
// back to http.DefaultClient.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// New returns a Client configured for the production mcstatus.io API with a
// bounded request timeout.
func New() *Client {
	return &Client{
		BaseURL:    defaultBaseURL,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// JavaStatus is a defensively-parsed summary of a Java Edition server.
type JavaStatus struct {
	Online        bool
	Host          string
	Port          int
	Version       string
	PlayersOnline int
	PlayersMax    int
	MOTD          string
}

// BedrockStatus is a defensively-parsed summary of a Bedrock Edition server.
type BedrockStatus struct {
	Online        bool
	Host          string
	Port          int
	Version       string
	PlayersOnline int
	PlayersMax    int
	MOTD          string
}

// wireVersion, wirePlayers, and wireMOTD mirror the nested objects
// mcstatus.io returns. Any of these (or their fields) may be absent when a
// server is offline or unreachable, so every field is optional and the
// caller only reads through the accessor helpers below.
type wireVersion struct {
	NameClean string `json:"name_clean"`
	Name      string `json:"name"`
}

func (v wireVersion) displayName() string {
	if v.NameClean != "" {
		return v.NameClean
	}
	return v.Name
}

type wirePlayers struct {
	Online int `json:"online"`
	Max    int `json:"max"`
}

type wireMOTD struct {
	Clean string `json:"clean"`
}

type javaResponse struct {
	Online  bool        `json:"online"`
	Host    string      `json:"host"`
	Port    int         `json:"port"`
	Version wireVersion `json:"version"`
	Players wirePlayers `json:"players"`
	MOTD    wireMOTD    `json:"motd"`
}

type bedrockResponse struct {
	Online  bool        `json:"online"`
	Host    string      `json:"host"`
	Port    int         `json:"port"`
	Version wireVersion `json:"version"`
	Players wirePlayers `json:"players"`
	MOTD    wireMOTD    `json:"motd"`
}

// Java queries GET /v2/status/java/<address>, where address is a public host
// or a host:port pair when a nonstandard port is needed.
func (c *Client) Java(ctx context.Context, address string) (*JavaStatus, error) {
	body, err := c.fetch(ctx, "/status/java/"+address)
	if err != nil {
		return nil, err
	}

	var raw javaResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("mcstatus: decode java status for %q: %w", address, err)
	}

	return &JavaStatus{
		Online:        raw.Online,
		Host:          raw.Host,
		Port:          raw.Port,
		Version:       raw.Version.displayName(),
		PlayersOnline: raw.Players.Online,
		PlayersMax:    raw.Players.Max,
		MOTD:          raw.MOTD.Clean,
	}, nil
}

// Bedrock queries GET /v2/status/bedrock/<address>, where address is a public
// host or a host:port pair when a nonstandard port is needed.
func (c *Client) Bedrock(ctx context.Context, address string) (*BedrockStatus, error) {
	body, err := c.fetch(ctx, "/status/bedrock/"+address)
	if err != nil {
		return nil, err
	}

	var raw bedrockResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("mcstatus: decode bedrock status for %q: %w", address, err)
	}

	return &BedrockStatus{
		Online:        raw.Online,
		Host:          raw.Host,
		Port:          raw.Port,
		Version:       raw.Version.displayName(),
		PlayersOnline: raw.Players.Online,
		PlayersMax:    raw.Players.Max,
		MOTD:          raw.MOTD.Clean,
	}, nil
}

// fetch performs a GET against BaseURL+path and returns the raw response
// body, translating transport failures and non-2xx responses into errors.
func (c *Client) fetch(ctx context.Context, path string) ([]byte, error) {
	if strings.TrimSpace(path) == "/status/java/" || strings.TrimSpace(path) == "/status/bedrock/" {
		return nil, errors.New("mcstatus: address is required")
	}

	url := strings.TrimRight(c.baseURL(), "/") + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("mcstatus: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcstatus: request %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mcstatus: read response from %s: %w", url, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mcstatus: %s returned status %d", url, resp.StatusCode)
	}

	return body, nil
}

func (c *Client) baseURL() string {
	if c == nil || c.BaseURL == "" {
		return defaultBaseURL
	}
	return c.BaseURL
}

func (c *Client) httpClient() *http.Client {
	if c == nil || c.HTTPClient == nil {
		return http.DefaultClient
	}
	return c.HTTPClient
}

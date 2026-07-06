// Package cli implements the nethernode server admin CLI: lifecycle commands
// (start, stop, restart, status, save-server, backup-server) built on top of
// internal/rcon, internal/backup, internal/compose, and internal/mcstatus.
package cli

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds every environment-derived setting the CLI needs. Defaults
// match the repo's local compose.yaml / .env.example values.
type Config struct {
	ContainerName string // MINECRAFT_CONTAINER_NAME
	ComposeFile   string // COMPOSE_FILE
	DataDir       string // MINECRAFT_DATA_DIR

	BackupDest      string // BACKUP_DEST
	BackupRetention int    // BACKUP_RETENTION
	BackupLabel     string // BACKUP_LABEL

	RCONHost     string // fixed 127.0.0.1 (RCON is only ever reachable locally)
	RCONPort     string // MINECRAFT_RCON_PORT
	RCONPassword string // MINECRAFT_RCON_PASSWORD (env, falling back to .env file)
	RCONTimeout  time.Duration

	PublicHost  string // MINECRAFT_PUBLIC_HOST, used for mcstatus.io lookups; overridable by --host
	JavaPort    string // MINECRAFT_PORT
	BedrockPort string // MINECRAFT_BEDROCK_PORT
}

// RCONAddr returns the "host:port" pair used to dial RCON.
func (c Config) RCONAddr() string {
	return c.RCONHost + ":" + c.RCONPort
}

// JavaAddr returns the "host:port" pair used for the Java mcstatus.io lookup,
// using host in place of PublicHost when host is non-empty.
func (c Config) JavaAddr(host string) string {
	if host == "" {
		host = c.PublicHost
	}
	return host + ":" + c.JavaPort
}

// BedrockAddr returns the "host:port" pair used for the Bedrock mcstatus.io
// lookup, using host in place of PublicHost when host is non-empty.
func (c Config) BedrockAddr(host string) string {
	if host == "" {
		host = c.PublicHost
	}
	return host + ":" + c.BedrockPort
}

// getenvFunc mirrors os.Getenv's signature so config loading can be tested
// without mutating the real process environment.
type getenvFunc func(key string) string

// readFileFunc mirrors os.ReadFile's signature so the .env fallback can be
// tested without touching the real filesystem.
type readFileFunc func(name string) ([]byte, error)

// LoadConfig builds a Config from the real process environment, falling
// back to a ".env" file (or ENV_FILE, if set) for MINECRAFT_RCON_PASSWORD
// when it is not already exported.
func LoadConfig() Config {
	return LoadConfigFrom(os.Getenv, os.ReadFile)
}

// LoadConfigFrom builds a Config using the supplied getenv/readFile
// functions, so tests can inject fakes instead of the real environment/disk.
func LoadConfigFrom(getenv getenvFunc, readFile readFileFunc) Config {
	cfg := Config{
		ContainerName: firstNonEmpty(getenv("MINECRAFT_CONTAINER_NAME"), "nethernode-minecraft"),
		ComposeFile:   firstNonEmpty(getenv("COMPOSE_FILE"), "compose.yaml"),
		DataDir:       firstNonEmpty(getenv("MINECRAFT_DATA_DIR"), "./data/minecraft"),

		BackupDest:      firstNonEmpty(getenv("BACKUP_DEST"), "./backups"),
		BackupRetention: firstPositiveInt(getenv("BACKUP_RETENTION"), 5),
		BackupLabel:     firstNonEmpty(getenv("BACKUP_LABEL"), "minecraft"),

		RCONHost:     "127.0.0.1",
		RCONPort:     firstNonEmpty(getenv("MINECRAFT_RCON_PORT"), "25575"),
		RCONPassword: getenv("MINECRAFT_RCON_PASSWORD"),
		RCONTimeout:  5 * time.Second,

		PublicHost:  firstNonEmpty(getenv("MINECRAFT_PUBLIC_HOST"), "localhost"),
		JavaPort:    firstNonEmpty(getenv("MINECRAFT_PORT"), "25565"),
		BedrockPort: firstNonEmpty(getenv("MINECRAFT_BEDROCK_PORT"), "19132"),
	}

	if cfg.RCONPassword == "" {
		envFile := firstNonEmpty(getenv("ENV_FILE"), ".env")
		if body, err := readFile(envFile); err == nil {
			if v, ok := parseDotenv(body)["MINECRAFT_RCON_PASSWORD"]; ok {
				cfg.RCONPassword = v
			}
		}
	}

	return cfg
}

// parseDotenv does a minimal KEY=VALUE parse of a ".env"-style file: blank
// lines and lines starting with '#' are skipped, values are not
// shell-expanded, and surrounding matching quotes are stripped.
func parseDotenv(body []byte) map[string]string {
	out := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		out[key] = value
	}
	return out
}

func firstNonEmpty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func firstPositiveInt(v string, fallback int) int {
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

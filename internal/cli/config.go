// Package cli implements the nethernode server admin CLI: lifecycle commands
// (start, stop, restart, status, save-server, backup-server) built on top of
// internal/rcon, internal/backup, internal/compose, and internal/mcstatus.
package cli

import (
	"bufio"
	"os"
	"path/filepath"
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

	ScriptDir string // NETHERNODE_SCRIPT_DIR, for legacy/shell-only helpers
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

// deployedRoot is where EC2 hosts keep the app checkout (see infra
// user-data); the CLI falls back to it so `nethernode` works from any CWD.
const deployedRoot = "/opt/nethernode/app"

// LoadConfig builds a Config from the real process environment plus the app
// root's ".env" file (env wins). The app root is NETHERNODE_ROOT when set,
// otherwise the nearest ancestor of the CWD containing compose.yaml,
// otherwise /opt/nethernode/app when it holds a compose.yaml.
func LoadConfig() Config {
	exists := func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}
	root := ResolveRoot(os.Getenv, exists, os.Getwd)
	return LoadConfigFromRoot(root, os.Getenv, os.ReadFile)
}

// ResolveRoot picks the directory that relative paths (compose.yaml,
// ./data/minecraft, ./backups, .env) resolve against. Empty means "use the
// CWD as-is" (local-dev behavior).
func ResolveRoot(getenv getenvFunc, exists func(string) bool, getwd func() (string, error)) string {
	if root := getenv("NETHERNODE_ROOT"); root != "" {
		return root
	}
	if wd, err := getwd(); err == nil {
		dir := wd
		for i := 0; i < 16; i++ {
			if exists(filepath.Join(dir, "compose.yaml")) {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if exists(filepath.Join(deployedRoot, "compose.yaml")) {
		return deployedRoot
	}
	return ""
}

// LoadConfigFrom builds a Config with no app root, using the supplied
// getenv/readFile functions. Kept for compatibility; see LoadConfigFromRoot.
func LoadConfigFrom(getenv getenvFunc, readFile readFileFunc) Config {
	return LoadConfigFromRoot("", getenv, readFile)
}

// LoadConfigFromRoot builds a Config resolving relative paths against root
// (when non-empty) and reading root's ".env" (or ENV_FILE) as a fallback for
// every setting: process env wins, then .env, then defaults.
func LoadConfigFromRoot(root string, getenv getenvFunc, readFile readFileFunc) Config {
	dotenv := map[string]string{}
	envFile := firstNonEmpty(getenv("ENV_FILE"), rootJoin(root, ".env"))
	if body, err := readFile(envFile); err == nil {
		dotenv = parseDotenv(body)
	}
	lookup := func(key string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return dotenv[key]
	}

	cfg := Config{
		ContainerName: firstNonEmpty(lookup("MINECRAFT_CONTAINER_NAME"), "nethernode-minecraft"),
		ComposeFile:   rootJoin(root, firstNonEmpty(lookup("COMPOSE_FILE"), "compose.yaml")),
		DataDir:       rootJoin(root, firstNonEmpty(lookup("MINECRAFT_DATA_DIR"), "./data/minecraft")),

		BackupDest:      rootJoin(root, firstNonEmpty(lookup("BACKUP_DEST"), "./backups")),
		BackupRetention: firstPositiveInt(lookup("BACKUP_RETENTION"), 5),
		BackupLabel:     firstNonEmpty(lookup("BACKUP_LABEL"), "minecraft"),

		RCONHost:     "127.0.0.1",
		RCONPort:     firstNonEmpty(lookup("MINECRAFT_RCON_PORT"), "25575"),
		RCONPassword: lookup("MINECRAFT_RCON_PASSWORD"),
		RCONTimeout:  5 * time.Second,

		PublicHost:  firstNonEmpty(lookup("MINECRAFT_PUBLIC_HOST"), "localhost"),
		JavaPort:    firstNonEmpty(lookup("MINECRAFT_PORT"), "25565"),
		BedrockPort: firstNonEmpty(lookup("MINECRAFT_BEDROCK_PORT"), "19132"),

		ScriptDir: firstNonEmpty(lookup("NETHERNODE_SCRIPT_DIR"), "/opt/nethernode/scripts"),
	}

	return cfg
}

// rootJoin resolves path against root when root is set and path is relative.
func rootJoin(root, path string) string {
	if root == "" || path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
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

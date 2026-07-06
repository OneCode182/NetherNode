package cli

import (
	"errors"
	"testing"
)

func TestLoadConfigFrom_Defaults(t *testing.T) {
	getenv := func(string) string { return "" }
	readFile := func(string) ([]byte, error) { return nil, errors.New("not found") }

	cfg := LoadConfigFrom(getenv, readFile)

	want := Config{
		ContainerName:   "nethernode-minecraft",
		ComposeFile:     "compose.yaml",
		DataDir:         "./data/minecraft",
		BackupDest:      "./backups",
		BackupRetention: 5,
		BackupLabel:     "minecraft",
		RCONHost:        "127.0.0.1",
		RCONPort:        "25575",
		RCONPassword:    "",
		PublicHost:      "localhost",
		JavaPort:        "25565",
		BedrockPort:     "19132",
	}
	if cfg.ContainerName != want.ContainerName ||
		cfg.ComposeFile != want.ComposeFile ||
		cfg.DataDir != want.DataDir ||
		cfg.BackupDest != want.BackupDest ||
		cfg.BackupRetention != want.BackupRetention ||
		cfg.BackupLabel != want.BackupLabel ||
		cfg.RCONHost != want.RCONHost ||
		cfg.RCONPort != want.RCONPort ||
		cfg.RCONPassword != want.RCONPassword ||
		cfg.PublicHost != want.PublicHost ||
		cfg.JavaPort != want.JavaPort ||
		cfg.BedrockPort != want.BedrockPort {
		t.Fatalf("LoadConfigFrom() = %+v, want %+v", cfg, want)
	}
	if cfg.RCONTimeout <= 0 {
		t.Fatalf("RCONTimeout = %v, want > 0", cfg.RCONTimeout)
	}
}

func TestLoadConfigFrom_EnvOverrides(t *testing.T) {
	env := map[string]string{
		"MINECRAFT_CONTAINER_NAME": "custom-mc",
		"COMPOSE_FILE":             "custom.yaml",
		"MINECRAFT_DATA_DIR":       "/srv/data",
		"BACKUP_DEST":              "/srv/backups",
		"BACKUP_RETENTION":         "9",
		"BACKUP_LABEL":             "world",
		"MINECRAFT_RCON_PORT":      "26575",
		"MINECRAFT_RCON_PASSWORD":  "from-env",
		"MINECRAFT_PUBLIC_HOST":    "play.example.com",
		"MINECRAFT_PORT":           "26565",
		"MINECRAFT_BEDROCK_PORT":   "20132",
	}
	getenv := func(k string) string { return env[k] }
	readFile := func(string) ([]byte, error) { return nil, errors.New("should not be read") }

	cfg := LoadConfigFrom(getenv, readFile)

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"ContainerName", cfg.ContainerName, "custom-mc"},
		{"ComposeFile", cfg.ComposeFile, "custom.yaml"},
		{"DataDir", cfg.DataDir, "/srv/data"},
		{"BackupDest", cfg.BackupDest, "/srv/backups"},
		{"BackupLabel", cfg.BackupLabel, "world"},
		{"RCONPort", cfg.RCONPort, "26575"},
		{"RCONPassword", cfg.RCONPassword, "from-env"},
		{"PublicHost", cfg.PublicHost, "play.example.com"},
		{"JavaPort", cfg.JavaPort, "26565"},
		{"BedrockPort", cfg.BedrockPort, "20132"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
	if cfg.BackupRetention != 9 {
		t.Errorf("BackupRetention = %d, want 9", cfg.BackupRetention)
	}
	// RCON is always local regardless of env.
	if cfg.RCONHost != "127.0.0.1" {
		t.Errorf("RCONHost = %q, want 127.0.0.1", cfg.RCONHost)
	}
}

func TestLoadConfigFrom_BackupRetentionInvalidFallsBackToDefault(t *testing.T) {
	cases := []string{"", "0", "-3", "not-a-number"}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			getenv := func(k string) string {
				if k == "BACKUP_RETENTION" {
					return v
				}
				return ""
			}
			readFile := func(string) ([]byte, error) { return nil, errors.New("not found") }

			cfg := LoadConfigFrom(getenv, readFile)
			if cfg.BackupRetention != 5 {
				t.Fatalf("BackupRetention with input %q = %d, want default 5", v, cfg.BackupRetention)
			}
		})
	}
}

func TestLoadConfigFrom_RCONPasswordFallsBackToEnvFile(t *testing.T) {
	getenv := func(k string) string { return "" }
	readFile := func(name string) ([]byte, error) {
		if name != ".env" {
			return nil, errors.New("unexpected path: " + name)
		}
		return []byte("# comment\nMINECRAFT_RCON_PASSWORD=from-dotenv\nOTHER=ignored\n"), nil
	}

	cfg := LoadConfigFrom(getenv, readFile)
	if cfg.RCONPassword != "from-dotenv" {
		t.Fatalf("RCONPassword = %q, want %q", cfg.RCONPassword, "from-dotenv")
	}
}

func TestLoadConfigFrom_RCONPasswordEnvWinsOverEnvFile(t *testing.T) {
	getenv := func(k string) string {
		if k == "MINECRAFT_RCON_PASSWORD" {
			return "from-process-env"
		}
		return ""
	}
	readFileCalled := false
	readFile := func(name string) ([]byte, error) {
		readFileCalled = true
		return []byte("MINECRAFT_RCON_PASSWORD=from-dotenv\n"), nil
	}

	cfg := LoadConfigFrom(getenv, readFile)
	if cfg.RCONPassword != "from-process-env" {
		t.Fatalf("RCONPassword = %q, want process-env value", cfg.RCONPassword)
	}
	if readFileCalled {
		t.Fatal(".env file should not be read when MINECRAFT_RCON_PASSWORD is already set")
	}
}

func TestLoadConfigFrom_CustomEnvFilePath(t *testing.T) {
	getenv := func(k string) string {
		if k == "ENV_FILE" {
			return "custom.env"
		}
		return ""
	}
	var readPath string
	readFile := func(name string) ([]byte, error) {
		readPath = name
		return []byte("MINECRAFT_RCON_PASSWORD=custom-path-secret\n"), nil
	}

	cfg := LoadConfigFrom(getenv, readFile)
	if readPath != "custom.env" {
		t.Fatalf("read path = %q, want custom.env", readPath)
	}
	if cfg.RCONPassword != "custom-path-secret" {
		t.Fatalf("RCONPassword = %q, want custom-path-secret", cfg.RCONPassword)
	}
}

func TestParseDotenv_QuotesAndComments(t *testing.T) {
	body := []byte(`
# leading comment
KEY_A=plain
KEY_B="double quoted"
KEY_C='single quoted'
  KEY_D = spaced

NOT_AN_ASSIGNMENT
`)
	got := parseDotenv(body)

	want := map[string]string{
		"KEY_A": "plain",
		"KEY_B": "double quoted",
		"KEY_C": "single quoted",
		"KEY_D": "spaced",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("parseDotenv()[%q] = %q, want %q", k, got[k], v)
		}
	}
	if _, ok := got["NOT_AN_ASSIGNMENT"]; ok {
		t.Error("parseDotenv() should not produce an entry for a line with no '='")
	}
}

func TestConfig_AddrHelpers(t *testing.T) {
	cfg := Config{
		RCONHost:    "127.0.0.1",
		RCONPort:    "25575",
		PublicHost:  "localhost",
		JavaPort:    "25565",
		BedrockPort: "19132",
	}

	if got, want := cfg.RCONAddr(), "127.0.0.1:25575"; got != want {
		t.Errorf("RCONAddr() = %q, want %q", got, want)
	}
	if got, want := cfg.JavaAddr(""), "localhost:25565"; got != want {
		t.Errorf("JavaAddr(\"\") = %q, want %q", got, want)
	}
	if got, want := cfg.JavaAddr("play.example.com"), "play.example.com:25565"; got != want {
		t.Errorf("JavaAddr(host) = %q, want %q", got, want)
	}
	if got, want := cfg.BedrockAddr(""), "localhost:19132"; got != want {
		t.Errorf("BedrockAddr(\"\") = %q, want %q", got, want)
	}
	if got, want := cfg.BedrockAddr("switch.example.com"), "switch.example.com:19132"; got != want {
		t.Errorf("BedrockAddr(host) = %q, want %q", got, want)
	}
}

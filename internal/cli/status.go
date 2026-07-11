package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StatusReport aggregates every status source nethernode status checks.
// Each section degrades independently: a source that fails (container not
// running, RCON unreachable, mcstatus.io down, backup dir missing, df
// failing) reports its own Error instead of aborting the whole report.
type StatusReport struct {
	GeneratedAt time.Time     `json:"generated_at"`
	StatusHost  string        `json:"status_host,omitempty"`
	Container   ContainerInfo `json:"container"`
	RCON        RCONInfo      `json:"rcon"`
	Java        ProtocolInfo  `json:"java"`
	Bedrock     ProtocolInfo  `json:"bedrock"`
	Backups     BackupsInfo   `json:"backups"`
	Disk        DiskInfo      `json:"disk"`
}

// ContainerInfo reports docker's view of the Minecraft container.
type ContainerInfo struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
}

// RCONInfo reports whether RCON answered and the raw "list" output.
type RCONInfo struct {
	Reachable bool   `json:"reachable"`
	Transport string `json:"transport,omitempty"`
	Raw       string `json:"raw,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ProtocolInfo reports a single mcstatus.io Java or Bedrock summary.
type ProtocolInfo struct {
	Queried       bool   `json:"queried"`
	Skipped       bool   `json:"skipped,omitempty"`
	Online        bool   `json:"online"`
	Version       string `json:"version,omitempty"`
	PlayersOnline int    `json:"players_online,omitempty"`
	PlayersMax    int    `json:"players_max,omitempty"`
	Error         string `json:"error,omitempty"`
}

// BackupsInfo reports the local backup archive count and the newest one.
type BackupsInfo struct {
	Count    int       `json:"count"`
	Newest   string    `json:"newest,omitempty"`
	NewestAt time.Time `json:"newest_at,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// DiskInfo reports free space on the Minecraft data volume, via `df -h`.
type DiskInfo struct {
	Raw   string `json:"raw,omitempty"`
	Error string `json:"error,omitempty"`
}

// CmdStatus aggregates docker container state, RCON reachability/player
// list, mcstatus.io Java+Bedrock summaries, local backup inventory, and
// disk free space into a StatusReport, printed as text or (--json) JSON.
func CmdStatus(a *App, args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	host := fs.String("host", "", "override public host used for mcstatus.io java/bedrock lookups")
	asJSON := fs.Bool("json", false, "print status as JSON")
	colorMode := fs.String("color", "auto", "color: auto, always, never")
	noColor := fs.Bool("no-color", false, "same as --color=never")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if a.DryRun {
		a.printf("[dry-run] docker inspect -f {{.State.Running}} %s\n", a.Config.ContainerName)
		a.printf("[dry-run] docker exec %s rcon-cli list (tcp fallback @ %s)\n", a.Config.ContainerName, a.Config.RCONAddr())
		a.printf("[dry-run] mcstatus.io java %s\n", a.Config.JavaAddr(*host))
		a.printf("[dry-run] mcstatus.io bedrock %s\n", a.Config.BedrockAddr(*host))
		a.printf("[dry-run] scan backups in %s (label=%s-*.tar.gz)\n", a.Config.BackupDest, a.Config.BackupLabel)
		a.printf("[dry-run] df -h %s\n", a.Config.DataDir)
		return nil
	}
	if *noColor {
		*colorMode = "never"
	}
	if !validColorMode(*colorMode) {
		return fmt.Errorf("status: invalid --color value %q (want auto, always, or never)", *colorMode)
	}

	ctx := context.Background()
	statusHost := strings.TrimSpace(*host)
	if statusHost == "" {
		statusHost = a.Config.StatusHost
	}
	report := StatusReport{
		GeneratedAt: a.Now(),
		StatusHost:  statusHost,
		Container:   ContainerInfo{Name: a.Config.ContainerName},
	}

	running, err := a.Compose.ContainerRunning(ctx, a.Config.ContainerName)
	if err != nil {
		report.Container.Error = err.Error()
	} else {
		report.Container.Running = running
	}

	report.RCON = a.rconStatus(ctx)
	report.Java = a.protocolStatus(ctx, protocolJava, a.Config.JavaAddr(statusHost))
	report.Bedrock = a.protocolStatus(ctx, protocolBedrock, a.Config.BedrockAddr(statusHost))
	report.Backups = a.backupsStatus()
	report.Disk = a.diskStatus(ctx)

	if *asJSON {
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	a.printStatusHuman(report, statusColorEnabled(*colorMode, a.Stdout))
	return nil
}

func (a *App) rconStatus(ctx context.Context) RCONInfo {
	// Prefer the tested client shipped in the running Minecraft container. This
	// avoids Paper closing a host-side TCP RCON negotiation after an image update.
	out, execErr := a.Compose.ContainerCommand(ctx, a.Config.ContainerName, "rcon-cli", "list")
	if execErr == nil {
		return RCONInfo{Reachable: true, Transport: "docker exec rcon-cli", Raw: strings.TrimSpace(out)}
	}

	client, err := a.DialRCON(a.Config.RCONAddr(), a.Config.RCONPassword, a.Config.RCONTimeout)
	if err != nil {
		return RCONInfo{Error: fmt.Sprintf("docker exec rcon-cli: %v; tcp fallback: %v", execErr, err)}
	}
	defer client.Close()

	tcpOut, err := client.Command("list")
	if err != nil {
		return RCONInfo{Reachable: true, Transport: "tcp fallback", Error: err.Error()}
	}
	return RCONInfo{Reachable: true, Transport: "tcp fallback", Raw: strings.TrimSpace(tcpOut)}
}

type protocolKind int

const (
	protocolJava protocolKind = iota
	protocolBedrock
)

func (a *App) protocolStatus(ctx context.Context, kind protocolKind, addr string) ProtocolInfo {
	if addr == "" {
		return ProtocolInfo{Skipped: true, Error: "status host not configured; set MINECRAFT_STATUS_HOST"}
	}
	switch kind {
	case protocolJava:
		s, err := a.MCStatus.Java(ctx, addr)
		if err != nil {
			return ProtocolInfo{Queried: true, Error: err.Error()}
		}
		return ProtocolInfo{Queried: true, Online: s.Online, Version: s.Version, PlayersOnline: s.PlayersOnline, PlayersMax: s.PlayersMax}
	case protocolBedrock:
		s, err := a.MCStatus.Bedrock(ctx, addr)
		if err != nil {
			return ProtocolInfo{Queried: true, Error: err.Error()}
		}
		return ProtocolInfo{Queried: true, Online: s.Online, Version: s.Version, PlayersOnline: s.PlayersOnline, PlayersMax: s.PlayersMax}
	default:
		return ProtocolInfo{Error: "unknown protocol kind"}
	}
}

func (a *App) backupsStatus() BackupsInfo {
	entries, err := os.ReadDir(a.Config.BackupDest)
	if err != nil {
		return BackupsInfo{Error: err.Error()}
	}

	pattern := a.Config.BackupLabel + "-*.tar.gz"
	var info BackupsInfo
	var newestTime time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		matched, err := filepath.Match(pattern, e.Name())
		if err != nil || !matched {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		info.Count++
		if fi.ModTime().After(newestTime) {
			newestTime = fi.ModTime()
			info.Newest = e.Name()
			info.NewestAt = fi.ModTime()
		}
	}
	return info
}

func (a *App) diskStatus(ctx context.Context) DiskInfo {
	out, err := a.Compose.Run(ctx, "df", "-h", a.Config.DataDir)
	if err != nil {
		return DiskInfo{Error: err.Error()}
	}
	return DiskInfo{Raw: strings.TrimSpace(out)}
}

func (a *App) printStatusHuman(r StatusReport, color bool) {
	section := func(name string) { a.printf("\n%s\n", statusStyle(color, "1;36", name)) }
	state := func(kind, text string) string {
		code := "32"
		if kind == "WARN" {
			code = "33"
		} else if kind == "FAIL" {
			code = "31"
		} else if kind == "SKIP" {
			code = "90"
		}
		return statusStyle(color, code, "["+kind+"]") + " " + text
	}

	a.printf("%s\n", statusStyle(color, "1;36", "NetherNode status"))
	a.printf("  time  %s\n", r.GeneratedAt.UTC().Format(time.RFC3339))
	if r.StatusHost != "" {
		a.printf("  host  %s\n", r.StatusHost)
	}

	section("Runtime")
	if r.Container.Error != "" {
		a.printf("  container  %s\n", state("FAIL", r.Container.Error))
	} else if r.Container.Running {
		a.printf("  container  %s\n", state("OK", r.Container.Name+" running"))
	} else {
		a.printf("  container  %s\n", state("WARN", r.Container.Name+" stopped"))
	}
	if r.RCON.Error != "" {
		a.printf("  rcon       %s\n", state("FAIL", r.RCON.Error))
	} else {
		transport := r.RCON.Transport
		if transport != "" {
			transport = " via " + transport
		}
		a.printf("  rcon       %s\n", state("OK", r.RCON.Raw+transport))
	}

	section("Network")
	printProtocol := func(name string, p ProtocolInfo) {
		if p.Skipped {
			a.printf("  %-10s %s\n", name, state("SKIP", p.Error))
			return
		}
		if p.Error != "" {
			a.printf("  %-10s %s\n", name, state("FAIL", p.Error))
			return
		}
		if !p.Online {
			a.printf("  %-10s %s\n", name, state("WARN", "offline"))
			return
		}
		a.printf("  %-10s %s\n", name, state("OK", fmt.Sprintf("%s | players %d/%d", p.Version, p.PlayersOnline, p.PlayersMax)))
	}
	printProtocol("Java", r.Java)
	printProtocol("Bedrock", r.Bedrock)

	section("Storage")
	if r.Backups.Error != "" {
		a.printf("  backups    %s\n", state("FAIL", r.Backups.Error))
	} else if r.Backups.Count == 0 {
		a.printf("  backups    %s\n", state("WARN", "none found in "+a.Config.BackupDest))
	} else {
		a.printf("  backups    %s\n", state("OK", fmt.Sprintf("%d archives | newest %s (%s)", r.Backups.Count, r.Backups.Newest, r.Backups.NewestAt.UTC().Format(time.RFC3339))))
	}
	if r.Disk.Error != "" {
		a.printf("  disk       %s\n", state("FAIL", r.Disk.Error))
	} else {
		a.printf("  disk       %s\n", state("OK", r.Disk.Raw))
	}
}

func validColorMode(mode string) bool {
	return mode == "auto" || mode == "always" || mode == "never"
}

func statusColorEnabled(mode string, output io.Writer) bool {
	if mode == "always" {
		return true
	}
	if mode == "never" || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	file, ok := output.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func statusStyle(enabled bool, code, value string) string {
	if !enabled {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}

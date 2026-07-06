package cli

import (
	"context"
	"encoding/json"
	"flag"
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
	Raw       string `json:"raw,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ProtocolInfo reports a single mcstatus.io Java or Bedrock summary.
type ProtocolInfo struct {
	Queried       bool   `json:"queried"`
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
	host := fs.String("host", "", "override host used for mcstatus.io java/bedrock lookups")
	asJSON := fs.Bool("json", false, "print status as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if a.DryRun {
		a.printf("[dry-run] docker inspect -f {{.State.Running}} %s\n", a.Config.ContainerName)
		a.printf("[dry-run] rcon list @ %s\n", a.Config.RCONAddr())
		a.printf("[dry-run] mcstatus.io java %s\n", a.Config.JavaAddr(*host))
		a.printf("[dry-run] mcstatus.io bedrock %s\n", a.Config.BedrockAddr(*host))
		a.printf("[dry-run] scan backups in %s (label=%s-*.tar.gz)\n", a.Config.BackupDest, a.Config.BackupLabel)
		a.printf("[dry-run] df -h %s\n", a.Config.DataDir)
		return nil
	}

	ctx := context.Background()
	report := StatusReport{
		GeneratedAt: a.Now(),
		Container:   ContainerInfo{Name: a.Config.ContainerName},
	}

	running, err := a.Compose.ContainerRunning(ctx, a.Config.ContainerName)
	if err != nil {
		report.Container.Error = err.Error()
	} else {
		report.Container.Running = running
	}

	report.RCON = a.rconStatus()
	report.Java = a.protocolStatus(ctx, protocolJava, a.Config.JavaAddr(*host))
	report.Bedrock = a.protocolStatus(ctx, protocolBedrock, a.Config.BedrockAddr(*host))
	report.Backups = a.backupsStatus()
	report.Disk = a.diskStatus(ctx)

	if *asJSON {
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	a.printStatusHuman(report)
	return nil
}

func (a *App) rconStatus() RCONInfo {
	client, err := a.DialRCON(a.Config.RCONAddr(), a.Config.RCONPassword, a.Config.RCONTimeout)
	if err != nil {
		return RCONInfo{Error: err.Error()}
	}
	defer client.Close()

	out, err := client.Command("list")
	if err != nil {
		return RCONInfo{Reachable: true, Error: err.Error()}
	}
	return RCONInfo{Reachable: true, Raw: strings.TrimSpace(out)}
}

type protocolKind int

const (
	protocolJava protocolKind = iota
	protocolBedrock
)

func (a *App) protocolStatus(ctx context.Context, kind protocolKind, addr string) ProtocolInfo {
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

func (a *App) printStatusHuman(r StatusReport) {
	a.printf("nethernode status @ %s\n", r.GeneratedAt.UTC().Format(time.RFC3339))

	a.printf("container %s: ", r.Container.Name)
	if r.Container.Error != "" {
		a.printf("unknown (%s)\n", r.Container.Error)
	} else {
		a.printf("running=%v\n", r.Container.Running)
	}

	a.printf("rcon: ")
	if r.RCON.Error != "" {
		a.printf("error (%s)\n", r.RCON.Error)
	} else {
		a.printf("ok, %s\n", r.RCON.Raw)
	}

	printProtocol := func(name string, p ProtocolInfo) {
		a.printf("%s: ", name)
		if p.Error != "" {
			a.printf("error (%s)\n", p.Error)
			return
		}
		a.printf("online=%v version=%q players=%d/%d\n", p.Online, p.Version, p.PlayersOnline, p.PlayersMax)
	}
	printProtocol("java", r.Java)
	printProtocol("bedrock", r.Bedrock)

	a.printf("backups: ")
	if r.Backups.Error != "" {
		a.printf("error (%s)\n", r.Backups.Error)
	} else if r.Backups.Count == 0 {
		a.printf("none found in %s\n", a.Config.BackupDest)
	} else {
		a.printf("%d found, newest %s (%s)\n", r.Backups.Count, r.Backups.Newest, r.Backups.NewestAt.UTC().Format(time.RFC3339))
	}

	a.printf("disk: ")
	if r.Disk.Error != "" {
		a.printf("error (%s)\n", r.Disk.Error)
	} else {
		a.printf("%s\n", r.Disk.Raw)
	}
}

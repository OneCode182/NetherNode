package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/onecode182/nethernode/internal/backup"
	"github.com/onecode182/nethernode/internal/pluginupdate"
)

func (a *App) runPluginUpdate(action string, args []string) error {
	if len(args) == 0 || args[0] != "geyser" {
		return fmt.Errorf("plugins %s: expected target geyser", action)
	}
	apply := false
	for _, arg := range args[1:] {
		if arg != "--apply" || apply || action == "check" {
			return fmt.Errorf("plugins %s: unknown or repeated argument %q", action, arg)
		}
		apply = true
	}
	if action == "rollback" && !apply {
		return errors.New("plugins rollback geyser requires --apply")
	}
	if a.DryRun {
		a.printf("[dry-run] plugins %s geyser (apply=%t): fixed Geyser/ViaVersion/ViaBackwards bundle; no network, Docker, RCON, or file writes\n", action, apply)
		return nil
	}
	pluginsDir := filepath.Join(a.Config.DataDir, "plugins")
	if action == "check" {
		return a.checkGeyserUpdate(pluginsDir)
	}
	if a.Config.PluginUpdateDir == "" {
		return errors.New("plugin update: update state directory is missing")
	}
	info, err := os.Lstat(pluginsDir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("plugin update: plugins path must be a real directory")
	}
	if err := validatePluginStateLocation(a.Config.PluginUpdateDir, a.Config.DataDir, a.Config.BackupDest); err != nil {
		return err
	}
	unlock, err := lockPluginUpdates(filepath.Join(pluginsDir, ".nethernode-plugin-update.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	if err := os.MkdirAll(a.Config.PluginUpdateDir, 0o700); err != nil {
		return err
	}
	if action == "update" && !apply {
		return a.stageGeyserUpdate(pluginsDir)
	}
	return a.applyGeyserUpdate(pluginsDir, action == "rollback")
}

func validatePluginStateLocation(stateDir, dataDir, backupDir string) error {
	state, err := filepath.Abs(stateDir)
	if err != nil {
		return err
	}
	for _, protected := range []string{dataDir, backupDir} {
		base, err := filepath.Abs(protected)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, state)
		if err != nil {
			return err
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return errors.New("plugin update: state directory must be outside Minecraft data and backup directories")
		}
		reverse, err := filepath.Rel(state, base)
		if err != nil {
			return err
		}
		if reverse == "." || (reverse != ".." && !strings.HasPrefix(reverse, ".."+string(filepath.Separator))) {
			return errors.New("plugin update: state directory cannot contain Minecraft data or backup directories")
		}
	}
	for path := state; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("plugin update: refusing symlinked state directory %s", path)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if filepath.Dir(path) == path {
			break
		}
	}
	return nil
}

func (a *App) pluginHTTPClient() *http.Client {
	if a.PluginHTTPClient != nil {
		return a.PluginHTTPClient
	}
	return &http.Client{Timeout: 45 * time.Second}
}

func (a *App) checkGeyserUpdate(pluginsDir string) error {
	installed, err := inspectInstalledPlugins(pluginsDir)
	if err != nil {
		return err
	}
	serverVersion, err := a.pluginServerVersion()
	if err != nil {
		return err
	}
	bundle, err := pluginupdate.ResolveBundle(context.Background(), serverVersion, a.pluginHTTPClient())
	if err != nil {
		return err
	}
	for index, artifact := range bundle.Artifacts {
		a.printf("%s: installed=%s (%s) available=%s (%s) %s=%s\n", artifact.Name, installed[index].Version, installed[index].Filename, artifact.Version, artifact.Filename, artifact.HashAlgorithm, artifact.Hash)
	}
	a.printf("check only: no artifacts downloaded or server changes made; target client login remains an operator verification\n")
	return nil
}

func (a *App) stageGeyserUpdate(pluginsDir string) error {
	installed, err := inspectInstalledPlugins(pluginsDir)
	if err != nil {
		return err
	}
	serverVersion, err := a.pluginServerVersion()
	if err != nil {
		return err
	}
	bundle, err := pluginupdate.ResolveBundle(context.Background(), serverVersion, a.pluginHTTPClient())
	if err != nil {
		return err
	}
	temporary, err := os.MkdirTemp(a.Config.PluginUpdateDir, ".staging-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	// StageBundle owns its directory and verifies all three JAR identities/hashes.
	bundle, err = pluginupdate.StageBundle(context.Background(), temporary, bundle, a.pluginHTTPClient())
	if err != nil {
		return err
	}
	if err := writePluginJSON(filepath.Join(temporary, "baseline.json"), installed); err != nil {
		return err
	}
	current, err := inspectInstalledPlugins(pluginsDir)
	if err != nil {
		return err
	}
	if current != installed {
		return errors.New("plugin update: installed JARs changed during staging; retry")
	}
	staged := filepath.Join(a.Config.PluginUpdateDir, "staged")
	previous := filepath.Join(a.Config.PluginUpdateDir, ".previous-staged")
	if _, err := os.Lstat(previous); err == nil {
		return errors.New("plugin update: interrupted staging requires operator inspection of .previous-staged")
	} else if !os.IsNotExist(err) {
		return err
	}
	hadPrevious := false
	if _, err := os.Lstat(staged); err == nil {
		if err := os.Rename(staged, previous); err != nil {
			return err
		}
		hadPrevious = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(temporary, staged); err != nil {
		if hadPrevious {
			_ = os.Rename(previous, staged)
		}
		return err
	}
	if hadPrevious {
		if err := os.RemoveAll(previous); err != nil {
			return err
		}
	}
	for _, artifact := range bundle.Artifacts {
		a.printf("staged %s %s (%s)\n", artifact.Name, artifact.Version, artifact.Filename)
	}
	a.printf("staged only; activate during an empty-server maintenance window: nethernode plugins update geyser --apply\n")
	return nil
}

var liveMinecraftPattern = regexp.MustCompile(`\(MC:\s*([0-9]+(?:\.[0-9]+){1,3})\)`)
var livePaperPattern = regexp.MustCompile(`(?i)\bPaper version ([0-9]+(?:\.[0-9]+){1,3})(?:[-\s]|$)`)
var playerCountPattern = regexp.MustCompile(`(?i)There are ([0-9]+) of a max of [0-9]+ players online`)
var minecraftFormatting = regexp.MustCompile(`§[0-9A-FK-ORa-fk-or]|\x1b\[[0-9;]*m`)

func (a *App) pluginServerVersion() (string, error) {
	running, runningErr := a.pluginContainerRunning()
	if runningErr == nil && running {
		if version, err := a.livePluginMinecraftVersion(); err == nil {
			return version, nil
		}
	}
	if a.Config.MinecraftVersion == "" {
		return "", errors.New("plugin update: live/configured Minecraft version unavailable")
	}
	a.warnf("warning: live Minecraft version unavailable; resolving configured %s; apply will require matching live version\n", a.Config.MinecraftVersion)
	return a.Config.MinecraftVersion, nil
}

func parseLivePluginMinecraftVersion(output string) (string, error) {
	output = minecraftFormatting.ReplaceAllString(output, "")
	for _, pattern := range []*regexp.Regexp{liveMinecraftPattern, livePaperPattern} {
		if match := pattern.FindStringSubmatch(output); len(match) == 2 {
			return match[1], nil
		}
	}
	return "", errors.New("plugin update: cannot parse live Paper Minecraft version")
}

func (a *App) livePluginMinecraftVersion() (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		outputs, err := a.rconRun("version")
		if err != nil {
			return "", err
		}
		version, err := parseLivePluginMinecraftVersion(outputs[0])
		if err == nil {
			return version, nil
		}
		lastErr = err
		if !strings.Contains(strings.ToLower(outputs[0]), "checking version") {
			break
		}
		if attempt < 2 {
			time.Sleep(250 * time.Millisecond)
		}
	}
	return "", lastErr
}

func emptyPlayerCount(output string) error {
	match := playerCountPattern.FindStringSubmatch(minecraftFormatting.ReplaceAllString(output, ""))
	if len(match) != 2 {
		return errors.New("plugin update: cannot parse RCON player count")
	}
	count, err := strconv.Atoi(match[1])
	if err != nil || count != 0 {
		return fmt.Errorf("plugin update: server must have zero players (reported %s)", match[1])
	}
	return nil
}

func enabledPluginOutput(output string) error {
	// Bukkit uses green for enabled plugins, red for disabled plugins. Requiring
	// explicit green avoids treating a name in an error/disabled list as success.
	for _, name := range []string{"Geyser-Spigot", "ViaVersion", "ViaBackwards"} {
		pattern := regexp.MustCompile(`(?i)(?:§[a2]|\x1b\[(?:0;)?(?:32|92)m)` + regexp.QuoteMeta(name) + `(?:§|\x1b|[,\s]|$)`)
		if !pattern.MatchString(output) {
			return fmt.Errorf("plugin update: cannot confirm enabled plugin %s from RCON plugins", name)
		}
	}
	return nil
}

func (a *App) pluginPreflight(expectedVersion string) error {
	running, err := a.pluginContainerRunning()
	if err != nil {
		return err
	}
	if !running {
		return errors.New("plugin update: server must already be running")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	mount, err := a.Compose.MinecraftDataMount(ctx, a.Config.ContainerName)
	if err != nil {
		return err
	}
	actual, err := filepath.EvalSymlinks(mount)
	if err != nil || !filepath.IsAbs(mount) {
		return errors.New("plugin update: cannot verify container /data mount")
	}
	wanted, err := filepath.Abs(a.Config.DataDir)
	if err != nil {
		return err
	}
	wanted, err = filepath.EvalSymlinks(wanted)
	if err != nil {
		return err
	}
	if actual != wanted {
		return errors.New("plugin update: configured Minecraft data directory differs from running container /data mount")
	}
	version, err := a.livePluginMinecraftVersion()
	if err != nil {
		return err
	}
	if version != expectedVersion {
		return fmt.Errorf("plugin update: live Paper version does not match configured/staged Minecraft %s", expectedVersion)
	}
	outputs, err := a.rconRun("list", "plugins")
	if err != nil {
		return err
	}
	if err := emptyPlayerCount(outputs[0]); err != nil {
		return err
	}
	return enabledPluginOutput(outputs[1])
}

func requireSaveResponse(command, response string) error {
	response = strings.ToLower(minecraftFormatting.ReplaceAllString(response, ""))
	switch command {
	case "save-all flush":
		if strings.Contains(response, "saved the game") || strings.Contains(response, "saved the world") {
			return nil
		}
	case "save-off":
		if strings.Contains(response, "saving is now disabled") || strings.Contains(response, "saving is already turned off") || strings.Contains(response, "saving is already disabled") {
			return nil
		}
	case "save-on":
		if strings.Contains(response, "saving is now enabled") || strings.Contains(response, "saving is already turned on") || strings.Contains(response, "saving is already enabled") {
			return nil
		}
	}
	return fmt.Errorf("plugin update: cannot confirm successful %s response", command)
}

func (a *App) strictPluginBackup() (archive string, returnErr error) {
	client, err := a.DialRCON(a.Config.RCONAddr(), a.Config.RCONPassword, a.Config.RCONTimeout)
	if err != nil {
		return "", err
	}
	defer client.Close()
	paused := false
	defer func() {
		if paused {
			response, err := client.Command("save-on")
			if err == nil {
				err = requireSaveResponse("save-on", response)
			}
			if err != nil {
				returnErr = errors.Join(returnErr, fmt.Errorf("plugin update: restore autosave: %w", err))
			}
		}
	}()
	for _, command := range []string{"save-all flush", "save-off", "save-all flush"} {
		// Even an unrecognized save-off reply may have disabled autosave.
		if command == "save-off" {
			paused = true
		}
		response, err := client.Command(command)
		if err != nil {
			return "", fmt.Errorf("plugin update: %s: %w", command, err)
		}
		if err := requireSaveResponse(command, response); err != nil {
			return "", err
		}
	}
	opts := a.backupOptions(0)
	opts.Label += "-plugin-update-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	result, err := backup.Create(opts, a.Now())
	if err != nil {
		return "", err
	}
	if err := backup.VerifyArchive(result.ArchivePath); err != nil {
		return "", err
	}
	a.printf("verified full backup: %s\n", result.ArchivePath)
	return result.ArchivePath, nil
}

func (a *App) waitForPluginHealth(expected installedPlugins, bundle *pluginupdate.Bundle) error {
	attempts := a.PluginHealthAttempts
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	deadline := time.Now().Add(2 * time.Minute)
	for attempt := 0; attempt < attempts; attempt++ {
		lastErr = a.verifyPluginHealth(expected, bundle)
		if lastErr == nil {
			return nil
		}
		if time.Now().After(deadline) {
			break
		}
		if attempt+1 < attempts {
			time.Sleep(a.PluginHealthDelay)
		}
	}
	return fmt.Errorf("plugin update: startup health check failed: %w", lastErr)
}

func (a *App) verifyPluginHealth(expected installedPlugins, bundle *pluginupdate.Bundle) error {
	running, err := a.pluginContainerRunning()
	if err != nil {
		return err
	}
	if !running {
		return errors.New("Minecraft container is not running")
	}
	outputs, err := a.rconRun("list", "plugins", "version Geyser-Spigot", "version ViaVersion", "version ViaBackwards", "geyser version")
	if err != nil {
		return err
	}
	if !playerCountPattern.MatchString(minecraftFormatting.ReplaceAllString(outputs[0], "")) {
		return errors.New("RCON player listing is not ready")
	}
	if err := enabledPluginOutput(outputs[1]); err != nil {
		return err
	}
	for index, plugin := range expected {
		output := minecraftFormatting.ReplaceAllString(outputs[index+2], "")
		if !strings.Contains(strings.ToLower(output), "version "+strings.ToLower(plugin.Version)) {
			return fmt.Errorf("loaded %s version differs from expected %s", plugin.Name, plugin.Version)
		}
	}
	geyser := strings.ToLower(minecraftFormatting.ReplaceAllString(outputs[5], ""))
	if !strings.Contains(geyser, "geyser") || !strings.Contains(geyser, "version") {
		return errors.New("Geyser version command is not ready")
	}
	if bundle != nil {
		artifact := bundle.Artifacts[0]
		buildMatch := regexp.MustCompile(`/builds/([0-9]+)/`).FindStringSubmatch(artifact.URL)
		if len(buildMatch) != 2 || !strings.Contains(geyser, strings.ToLower(artifact.Version)) || !regexp.MustCompile(`(?i)(?:-b|build[ :#]*|\(git-master-)(?:`+regexp.QuoteMeta(buildMatch[1])+`)(?:[^0-9]|$)`).MatchString(geyser) {
			return errors.New("loaded Geyser build differs from staged metadata")
		}
		if !geyserSupportsTargetBedrock(geyser) {
			return errors.New("loaded Geyser does not report support for Bedrock 26.52")
		}
	}
	return nil
}

func (a *App) pluginContainerRunning() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return a.Compose.ContainerRunning(ctx, a.Config.ContainerName)
}

func (a *App) stopPluginContainer() error {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	_, err := a.Compose.StopMinecraft(ctx)
	return err
}

func (a *App) startPluginContainer() error {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	_, err := a.Compose.StartMinecraft(ctx)
	return err
}

func geyserSupportsTargetBedrock(output string) bool {
	versions := regexp.MustCompile(`([0-9]+\.[0-9]+(?:\.[0-9]+)?)(?:\s*(?:-|to)\s*([0-9]+\.[0-9]+(?:\.[0-9]+)?))?`)
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(strings.ToLower(line), "bedrock") {
			continue
		}
		for _, match := range versions.FindAllStringSubmatch(line, -1) {
			if match[2] == "" {
				if compareMinecraftVersions(match[1], "26.52") == 0 {
					return true
				}
				continue
			}
			if compareMinecraftVersions(match[1], "26.52") <= 0 && compareMinecraftVersions(match[2], "26.52") >= 0 {
				return true
			}
		}
	}
	return false
}

func compareMinecraftVersions(left, right string) int {
	l, r := strings.Split(left, "."), strings.Split(right, ".")
	for index := 0; index < 4; index++ {
		var a, b int
		if index < len(l) {
			a, _ = strconv.Atoi(l[index])
		}
		if index < len(r) {
			b, _ = strconv.Atoi(r[index])
		}
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
	}
	return 0
}

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/onecode182/nethernode/internal/pluginupdate"
)

func (a *App) applyGeyserUpdate(pluginsDir string, rollback bool) error {
	pending := filepath.Join(a.Config.PluginUpdateDir, ".rollback-pending")
	if _, err := os.Lstat(pending); err == nil {
		return fmt.Errorf("plugin update: interrupted transaction at %s requires operator recovery", pending)
	} else if !os.IsNotExist(err) {
		return err
	}
	current, err := inspectInstalledPlugins(pluginsDir)
	if err != nil {
		return err
	}
	if rollback {
		return a.rollbackGeyserUpdate(pluginsDir, current)
	}
	staged := filepath.Join(a.Config.PluginUpdateDir, "staged")
	bundle, err := pluginupdate.ReadStaged(staged)
	if err != nil {
		return err
	}
	var baseline installedPlugins
	if err := readPluginJSON(filepath.Join(staged, "baseline.json"), &baseline); err != nil {
		return err
	}
	if err := validateInstalledSet(baseline); err != nil {
		return err
	}
	if current != baseline {
		return errors.New("plugin update: installed JAR hash/version drift since staging; restage")
	}
	target, err := inspectInstalledPlugins(staged)
	if err != nil {
		return err
	}
	if current == target {
		a.printf("Geyser bundle already installed; no restart required\n")
		return nil
	}
	if err := rejectPluginDestinationCollisions(pluginsDir, current, target); err != nil {
		return err
	}
	if err := a.pluginPreflight(bundle.ServerVersion); err != nil {
		return err
	}
	if err := os.Mkdir(pending, 0o700); err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(pending)
		}
	}()
	for _, plugin := range current {
		if err := copyPluginFile(filepath.Join(pluginsDir, plugin.Filename), filepath.Join(pending, plugin.Filename)); err != nil {
			return err
		}
	}
	if err := verifyInstalledSet(pending, current); err != nil {
		return err
	}
	managedBefore, err := readManagedPluginRows(pluginsDir)
	if err != nil {
		return err
	}
	archive, err := a.strictPluginBackup()
	if err != nil {
		return err
	}
	transaction := pluginTransaction{Before: current, After: target, Backup: archive, ManagedBefore: managedBefore, ServerVersion: bundle.ServerVersion}
	if err := writePluginJSON(filepath.Join(pending, "transaction.json"), transaction); err != nil {
		return err
	}
	// Backups can take minutes. Recheck both baseline hashes and player count
	// immediately before the maintenance stop.
	afterBackup, err := inspectInstalledPlugins(pluginsDir)
	if err != nil {
		return err
	}
	if afterBackup != current {
		return errors.New("plugin update: JARs changed during backup; refusing to stop")
	}
	if err := a.pluginPreflight(bundle.ServerVersion); err != nil {
		return err
	}
	cleanup = false
	if err := a.stopPluginContainer(); err != nil {
		return fmt.Errorf("plugin update: stop failed; no JARs changed; recovery state retained at %s: %w", pending, err)
	}
	if err := a.confirmPluginContainerStopped(); err != nil {
		return err
	}
	applyErr := swapPluginSet(pluginsDir, staged, current, target)
	if applyErr == nil {
		applyErr = writeManagedPluginRows(pluginsDir, rowsForPlugins(target))
	}
	if applyErr == nil {
		applyErr = a.startPluginContainer()
	}
	if applyErr == nil {
		applyErr = a.waitForPluginHealth(target, &bundle)
	}
	if applyErr != nil {
		if recoveryErr := a.restorePluginTransaction(pluginsDir, pending, transaction); recoveryErr != nil {
			return fmt.Errorf("plugin update failed: %v; automatic JAR rollback failed: %w; server/recovery state requires operator inspection at %s", applyErr, recoveryErr, pending)
		}
		cleanup = true
		return fmt.Errorf("plugin update failed; previous JARs restored and healthy: %w", applyErr)
	}
	if err := finalizePluginRollback(a.Config.PluginUpdateDir, pending); err != nil {
		return fmt.Errorf("plugin update active, but finalize rollback state failed: %w", err)
	}
	a.printf("Geyser/ViaVersion/ViaBackwards bundle active; Java + Bedrock target-client login verification required\n")
	return nil
}

func (a *App) confirmPluginContainerStopped() error {
	running, err := a.pluginContainerRunning()
	if err != nil {
		return fmt.Errorf("plugin update: cannot verify stopped container: %w", err)
	}
	if running {
		return errors.New("plugin update: container still running; refusing JAR swap")
	}
	return nil
}

func (a *App) restorePluginTransaction(pluginsDir, recoveryDir string, transaction pluginTransaction) error {
	if err := a.stopPluginContainer(); err != nil {
		return err
	}
	if err := a.confirmPluginContainerStopped(); err != nil {
		return err
	}
	if err := swapPluginSet(pluginsDir, recoveryDir, transaction.After, transaction.Before); err != nil {
		return err
	}
	if err := writeManagedPluginRows(pluginsDir, transaction.ManagedBefore); err != nil {
		return err
	}
	if err := a.startPluginContainer(); err != nil {
		return err
	}
	return a.waitForPluginHealth(transaction.Before, nil)
}

func (a *App) rollbackGeyserUpdate(pluginsDir string, current installedPlugins) error {
	dir := filepath.Join(a.Config.PluginUpdateDir, "rollback")
	var transaction pluginTransaction
	if err := readPluginJSON(filepath.Join(dir, "transaction.json"), &transaction); err != nil {
		return err
	}
	if err := validateInstalledSet(transaction.After); err != nil {
		return err
	}
	if current != transaction.After {
		return errors.New("plugin update: active JARs drifted since update; refusing rollback")
	}
	if err := verifyInstalledSet(dir, transaction.Before); err != nil {
		return err
	}
	if err := rejectPluginDestinationCollisions(pluginsDir, current, transaction.Before); err != nil {
		return err
	}
	if transaction.ServerVersion == "" {
		return errors.New("plugin update: rollback state lacks Minecraft version")
	}
	if err := a.pluginPreflight(transaction.ServerVersion); err != nil {
		return err
	}
	if _, err := a.strictPluginBackup(); err != nil {
		return err
	}
	if err := a.pluginPreflight(transaction.ServerVersion); err != nil {
		return err
	}
	// Keep the currently active bundle until rollback itself is healthy.
	forward := filepath.Join(a.Config.PluginUpdateDir, ".rollback-pending")
	if err := os.Mkdir(forward, 0o700); err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(forward)
		}
	}()
	for _, plugin := range current {
		if err := copyPluginFile(filepath.Join(pluginsDir, plugin.Filename), filepath.Join(forward, plugin.Filename)); err != nil {
			return err
		}
	}
	if err := verifyInstalledSet(forward, current); err != nil {
		return err
	}
	managedForward, err := readManagedPluginRows(pluginsDir)
	if err != nil {
		return err
	}
	forwardTransaction := pluginTransaction{Before: current, After: transaction.Before, ManagedBefore: managedForward, ServerVersion: transaction.ServerVersion}
	if err := writePluginJSON(filepath.Join(forward, "transaction.json"), forwardTransaction); err != nil {
		return err
	}
	cleanup = false
	rollbackErr := a.restorePluginTransaction(pluginsDir, dir, transaction)
	if rollbackErr != nil {
		if recoveryErr := a.restorePluginTransaction(pluginsDir, forward, forwardTransaction); recoveryErr != nil {
			cleanup = false
			return fmt.Errorf("plugin rollback failed: %v; active-bundle recovery failed: %w; inspect %s", rollbackErr, recoveryErr, forward)
		}
		cleanup = true
		return fmt.Errorf("plugin rollback failed; previously active bundle restored: %w", rollbackErr)
	}
	if err := os.Remove(filepath.Join(dir, "transaction.json")); err != nil {
		return err
	}
	cleanup = true
	a.printf("previous Geyser/ViaVersion/ViaBackwards JARs restored; world/configs/Floodgate preserved\n")
	return nil
}

func rejectPluginDestinationCollisions(dir string, from, to installedPlugins) error {
	allowed := make(map[string]bool, len(from))
	for _, plugin := range from {
		allowed[plugin.Filename] = true
	}
	for _, plugin := range to {
		if allowed[plugin.Filename] {
			continue
		}
		if _, err := os.Lstat(filepath.Join(dir, plugin.Filename)); err == nil {
			return fmt.Errorf("plugin update: destination %s belongs to another artifact", plugin.Filename)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func finalizePluginRollback(root, pending string) error {
	rollback := filepath.Join(root, "rollback")
	previous := filepath.Join(root, ".previous-rollback")
	if _, err := os.Lstat(previous); err == nil {
		return errors.New("interrupted rollback-state rotation requires operator inspection")
	} else if !os.IsNotExist(err) {
		return err
	}
	hadPrevious := false
	if _, err := os.Lstat(rollback); err == nil {
		if err := os.Rename(rollback, previous); err != nil {
			return err
		}
		hadPrevious = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(pending, rollback); err != nil {
		if hadPrevious {
			_ = os.Rename(previous, rollback)
		}
		return err
	}
	if hadPrevious {
		return os.RemoveAll(previous)
	}
	return nil
}

func rowsForPlugins(plugins installedPlugins) [3]string {
	var rows [3]string
	for index, plugin := range plugins {
		rows[index] = string(plugin.Name) + "|" + plugin.Filename
	}
	return rows
}

func readManagedPluginRows(pluginsDir string) ([3]string, error) {
	var rows [3]string
	path := filepath.Join(pluginsDir, ".nethernode-plugins.state")
	body, err := readManagedStateFile(path)
	if os.IsNotExist(err) {
		return rows, nil
	}
	if err != nil {
		return rows, err
	}
	for _, row := range strings.Split(string(body), "\n") {
		name, _, found := strings.Cut(row, "|")
		if !found {
			continue
		}
		for index, wanted := range pluginNames {
			if name == string(wanted) {
				if rows[index] != "" {
					return rows, fmt.Errorf("plugin update: duplicate managed state row %s", name)
				}
				rows[index] = row
			}
		}
	}
	return rows, nil
}

func readManagedStateFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, errors.New("plugin update: unsafe managed state file")
	}
	return os.ReadFile(path)
}

func writeManagedPluginRows(pluginsDir string, rows [3]string) error {
	path := filepath.Join(pluginsDir, ".nethernode-plugins.state")
	body, err := readManagedStateFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var kept []string
	for _, row := range strings.Split(strings.TrimSuffix(string(body), "\n"), "\n") {
		if row == "" {
			continue
		}
		name, _, _ := strings.Cut(row, "|")
		managed := false
		for _, wanted := range pluginNames {
			if name == string(wanted) {
				managed = true
			}
		}
		if !managed {
			kept = append(kept, row)
		}
	}
	for index, row := range rows {
		if row == "" {
			continue
		}
		name, filename, found := strings.Cut(row, "|")
		if !found || name != string(pluginNames[index]) || filename != filepath.Base(filename) || strings.ContainsAny(filename, "/\\\r\n|") {
			return errors.New("plugin update: unsafe managed state row")
		}
		kept = append(kept, row)
	}
	file, err := os.CreateTemp(pluginsDir, ".plugin-state-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0o644); err != nil {
		file.Close()
		return err
	}
	if _, err := file.WriteString(strings.Join(kept, "\n") + "\n"); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

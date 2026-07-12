package com.onecode.nethernode.admin;

import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.UUID;
import org.bukkit.ChatColor;
import org.bukkit.command.Command;
import org.bukkit.command.CommandExecutor;
import org.bukkit.command.CommandSender;
import org.bukkit.command.TabCompleter;
import org.bukkit.entity.Player;
import org.bukkit.event.EventHandler;
import org.bukkit.event.EventPriority;
import org.bukkit.event.Listener;
import org.bukkit.event.entity.EntityDamageEvent;
import org.bukkit.plugin.java.JavaPlugin;

/**
 * Owns private-server admin protections that must persist across player death,
 * reconnects, and Paper restarts without changing world or player NBT data.
 */
public final class NetherNodeAdminPlugin extends JavaPlugin implements Listener, CommandExecutor, TabCompleter {
  private static final String IMMUNE_PLAYERS_KEY = "immune-player-uuids";
  private static final String DAMAGE_PERMISSION = "nethernode.damage";

  private final Set<UUID> immunePlayers = new HashSet<>();

  @Override
  public void onEnable() {
    saveDefaultConfig();
    loadImmunePlayers();
    getServer().getPluginManager().registerEvents(this, this);

    var command = getCommand("nethernode");
    if (command == null) {
      throw new IllegalStateException("nethernode command missing from plugin.yml");
    }
    command.setExecutor(this);
    command.setTabCompleter(this);
    getLogger().info("Loaded " + immunePlayers.size() + " persisted damage-immunity entrie(s).");
  }

  @Override
  public boolean onCommand(CommandSender sender, Command command, String label, String[] args) {
    if (!(sender instanceof Player player)) {
      sender.sendMessage(ChatColor.RED + "Only an in-game player can change their own damage immunity.");
      return true;
    }
    if (!player.hasPermission(DAMAGE_PERMISSION)) {
      player.sendMessage(ChatColor.RED + "You do not have permission: " + DAMAGE_PERMISSION);
      return true;
    }
    if (args.length != 2 || !"damage".equalsIgnoreCase(args[0])) {
      player.sendMessage(ChatColor.YELLOW + "Usage: /nethernode damage <off|on>");
      return true;
    }

    if ("off".equalsIgnoreCase(args[1])) {
      setDamageImmunity(player.getUniqueId(), true);
      player.sendMessage(ChatColor.GREEN + "Damage immunity enabled.");
      return true;
    }
    if ("on".equalsIgnoreCase(args[1])) {
      setDamageImmunity(player.getUniqueId(), false);
      player.sendMessage(ChatColor.YELLOW + "Damage immunity disabled.");
      return true;
    }

    player.sendMessage(ChatColor.YELLOW + "Usage: /nethernode damage <off|on>");
    return true;
  }

  @Override
  public List<String> onTabComplete(CommandSender sender, Command command, String alias, String[] args) {
    if (!sender.hasPermission(DAMAGE_PERMISSION)) {
      return List.of();
    }
    if (args.length == 1) {
      return prefixMatches(args[0], List.of("damage"));
    }
    if (args.length == 2 && "damage".equalsIgnoreCase(args[0])) {
      return prefixMatches(args[1], List.of("off", "on"));
    }
    return List.of();
  }

  @EventHandler(priority = EventPriority.HIGHEST, ignoreCancelled = true)
  public void cancelDamageForImmunePlayers(EntityDamageEvent event) {
    if (event.getEntity() instanceof Player player && immunePlayers.contains(player.getUniqueId())) {
      event.setCancelled(true);
    }
  }

  private void loadImmunePlayers() {
    for (String rawUUID : getConfig().getStringList(IMMUNE_PLAYERS_KEY)) {
      try {
        immunePlayers.add(UUID.fromString(rawUUID));
      } catch (IllegalArgumentException ignored) {
        getLogger().warning("Ignoring invalid UUID in " + IMMUNE_PLAYERS_KEY + ": " + rawUUID);
      }
    }
  }

  private void setDamageImmunity(UUID playerUUID, boolean enabled) {
    if (enabled) {
      immunePlayers.add(playerUUID);
    } else {
      immunePlayers.remove(playerUUID);
    }
    getConfig().set(IMMUNE_PLAYERS_KEY, immunePlayers.stream().map(UUID::toString).sorted().toList());
    saveConfig();
  }

  private static List<String> prefixMatches(String token, List<String> candidates) {
    String normalized = token.toLowerCase(java.util.Locale.ROOT);
    return candidates.stream().filter(candidate -> candidate.startsWith(normalized)).toList();
  }
}

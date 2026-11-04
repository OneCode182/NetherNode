# Minecraft Runtime Architecture

## Runtime

- Docker Compose service `minecraft`.
- Image: GHCR-published NetherNode wrapper based on `itzg/minecraft-server:stable-java25`.
- Type: `PAPER` by default (was `FABRIC` pre-V2).
- Versioned runtime defaults: `server/runtime.env`.
- Local persistent world: `./data/minecraft` (same volume across the Fabric->Paper switch; `world/`, `ops.json`, whitelist, bans, usercache, stats, player data preserved).
- Cloud persistent world: `/opt/nethernode/data/minecraft`.
- Java port: `25565/tcp`.
- Bedrock/Geyser port: `19132/udp`.
- `online-mode=false` kept during first Paper migration to preserve offline UUIDs; flipping to `true` is a separate risky step needing UUID mapping.

## Plugin Policy

- Paper plugins live in `/data/plugins`; leftover Fabric `mods/` is inert under Paper.
- Crossplay stack is managed: `server/plugins.manifest` declares Geyser-Spigot, Floodgate-Spigot (GeyserMC download API; Floodgate publishes no Spigot artifact on Modrinth), ViaVersion, ViaBackwards, and SkinsRestorer (Modrinth API, loader `paper`, game version from `MINECRAFT_VERSION`).
- `ops/plugins-sync.sh` (`nethernode plugins sync [--dry-run]`, `plugins list`) resolves, checksum-verifies, installs, and prunes superseded jars; state in `/data/plugins/.nethernode-plugins.state`.
- `MINECRAFT_MODRINTH_PROJECTS` stays empty: one management mechanism only.
- Geyser config template `server/config/geyser/config.yml` installs only when missing; Floodgate key persists at `/data/plugins/floodgate/key.pem`.
- SkinsRestorer needs no versioned config override: its `skinsrestorer.player` permission group defaults to all players, while `skinsrestorer.admin` remains operator-only. Its config and per-player skin data persist under `/data/plugins/SkinsRestorer` and are covered by backups.
- `NetherNodeAdmin` is a small built-in Paper plugin. The Docker build compiles it with Java 25 and Paper API `26.1.2`, then places its jar at image path `/plugins/NetherNodeAdmin.jar`; the itzg runtime synchronizes that into persistent `/data/plugins` on startup. `/nethernode damage off|on` stores only selected player UUIDs in `/data/plugins/NetherNodeAdmin/config.yml` and cancels damage events for them. Permission `nethernode.damage` defaults to OP; the in-game command only toggles the executing player's state.
- Switch/Bedrock compatibility depends on Geyser/Floodgate support for selected Minecraft version; Switch needs BedrockConnect-style DNS workaround.

## Paper Migration Safety

- Migration source of truth is the backup archive, not live hand-edits.
- Flow: `nethernode save-server` -> `nethernode backup-server --retention 5` -> restore backup into a staging target -> verify Paper/plugins/players -> replace live data only after pass.
- Preserve: `world/`, `world_nether/`, `world_the_end/`, `level.dat`, `playerdata/`, `stats/`, `advancements/`, `ops.json`, whitelist/bans, `usercache.json`, and `server.properties`.
- Treat Fabric `mods/` and Fabric-only config as rollback evidence, not active Paper runtime.
- Keep `online-mode=false` for the first Paper migration when the existing server used offline UUIDs. Moving to `online-mode=true` needs a separate UUID mapping migration; otherwise inventories, XP, locations, stats, advancements, and admin identities can drift.
- Rollback path: stop server, restore known-good backup into `/opt/nethernode/data/minecraft` with explicit `--target` and `--force`, then start and verify join/save.

## World Version Downgrade (Chunker)

Minecraft never downgrades worlds natively; use Chunker
(https://github.com/HiveGamesOSS/Chunker) on a COPY when a world from a newer
MC version must run on an older server (e.g. `26.2` world onto the
Geyser-supported `26.1.2`). Procedure proven 2026-07-07 (dev world -> aux):

1. Extract the newest backup locally; never work on live data.
2. `java -jar chunker-cli.jar -i world -f JAVA_26_1_2 -o output-world`
   (verify `level.dat` DataVersion dropped, e.g. 4903 -> 4790).
3. Chunker does NOT convert `players/`, `datapacks/`, or several
   `data/minecraft/*.dat` (scoreboard, wandering trader, ...): copy them from
   the source world and byte-patch their NBT `DataVersion` int to the target
   (safe across adjacent minor versions).
4. Known losses: unmapped entities are dropped (observed: WITCH,
   EXPERIENCE_ORB); advancements/stats have limited conversion.
5. On the target host: backup current world, stop, swap `world/`, copy
   `ops.json`/whitelist/usercache/ban lists, `chown -R 1000:1000`, start.
6. Verify: `Done preparing level` with no datafixer errors, `seed` matches,
   ops intact, mcstatus java+bedrock online, then `bluemap purge <map>`.
7. Upgrading back (older -> newer) is native: just bump `MINECRAFT_VERSION`.

## Local Workflow

1. Copy `.env.example` to `.env`.
2. Set `MINECRAFT_EULA=TRUE`.
3. Run `make up`; this syncs `server/runtime.env` into `.env`.
4. Validate with `make status`, `make logs`, `make observability`, and client join test.

## Cloud Workflow

1. GitHub publishes GHCR image on `main`.
2. `start-server` starts EC2 and runs `ops/start.sh`.
3. `stop-server` runs `ops/stop-safe.sh` and stops EC2.

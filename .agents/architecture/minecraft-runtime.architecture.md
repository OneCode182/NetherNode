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
- Crossplay stack is managed: `server/plugins.manifest` declares Geyser-Spigot, Floodgate-Spigot (GeyserMC download API; Floodgate publishes no Spigot artifact on Modrinth), ViaVersion, ViaBackwards (Modrinth API, loader `paper`, game version from `MINECRAFT_VERSION`).
- `ops/plugins-sync.sh` (`nethernode plugins sync [--dry-run]`, `plugins list`) resolves, checksum-verifies, installs, and prunes superseded jars; state in `/data/plugins/.nethernode-plugins.state`.
- `MINECRAFT_MODRINTH_PROJECTS` stays empty: one management mechanism only.
- Geyser config template `server/config/geyser/config.yml` installs only when missing; Floodgate key persists at `/data/plugins/floodgate/key.pem`.
- Switch/Bedrock compatibility depends on Geyser/Floodgate support for selected Minecraft version; Switch needs BedrockConnect-style DNS workaround.

## Local Workflow

1. Copy `.env.example` to `.env`.
2. Set `MINECRAFT_EULA=TRUE`.
3. Run `make up`; this syncs `server/runtime.env` into `.env`.
4. Validate with `make status`, `make logs`, `make observability`, and client join test.

## Cloud Workflow

1. GitHub publishes GHCR image on `main`.
2. `start-server` starts EC2 and runs `ops/start.sh`.
3. `stop-server` runs `ops/stop-safe.sh` and stops EC2.

# Minecraft Runtime Architecture

## Runtime

- Docker Compose service `minecraft`.
- Image: GHCR-published NetherNode wrapper based on `itzg/minecraft-server:stable-java25`.
- Type: `FABRIC` by default.
- Versioned runtime defaults: `server/runtime.env`.
- Local persistent world: `./data/minecraft`.
- Cloud persistent world: `/opt/nethernode/data/minecraft`.
- Java port: `25565/tcp`.
- Bedrock/Geyser future port: `19132/udp`.

## Mod Policy

- Server-side Fabric mods may work without client install.
- Client-side/content mods require players to install matching client mods.
- Switch/Bedrock compatibility depends on Geyser/Floodgate/ViaProxy support for selected Minecraft version.

## Local Workflow

1. Copy `.env.example` to `.env`.
2. Set `MINECRAFT_EULA=TRUE`.
3. Run `make up`; this syncs `server/runtime.env` into `.env`.
4. Validate with `make status`, `make logs`, `make observability`, and client join test.

## Cloud Workflow

1. GitHub publishes GHCR image on `main`.
2. `start-server` starts EC2 and runs `ops/start.sh`.
3. `stop-server` runs `ops/stop-safe.sh` and stops EC2.

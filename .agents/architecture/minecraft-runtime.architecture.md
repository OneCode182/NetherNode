# Minecraft Runtime Architecture

## Runtime

- Docker Compose service `minecraft`.
- Image: `itzg/minecraft-server`.
- Type: `FABRIC` by default.
- Persistent world: `./data/minecraft`.
- Java port: `25565/tcp`.
- Bedrock/Geyser future port: `19132/udp`.

## Mod Policy

- Server-side Fabric mods may work without client install.
- Client-side/content mods require players to install matching client mods.
- Switch/Bedrock compatibility depends on Geyser/Floodgate/ViaProxy support for selected Minecraft version.

## Local Workflow

1. Copy `.env.example` to `.env`.
2. Set `MINECRAFT_EULA=TRUE`.
3. Run `make up`.
4. Validate with `make status`, `make logs`, and client join test.

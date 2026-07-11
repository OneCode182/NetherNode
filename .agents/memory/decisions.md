# Decisions

## 2026-07-04 - Use EC2 direct for MVP

- Context: Small private Minecraft server, low cost, scheduled nights.
- Decision: Use stoppable EC2 + EBS + Docker Compose.
- Rationale: Lowest cost/complexity for persistent world and TCP/UDP.
- Consequence: Operator manages one VM; scripts handle safe stop and backup.

## 2026-07-04 - Graphify is navigation only

- Context: Harness needs low-token routing.
- Decision: Markdown docs remain authority; Graphify narrows discovery.
- Rationale: Semantic graph may be stale or missing without backend key.
- Consequence: Agents check `project/` and `architecture/` first.

## 2026-07-04 - Keep cloud architecture lean

- Context: Operator has AWS trial credits but wants minimal spend and quick manual start/stop.
- Decision: Use GHCR + one stoppable EC2 + Docker Compose; avoid ECS, Fargate, NLB, EFS, ECR, and Elastic IP for MVP.
- Rationale: Fewer AWS services lowers fixed cost and keeps later Azure migration simple.
- Consequence: GitHub workflows publish images and control EC2 through OIDC + SSM; EC2 runs only during play sessions.

## 2026-07-05 - c7i-flex.large under AWS Free Plan constraint

- Context: Account is AWS Free Plan (USD 90 credits); EC2 restricted to free-tier-eligible types. `t4g.medium` rejected at RunInstances (InvalidParameterCombination).
- Decision: Use `c7i-flex.large` (x86_64, 4 GiB) with x86_64 AL2023 AMI via new `ami_architecture` variable; keep repo default `t4g.small`/arm64. Reuse pre-existing account GitHub OIDC provider via `github_oidc_provider_arn` tfvar.
- Rationale: 4 GiB matches original t4g.medium intent; GHCR image is currently amd64-only, so x86 avoids a multi-arch rebuild. ~USD 0.085/h, within credits.
- Consequence: Higher hourly cost than Graviton; revisit t4g after account upgrade + multi-arch image build.

## 2026-07-06 - Paper 26.2 as default runtime

- Context: NetherNode V2 targets Java + Bedrock crossplay; Geyser/Floodgate/ViaVersion/ViaBackwards are Paper plugins, not Fabric mods.
- Decision: `MINECRAFT_TYPE=PAPER`, `MINECRAFT_VERSION=26.2` in `server/runtime.env`; same itzg base image and same `/data` volume; `online-mode=false` preserved.
- Rationale: Paper is the supported host for the crossplay plugin stack; itzg image switches type via env without touching world data.
- Consequence: `mods/` is inert; plugins land in `/data/plugins` (S2). Flipping `online-mode=true` stays a separate documented migration (UUID mapping risk).

## 2026-07-06 - Script-managed crossplay plugins over itzg MODRINTH_PROJECTS

- Context: Floodgate publishes no Spigot/Paper artifact on Modrinth (only fabric/neoforge; live API check); Geyser's latest (2.10.1 b1177) declares support only up to MC 26.1.x; itzg `MODRINTH_PROJECTS` cannot cover Floodgate and gives no offline dry-run.
- Decision: single mechanism `server/plugins.manifest` + `ops/plugins-sync.sh` (`nethernode plugins sync`); Geyser/Floodgate from download.geysermc.org v2 API (sha256), Via* from Modrinth v2 API (sha512, loader `paper`, `game_versions=[MINECRAFT_VERSION]`); `MINECRAFT_MODRINTH_PROJECTS` stays empty.
- Rationale: uniform pin/resolve/verify/prune semantics, `--dry-run` verifiable in CI-less contexts, direct port path to the Go CLI (S3+).
- Consequence: sync needs `curl` + `python3`; Bedrock join on 26.2 blocked upstream until Geyser ships 26.2 support (re-run sync); Via* 26.2 builds are SNAPSHOT channel.

## 2026-07-06 - Go CLI admin/settings writes local truth safely

- Context: Operators need `nethernode admin ...` and `nethernode settings ...` on the EC2 host without hand-editing JSON/properties files.
- Decision: Go CLI owns `ops.json` and `server.properties` editing with atomic writes; live changes use RCON when Minecraft exposes an immediate command, otherwise CLI writes file truth and reports restart/reload need.
- Rationale: keeps admin/settings repeatable, testable, dry-run capable, and independent from shell-only scripts.
- Consequence: `ops.json` fallback for new players assumes current V2 `online-mode=false` UUID mode; future `online-mode=true` migration must map UUIDs before adding new admins through offline file patching.

## 2026-07-06 - Azure stays an extension scaffold

- Context: NetherNode may migrate cloud later, but current MVP is AWS EC2 stoppable and must remain low-complexity.
- Decision: Add `infra/azure` as validate-only Terraform scaffold; do not add Azure workflows, secrets, or deploy behavior.
- Rationale: preserves portability without widening live operational surface or cost risk.
- Consequence: AWS remains the default path; Azure work maps the same Docker Compose/env/volume/CLI boundary to a VM/network shape only.

## 2026-07-11 - Manage offline-mode Java skins with SkinsRestorer

- Context: NetherNode runs `online-mode=false`; private Java players need persistent individual skins while Floodgate/Geyser preserves Bedrock profile skins.
- Decision: Add SkinsRestorer to the existing Modrinth-managed Paper plugin manifest. Keep its default player permission group; do not add LuckPerms or a custom plugin config.
- Rationale: SkinsRestorer `15.12.4` resolves for both aux `26.1.2` and repo-default `26.2`. Its `skinsrestorer.player` group is granted by default, so all normal Java players can set only their own skin without widening administration privileges.
- Consequence: Plugin/config/player skin data live in the existing persistent `/data/plugins` volume and are backed up with the world. Sync can prune only a superseded SkinsRestorer jar; it does not alter worlds or backups.

## 2026-07-11 - Status uses public DNS and container RCON first

- Context: mcstatus.io is external and cannot reach EC2-local `localhost`; direct host-side TCP RCON can reset after Paper/image changes even when image `rcon-cli` works.
- Decision: `MINECRAFT_STATUS_HOST` selects a public DNS/IP target, with legacy `MINECRAFT_PUBLIC_HOST` fallback. Standard Java/Bedrock ports omit `:port` in mcstatus.io paths. `nethernode status` calls `docker exec <container> rcon-cli list` before TCP RCON fallback.
- Rationale: status matches the public player path and uses the runtime's known-good RCON client without changing save/admin operations.
- Consequence: EC2 must set `MINECRAFT_STATUS_HOST=oneminecraft.duckdns.org`; colorized human output is terminal-aware, while JSON stays machine-safe.

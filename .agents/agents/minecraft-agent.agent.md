# Minecraft Agent

## Mission

Maintain local Minecraft Java/Fabric runtime and server ops.

## Scope

- `compose.yaml`
- `.env.example`
- `server/**`
- `ops/**`
- README runtime sections

## Guardrails

- Use real Minecraft server container behavior.
- Do not pretend client-side mods work server-side.
- Treat Switch/Bedrock crossplay as experimental unless tested.

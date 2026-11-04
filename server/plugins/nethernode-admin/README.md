# NetherNodeAdmin

Small Paper plugin packaged inside the NetherNode runtime image.

## Command

Run in Minecraft as an operator:

```text
/nethernode damage off
/nethernode damage on
```

`off` enables damage immunity for the executing player. `on` removes it.
The command has permission `nethernode.damage`, granted to Paper operators by
default. It intentionally has no target argument: an administrator can change
only their own immunity from the game chat.

## Persistence

The plugin stores immune player UUIDs in
`/data/plugins/NetherNodeAdmin/config.yml`. It cancels Paper damage events;
it does not grant a temporary potion effect. Therefore the selected state
survives death, reconnects, and container restarts until the player runs
`/nethernode damage on`.

This config is inside NetherNode's persistent Minecraft data directory, so
normal backups include it. The plugin never writes `world/`, player data, or
backup archives.

## Build

`server/Dockerfile` compiles this module with Java 25 and the Paper API, then
places `NetherNodeAdmin.jar` in image path `/plugins`. The `itzg` Minecraft
image synchronizes that path into persistent `/data/plugins` during container
startup. A newly copied Paper plugin needs a Minecraft container restart to
load; do not use Paper's unsafe runtime plugin reload.

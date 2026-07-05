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

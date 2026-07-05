# Dynamic Workflows - Codex

## Orquestacion Obligatoria

Usa subagentes de Codex cuando el entorno los exponga y el usuario autorice trabajo paralelo.

## Modelo Lider

- Modelo: Codex lider actual.
- Effort: `medium` cuando sea configurable.
- Rol: lider orquestador, reviewer, integrador y decisor final.

## Subagentes

- Default: `gpt-5.3-codex-spark`, `xhigh effort`.
- Tareas simples: `gpt-5.4-mini`.
- Nunca usar subagentes con el mismo modelo del lider si existe alternativa disponible.
- Escalar como maximo a `gpt-5.5`, `xhigh effort`, solo si hay bloqueo tecnico fuerte o decision critica que el subagente default no resuelve.
- Antes de escalar a `gpt-5.5`, documentar razon tecnica concreta.

## Estrategia

1. Agente lider lee harness, repo, CI actual, frontend actual e infra actual.
2. Agente lider crea planning de implementacion y planning de commits antes de tocar archivos.
3. Subagentes pueden investigar AWS, Terraform, GitHub Actions y frontend CI en paralelo.
4. Agente lider integra resultados, decide arquitectura final y ejecuta cambios.

## Reglas De Operacion

- Usar `multi_agent_v1.spawn_agent` solo para subtareas concretas, acotadas e independientes.
- Usar `explorer` para investigacion read-only.
- Usar `worker` solo cuando haya scope de archivos claro y disjunto.
- No duplicar trabajo entre lider y subagentes.
- No delegar trabajo que bloquee el siguiente paso inmediato del lider.
- Subagentes no deciden arquitectura final; solo proponen con evidencia.
- El lider revisa resultados antes de integrar cambios.
- Cambios deben ser minimos, atomicos y alineados al producto NetherNode.
- No ejecutar `terraform apply`, crear recursos AWS, hacer push, ni exponer secretos sin aprobacion humana explicita.

## Salida Esperada Del Lider

- Plan de implementacion.
- Plan de commits atomicos.
- Matriz de subagentes y responsabilidades.
- Evidencia usada.
- Checks ejecutados.
- Riesgos restantes.
- Commits realizados o plan de commits pendiente.

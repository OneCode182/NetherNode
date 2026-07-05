# Dynamic Workflows - Claude Code

## Orquestacion Obligatoria

Usa `dynamic-workflows` de Claude Code.

## Modelo Lider

- Modelo: `Fable 5`.
- Effort: `medium`.
- Rol: lider orquestador, reviewer, integrador y decisor final.

## Subagentes

- Default: `Sonnet 5`, `xhigh effort`.
- Tareas simples: `Haiku`.
- Nunca usar subagentes `Fable 5`.
- Maximo `Opus` en `max effort` solo si hay bloqueo tecnico fuerte o decision critica que `Sonnet 5` no resuelve.
- Antes de escalar a `Opus`, documentar razon tecnica concreta.

## Estrategia

1. Agente lider lee harness, repo, CI actual, frontend actual e infra actual.
2. Agente lider crea planning de implementacion y planning de commits antes de tocar archivos.
3. Subagentes pueden investigar AWS, Terraform, GitHub Actions y frontend CI en paralelo.
4. Agente lider integra resultados, decide arquitectura final y ejecuta cambios.

## Reglas De Operacion

- Subagentes investigan y proponen; el lider decide.
- Subagentes no amplian scope sin permiso del lider.
- Subagentes reportan evidencia, archivos revisados, checks corridos, riesgos y bloqueos.
- El lider elimina contradicciones entre reportes antes de editar.
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

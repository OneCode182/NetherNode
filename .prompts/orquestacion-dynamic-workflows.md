# Orquestación Obligatoria - Dynamic Workflows

Usa `dynamic-workflows` de Claude Code.

## Regla De Oro

Usa `$caveman ultra` para líder y subagentes.

Comunicación: corta, técnica, sin relleno. Sustancia completa. No perder precisión.

## Modelo Líder Orquestador

- Modelo: `GPT 5.5`
- Esfuerzo: `xhigh` / `Extra High`
- Rol: orquestador, planner, reviewer, decisor final.
- El líder no escribe código directamente.
- El líder sí:
  - lee harness, repo, CI, frontend e infra;
  - define plan de implementación;
  - define plan de commits;
  - asigna subtareas;
  - revisa trabajo de subagentes;
  - resuelve bloqueos;
  - toma decisiones finales;
  - integra resultado final.

## Subagentes

Default:
- Modelo: `5.3-codex-spark`
- Esfuerzo: `high` o `medium`

Tareas complejas:
- Modelo: `5.4` o `5.5`
- Esfuerzo: `medium`
- Usar solo cuando `5.3-codex-spark` no baste por complejidad, precisión, integración o ejecución.

Reglas:
- Subagentes no deciden.
- Subagentes no infieren scope.
- Subagentes ejecutan solo lo que líder asigna.
- Si subagente queda bloqueado, pregunta al líder.
- Líder replantea subtarea, actualiza plan y subagente continúa.
- Cada subagente debe tener meta `/goal` clara, verificable y acotada.
- Subagentes deben trabajar en paralelo cuando tareas sean independientes.
- Subagentes deben compartir hallazgos entre ellos vía `dynamic-workflows` para evitar duplicación y contradicción.

## Estrategia Obligatoria

1. Líder lee contexto real:
   - harness;
   - repo;
   - CI actual;
   - frontend actual;
   - infra actual;
   - docs relevantes.

2. Líder crea antes de tocar archivos:
   - planning de implementación;
   - planning de commits;
   - matriz de subtareas;
   - criterios de aceptación;
   - riesgos y bloqueos esperados.

3. Subagentes investigan/ejecutan en paralelo según asignación del líder:
   - rúbrica y fuentes del curso;
   - estado real backend/API;
   - Admin UI, frontend CI e infra AWS;
   - workflow n8n y arquitectura conversacional;
   - bibliografía y diagramas;
   - pruebas, validación y evidencia.

4. Líder integra resultados:
   - compara hallazgos;
   - elimina contradicciones;
   - decide edición final;
   - mantiene scope controlado;
   - exige evidencia real.

5. Cambios:
   - evitar cambios masivos no justificados;
   - mejorar calidad, conexión y evidencia;
   - no rehacer todo desde cero;
   - preservar estructura útil existente;
   - tocar solo archivos necesarios.

## Flujo De Trabajo

1. Crear `/goal` global del líder.
2. Leer repo/contexto.
3. Crear plan maestro.
4. Crear `/goal` por subagente.
5. Lanzar subagentes en paralelo.
6. Subagentes reportan:
   - qué hicieron;
   - evidencia;
   - archivos tocados;
   - tests/checks corridos;
   - bloqueos.
7. Líder revisa.
8. Si hay bloqueo:
   - líder documenta causa técnica concreta;
   - líder replantea subtarea;
   - subagente continúa.
9. Líder valida integración final.
10. Líder arma commits según planning.

## Política De Decisión

- Solo líder decide.
- Subagentes pueden proponer, pero no cambiar dirección sin aprobación.
- Si hay conflicto entre subagentes, líder decide usando evidencia del repo.
- Si falta evidencia, líder ordena investigación adicional.
- Si decisión afecta arquitectura, CI, API, infra o entregable final, líder documenta razón.

## Criterios De Calidad

- Cambios mínimos suficientes.
- Evidencia antes que opinión.
- Tests o checks cuando apliquen.
- No romper CI.
- No inventar estado del backend/API/infra.
- No alterar diseño, arquitectura o docs sin razón explícita.
- Resultado final debe quedar listo para review humano.

## Output Final Esperado

Líder entrega:
- resumen corto;
- archivos cambiados;
- decisiones tomadas;
- evidencia usada;
- tests/checks ejecutados;
- riesgos restantes;
- plan de commits o commits realizados, según permiso.

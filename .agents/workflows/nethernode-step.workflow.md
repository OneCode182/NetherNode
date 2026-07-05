# NetherNode Step Workflow

Each step must close this loop before next step starts:

1. Idea + Diseno Base
2. Implementacion
3. Testeo + Verificacion
4. Evaluacion + Correcciones
5. Documentacion
6. Commit Atomico

## Failure Routing

- Test fails -> return to Implementacion.
- Design mismatch -> return to Idea + Diseno Base.
- Docs stale -> return to Documentacion.
- Same failure twice -> write escalation in task file.
- No next step until verification passes or skip is documented.

## Commit Gate

Use `.agents/protocols/atomic-commit.protocol.md` and `atomic-commit-helper`.

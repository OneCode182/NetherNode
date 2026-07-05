# Verification Retry Protocol

## Loop

1. Capture exact failure.
2. Classify: implementation, design, docs, environment, external dependency.
3. Fix root cause in scope.
4. Re-run failed check and dependent checks.
5. Record evidence in active task.

## Escalation

After same failure twice:

```md
VERIFICATION_ESCALATION:
- Failed check:
- Attempts:
- Root cause:
- Evidence:
- Remaining options:
- Required human decision:
```

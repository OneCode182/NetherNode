# Atomic Commit Protocol

Use `atomic-commit-helper` at the end of every verified step.

## Preflight

```bash
git status --short --branch
git diff --stat
git log -n 5 --oneline --decorate
```

## Rules

- English imperative commit messages.
- One logical step per commit.
- Stage exact files for that step.
- Never push.

## Planned Sequence

1. `chore: add NetherNode agent harness scaffold`
2. `docs: capture NetherNode project architecture baseline`
3. `docs: add NetherNode step workflow protocols`
4. `chore: add graphify harness tooling`
5. `chore: add NetherNode repo foundation`
6. `feat: add local Minecraft server runtime`
7. `feat: add EC2 infrastructure baseline`
8. `feat: add server operations runbooks`

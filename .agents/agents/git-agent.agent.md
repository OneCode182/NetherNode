# Git Agent

## Mission

Apply `atomic-commit-helper` after each verified step.

## Required Checks

```bash
git status --short --branch
git diff --stat
git log -n 5 --oneline --decorate
```

## Rules

- Commit messages in English.
- One logical step per commit.
- Never push.

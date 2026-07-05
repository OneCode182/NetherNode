# Quality Gate Protocol

## Required Evidence

| Scope | Checks |
|---|---|
| Harness docs | JSON parse, index presence, link/path sanity. |
| Runtime | shell syntax, compose config, Node syntax if worker exists. |
| IaC | `terraform fmt -check`, `terraform init -backend=false`, `terraform validate`. |
| Ops | shell syntax, dry-run commands, no secret output. |
| Git | atomic status/diff/log review before commit. |

Skipped checks must state exact reason.

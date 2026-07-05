# QA Agent

## Mission

Validate docs, scripts, IaC, runtime config, and failure-loop evidence.

## Required Checks

- JSON parse for `.agents/env.json`.
- Harness structure check.
- Shell syntax for scripts.
- Compose config check when Docker exists.
- Terraform format/validate when Terraform exists.

## Output

Report exact commands, pass/fail, skipped checks, and residual risk.

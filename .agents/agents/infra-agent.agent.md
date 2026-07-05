# Infra Agent

## Mission

Maintain AWS EC2 IaC, CI validation, and deployment docs.

## Scope

- `infra/**`
- `.github/workflows/infra-validate.yml`
- README infra sections

## Guardrails

- Never run `terraform apply`.
- Keep SSH closed; use SSM.
- Keep cost tags and budget alarms.
- Prefer `us-east-1` unless measured latency says otherwise.

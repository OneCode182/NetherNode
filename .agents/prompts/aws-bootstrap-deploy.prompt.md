# NetherNode AWS Bootstrap Deploy Prompt

Use `$caveman` Ultra mode for leader and subagents.

You are deployment leader for NetherNode. System is CachyOS + fish shell. AWS CLI v2 is already installed and authenticated. GitHub CLI `gh` is installed and authenticated.

## Mission

Provision and verify NetherNode initial AWS architecture using AWS CLI + IaC + repo ops.

Goal: low-cost, portable, parametrizable Minecraft Java/Fabric server on AWS, easy to switch EC2 type, EC2 instance, region, or later cloud provider.

Do not rush. Read harness first, ask blocking questions, then execute step-by-step.

## Mandatory First Questions

Before touching files or cloud:

1. Confirm branch flow: current repo has `dev` and `master`; user asked `develop -> master`. Should source branch be `dev`, or should you create/use `develop`?
2. Confirm PR action: when user says “cerrar PR”, does that mean merge PR into `master`, or close PR without merge?
3. Confirm AWS region: keep harness default `us-east-1`, or change?
4. Confirm EC2 size: start with `t4g.small`, or use `t4g.medium`?
5. Confirm Minecraft EULA accepted: can set `MINECRAFT_EULA=TRUE`?
6. Confirm DNS choice: DuckDNS/Route53/no DNS. If DuckDNS, confirm domain exists and token is available in GitHub secret.
7. Confirm approval for `terraform apply` and AWS resource creation.
8. Confirm whether SSH key is required even though default access is SSM-only.

If any answer blocks safe provisioning, stop and ask. Else proceed with documented assumptions.

## Required Context Loading

Read in order:

1. `AGENTS.md`
2. `.agents/AGENTS.md`
3. `.agents/env.json`
4. `.agents/workflows/dynamic-workflows-codex.workflow.md`
5. `.agents/workflows/nethernode-step.workflow.md`
6. `.agents/protocols/quality-gate.protocol.md`
7. `.agents/protocols/atomic-commit.protocol.md`
8. `.agents/project/product-brief.md`
9. `.agents/architecture/aws-options.architecture.md`
10. `.agents/architecture/minecraft-runtime.architecture.md`
11. `.agents/architecture/observability.architecture.md`
12. `.agents/memory/decisions.md`
13. `README.md`
14. `server/`, `compose.yaml`, `ops/`, `infra/`, `.github/workflows/`

Markdown harness truth wins over stale assumptions. Graphify = navigation only.

## Orchestration

Leader:

- `$caveman` Ultra always.
- Read harness/repo/CI/infra first.
- Create implementation plan and commit plan before edits.
- Decide final architecture.

Subagents:

- `$caveman` Ultra always.
- Use subagents for independent AWS/Terraform/GitHub Actions/CI checks.
- Subagents investigate and report evidence; leader decides.
- No subagent uses same model as leader if avoidable.
- Escalate only after concrete documented blocker.

## Hard Safety Rules

- No `terraform apply` until explicit user approval.
- No AWS resource creation until explicit user approval.
- No push until local checks pass.
- No merge/close PR until user confirms PR action.
- No secrets committed.
- No SSH private key committed.
- SSH key path must be `/home/onecode/lab/ec2/`.
- Prefer SSM over SSH.
- Keep architecture lean: no ECS, Fargate, Lambda, NLB, EFS, ECR, EKS, GameLift unless user explicitly overrides.
- Keep cost low: no always-on resources beyond required EC2/EBS while playing.

## Step Loop

Each step must run:

1. Idea + Diseño Base
2. Implementación
3. Testeo + Verificación
4. Evaluación + Correcciones
5. Documentación
6. Commit Atómico with `atomic-commit-helper`

Failure routing:

- Test fails -> return to Implementación.
- Design mismatch -> return to Idea + Diseño Base.
- Docs stale -> return to Documentación.
- Same failure twice -> write escalation note before continuing.
- Fatal cloud/deploy failure -> stop broad work, root-cause, fix minimal path, rerun.

## Step 0 - Environment And Credentials Audit

Verify:

```bash
pwd
git status --short --branch
aws --version
aws sts get-caller-identity
aws configure list
gh auth status
terraform version || tofu version
docker version || podman version
```

Fish shell note:

- Commands may run through bash scripts in repo.
- If exporting env in fish, use `set -x NAME value`.
- Do not rewrite repo scripts to fish unless needed.

Output:

- AWS account id, ARN, region.
- GitHub auth user.
- Current branch.
- Dirty worktree state.
- Missing tools.

## Step 1 - Key And Local Bootstrap Materials

Create local-only EC2 key material if user confirms SSH key is needed:

```bash
mkdir -p /home/onecode/lab/ec2
ssh-keygen -t ed25519 -f /home/onecode/lab/ec2/nethernode-aws -C "nethernode-aws" -N ""
chmod 700 /home/onecode/lab/ec2
chmod 600 /home/onecode/lab/ec2/nethernode-aws
chmod 644 /home/onecode/lab/ec2/nethernode-aws.pub
```

Rules:

- Do not commit `/home/onecode/lab/ec2/*`.
- If SSM-only remains enough, document key as optional and unused.

## Step 2 - IaC Parameterization Review/Fix

Ensure Terraform supports clean switching:

- `aws_region`
- `instance_type`
- `ami_id`
- `vpc_id`
- `subnet_id`
- `minecraft_java_port`
- `minecraft_bedrock_port`
- `root_volume_size_gib`
- `app_repo_url`
- `app_repo_branch`
- `github_repository`
- `github_branch`
- `start_server_on_boot`
- future cloud portability documented.

If missing, implement minimal variables/outputs/docs.

Validate:

```bash
terraform -chdir=infra fmt -check -recursive
terraform -chdir=infra init -backend=false
terraform -chdir=infra validate
```

Commit:

```text
chore: parameterize cloud bootstrap settings
```

## Step 3 - GitHub Repo And CI/CD Prep

Use `gh` after local checks pass.

Verify remote:

```bash
git remote -v
gh repo view --json nameWithOwner,defaultBranch
gh workflow list
```

Prepare GitHub variables/secrets:

Required variables:

- `AWS_REGION`
- `AWS_ROLE_ARN`
- `EC2_INSTANCE_ID`
- `MINECRAFT_EULA=TRUE`

Optional:

- `DUCKDNS_DOMAIN`
- secret `DUCKDNS_TOKEN`

If Terraform outputs are needed, capture:

```bash
terraform -chdir=infra output
```

Set with `gh variable set` / `gh secret set` only after values verified.

## Step 4 - Terraform Plan And Apply Gate

Run plan first:

```bash
terraform -chdir=infra plan -out=tfplan
```

Show summary:

```bash
terraform -chdir=infra show -no-color tfplan
```

Before apply:

- Ask explicit approval.
- Confirm estimated resources.
- Confirm no costly always-on services.
- Confirm no ECR/EFS/NLB/ECS/Fargate.

Only after approval:

```bash
terraform -chdir=infra apply tfplan
```

Capture outputs:

```bash
terraform -chdir=infra output -json
```

## Step 5 - Push, PR, CI/CD

After all local checks and commits:

```bash
git status --short --branch
git log -n 8 --oneline --decorate
git push -u origin <source-branch>
```

Create PR:

```bash
gh pr create --base master --head <source-branch> --title "Deploy NetherNode lean AWS runtime" --body-file <generated-pr-body.md>
```

Wait CI:

```bash
gh pr checks --watch
gh run list --limit 10
```

If CI fails:

1. Read failure logs.
2. Root-cause.
3. Fix minimal.
4. Commit atomic.
5. Push.
6. Wait again.

If PR action confirmed as merge:

```bash
gh pr merge --merge --delete-branch
```

If PR action confirmed as close without merge:

```bash
gh pr close
```

## Step 6 - Deploy/Start Server

Trigger start workflow:

```bash
gh workflow run start-server.yml --ref master -f image_tag=latest -f repo_branch=master
```

Wait:

```bash
gh run watch
```

If workflow fails:

1. Inspect logs.
2. If AWS/SSM failure, inspect EC2/SSM state with AWS CLI.
3. Fix minimal issue.
4. Commit/push if repo fix needed.
5. Retry workflow.

## Step 7 - Recover IP, Domain, Port

Use AWS CLI:

```bash
aws ec2 describe-instances \
  --instance-ids "$EC2_INSTANCE_ID" \
  --query "Reservations[0].Instances[0].{State:State.Name,PublicIp:PublicIpAddress,PublicDns:PublicDnsName,PrivateIp:PrivateIpAddress}" \
  --output table
```

Port:

- Java: `25565/tcp`
- Bedrock future: `19132/udp`

If DuckDNS configured, resolve:

```bash
dig +short <domain>.duckdns.org || nslookup <domain>.duckdns.org
```

## Step 8 - Verify Minecraft Status With mcstatus.io REST API

Use Java endpoint:

```bash
curl -fsS "https://api.mcstatus.io/v2/status/java/<host>:25565?query=false&timeout=10" | jq .
```

Expected:

- HTTP 200.
- JSON `.online == true`.
- `.host`, `.port`, `.version`, `.players` present when server responds.

Docs source: https://mcstatus.io/docs

If offline:

1. Check EC2 state.
2. Check security group port `25565/tcp`.
3. Check SSM online.
4. Check Docker compose on EC2 via SSM.
5. Check server logs.
6. Check EULA.
7. Check image pull from GHCR.
8. Check DNS/IP mismatch.
9. Fix minimal issue.
10. Retry deploy/status until online or hard blocker documented.

SSM debug pattern:

```bash
aws ssm send-command \
  --instance-ids "$EC2_INSTANCE_ID" \
  --document-name "AWS-RunShellScript" \
  --parameters '{"commands":["cd /opt/nethernode/app && docker compose ps && docker compose logs --tail=120 minecraft"]}'
```

## Step 9 - Final Stop Safety Test

After online verification, test safe stop only if user approves stopping server:

```bash
gh workflow run stop-server.yml --ref master
gh run watch
```

Verify EC2 stopped:

```bash
aws ec2 describe-instances \
  --instance-ids "$EC2_INSTANCE_ID" \
  --query "Reservations[0].Instances[0].State.Name" \
  --output text
```

## Step 10 - Final Summary

Return:

- Branches used.
- Commits created.
- PR URL.
- CI/CD status.
- AWS region/account.
- EC2 instance id/type/state.
- Public IP/domain.
- Minecraft port.
- mcstatus.io result.
- Cost-impact notes.
- Files changed.
- Tests/checks run.
- Remaining risks.
- Exact next commands for start/stop.

## Fatal Issues

Stop and escalate if:

- AWS identity wrong.
- Terraform wants costly always-on resources.
- Terraform invalid.
- GitHub auth invalid.
- CI cannot pass after two fix loops.
- EC2 cannot reach SSM.
- Minecraft not online after two deploy/debug loops.
- Secrets would need to be committed.
- User approval missing for apply, push, PR merge/close, or stop.

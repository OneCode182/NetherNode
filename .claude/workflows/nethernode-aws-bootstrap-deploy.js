export const meta = {
  name: 'nethernode-aws-bootstrap-deploy',
  description: 'Bootstrap NetherNode AWS: IaC edits, commits, PR+CI+merge, terraform plan/apply gate, GH vars, start server, verify mcstatus',
  phases: [
    { title: 'Prep', detail: 'IaC edits + repo/GHCR recon in parallel' },
    { title: 'Commit+PR', detail: 'atomic commits, push, PR, CI, merge' },
    { title: 'Provision', detail: 'terraform plan -> verify gate -> apply' },
    { title: 'Wire', detail: 'GitHub vars/secrets from outputs' },
    { title: 'Deploy', detail: 'start-server workflow + debug loop' },
    { title: 'Verify', detail: 'IP + mcstatus.io online check' },
  ],
}

const REPO = '/home/onecode/lab/NetherNode'
const CAVEMAN = 'Communicate caveman-ultra: terse, technical, zero fluff, full substance.'
const RULES = `HARD RULES: never print secret/token/passphrase values. No AWS resource creation outside the gated terraform apply step. No terraform destroy. Work only in ${REPO} unless told. ${CAVEMAN}`

const STEP = {
  type: 'object', required: ['ok', 'summary', 'evidence', 'blockers'],
  properties: {
    ok: { type: 'boolean' },
    summary: { type: 'string' },
    evidence: { type: 'array', items: { type: 'string' }, description: 'commands run with exit codes / key outputs' },
    blockers: { type: 'array', items: { type: 'string' } },
    data: { type: 'object', description: 'step-specific key/values' },
  },
}

// ---------- Phase 1: Prep (parallel) ----------
phase('Prep')

const [edits, recon] = await parallel([
  () => agent(`${RULES}
You are the IaC edit executor for NetherNode. Leader already decided ALL edits — apply them EXACTLY, nothing more. Repo: ${REPO}.

EDIT 1 — infra/variables.tf:
- change default of variable "app_repo_branch" from "main" to "master"
- change default of variable "github_branch" from "main" to "master"
- append new variable:
variable "ssh_public_key" {
  description = "Optional SSH public key material. Empty skips key pair creation."
  type        = string
  default     = ""
}

EDIT 2 — infra/main.tf:
- in resource "aws_iam_openid_connect_provider" "github": replace thumbprint_list = [] with thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1", "1c58a3a8518e8759bf075b76b750d4f2df264fcd"]
- add resource:
resource "aws_key_pair" "app" {
  count      = var.ssh_public_key != "" ? 1 : 0
  key_name   = "\${var.project_name}-\${var.environment}-key"
  public_key = var.ssh_public_key
}
- in resource "aws_instance" "app" add attribute: key_name = var.ssh_public_key != "" ? aws_key_pair.app[0].key_name : null

EDIT 3 — .github/workflows/image.yml: change branch trigger "- main" to "- master" AND the expression refs/heads/main to refs/heads/master.

EDIT 4 — .github/workflows/start-server.yml: change repo_branch input default from "main" to "master".

EDIT 5 — .gitignore: append lines:
infra/*.tfvars
infra/terraform.tfstate*
infra/tfplan

EDIT 6 — create ${REPO}/infra/terraform.tfvars (LOCAL ONLY, now gitignored) with EXACTLY:
instance_type              = "t4g.medium"
minecraft_eula_accepted    = true
github_repository          = "OneCode182/NetherNode"
github_branch              = "master"
app_repo_branch            = "master"
ssh_public_key             = "<paste the single-line content of /home/onecode/lab/ec2/nethernode-aws.pub>"
budget_notification_emails = ["dive2365@gmail.com"]

VALIDATE (report every exit code):
terraform -chdir=${REPO}/infra fmt -recursive && terraform -chdir=${REPO}/infra fmt -check -recursive
terraform -chdir=${REPO}/infra init -backend=false
terraform -chdir=${REPO}/infra validate
python3 -c "import yaml,glob; [yaml.safe_load(open(f)) for f in glob.glob('${REPO}/.github/workflows/*.yml')]"
git -C ${REPO} check-ignore infra/terraform.tfvars  (must exit 0)
ok=true only if all validations pass.`,
    { label: 'edit:iac', phase: 'Prep', schema: STEP, model: 'sonnet', effort: 'xhigh' }),

  () => agent(`${RULES}
Read-only GitHub recon for NetherNode deploy. Run and report:
1. gh repo view OneCode182/NetherNode --json nameWithOwner,defaultBranchRef,visibility
2. gh variable list -R OneCode182/NetherNode ; gh secret list -R OneCode182/NetherNode
3. GHCR package check: gh api user/packages/container/nethernode -q '.visibility' (may 404 if never published or token lacks scope — report exact result, not fatal)
4. gh workflow list -R OneCode182/NetherNode
5. gh run list -R OneCode182/NetherNode --limit 5
Set data.visibility (repo), data.default_branch, data.ghcr_visibility ('public'|'private'|'unknown'), data.has_duckdns_domain (bool, DUCKDNS_DOMAIN in variable list), data.has_duckdns_token (bool, DUCKDNS_TOKEN in secret list).
CRITICAL: if repo visibility is PRIVATE add blocker 'repo private: EC2 SSM clone via https will fail' (deploy gate uses this). ok=true if commands ran.`,
    { label: 'recon:github', phase: 'Prep', schema: STEP, model: 'sonnet', effort: 'medium' }),
])

if (!edits || !edits.ok) return { stopped: 'IaC edits failed', edits, recon }
if (recon && recon.data && String(recon.data.visibility).toUpperCase().includes('PRIVATE'))
  return { stopped: 'BLOCKER: repo is PRIVATE — EC2 clones via anonymous https in start-server.yml. Make repo public or add auth before deploy.', edits, recon }

// ---------- Phase 2: Commit + PR + CI + merge ----------
phase('Commit+PR')

const pr = await agent(`${RULES}
Git/GitHub executor for NetherNode. Repo ${REPO}, branch dev. Leader-approved flow: commit -> push -> PR dev->master -> wait CI green -> merge (normal merge). Approval ALREADY given by human; execute.

COMMITS (atomic, exactly these two; NEVER stage infra/terraform.tfvars, .agents/, .claude/, .env):
1. git add infra/variables.tf infra/main.tf .gitignore && git commit -m "chore(infra): parameterize ssh key and target master branch"
2. git add .github/workflows/image.yml .github/workflows/start-server.yml && git commit -m "ci: target master branch for image publish and deploy defaults"
End each commit message body with: Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>
Verify: git status --short (tfvars must NOT appear staged), git log -3 --oneline.

PUSH (remote is ssh; avoid passphrase prompt by pushing over https with gh credential helper):
gh auth setup-git
git push https://github.com/OneCode182/NetherNode.git dev:dev
If that fails, fallback: git push origin dev using SSH_ASKPASS: write a +x script that echoes the passphrase 7643, then SSH_ASKPASS=<script> SSH_ASKPASS_REQUIRE=force setsid git push origin dev. Never echo the passphrase to logs; delete the script after.

PR:
gh pr create --base master --head dev --title "Deploy NetherNode lean AWS runtime" --body "Parameterize infra (ssh key, master branch targets), fix image/deploy workflow branch drift. Audit fixes already on dev.

🤖 Generated with [Claude Code](https://claude.com/claude-code)"
Then: gh pr checks --watch (if a check takes long, poll gh pr checks every 30s, max 15 min).
CI FAIL -> read logs (gh run view --log-failed), root-cause, minimal fix, atomic commit, push, re-watch. Max 2 fix loops; then ok=false with blocker.
CI GREEN -> gh pr merge --merge (do NOT delete dev branch).
After merge: image.yml should trigger on master. Watch: gh run list --workflow=image.yml --limit 1, poll until completed (max 15 min). Report GHCR publish result + image URI ghcr.io/onecode182/nethernode:latest.
data must include: pr_url, merged (bool), image_run_conclusion.`,
  { label: 'git:pr-ci-merge', phase: 'Commit+PR', schema: STEP, model: 'sonnet', effort: 'xhigh' })

if (!pr || !pr.ok) return { stopped: 'PR/CI/merge failed', edits, recon, pr }

// ---------- Phase 3: Provision (plan -> verify gate -> apply) ----------
phase('Provision')

const plan = await agent(`${RULES}
Terraform plan executor. Repo ${REPO}. Local state (no remote backend configured — state stays in infra/, gitignored).
Run (report exit codes):
terraform -chdir=${REPO}/infra init
terraform -chdir=${REPO}/infra plan -out=tfplan
terraform -chdir=${REPO}/infra show -no-color tfplan > /tmp/claude-1000/-home-onecode/cd3e1908-0ea0-40f2-a53a-a9a33d8ab890/scratchpad/tfplan.txt
Read the plan text. data MUST include: add (int), change (int), destroy (int), resource_types (array of every resource type planned), instance_type_planned, forbidden (array: any of ecs,fargate,lambda,elasticloadbalancing/lb,efs,ecr,eks,gamelift,eip/elastic ip,nat found in plan — empty expected).
ok=true only if plan succeeded AND destroy==0.`,
  { label: 'tf:plan', phase: 'Provision', schema: STEP, model: 'sonnet', effort: 'xhigh' })

if (!plan || !plan.ok) return { stopped: 'terraform plan failed or wants destroys', edits, recon, pr, plan }

const gate = await agent(`${RULES}
Skeptic plan verifier. Human pre-approved apply ONLY IF plan matches: single EC2 t4g.medium + EBS gp3 20GiB + security group (25565/tcp,19132/udp only, NO port 22 ingress) + IAM (ssm role/profile, github oidc provider+role+policy) + optional key pair + SNS/budget. ZERO always-on cost services: no ECS/Fargate/Lambda/LB/NLB/EFS/ECR/EKS/GameLift/Elastic IP/NAT. destroy must be 0.
Read /tmp/claude-1000/-home-onecode/cd3e1908-0ea0-40f2-a53a-a9a33d8ab890/scratchpad/tfplan.txt YOURSELF and try to REFUTE compliance. Check: instance type is t4g.medium, sg ingress ports exactly 25565/19132, no port 22 ingress, no forbidden resource types, no aws_eip, destroy 0.
ok=true ONLY if fully compliant. blockers list every violation.`,
  { label: 'gate:plan-verify', phase: 'Provision', schema: STEP, model: 'sonnet', effort: 'xhigh' })

if (!gate || !gate.ok) return { stopped: 'Plan gate REFUSED apply — violations found. NO resources created.', plan, gate, pr, recon }

log('Plan gate passed — applying (pre-approved by human)')

const apply = await agent(`${RULES}
Terraform apply executor. Plan already human-pre-approved + gate-verified. Run:
terraform -chdir=${REPO}/infra apply tfplan
(timeout generously; EC2+IAM ~2-4 min). If apply errors: report exact error, do NOT retry destructive paths, do NOT destroy. One retry allowed ONLY for transient/eventual-consistency errors (e.g. IAM propagation) by re-running plan+apply of the SAME config.
Then: terraform -chdir=${REPO}/infra output -json
data MUST include every output key/value (instance id, role arn, public ip if any, etc). ok=true if apply completed and outputs captured.`,
  { label: 'tf:apply', phase: 'Provision', schema: STEP, model: 'sonnet', effort: 'xhigh' })

if (!apply || !apply.ok) return { stopped: 'terraform apply failed', apply, plan, gate, pr }

// ---------- Phase 4: Wire GitHub vars ----------
phase('Wire')

const wire = await agent(`${RULES}
Set GitHub Actions variables for OneCode182/NetherNode from terraform outputs. Get values: terraform -chdir=${REPO}/infra output -json (parse yourself).
Set:
gh variable set AWS_REGION -R OneCode182/NetherNode --body "us-east-1"
gh variable set AWS_ROLE_ARN -R OneCode182/NetherNode --body "<github actions role arn output>"
gh variable set EC2_INSTANCE_ID -R OneCode182/NetherNode --body "<instance id output>"
gh variable set MINECRAFT_EULA -R OneCode182/NetherNode --body "TRUE"
DuckDNS: check gh variable list / gh secret list. If DUCKDNS_DOMAIN or DUCKDNS_TOKEN missing, DO NOT invent values — add note 'DuckDNS pending: user must run gh variable set DUCKDNS_DOMAIN + gh secret set DUCKDNS_TOKEN' in blockers (non-fatal, deploy proceeds without DNS).
Verify: gh variable list -R OneCode182/NetherNode. data: {vars_set:[...], duckdns_ready: bool}. ok=true if the 4 required vars set.`,
  { label: 'gh:vars', phase: 'Wire', schema: STEP, model: 'haiku', effort: 'medium' })

if (!wire || !wire.ok) return { stopped: 'GitHub vars wiring failed', wire, apply, pr }

// ---------- Phase 5: Deploy ----------
phase('Deploy')

const deploy = await agent(`${RULES}
Deploy executor: start NetherNode Minecraft on EC2 via GitHub workflow. Repo OneCode182/NetherNode.
PRECHECK GHCR pull-ability: token=$(curl -fsS "https://ghcr.io/token?scope=repository:onecode182/nethernode:pull" | jq -r .token); curl -fsS -H "Authorization: Bearer $token" -H "Accept: application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.docker.distribution.manifest.v2+json" "https://ghcr.io/v2/onecode182/nethernode/manifests/latest" >/dev/null && echo PULL_OK || echo PULL_FAIL
If PULL_FAIL: package likely private (GHCR default). Report blocker 'make GHCR package public: github.com/users/OneCode182/packages/container/nethernode/settings' and STOP (ok=false) — EC2 cannot pull.
If PULL_OK:
gh workflow run start-server.yml -R OneCode182/NetherNode --ref master -f image_tag=latest -f repo_branch=master
sleep 10; run_id=$(gh run list -R OneCode182/NetherNode --workflow=start-server.yml --limit 1 --json databaseId -q '.[0].databaseId')
Poll gh run view $run_id every 30s until completed (max 20 min).
FAIL -> gh run view $run_id --log-failed; root-cause. Debug via AWS CLI read-only + SSM:
aws ec2 describe-instances --instance-ids <id> ...
aws ssm send-command --instance-ids <id> --document-name AWS-RunShellScript --parameters '{"commands":["cd /opt/nethernode/app && docker compose ps && docker compose logs --tail=120 minecraft"]}' then get-command-invocation.
Minimal fixes allowed: re-run workflow, SSM-side state fixes. Repo code changes NOT allowed (report blocker instead). Max 2 debug loops.
data: {run_url, conclusion, debug_notes}. ok=true if workflow concluded success.`,
  { label: 'deploy:start-server', phase: 'Deploy', schema: STEP, model: 'sonnet', effort: 'xhigh' })

// ---------- Phase 6: Verify ----------
phase('Verify')

const verify = await agent(`${RULES}
Verify NetherNode Minecraft online. EC2 instance id: get from terraform -chdir=${REPO}/infra output -json.
1. aws ec2 describe-instances --instance-ids <id> --query "Reservations[0].Instances[0].{State:State.Name,PublicIp:PublicIpAddress,PublicDns:PublicDnsName}" --output json
2. If DUCKDNS_DOMAIN variable set in repo (gh variable list -R OneCode182/NetherNode): dig +short <domain>.duckdns.org, compare with public IP.
3. mcstatus: curl -fsS "https://api.mcstatus.io/v2/status/java/<PublicIp>:25565?query=false&timeout=10" | jq '{online,host,port,version:.version.name_clean,players:.players.online}'
   Retry up to 6 times, 30s apart (server boot + Fabric download takes minutes).
4. Optional SSM state snapshot: docker compose ps via ssm send-command.
data: {state, public_ip, public_dns, duckdns, mcstatus_online (bool), mcstatus_version, players}. ok=true if instance running (mcstatus_online may be false if deploy step failed — still report).`,
  { label: 'verify:mcstatus', phase: 'Verify', schema: STEP, model: 'sonnet', effort: 'high' })

return { edits, recon, pr, plan, gate, apply, wire, deploy, verify }
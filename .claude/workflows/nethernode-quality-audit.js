export const meta = {
  name: 'nethernode-quality-audit',
  description: 'End-to-end quality audit of NetherNode repo across 8 checklist dimensions',
  phases: [
    { title: 'Audit', detail: '6 parallel dimension auditors (Sonnet xhigh)' },
    { title: 'Verify', detail: 'skeptics refute fatal/high findings' },
  ],
}

const REPO = '/home/onecode/lab/NetherNode'

const COMMON = `You are a read-only senior auditor subagent for NetherNode (leader integrates; you investigate and propose only).
Repo: ${REPO}. Product: low-cost manually started/stopped Minecraft Java/Fabric server on AWS. MVP truth (from harness docs):
- ONE stoppable EC2 (Graviton t4g.small default, t4g.medium only after metrics), Docker Compose runtime, image on GHCR (itzg/minecraft-server:stable-java25 base, FABRIC).
- FORBIDDEN in MVP path: ECS, Fargate, Lambda, NLB, EFS, EKS, GameLift, ECR, Elastic IP, public SSH (SSM only), terraform auto-apply.
- Budget: <$30 target / $50 hard ceiling over 6 months, $90 credits. Region us-east-1. GitHub OIDC (no static AWS keys). DuckDNS after start instead of Elastic IP.
- Metrics targets: latency p95 <130ms from Cota/Bogota, TPS ~20, MSPT p95 <35ms, CPU p95 <70%, disk free >20%.
HARD RULES: do NOT edit any file, do NOT run terraform apply, do NOT push, do NOT create AWS resources, do NOT print secret values (report presence/leak-risk only, redact values). Bash read-only + the validation commands listed in your task are allowed.
Verify from repo evidence only — never assume. Cite file:line. If a check cannot run, mark SKIP with exact reason. Also check git state where relevant (git ls-files, .gitignore) — a file present on disk may or may not be tracked.
Your final output is data for the leader, not prose for a human.`

const FINDINGS_SCHEMA = {
  type: 'object',
  required: ['section', 'status', 'findings', 'commands', 'risks'],
  properties: {
    section: { type: 'string' },
    status: { enum: ['PASS', 'FAIL', 'SKIP'] },
    findings: {
      type: 'array',
      items: {
        type: 'object',
        required: ['severity', 'summary', 'evidence', 'fix'],
        properties: {
          severity: { enum: ['fatal', 'high', 'medium', 'low'] },
          summary: { type: 'string' },
          evidence: { type: 'string', description: 'file:line refs and/or command output excerpts' },
          fix: { type: 'string', description: 'exact minimal remediation steps' },
        },
      },
    },
    commands: {
      type: 'array',
      items: {
        type: 'object',
        required: ['cmd', 'result'],
        properties: { cmd: { type: 'string' }, result: { type: 'string', description: 'exit code + short output summary' } },
      },
    },
    risks: { type: 'array', items: { type: 'string' } },
  },
}

const VERDICT_SCHEMA = {
  type: 'object',
  required: ['confirmed', 'reason'],
  properties: {
    confirmed: { type: 'boolean', description: 'true = finding is real, false = refuted' },
    reason: { type: 'string' },
    corrected_severity: { enum: ['fatal', 'high', 'medium', 'low'] },
  },
}

const DIMENSIONS = [
  {
    key: 'product-docs',
    task: `Audit sections "Product Alignment" + "Documentation".
Product alignment — verify: repo supports one low-cost stoppable EC2 not always-on infra; NO ECS/Fargate/Lambda/NLB/EFS/EKS/GameLift/ECR anywhere in MVP path (grep infra/, .github/workflows/, ops/, docs); runtime = Docker Compose + GHCR; start/stop workflow supports quick play sessions; budget target below $50 ceiling; Azure migration plausible (runtime = compose + env + volume). Cross-check that product-brief, .agents/architecture/*.md, README.md, infra/, workflows, ops ALL tell the same story — flag every contradiction.
Documentation — verify README explains: local run, GHCR, AWS start/stop, cost model, required GitHub vars/secrets, DuckDNS, no-terraform-apply rule. Harness memory (.agents/) matches actual architecture. Docs do NOT mention a removed Node worker as active runtime. Azure note present. Test: could a new engineer operate this repo from docs alone?
Read: AGENTS.md, .agents/AGENTS.md, .agents/env.json, .agents/project/product-brief.md, .agents/architecture/*.md, .agents/memory/decisions.md, README.md, Makefile, and skim infra/ + .github/workflows/ + ops/ for forbidden services and doc drift.`,
  },
  {
    key: 'runtime',
    task: `Audit section "Runtime Architecture".
Verify: server/ has NO Node worker (note: server/src/ exists — check if empty/stale and whether tracked in git); server/Dockerfile is thin Minecraft runtime wrapper over itzg/minecraft-server:stable-java25; server/runtime.env controls Minecraft version + server defaults; server/server.jar is gitignored AND not required by CI (it exists on disk — run git ls-files server/ and git check-ignore -v server/server.jar); compose.yaml has exactly one minecraft service; persistent data path configurable via env; Java 25-compatible image; Fabric defaults explicit. Also check .env vs .env.example: is .env tracked in git (secret risk)?
Run and report exit codes:
  cd ${REPO} && docker compose -f compose.yaml config -q
  cd ${REPO} && docker build -f server/Dockerfile server
If docker daemon unavailable or build fails on network, mark that command SKIP/FAIL with exact error. Pass = runtime builds, compose validates, no hidden local binary dependency.`,
  },
  {
    key: 'ops',
    task: `Audit section "Ops Safety".
Read every script in ${REPO}/ops/ fully. Verify: start.sh syncs runtime env, pulls image, starts only minecraft; stop-safe.sh saves world (save-all/RCON), backs up, stops cleanly; backup.sh + restore.sh support --dry-run; observability.sh checks container, RCON, stats, disk/backups; dns-update.sh does NOT leak DuckDNS token (check set -x risk, echo of URL with token, curl verbose); scripts idempotent for repeated start/stop (re-runnable without corrupting state).
Run and report exit codes:
  cd ${REPO} && bash -n ops/*.sh (each file)
  cd ${REPO} && bash ops/backup.sh --dry-run
  cd ${REPO} && bash ops/stop-safe.sh --dry-run
  cd ${REPO} && bash ops/observability.sh --dry-run
Watch dry-run output for any secret/token appearing. Pass = no syntax errors, dry-runs work, no secret printed. Also flag any path assumptions that break local vs cloud (/opt/nethernode vs ./data).`,
  },
  {
    key: 'infra',
    task: `Audit section "Infrastructure".
Read all of ${REPO}/infra/. Verify: default instance t4g.small; root gp3 size cost-conscious; EC2 via SSM only, NO public SSH (port 22 ingress absent or restricted), security group public ingress ONLY Minecraft ports (25565/tcp, optionally 19132/udp); GitHub OIDC role least-privilege for start/stop/SSM (flag wildcards on Action or Resource); NO terraform auto-apply in CI (check .github/workflows/infra-validate.yml and others); start_server_on_boot defaults false; NO ECR/Elastic IP/NAT gateway/always-on cost resources. Check user-data.tftpl for secret handling and boot behavior.
Run and report exit codes:
  terraform -chdir=${REPO}/infra fmt -check -recursive
  terraform -chdir=${REPO}/infra init -backend=false
  terraform -chdir=${REPO}/infra validate
If terraform binary missing, try tofu; if both missing mark SKIP with reason. Pass = validates, no costly always-on services.`,
  },
  {
    key: 'cicd',
    task: `Audit section "CI/CD".
Read all 5 workflows in ${REPO}/.github/workflows/ (ci.yml, image.yml, infra-validate.yml, start-server.yml, stop-server.yml). Verify: CI validates repo/compose/docker/scripts/terraform; image.yml publishes GHCR on main; start-server.yml starts EC2 → waits SSM → deploys image → starts Minecraft; stop-server.yml safe-stops then stops EC2; OIDC (permissions: id-token: write, aws-actions/configure-aws-credentials with role-to-assume) NOT static keys (flag any AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY secrets usage); required vars/secrets documented in README; CI does NOT need server/server.jar; no terraform apply anywhere.
Run: parse each YAML (python3 -c "import yaml,sys; yaml.safe_load(open(sys.argv[1]))" file, or yq). Report exit codes. Cross-check workflow names/inputs against README instructions — flag drift. Pass = YAML parses, workflows match README, no long-lived AWS secrets.`,
  },
  {
    key: 'obs-coupling',
    task: `Audit sections "Observability And Performance" + "Coupling And Design Quality".
Observability — verify docs+scripts cover: TPS ~20 target, MSPT p95 <35ms, CPU p95 <70%, disk free >20%, backup count/size, player join verification, latency from Cota/Bogota <130ms p95. Check .agents/architecture/observability.architecture.md vs ops/observability.sh vs README — do documented targets have actual operator commands? Flag targets with no way to measure.
Coupling — assess: clear boundaries between runtime (server/, compose.yaml, ops/), infra (infra/, workflows), docs; cloud-specific logic ONLY in infra/ + workflows (grep ops/ and compose for hardcoded AWS-isms); secrets outside repo (check .gitignore covers .env, tfvars, tfstate; git ls-files for anything secret-shaped); Minecraft version change = edit server/runtime.env + rebuild GHCR + redeploy, nothing else (verify no version duplicated elsewhere — grep for version strings across repo); no hidden manual-only steps except first terraform bootstrap + GitHub vars/secrets. Pass = low coupling, no unnecessary AWS lock-in.`,
  },
]

phase('Audit')
log('6 dimension auditors launch (sonnet, xhigh)')

const results = await pipeline(
  DIMENSIONS,
  d =>
    agent(`${COMMON}\n\n## Assigned dimension: ${d.key}\n\n${d.task}\n\nReturn structured findings. section="${d.key}". status: PASS only if every pass criterion met with evidence; FAIL if any criterion fails; SKIP only if the whole dimension could not be checked. Include EVERY command you ran in commands[] with exit code.`,
      { label: `audit:${d.key}`, phase: 'Audit', schema: FINDINGS_SCHEMA, model: 'sonnet', effort: 'xhigh' }),
  (report, d) => {
    if (!report) return null
    const serious = report.findings.filter(f => f.severity === 'fatal' || f.severity === 'high')
    if (serious.length === 0) return { ...report, verified: [] }
    return parallel(
      serious.map(f => () =>
        agent(`You are a skeptic verifier for a NetherNode audit finding. Repo: ${REPO}. Read-only; no edits, no terraform apply, no push.
Finding (dimension ${d.key}, severity ${f.severity}): ${f.summary}
Claimed evidence: ${f.evidence}
Claimed fix: ${f.fix}
Try to REFUTE it: re-check the actual files/commands yourself. Confirm only if evidence holds under your own inspection. If real but severity wrong, set corrected_severity. If uncertain after checking, confirmed=false.`,
          { label: `verify:${d.key}:${f.summary.slice(0, 40)}`, phase: 'Verify', schema: VERDICT_SCHEMA, model: 'sonnet', effort: 'high' })
          .then(v => ({ finding: f, verdict: v }))
      )
    ).then(verified => ({ ...report, verified: verified.filter(Boolean) }))
  }
)

const reports = results.filter(Boolean)
log(`${reports.length}/6 dimensions reported`)
return { reports }
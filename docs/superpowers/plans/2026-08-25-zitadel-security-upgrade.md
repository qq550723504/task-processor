# ZITADEL Security Upgrade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade production ZITADEL and Login V2 from v4.13.1 to v4.17.1 before enabling new phone or external-IdP paths.

**Architecture:** This is an operational hardening slice. It adds a read-only preflight and a recovery-first runbook; the production image change is a separate explicitly authorized operation.

**Tech Stack:** Kubernetes, PostgreSQL 18, PowerShell 7, Pester, ZITADEL v4.17.1.

**Spec:** `docs/superpowers/specs/2026-08-25-listingkit-phone-identity-design.md`

## Global Constraints

- Preserve `yudao-cloud/postgresql-v18` database `zitadel_auth`; never point ZITADEL at a ListingKit or Yudao business database.
- Do not enable OTP SMS or Generic OIDC until both Deployments run v4.17.1 and existing login passes.
- Do not print database passwords, PATs, bearer tokens, cookies, phone numbers, or codes.
- Database snapshot, restore rehearsal, production image update, and real-device login require explicit production authorization.

---

### Task 1: Add a non-mutating upgrade preflight

**Files:**
- Create: `scripts/zitadel-v4-upgrade-preflight.ps1`
- Test: `scripts/zitadel-v4-upgrade-preflight.Tests.ps1`

**Interfaces:**
- Consumes: current kubeconfig and optional `-Namespace zitadel`, `-TargetVersion v4.17.1`.
- Produces: JSON with `coreImage`, `loginImage`, readiness, generations, target version, and `upgradeRequired`; no Secret data.

- [x] **Step 1: Write the failing Pester tests.**

```powershell
Describe "zitadel-v4-upgrade-preflight" {
  It "uses only deployment reads" {
    $content = Get-Content -Raw scripts/zitadel-v4-upgrade-preflight.ps1
    $content | Should Match 'kubectl.+get deployment zitadel'
    $content | Should Match 'kubectl.+get deployment zitadel-login'
    $content | Should Not Match 'get secret|\.data|Authorization:|Bearer '
  }
  It "pins the approved target" {
    (Get-Content -Raw scripts/zitadel-v4-upgrade-preflight.ps1) | Should Match 'v4\.17\.1'
  }
}
```

- [x] **Step 2: Run the failing test.**

Run: `Invoke-Pester scripts/zitadel-v4-upgrade-preflight.Tests.ps1 -Output Detailed`

Expected: FAIL because the script is absent.

- [x] **Step 3: Implement the smallest read-only report.**

```powershell
[CmdletBinding()] param([string]$Namespace = "zitadel", [string]$TargetVersion = "v4.17.1")
function Get-Snapshot([string]$Name) {
  $d = kubectl -n $Namespace get deployment $Name -o json | ConvertFrom-Json
  $c = @($d.spec.template.spec.containers | Where-Object name -eq $Name)[0]
  [pscustomobject]@{ image=[string]$c.image; ready=("{0}/{1}" -f $d.status.readyReplicas,$d.status.replicas); generation=[int64]$d.metadata.generation }
}
$core = Get-Snapshot zitadel; $login = Get-Snapshot zitadel-login
[pscustomobject]@{ coreImage=$core.image; loginImage=$login.image; coreReady=$core.ready; loginReady=$login.ready; coreGeneration=$core.generation; loginGeneration=$login.generation; targetVersion=$TargetVersion; upgradeRequired=(($core.image -notmatch [regex]::Escape($TargetVersion)) -or ($login.image -notmatch [regex]::Escape($TargetVersion))) } | ConvertTo-Json -Compress
```

- [x] **Step 4: Verify focused tests and a sanitized baseline.**

Run: `Invoke-Pester scripts/zitadel-v4-upgrade-preflight.Tests.ps1 -Output Detailed; ./scripts/zitadel-v4-upgrade-preflight.ps1`

Expected: PASS; JSON contains image/health metadata only.

- [x] **Step 5: Commit the preflight.**

```powershell
git add scripts/zitadel-v4-upgrade-preflight.ps1 scripts/zitadel-v4-upgrade-preflight.Tests.ps1
git commit -m "ops: add ZITADEL upgrade preflight"
```

### Task 2: Create the recovery-first operator runbook

**Files:**
- Create: `docs/operations/zitadel-v4-security-upgrade-runbook.md`
- Modify: `deployments/kubernetes/zitadel/local/README.md`

**Interfaces:**
- Consumes: Task 1 report, backup identifier, restore-rehearsal evidence, and a separately approved change window.
- Produces: v4.17.1 core/login deployments plus recorded health and legacy login regression evidence.

- [x] **Step 1: Write the runbook acceptance checklist before update commands.**

```markdown
- [x] Database backup identifier and isolated restore rehearsal timestamp are recorded without credentials.
- [x] `zitadel` and `zitadel-login` are Ready on v4.17.1.
- [x] `https://auth.shuomiai.com/.well-known/openid-configuration` returns 200.
- [x] Incognito email/password and ListingKit callback login both work.
- [x] OTP SMS and Generic OIDC remain disabled.
```

- [x] **Step 2: Add this exact approved-change sequence.**

```bash
kubectl -n zitadel set image deployment/zitadel zitadel=ghcr.io/zitadel/zitadel:v4.17.1
kubectl -n zitadel set image deployment/zitadel-login zitadel-login=ghcr.io/zitadel/zitadel-login:v4.17.1
kubectl -n zitadel rollout status deployment/zitadel --timeout=10m
kubectl -n zitadel rollout status deployment/zitadel-login --timeout=10m
curl --fail --silent --show-error https://auth.shuomiai.com/.well-known/openid-configuration >/dev/null
```

- [x] **Step 3: State the failure boundary.**

```markdown
If a rollout fails, stop feature activation. Do not downgrade a migrated ZITADEL database in place. Restore the rehearsed backup into the isolated recovery target, validate there, and obtain a new approval before moving production traffic.
```

- [x] **Step 4: Validate the runbook and preflight.**

Run: `rg -n 'v4.17.1|restore rehearsal|Do not downgrade' docs/operations/zitadel-v4-security-upgrade-runbook.md; ./scripts/zitadel-v4-upgrade-preflight.ps1`

Expected: all three safeguards are documented; the preflight remains read-only.

- [x] **Step 5: Commit the runbook.**

```powershell
git add docs/operations/zitadel-v4-security-upgrade-runbook.md deployments/kubernetes/zitadel/local/README.md
git commit -m "docs: add ZITADEL v4 security upgrade runbook"
```

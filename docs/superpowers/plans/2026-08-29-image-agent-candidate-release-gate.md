# Image Agent Candidate Release Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Validate a candidate image worker on an isolated queue before any production worker rollout mutation.

**Architecture:** The release workflow starts a candidate deployment using the final image digest and a distinct queue, waits for its health/canary, then promotes that same digest to the production worker and removes the candidate.

**Tech Stack:** GitHub Actions, Kubernetes, PowerShell.

**Spec:** `docs/superpowers/specs/2026-08-29-image-agent-candidate-release-gate-design.md`

## Global Constraints

- Candidate health/canary must precede every production apply/restart command.
- Candidate and production queues differ explicitly and share one immutable image digest.
- Implementation changes sequencing only; it does not deploy.

---

### Task 1: Enforce release ordering in a script contract

**Files:**
- Modify: `.github/workflows/listingkit-deploy.yml`
- Create: `.github/scripts/test-listingkit-deploy-order.ps1`

- [ ] **Step 1: Write failing assertion**

```powershell
$candidate = $workflow.IndexOf('candidate compatibility canary')
$production = $workflow.IndexOf('roll out image-agent-manual-v3')
if ($candidate -lt 0 -or $production -lt 0 -or $candidate -ge $production) { throw 'candidate gate must precede production rollout' }
```

- [ ] **Step 2: Verify RED**

Run: `powershell -ExecutionPolicy Bypass -File .github/scripts/test-listingkit-deploy-order.ps1`

Expected: FAIL because production rollout currently precedes canary.

- [ ] **Step 3: Implement isolated candidate gate**

Introduce explicit candidate queue/deployment/image-digest variables, candidate apply/health/canary/cleanup, and move production apply/restart after successful canary. Reuse the same digest variable for promotion.

- [ ] **Step 4: Verify and commit**

Run: `powershell -ExecutionPolicy Bypass -File .github/scripts/test-listingkit-deploy-order.ps1`

Expected: PASS.

Run: `git add .github/workflows/listingkit-deploy.yml .github/scripts/test-listingkit-deploy-order.ps1` then `git commit -m "fix: gate image worker rollout on candidate canary"`.

### Task 2: Assert digest and failure-path safety

**Files:**
- Modify: `.github/scripts/test-listingkit-deploy-order.ps1`
- Test: `.github/workflows/listingkit-deploy.yml`

- [ ] **Step 1: Write failing checks**

```powershell
if ($workflow -notmatch 'CANDIDATE_QUEUE' -or $workflow -notmatch 'IMAGE_DIGEST') { throw 'candidate contract missing' }
if ($workflow.IndexOf('candidate cleanup') -lt $workflow.IndexOf('candidate compatibility canary')) { throw 'cleanup must follow canary' }
```

- [ ] **Step 2: Verify RED, implement, and verify GREEN**

Run: `powershell -ExecutionPolicy Bypass -File .github/scripts/test-listingkit-deploy-order.ps1`

Expected before implementation: FAIL; expected after implementation: PASS.

- [ ] **Step 3: Commit**

Run: `git add .github/scripts/test-listingkit-deploy-order.ps1` then `git commit -m "test: verify image worker candidate gate"`.

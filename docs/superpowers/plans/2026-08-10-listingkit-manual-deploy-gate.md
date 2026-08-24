# ListingKit Manual Deploy Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the documented manual ListingKit production deploy path enforce the same identity-preflight and immutable API-image gate as CI.

**Architecture:** The PowerShell helper remains responsible for build and push only. It invokes the existing tested Bash preflight and immutable-API-apply drivers, passing its exact versioned API image. The helper may update the matching UI only after that API gate succeeds; `-SkipApply` skips every Kubernetes mutation.

**Tech Stack:** PowerShell, Pester, Bash release drivers, kubectl, Kubernetes manifests.

## Global Constraints

- Reuse `scripts/listingkit-identity-preflight-job.sh` and `scripts/listingkit-apply-api-deployment.sh`; do not duplicate their rendering or validation logic.
- The exact full release candidate API image must be versioned and must never be `latest`; preflight and API apply receive that identical string.
- No Kubernetes mutation is permitted after `-SkipApply`, preflight failure, or immutable-API-apply failure.
- The preflight's metadata database must be exactly one of `zitadel_auth` or `zitadel`, with read-only CONNECT/SELECT access to `projections.org_metadata2`.

---

### Task 1: Gate the manual PowerShell release helper

**Files:**
- Modify: `scripts/build-push-deploy-listingkit-workbench.ps1`
- Test: `scripts/build-push-deploy-listingkit-workbench.Tests.ps1`

**Interfaces:**
- Consumes: `listingkit-identity-preflight-job.sh --manifest PATH --namespace NAMESPACE --image IMMUTABLE_IMAGE`.
- Consumes: `listingkit-apply-api-deployment.sh --manifest PATH --namespace NAMESPACE --image IMAGE`.
- Produces: an ordered manual deploy sequence: preflight, one immutable API apply, UI update, rollout waits.

- [x] **Step 1: Write failing behavior tests**

Run the real helper with stubbed `docker`, `bash`, and `kubectl`. Assert `-SkipApply` emits no Kubernetes command; assert a normal release invokes preflight before the immutable API driver and only then updates the UI; assert a preflight failure prevents later mutations; assert `-Tag latest` fails before external commands.

- [x] **Step 2: Verify RED**

Run: `Invoke-Pester .\scripts\build-push-deploy-listingkit-workbench.Tests.ps1`

Expected: FAIL because the old helper applies the overlay and updates deployments even with `-SkipApply`, and has no preflight driver.

- [x] **Step 3: Write minimal implementation**

Reject a blank or `latest` candidate tag. Replace raw overlay apply and API `set image` with the two existing drivers in sequence. Retain the UI image update only after successful immutable API apply. Put every Kubernetes call, including UI rollout waits, under `if (-not $SkipApply)`.

- [x] **Step 4: Verify GREEN**

Run: `Invoke-Pester .\scripts\build-push-deploy-listingkit-workbench.Tests.ps1`

Expected: PASS; the test log proves order and absence of bypasses.

### Task 2: Record operational prerequisites and verification evidence

**Files:**
- Modify: `deployments/kubernetes/listingkit-workbench/README.md`
- Modify: `docs/development/listingkit-local-debug.md`
- Modify: `docs/development/listingkit-shein-enrollment-source-sds-troubleshooting.md`
- Modify: `docs/superpowers/verification/2026-08-09-listingkit-canonical-zitadel-subject.md`

- [x] **Step 1: Replace unsafe manual instructions**

Document the gated PowerShell helper and remove instructions to apply the floating API Deployment directly. Describe the single-candidate ZITADEL metadata database requirement and least read-only permissions without printing credentials.

- [x] **Step 2: Correct canonical identity wording**

State that Auth.js and Go both use verified `sub`; legacy session fields are diagnostic-only.

- [x] **Step 3: Refresh verification note**

Record final current-HEAD focused Go, Pester/driver, frontend, Docker, Kubernetes and full-suite results. State the untouched crawler baseline failure as a failure, not a pass.

### Task 3: Verify and commit

- [x] **Step 1: Run focused regression and syntax checks**

Run Pester, both Bash driver suites via Git Bash, `go test ./tests`, selected preflight packages, and `git diff --check`.

- [x] **Step 2: Run release artifact checks**

Run current-HEAD frontend tests/typecheck/lint/build, API Docker build plus in-image preflight help, and production Kustomize plus Job client dry-run.

- [x] **Step 3: Commit scoped files**

Commit the helper, Pester test, docs, and refreshed verification note together after evidence is recorded.

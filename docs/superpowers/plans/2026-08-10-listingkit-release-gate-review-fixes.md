# ListingKit Release-Gate Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Block ownerless data and make every ListingKit release gate digest-pinned, rollback-capable, and least-privilege.

**Architecture:** Owner aggregation retains blank subjects as blockers. Release tooling carries a candidate API digest and an independent preflight-runner digest; the Job runs only the latter and annotates the former. Kubernetes projects only the seven Secret values used by preflight.

**Tech Stack:** Go 1.26, sqlmock, Bash, PowerShell/Pester, Docker Buildx, GitHub Actions, Kubernetes YAML.

## Global Constraints

- Never expose raw owner values, tokens, or Secret contents.
- Accept only `repository@sha256:<64 hex>` for candidate and runner image references.
- Preserve blocking order: configuration, invitation Secret checks, preflight, immutable API apply, rollout.
- The preflight Secret allowlist is DB host/port/user/password/name, issuer URL, and directory token.

---

### Task 1: Block ownerless persisted data

**Files:**
- Modify: `internal/listingkit/identitypreflight/repository.go`
- Modify: `internal/listingkit/identitypreflight/preflight.go`
- Test: `internal/listingkit/identitypreflight/repository_test.go`
- Test: `internal/listingkit/identitypreflight/preflight_test.go`

**Interfaces:** Preserve a blank `PersistedOwner.UserID` when its tenant exists; `Service.Run` reports it as redacted `reason=missing_subject` and returns a typed blocker without calling the directory.

- [ ] Write `TestPostgresOwnerRepositoryIncludesBlankOwnerForNonBlankTenant` and `TestServiceBlocksBlankOwnerWithoutDirectoryLookup`.
- [ ] Run `go test ./internal/listingkit/identitypreflight -run 'Test(PostgresOwnerRepositoryIncludesBlankOwnerForNonBlankTenant|ServiceBlocksBlankOwnerWithoutDirectoryLookup)$' -count=1`; expect RED because the aggregate filters blank owners.
- [ ] Keep the nonblank-tenant SQL filter, remove only blank-owner exclusion, and classify blank subjects before directory lookup.
- [ ] Run `go test ./internal/listingkit/identitypreflight -count=1` and commit `fix(listingkit): block ownerless preflight rows`.

### Task 2: Require digest references in release drivers

**Files:**
- Modify: `scripts/lib/listingkit-immutable-image.sh`
- Modify: `scripts/listingkit-identity-preflight-job.sh`
- Modify: `scripts/listingkit-apply-api-deployment.sh`
- Modify: `scripts/tests/listingkit-identity-preflight-job-test.sh`
- Modify: `scripts/tests/listingkit-apply-api-deployment-test.sh`
- Modify: `scripts/build-push-deploy-listingkit-workbench.ps1`
- Test: `scripts/build-push-deploy-listingkit-workbench.Tests.ps1`

**Interfaces:** Drivers accept only digest references; the local helper resolves a repository digest after push and passes that exact API digest to both preflight and apply.

- [ ] Add shell tests rejecting a non-`latest` tag and a Pester test that records one resolved API digest in both commands.
- [ ] Run both shell suites and Pester; expect RED because `:release-20260810` is accepted.
- [ ] Restrict the shared validator to `@sha256:` plus 64 hex; resolve pushed repository digests before Kubernetes operations.
- [ ] Rerun the suites and commit `fix(release): require digest-pinned ListingKit images`.

### Task 3: Decouple the preflight runner from API candidates

**Files:**
- Create: `deployments/docker/Dockerfile.listingkit-identity-preflight`
- Modify: `.github/workflows/listingkit-deploy.yml`
- Modify: `deployments/kubernetes/listingkit-workbench/jobs/listingkit-identity-preflight-job.yaml`
- Modify: `scripts/listingkit-identity-preflight-job.sh`
- Test: `scripts/tests/listingkit-identity-preflight-job-test.sh`
- Test: `tests/listingkit_deploy_workflow_test.go`
- Modify: `deployments/kubernetes/listingkit-workbench/README.md`

**Interfaces:** The driver receives `--image API_CANDIDATE_DIGEST` and `--runner-image PREFLIGHT_RUNNER_DIGEST`; the Job runs the runner and annotates the candidate.

- [ ] Add shell/YAML tests proving runner and candidate are separate digest references; run them for RED.
- [ ] Build a non-root distroless runner containing static preflight binary, configuration, prompts, and CA certificates.
- [ ] Build/push the runner from the workflow tooling revision, export its digest, pass it to the driver, and check out gate drivers from the workflow revision rather than rollback source.
- [ ] Run driver/workflow tests and `docker build -f deployments/docker/Dockerfile.listingkit-identity-preflight .`; commit `fix(release): run identity preflight outside API candidate`.

### Task 4: Project only required preflight Secret keys

**Files:**
- Modify: `deployments/kubernetes/listingkit-workbench/jobs/listingkit-identity-preflight-job.yaml`
- Test: `tests/listingkit_secret_boundary_test.go`
- Modify: `deployments/kubernetes/listingkit-workbench/README.md`

**Interfaces:** Job imports ConfigMap and exactly seven required `secretKeyRef` environment variables; it has no shared-Secret `envFrom`.

- [ ] Add parsed-manifest `TestListingKitIdentityPreflightUsesExactSharedSecretKeys`; run it for RED.
- [ ] Replace secret `envFrom` with required explicit DB, issuer, and directory-token mappings.
- [ ] Run the parsed boundary tests plus `kubectl apply --dry-run=client -f deployments/kubernetes/listingkit-workbench/jobs/listingkit-identity-preflight-job.yaml`; commit `fix(release): restrict identity preflight credentials`.

### Task 5: Verify and update the PR

**Files:**
- Modify: `docs/superpowers/verification/2026-08-09-listingkit-canonical-zitadel-subject.md`

- [ ] Run `go test ./internal/listingkit/identitypreflight ./tests -count=1`, both shell driver suites, Pester, `go test ./... -count=1 -timeout=30m`, and `go vet ./...`.
- [ ] Run API and preflight Docker builds, prod Kustomize render, Job dry-run, `git diff --check`, then record actual evidence.
- [ ] Commit the evidence, push the branch, and recheck PR CI and unresolved threads. Do not reply or resolve review threads without separate authorization.

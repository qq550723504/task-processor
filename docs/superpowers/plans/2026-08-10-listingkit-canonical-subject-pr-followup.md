# ListingKit Canonical Subject PR Follow-up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cover user-scoped AI credentials, canonical UI allowlists, and source-aligned preflight runners before merge.

**Architecture:** The fixed preflight inventory receives an explicit blank-user policy, the UI receives only canonical allowlist key projections, and the runner build selects candidate source except for digest rollback.

**Tech Stack:** Go 1.26, `database/sql`, Go testing, GitHub Actions YAML, Kubernetes YAML, `gopkg.in/yaml.v3`.

## Global Constraints

- SQL identifiers remain fixed inventory constants and validate before queries.
- Empty `ai_client_credentials.user_id` is valid tenant-wide configuration; empty normal-owner values remain blockers.
- UI projects canonical tenant, subject, and role allowlist keys only.
- Normal releases run the candidate-source runner; digest rollback runs workflow-version tooling.
- Each production change starts with a RED test and ends with a GREEN test.

---

### Task 1: Preflight user-scoped AI credentials

**Files:** `internal/listingkit/identitypreflight/inventory.go`, `repository.go`, `repository_test.go`, `preflight_test.go`, and `inventory_test.go`.

**Interfaces:** `OwnerTable` gains `BlankUserPolicy`. `ai_client_credentials` has `BlankUserPolicyIgnore`; all current tables remain `BlankUserPolicyBlock`.

- [ ] **Step 1: Write the failing tests**

Add `TestPostgresOwnerRepositorySkipsBlankAIClientCredentialUser`, asserting the credential query filters `NULLIF(BTRIM(CAST(user_id AS text)), '') IS NOT NULL`. Add `TestServiceRunBlocksUserScopedAIClientCredential`, returning a non-empty credential subject absent from the tenant directory and asserting `*ErrUnknownOwners`.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/listingkit/identitypreflight -run 'Test(PostgresOwnerRepositorySkipsBlankAIClientCredentialUser|ServiceRunBlocksUserScopedAIClientCredential)' -count=1`

Expected: fail because no blank-user policy or credential inventory row exists.

- [ ] **Step 3: Implement the minimal policy**

Define `BlankUserPolicyBlock` and `BlankUserPolicyIgnore`; add the literal credential inventory entry. In `ownerAggregateQuery`, filter blank user IDs only for `BlankUserPolicyIgnore`; retain existing `COALESCE` grouping for blocking tables. Reject unknown policies during inventory validation and update the AST inventory guard for the infrastructure model.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/listingkit/identitypreflight -count=1`

Expected: pass; user-scoped credentials block and tenant-wide credentials are omitted before directory lookup.

- [ ] **Step 5: Commit**

Run: `git add internal/listingkit/identitypreflight/inventory.go internal/listingkit/identitypreflight/repository.go internal/listingkit/identitypreflight/repository_test.go internal/listingkit/identitypreflight/preflight_test.go internal/listingkit/identitypreflight/inventory_test.go; git commit -m "fix(listingkit): preflight user-scoped AI credentials"`

### Task 2: Canonical UI allowlist projection

**Files:** `deployments/kubernetes/listingkit-workbench/base/listingkit-ui-deployment.yaml` and `tests/listingkit_secret_boundary_test.go`.

**Interfaces:** UI receives required secret projections for `TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS`, `TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USER_IDS`, and `TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES`.

- [ ] **Step 1: Write the failing parsed-manifest test**

Add `TestListingKitUIUsesCanonicalZitadelAllowlists`. Parse the UI Deployment, require exactly the three `TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_*` key projections, and reject `LISTINGKIT_ZITADEL_ALLOWED_*` projections.

- [ ] **Step 2: Verify RED**

Run: `go test ./tests -run TestListingKitUIUsesCanonicalZitadelAllowlists -count=1`

Expected: fail because the manifest projects deprecated `LISTINGKIT_ZITADEL_ALLOWED_ROLES` only.

- [ ] **Step 3: Project canonical keys**

Replace the deprecated role mapping with the three required canonical mappings. Do not add deprecated aliases.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./tests -run TestListingKitUIUsesCanonicalZitadelAllowlists -count=1; kubectl create --dry-run=client -f deployments/kubernetes/listingkit-workbench/base/listingkit-ui-deployment.yaml`

Expected: pass.

- [ ] **Step 5: Commit**

Run: `git add deployments/kubernetes/listingkit-workbench/base/listingkit-ui-deployment.yaml tests/listingkit_secret_boundary_test.go; git commit -m "fix(listingkit): project canonical UI allowlists"`

### Task 3: Source-align the preflight runner

**Files:** `.github/workflows/listingkit-deploy.yml`, `tests/listingkit_deploy_workflow_test.go`, and `tests/commercial_readiness_workflow_test.go`.

**Interfaces:** `prepare.outputs.runner_source_ref` is candidate `source_ref` for normal builds and `github.workflow_sha` when `candidate_api_image` is non-empty. Runner checkout and image tag consume this output.

- [ ] **Step 1: Write the failing parsed-workflow test**

Add `TestListingKitPreflightRunnerUsesCandidateSourceExceptDigestRollback`. Require `runner_source_ref` selection, its use in runner checkout, and its use in runner tag.

- [ ] **Step 2: Verify RED**

Run: `go test ./tests -run TestListingKitPreflightRunnerUsesCandidateSourceExceptDigestRollback -count=1`

Expected: fail because checkout currently always uses `github.workflow_sha`.

- [ ] **Step 3: Implement source selection**

Set `runner_source_ref` in the metadata step after digest-mode validation. Use it in `build-preflight-runner` checkout and tag. Keep deploy tooling checkout at `github.workflow_sha`.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./tests -run 'TestListingKit(DeployPreflightsBeforeItsOnlyDeploymentMutation|DeployWorkflowSupportsDigestPinnedRollbackWithoutRebuild|PreflightRunnerUsesCandidateSourceExceptDigestRollback)' -count=1; & 'C:\Program Files\Git\bin\bash.exe' scripts/tests/listingkit-identity-preflight-job-test.sh; & 'C:\Program Files\Git\bin\bash.exe' scripts/tests/listingkit-apply-api-deployment-test.sh`

Expected: pass.

- [ ] **Step 5: Commit**

Run: `git add .github/workflows/listingkit-deploy.yml tests/listingkit_deploy_workflow_test.go tests/commercial_readiness_workflow_test.go; git commit -m "fix(ci): align preflight runner with release source"`

### Task 4: Final verification and push

**Files:** Verify only the task files above.

- [ ] **Step 1: Run complete verification**

Run: `go test ./... -count=1 -timeout=30m; go vet ./...; kubectl kustomize deployments/kubernetes/listingkit-workbench/overlays/prod | Out-Null; git diff --check HEAD~3..HEAD`

Expected: pass.

- [ ] **Step 2: Review and push**

Request independent read-only review against these three review requirements. Repair Critical or Important findings, then run `git push origin codex/listingkit-canonical-zitadel-subject`.

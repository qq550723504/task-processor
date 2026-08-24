# ListingKit Deploy Secret Preflight Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop the ListingKit API deployment before any Deployment mutation when its API-only invitation Secret is missing or incomplete.

**Architecture:** Add a versioned Bash preflight script that reads the dedicated Secret with `kubectl`, fails on a missing Secret, and checks the two required data keys without emitting their values. The workflow calls it after the legacy shared-Secret guard and before the API Deployment apply/set-image step. A Go test runs it with a fake `kubectl` command.

**Tech Stack:** GitHub Actions YAML, Bash, kubectl, Go test.

## Global Constraints

- Never log a Secret value or copy its data into another Secret.
- Retain the existing shared-Secret legacy credential guard.
- A failed dedicated-Secret check must occur before applying `product-listing-api-deployment.yaml`.
- Validate both `TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN` and `TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID` as present and non-empty.

---

### Task 1: Guard the API deployment with a dedicated Secret preflight

**Files:**
- Create: `scripts/validate-listingkit-invitation-secret.sh`
- Create: `tests/listingkit_invitation_secret_preflight_test.go`
- Modify: `tests/commercial_readiness_workflow_test.go:143-160`
- Modify: `.github/workflows/listingkit-deploy.yml:174-176`

**Interfaces:**
- Consumes: the Kubernetes Secret `listingkit-member-invitation-secret` in `${{ env.K8S_NAMESPACE }}`.
- Produces: a workflow failure with only the missing Secret/key names when prerequisite credentials are absent; no Deployment mutation on that path.

- [x] **Step 1: Write the failing workflow-contract test**

Add a table-driven test that invokes `scripts/validate-listingkit-invitation-secret.sh` with a fake `kubectl` command for a missing Secret, a missing token key, an empty project-ID key, and both required keys. Assert missing Secret/key names are reported, fake data values are never reported, and the ready case exits 0. Add a workflow-contract assertion that this command appears before `Update API deployment image`:

```go
"Validate dedicated member invitation Secret",
"./scripts/validate-listingkit-invitation-secret.sh",
```

- [x] **Step 2: Run the focused test to verify it fails**

Run: `go test ./tests -run TestListingKitMemberInvitationTokenIsAPIScoped -count=1`

Expected: FAIL because the preflight script is absent.

- [x] **Step 3: Add the minimal preflight**

Create the script with this behavior and call it after the shared-Secret guard and before `Update API deployment image`:

```yaml
- name: Validate dedicated member invitation Secret
  run: ./scripts/validate-listingkit-invitation-secret.sh ${{ env.K8S_NAMESPACE }}
```

- [x] **Step 4: Run focused verification**

Run: `go test ./tests -run TestListingKitMemberInvitationTokenIsAPIScoped -count=1`

Expected: PASS.

- [x] **Step 5: Run workflow and repository checks**

Run:

```powershell
kubectl kustomize deployments/kubernetes/listingkit-workbench/overlays/prod
go test ./tests -count=1
git diff --check
```

Expected: each command exits 0.

- [x] **Step 6: Commit the implementation**

```bash
git add .github/workflows/listingkit-deploy.yml scripts/validate-listingkit-invitation-secret.sh tests/listingkit_invitation_secret_preflight_test.go tests/commercial_readiness_workflow_test.go
git commit -m "fix: preflight ListingKit invitation secret"
```

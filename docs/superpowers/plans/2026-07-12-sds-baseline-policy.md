# SDS Baseline Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delegate deterministic SDS baseline cache classification to `sdspod` and align ready/unknown cache reuse across the API and Studio batch admission.

**Architecture:** `sdspod` receives a neutral cache snapshot and returns status, reason, and reusable decisions. ListingKit decodes cache payloads, maps them to snapshots, performs cache/remote/login work, and adapts the decision to existing readiness and Studio gate DTOs.

**Tech Stack:** Go 1.26, `internal/product/sourcing/sdspod`, `internal/catalog/canonical`, Go tests, `go vet`.

## Global Constraints

- `sdspod` must not import ListingKit, SDS clients, storage, runtime, HTTP, Temporal, or external SDKs.
- `ready + unknown` validation is reusable; `baseline_cached + unknown` remains non-reusable.
- Preserve cache schema, supported version, reason text, tenant resolution, payload decoding, remote validation, and persistence behavior.
- Keep `go.work.sum` unchanged.

---

## File Map

- Create: `internal/product/sourcing/sdspod/baseline.go` and `baseline_test.go`.
- Modify: `internal/listingkit/sds_baseline_readiness_support.go` — adapt decoded cache entries to the domain decision.
- Modify: `internal/listingkit/sds_baseline_service.go` — use the domain decision for API readiness status.
- Modify: `internal/listingkit/sds_baseline_readiness_test.go` and `studio_batch_task_gate_test.go` — prove API/gate agreement.
- Modify: `internal/product/sourcing/sdspod/boundary_guard_test.go`, `docs/refactoring/listingkit-boundary-checkpoint.md`.

### Task 1: Define the Neutral Baseline Decision

**Files:**

- Create: `internal/product/sourcing/sdspod/baseline.go`
- Create: `internal/product/sourcing/sdspod/baseline_test.go`

**Interfaces:**

- Produces: `EvaluateBaseline(snapshot BaselineSnapshot) BaselineDecision`.

- [ ] **Step 1: Write failing domain tests**

```go
func TestEvaluateBaselineTreatsReadyCacheWithUnknownValidationAsReusable(t *testing.T) {
	decision := EvaluateBaseline(BaselineSnapshot{
		CacheStatus: "ready", Version: SupportedBaselineVersion,
		PayloadState: BaselinePayloadPresent, ValidationStatus: "unknown",
	})
	if !decision.Reusable || decision.Status != "ready" { t.Fatalf("decision = %+v", decision) }
}

func TestEvaluateBaselineRejectsBaselineCachedWithUnknownValidation(t *testing.T) {
	decision := EvaluateBaseline(BaselineSnapshot{
		CacheStatus: "baseline_cached", Version: SupportedBaselineVersion,
		PayloadState: BaselinePayloadPresent, ValidationStatus: "unknown",
	})
	if decision.Reusable || decision.ReasonCode != "validation_not_ready" { t.Fatalf("decision = %+v", decision) }
}
```

- [ ] **Step 2: Verify RED**

```powershell
$env:GOWORK='off'
go test ./internal/product/sourcing/sdspod -run TestEvaluateBaseline -count=1
```

Expected: FAIL because the baseline policy API does not exist.

- [ ] **Step 3: Implement the pure policy**

Define:

```go
const SupportedBaselineVersion = 1
const (
	BaselinePayloadPresent = "present"
	BaselinePayloadMissing = "missing"
	BaselinePayloadInvalid = "invalid"
	BaselinePayloadEmpty   = "empty"
)
type BaselineSnapshot struct { CacheStatus string; Version int; PayloadState string; ValidationStatus string; ValidationReasonCode string; ValidationReason string }
type BaselineDecision struct { Reusable bool; Status string; CacheStatus string; ValidationStatus string; ReasonCode string; Reason string }
```

Implement `EvaluateBaseline` with the current cache-version/payload reason text. Branch `ready + unknown` before generic validation rejection; leave `baseline_cached + unknown` rejected with `validation_not_ready`.

- [ ] **Step 4: Verify GREEN and commit**

```powershell
gofmt -w internal/product/sourcing/sdspod/baseline.go internal/product/sourcing/sdspod/baseline_test.go
$env:GOWORK='off'
go test ./internal/product/sourcing/sdspod -count=1
git add internal/product/sourcing/sdspod
git commit -m "refactor: define sds baseline policy"
```

### Task 2: Adapt ListingKit Cache Entries to the Domain Decision

**Files:**

- Modify: `internal/listingkit/sds_baseline_readiness_support.go`
- Modify: `internal/listingkit/sds_baseline_service.go`
- Modify: `internal/listingkit/sds_baseline_readiness_test.go`
- Modify: `internal/listingkit/studio_batch_task_gate_test.go`

**Interfaces:**

- Consumes: `sdspod.EvaluateBaseline(sdspod.BaselineSnapshot)`.
- Produces: unchanged `SDSBaselineReadiness` and `sdsBaselineReusableReadiness` values.

- [ ] **Step 1: Add failing API/gate agreement tests**

Create a `Status: SDSBaselineStatusReady`, `ValidationStatus: SDSBaselineValidationStatusUnknown`, valid supported-version cache entry. Assert:

```go
if readiness.Status != SDSBaselineStatusReady { t.Fatalf("readiness = %+v", readiness) }
if reusable := evaluateSDSBaselineReusableReadiness(entry); !reusable.Reusable { t.Fatalf("reusable = %+v", reusable) }
```

Extend the eligible Studio batch gate fixture with the same baseline entry and assert `Evaluate` stays eligible.

- [ ] **Step 2: Verify RED**

```powershell
$env:GOWORK='off'
go test ./internal/listingkit -run 'TestGetSDSBaselineReadinessTreatsReadyCacheStatusWithUnknownValidationAsReady|TestStudioBatchTaskGate' -count=1
```

Expected: API test passes; reusable/gate assertion fails with `validation_not_ready`.

- [ ] **Step 3: Add root mapping helpers and delegate**

After payload decoding, map the entry to `sdspod.BaselineSnapshot` with one of the four payload-state constants. Convert the domain `BaselineDecision` back to `sdsBaselineReusableReadiness` and `SDSBaselineReadiness` without changing public fields. Keep `CanonicalProduct()` decoding, login reconciliation, revalidation, and repository calls in root.

- [ ] **Step 4: Verify GREEN and commit**

```powershell
gofmt -w internal/listingkit/sds_baseline_readiness_support.go internal/listingkit/sds_baseline_service.go internal/listingkit/sds_baseline_readiness_test.go internal/listingkit/studio_batch_task_gate_test.go
$env:GOWORK='off'
go test ./internal/listingkit -run 'TestGetSDSBaselineReadiness|TestSDSBaselineGetCachedBaseline|TestStudioBatchTaskGate' -count=1
git add internal/listingkit/sds_baseline_readiness_support.go internal/listingkit/sds_baseline_service.go internal/listingkit/sds_baseline_readiness_test.go internal/listingkit/studio_batch_task_gate_test.go
git commit -m "refactor: delegate sds baseline policy"
```

### Task 3: Guard and Verify

- [ ] **Step 1: Update boundaries and ownership**

Keep the `sdspod` import guard green. Add an AST guard requiring the ListingKit baseline support file to import `task-processor/internal/product/sourcing/sdspod` and call `sdspod.EvaluateBaseline`. Update the checkpoint to record baseline cache classification and reusability ownership.

- [ ] **Step 2: Run affected verification**

```powershell
git diff --check
$env:GOWORK='off'
go test ./internal/product/sourcing/sdspod -count=1
go test ./internal/listingkit -run 'TestGetSDSBaselineReadiness|TestSDSBaseline|TestStudioBatchTaskGate' -count=1
go test ./internal/listingkit/... -count=1
go vet ./internal/listingkit/... ./internal/product/sourcing/sdspod
```

- [ ] **Step 3: Commit checkpoint and guard**

```powershell
git add internal/product/sourcing/sdspod/boundary_guard_test.go internal/listingkit docs/refactoring/listingkit-boundary-checkpoint.md
git commit -m "docs: record sds baseline ownership"
```

## Final Acceptance Checklist

- [ ] API readiness and Studio batch gate agree that `ready + unknown` is reusable.
- [ ] `baseline_cached + unknown` remains non-reusable.
- [ ] `sdspod` owns only pure baseline status/reusability policy.
- [ ] Root retains decoding, remote validation, login, cache, tenancy, and persistence.
- [ ] Affected tests and `go vet` pass; `go.work.sum` is unchanged.

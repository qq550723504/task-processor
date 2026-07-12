# SDS POD Execution Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delegate deterministic SDS POD execution state policy to `sdspod` and clear stale failure details after non-failure transitions.

**Architecture:** `sdspod` receives neutral execution, SDS-result, and child-task values and returns normalized state plus submission/readiness decisions. Root ListingKit maps task DTOs, chooses request-level required/optional/disabled mode, maintains timestamps and audits, and adapts the domain decision into SHEIN readiness checks.

**Tech Stack:** Go 1.26, `internal/product/sourcing/sdspod`, Go standard library, `go test`, `go vet`.

## Global Constraints

- `sdspod` may import only the Go standard library and `internal/catalog/canonical`; it must not import ListingKit, SHEIN, persistence, runtime, HTTP, Temporal, or external SDKs.
- Do not change request-level POD mode selection, remote SDS execution, retry scheduling, persistence, audit schema, or public JSON fields.
- Preserve active-child then SDS-result then terminal-child precedence.
- Only `failed_blocking` and `failed_degraded` retain failure details; all other states clear them.
- Keep existing Chinese readiness text byte-for-byte unchanged.
- Keep `go.work.sum` unchanged.

---

## File Map

- Create: `internal/product/sourcing/sdspod/execution.go` — neutral execution values and pure policy.
- Create: `internal/product/sourcing/sdspod/execution_test.go` — domain policy table tests.
- Modify: `internal/product/sourcing/sdspod/boundary_guard_test.go` — retain import guard for the new policy file.
- Modify: `internal/listingkit/pod_execution.go` — root DTO adapters; retain timestamps, audit, snapshots, and request policy selection.
- Modify: `internal/listingkit/pod_execution_policy_support.go` — remove relocated policy helpers or reduce to adapters only.
- Modify: `internal/listingkit/pod_execution_test.go` — root regression for stale success failure details.
- Modify: `internal/listingkit/phase6_pod_execution_support_boundary_test.go` — AST/source guard for root delegation and retired helper removal.
- Modify: `docs/refactoring/listingkit-boundary-checkpoint.md` — record SDS POD execution-policy ownership.

### Task 1: Define the Pure Execution Policy

**Files:**

- Create: `internal/product/sourcing/sdspod/execution.go`
- Create: `internal/product/sourcing/sdspod/execution_test.go`

**Interfaces:**

- Produces: `NormalizeExecution`, `DeriveExecution`, `SubmissionBlocked`, and `ReadinessMessage`.
- Consumes: neutral `Execution`, `SDSResult`, and `ChildTask` value types.

- [ ] **Step 1: Write failing domain tests**

Create `execution_test.go` with these tests:

```go
func TestDeriveExecutionClearsHistoricalFailureDetailsAfterSuccess(t *testing.T) {
	result := DeriveExecution(DeriveInput{
		Current: Execution{Provider: ProviderSDS, DependencyMode: DependencyRequired,
			Status: StatusFailedBlocking, FailureReason: "old timeout", FallbackType: FallbackLocalMockup},
		SDS: &SDSResult{Status: "completed", Error: "old timeout"},
	})
	if result.Status != StatusSucceeded || result.FailureReason != "" || result.FallbackType != "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestDeriveExecutionPrioritizesActiveChildOverStaleSDSFailure(t *testing.T) {
	result := DeriveExecution(DeriveInput{
		Current: Execution{Provider: ProviderSDS, DependencyMode: DependencyRequired},
		SDS:     &SDSResult{Status: "failed", Error: "old failure"},
		Children: []ChildTask{{Kind: SDSDesignSyncKind, Status: "processing"}},
	})
	if result.Status != StatusProcessing || result.FailureReason != "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestSubmissionBlockedAndReadinessMessageRespectDependencyMode(t *testing.T) {
	cases := []struct { name string; execution Execution; blocked bool; message string }{
		{"required failure", Execution{Provider: ProviderSDS, DependencyMode: DependencyRequired, Status: StatusFailedBlocking, FailureReason: "timeout"}, true, "SDS 平台处理为发布前置，当前不可提交：timeout"},
		{"optional fallback", Execution{Provider: ProviderSDS, DependencyMode: DependencyOptional, Status: StatusFailedDegraded, FailureReason: "timeout"}, false, "SDS 平台处理失败，当前将按降级素材继续发布：timeout"},
	}
	for _, tt := range cases { t.Run(tt.name, func(t *testing.T) {
		if got := SubmissionBlocked(tt.execution); got != tt.blocked { t.Fatalf("blocked = %t", got) }
		if got := ReadinessMessage(tt.execution); got != tt.message { t.Fatalf("message = %q", got) }
	}) }
}
```

- [ ] **Step 2: Verify RED**

Run:

```powershell
$env:GOWORK='off'
go test ./internal/product/sourcing/sdspod -run 'TestDeriveExecution|TestSubmissionBlockedAndReadinessMessage' -count=1
```

Expected: compilation failure because the neutral execution API does not exist.

- [ ] **Step 3: Add neutral values and policy implementation**

In `execution.go`, define string constants and values:

```go
const (
	ProviderSDS = "sds"
	DependencyRequired = "required"
	DependencyOptional = "optional"
	DependencyDisabled = "disabled"
	StatusNotApplicable = "not_applicable"
	StatusPending = "pending"
	StatusProcessing = "processing"
	StatusSucceeded = "succeeded"
	StatusFailedBlocking = "failed_blocking"
	StatusFailedDegraded = "failed_degraded"
	StatusBypassed = "bypassed"
	FallbackLocalMockup = "local_mockup"
	SDSDesignSyncKind = "sds_design_sync"
)

type Execution struct { Provider, DependencyMode, Status, FailureReason, FallbackType, DecisionSource string }
type SDSResult struct { Status, Error string }
type ChildTask struct { Kind, Status, Error string }
type DeriveInput struct { Current Execution; SDS *SDSResult; Children []ChildTask }
```

Implement `NormalizeExecution`, `DeriveExecution`, `SubmissionBlocked`, and `ReadinessMessage` using the current root rules. Call a private `clearFailureDetailsForStatus` from normalization and derivation; it must preserve details only for the two failed statuses.

- [ ] **Step 4: Verify GREEN and commit the domain policy**

Run:

```powershell
gofmt -w internal/product/sourcing/sdspod/execution.go internal/product/sourcing/sdspod/execution_test.go
$env:GOWORK='off'
go test ./internal/product/sourcing/sdspod -count=1
git add internal/product/sourcing/sdspod
git commit -m "refactor: define sds pod execution policy"
```

Expected: PASS; package imports remain within its existing boundary.

### Task 2: Delegate Root POD State Decisions

**Files:**

- Modify: `internal/listingkit/pod_execution.go`
- Modify: `internal/listingkit/pod_execution_policy_support.go`
- Modify: `internal/listingkit/pod_execution_test.go`

**Interfaces:**

- Consumes: `sdspod.Execution`, `sdspod.DeriveInput`, `sdspod.DeriveExecution`, `sdspod.NormalizeExecution`, `sdspod.SubmissionBlocked`, and `sdspod.ReadinessMessage`.
- Produces: unchanged `PodExecutionSummary` and existing SHEIN readiness items.

- [ ] **Step 1: Add a root regression test for stale successful SDS error text**

Add to `pod_execution_test.go`:

```go
func TestDerivePodExecutionSummaryClearsSDSFailureDetailsAfterSuccess(t *testing.T) {
	result := derivePodExecutionSummary(
		&PodExecutionSummary{Provider: podProviderSDS, DependencyMode: podDependencyModeRequired,
			Status: podStatusFailedBlocking, FailureReason: "old timeout", FallbackType: podFallbackLocalMockup},
		&SDSSyncSummary{Status: "completed", Error: "old timeout"}, nil,
		newRequiredSDSGenerateRequest(), time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC),
	)
	if result.Status != podStatusSucceeded || result.FailureReason != "" || result.FallbackType != "" {
		t.Fatalf("result = %+v", result)
	}
}
```

Define `newRequiredSDSGenerateRequest` in the same test file with a SHEIN request, one image URL, `ProcessImages: false`, and `SDS.VariantID: 901`. Run this test before adapter changes; it must fail because the root derivation leaves `FailureReason` populated.

- [ ] **Step 2: Add root adapters**

Add private mapping helpers in `pod_execution.go`:

```go
func podExecutionPolicyState(pod *PodExecutionSummary) sdspod.Execution
func applyPodExecutionPolicyState(pod *PodExecutionSummary, state sdspod.Execution) *PodExecutionSummary
func podExecutionPolicySDS(sds *SDSSyncSummary) *sdspod.SDSResult
func podExecutionPolicyChildren(children []ChildTaskState) []sdspod.ChildTask
```

`applyPodExecutionPolicyState` returns a copied normalized `PodExecutionSummary`; it must not transfer audit history or timestamps. Leave request policy selection (`determinePODExecutionPolicy`) in root and populate missing provider, dependency mode, and decision source before calling the domain policy.

- [ ] **Step 3: Replace root pure helpers with domain delegation**

Make `normalizePodExecutionSummary` delegate to `sdspod.NormalizeExecution`. In `derivePodExecutionSummary`, keep disabled-state handling, timestamps, and request policy selection in root, then call:

```go
state := sdspod.DeriveExecution(sdspod.DeriveInput{
	Current:  podExecutionPolicyState(pod),
	SDS:      podExecutionPolicySDS(sds),
	Children: podExecutionPolicyChildren(childTasks),
})
pod = applyPodExecutionPolicyState(pod, state)
```

Replace `podSubmissionBlocked` and `podReadinessMessage` bodies with adapters to the domain functions. Delete the moved root helpers: `inferPodStatusFromSDS`, `inferActivePodStatusFromChildTasks`, `inferPodStatusFromChildTasks`, `mapSDSStatusToPODStatus`, and `podFailureStatusForMode`.

- [ ] **Step 4: Verify root behavior and commit**

Run:

```powershell
gofmt -w internal/listingkit/pod_execution.go internal/listingkit/pod_execution_policy_support.go internal/listingkit/pod_execution_test.go
$env:GOWORK='off'
go test ./internal/listingkit -run 'Test(EnsureTaskPodExecution|DerivePodExecutionSummary|MarkPodExecutionStatus)' -count=1
git add internal/listingkit/pod_execution.go internal/listingkit/pod_execution_policy_support.go internal/listingkit/pod_execution_test.go
git commit -m "refactor: delegate pod execution policy"
```

Expected: PASS, including stale failure-detail clearing and audit/timestamp tests.

### Task 3: Guard the Boundary and Verify the Slice

**Files:**

- Modify: `internal/product/sourcing/sdspod/boundary_guard_test.go`
- Modify: `internal/listingkit/phase6_pod_execution_support_boundary_test.go`
- Modify: `docs/refactoring/listingkit-boundary-checkpoint.md`

**Interfaces:**

- Protects: `sdspod` import boundary and root delegation to `sdspod`.

- [ ] **Step 1: Add a failing root AST boundary assertion**

Update `phase6_pod_execution_support_boundary_test.go` to parse `pod_execution.go`. Require import path `task-processor/internal/product/sourcing/sdspod` and selector calls to `sdspod.DeriveExecution` and `sdspod.NormalizeExecution`. Reject declarations named `inferPodStatusFromSDS`, `inferActivePodStatusFromChildTasks`, `inferPodStatusFromChildTasks`, `mapSDSStatusToPODStatus`, and `podFailureStatusForMode` in root policy files.

Run:

```powershell
$env:GOWORK='off'
go test ./internal/listingkit -run TestPodExecutionSupportBoundary -count=1
```

Expected: FAIL before Task 2 delegation; PASS after it.

- [ ] **Step 2: Record the ownership checkpoint**

Extend the `internal/product/sourcing/sdspod` section of `docs/refactoring/listingkit-boundary-checkpoint.md` to state that it owns deterministic SDS POD execution status mapping, failure-detail hygiene, submission blocking, and readiness messages. State that root retains request policy selection, DTO adaptation, timestamps, audit history, persistence, and SHEIN readiness-shape assembly.

- [ ] **Step 3: Run full affected verification**

Run:

```powershell
gofmt -w internal/product/sourcing/sdspod internal/listingkit/pod_execution.go internal/listingkit/pod_execution_policy_support.go internal/listingkit/pod_execution_test.go internal/listingkit/phase6_pod_execution_support_boundary_test.go
git diff --check
$env:GOWORK='off'
go test ./internal/product/sourcing/sdspod -count=1
go test ./internal/listingkit -run 'TestPodExecution|TestEnsureTaskPodExecution|TestDerivePodExecutionSummary|TestSheinSubmitReadiness' -count=1
go test ./internal/listingkit/... -count=1
go vet ./internal/listingkit/... ./internal/product/sourcing/sdspod
```

Expected: all commands exit 0; `go.work.sum` remains unchanged.

- [ ] **Step 4: Commit guards and ownership record**

```powershell
git add internal/product/sourcing/sdspod/boundary_guard_test.go internal/listingkit/phase6_pod_execution_support_boundary_test.go docs/refactoring/listingkit-boundary-checkpoint.md
git commit -m "docs: record sds pod execution ownership"
```

## Final Acceptance Checklist

- [ ] `sdspod` owns deterministic SDS POD execution policy without importing root ListingKit.
- [ ] Root keeps request policy selection, DTO adaptation, timestamps, audit, persistence, and SHEIN readiness shaping.
- [ ] Successful and active states clear stale failure details.
- [ ] Required/optional blocking semantics and existing Chinese messages are unchanged.
- [ ] Boundary guards validate actual delegation and retired helper removal.
- [ ] SDS POD and ListingKit affected tests plus `go vet` pass.
- [ ] `go.work.sum` is unchanged.

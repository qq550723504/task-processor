# Task Processor Framework Phase 6B ListingKit Summary Review Semantics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make ListingKit summary/review semantics more explicit so the current `shein_review` issue-ordering compatibility rule stops depending on an opaque temporary-warning suppression helper.

**Architecture:** Keep the work fully feature-owned inside `internal/listingkit` and stay within the existing finalization seam structure from `Phase 6A`. The slice should separate review-stage issue derivation from generic summary finalization, make the coverage-guard compatibility rule more explicit, and preserve the current `prepareReview -> coverage guard -> asset dispatch -> complete` orchestration order.

**Tech Stack:** Go, ListingKit workflow layer, existing `workflowRecorder`, `workflow_review_state.go`, `workflow_assets_test.go`, source-boundary tests, package-level Go tests

---

## Out of Scope For This Slice

- redesigning deferred asset-dispatch mutation contract
- changing platform post-processing behavior
- changing coverage-guard business policy
- introducing a generic workflow state machine or execution context
- moving workflow concerns into HTTP/runtime/bootstrap layers

## File Structure

### Existing hotspots

- [internal/listingkit/workflow_platform_summary_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_summary_phase.go:1)
  - current summary seam with `prepareReview(...)`, `complete(...)`, `run(...)`, and `withSheinVariantCoverageReviewSuppressed(...)`
- [internal/listingkit/workflow_review_state.go](/D:/code/task-processor/internal/listingkit/workflow_review_state.go:1)
  - current review-state helpers, including `addSheinReviewWorkflowIssues(...)`
- [internal/listingkit/workflow_platform_finalize_phase.go](/D:/code/task-processor/internal/listingkit/workflow_platform_finalize_phase.go:1)
  - current orchestration entry whose order must remain stable
- [internal/listingkit/workflow_assets_test.go](/D:/code/task-processor/internal/listingkit/workflow_assets_test.go:1)
  - current behavior harness for finalization and review-stage regressions
- [internal/listingkit/phase6a_platform_finalize_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase6a_platform_finalize_boundary_test.go:1)
  - current source-boundary guardrail for finalize orchestration

### Planned new files

- `internal/listingkit/workflow_platform_review_phase.go`
  - owns review-stage warning merge and review-issue derivation semantics
- `internal/listingkit/phase6b_summary_review_boundary_test.go`
  - locks the new review/finalization seam split and keeps compatibility logic from drifting back into opaque suppression

### Files expected to shrink

- `internal/listingkit/workflow_platform_summary_phase.go`
  - should stop being the primary home of the review-compatibility hack

Each new file should have one clear responsibility. The design goal is not “more files,” but “review-stage semantics stop hiding inside an opaque temporary suppression helper.”

## Task 1: Extract explicit review-stage seam from `workflow_platform_summary_phase.go`

**Files:**
- Create: `internal/listingkit/workflow_platform_review_phase.go`
- Modify: `internal/listingkit/workflow_platform_summary_phase.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing tests for explicit review-stage ownership**

Extend `workflow_assets_test.go` with a behavior-level test that directly locks the compatibility rule currently preserved by `withSheinVariantCoverageReviewSuppressed(...)`:

```go
func TestPlatformReviewPhaseKeepsCoverageWarningOutOfSheinReviewIssues(t *testing.T) {
\tt.Parallel()

\tcoverageWarning := "coverage guard warning"
\tresult := &ListingKitResult{
\t\tShein: &SheinPackage{
\t\t\tInspection: &SheinInspection{
\t\t\t\tNeedsReview: true,
\t\t\t\tSummary:     []string{"inspection review"},
\t\t\t},
\t\t\tReviewNotes: []string{coverageWarning},
\t\t\tMetadata: map[string]string{
\t\t\t\tsheinVariantImageCoverageStatusKey:  "blocked",
\t\t\t\tsheinVariantImageCoverageMessageKey: coverageWarning,
\t\t\t},
\t\t},
\t\tSummary: &GenerationSummary{
\t\t\tNeedsReview: true,
\t\t\tWarnings:    []string{coverageWarning},
\t\t},
\t\tReviewReasons: []string{coverageWarning},
\t}
\tsnapshot := &StandardProductSnapshot{
\t\tSummary: &GenerationSummary{Warnings: []string{"snapshot warning"}},
\t}

\tbuildPlatformReviewPhase().run(result, snapshot)

\tif !strings.Contains(strings.Join(result.Summary.Warnings, "\n"), "snapshot warning") {
\t\tt.Fatalf("summary warnings = %#v, want snapshot warning merged", result.Summary.Warnings)
\t}
\tfor _, issue := range result.WorkflowIssues {
\t\tif issue.Stage == "shein_review" && issue.Severity == WorkflowIssueSeverityReview && issue.Message == coverageWarning {
\t\t\tt.Fatalf("workflow issues = %+v, coverage warning should not become shein_review issue", result.WorkflowIssues)
\t\t}
\t}
}
```

This test must stay behavior-focused. Do not replace it with a source-string assertion.

- [ ] **Step 2: Run focused review-stage verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(PlatformReviewPhaseKeepsCoverageWarningOutOfSheinReviewIssues|PlatformSummaryPhaseDoesNotConvertCoverageGuardReasonIntoSheinReviewIssue|RunWorkflowAppliesVariantCoverageGuardAfterSheinReview)" -count=1
```

Expected: FAIL before the refactor because `buildPlatformReviewPhase()` does not exist yet.

- [ ] **Step 3: Add an explicit review-stage seam**

Create `internal/listingkit/workflow_platform_review_phase.go` with a focused helper such as:

```go
type platformReviewPhase struct{}

func buildPlatformReviewPhase() *platformReviewPhase {
\treturn &platformReviewPhase{}
}

func (p *platformReviewPhase) run(
\tfinal *ListingKitResult,
\tsnapshot *StandardProductSnapshot,
) {
\tif final.Summary == nil {
\t\tfinal.Summary = &GenerationSummary{}
\t}
\tif snapshot != nil && snapshot.Summary != nil {
\t\tfinal.Summary.Warnings = uniqueStrings(append(final.Summary.Warnings, snapshot.Summary.Warnings...))
\t}

\tsheinReviewStage := newWorkflowRecorder(final).Start("shein_review", "")
\tapplySheinInspectionReviewToSummary(final)
\twithCoverageGuardReviewIssuesSuppressed(final, func() {
\t\taddSheinReviewWorkflowIssues(final)
\t})
\tsheinReviewStage.Complete()
}
```

Important:

- keep the review-stage semantics unchanged
- keep the compatibility rule explicit and local to review-stage behavior
- do not move final summary completion or preview sync into this file

- [ ] **Step 4: Rewire `workflow_platform_summary_phase.go` through the review seam**

Update `workflow_platform_summary_phase.go` so `prepareReview(...)` delegates through the new review seam instead of directly owning warning merge and `shein_review` issue derivation.

- [ ] **Step 5: Re-run focused review-stage verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(PlatformReviewPhaseKeepsCoverageWarningOutOfSheinReviewIssues|PlatformSummaryPhaseDoesNotConvertCoverageGuardReasonIntoSheinReviewIssue|RunWorkflowAppliesVariantCoverageGuardAfterSheinReview)" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/workflow_platform_review_phase.go internal/listingkit/workflow_platform_summary_phase.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: extract listingkit platform review phase"
```

## Task 2: Replace opaque temporary slice mutation with clearer compatibility ownership

**Files:**
- Modify: `internal/listingkit/workflow_platform_review_phase.go`
- Modify: `internal/listingkit/workflow_platform_summary_phase.go`
- Modify: `internal/listingkit/workflow_review_state.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing tests for explicit compatibility ownership**

Extend `workflow_assets_test.go` so it locks the intended compatibility rule more directly:

```go
func TestPlatformReviewPhasePreservesCoverageWarningInSummaryAndReviewNotes(t *testing.T) {
\tt.Parallel()

\tcoverageWarning := "coverage guard warning"
\tresult := &ListingKitResult{
\t\tShein: &SheinPackage{
\t\t\tInspection: &SheinInspection{
\t\t\t\tNeedsReview: true,
\t\t\t\tSummary:     []string{"inspection review"},
\t\t\t},
\t\t\tReviewNotes: []string{coverageWarning},
\t\t\tMetadata: map[string]string{
\t\t\t\tsheinVariantImageCoverageStatusKey:  "blocked",
\t\t\t\tsheinVariantImageCoverageMessageKey: coverageWarning,
\t\t\t},
\t\t},
\t\tSummary: &GenerationSummary{
\t\t\tNeedsReview: true,
\t\t\tWarnings:    []string{coverageWarning},
\t\t},
\t\tReviewReasons: []string{coverageWarning},
\t}

\tbuildPlatformReviewPhase().run(result, nil)

\tif !strings.Contains(strings.Join(result.Summary.Warnings, "\n"), coverageWarning) {
\t\tt.Fatalf("summary warnings = %#v, want coverage warning preserved", result.Summary.Warnings)
\t}
\tif !strings.Contains(strings.Join(result.ReviewReasons, "\n"), coverageWarning) {
\t\tt.Fatalf("review reasons = %#v, want coverage reason preserved", result.ReviewReasons)
\t}
\tif !strings.Contains(strings.Join(result.Shein.ReviewNotes, "\n"), coverageWarning) {
\t\tt.Fatalf("shein review notes = %#v, want coverage note preserved", result.Shein.ReviewNotes)
\t}
}
```

This test is meant to ensure we do not “solve” the issue-ordering rule by accidentally dropping the warning from the final result state.

- [ ] **Step 2: Run focused compatibility verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(PlatformReviewPhaseKeepsCoverageWarningOutOfSheinReviewIssues|PlatformReviewPhasePreservesCoverageWarningInSummaryAndReviewNotes|RunWorkflowAppliesVariantCoverageGuardAfterSheinReview)" -count=1
```

Expected: PASS before the refactor, establishing the current compatibility baseline.

- [ ] **Step 3: Make the compatibility rule more explicit**

Refactor toward one of these bounded shapes:

- give `addSheinReviewWorkflowIssues(...)` an explicit way to ignore coverage-guard-origin review reasons during review-issue derivation, or
- move the suppression into a narrower helper that expresses “derive review issues while excluding coverage-guard warnings” instead of temporarily mutating three slices in place

The intended result is that `workflow_platform_summary_phase.go` stops being the primary home of opaque temporary state mutation.

Important:

- keep current behavior unchanged
- preserve `shein_cookie_unavailable` issue handling
- do not change `WorkflowIssue` severity policy

- [ ] **Step 4: Re-run focused compatibility verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(PlatformReviewPhaseKeepsCoverageWarningOutOfSheinReviewIssues|PlatformReviewPhasePreservesCoverageWarningInSummaryAndReviewNotes|RunWorkflowAppliesVariantCoverageGuardAfterSheinReview)" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/workflow_platform_review_phase.go internal/listingkit/workflow_platform_summary_phase.go internal/listingkit/workflow_review_state.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: clarify listingkit review issue compatibility"
```

## Task 3: Re-scope the summary seam to durable completion behavior only

**Files:**
- Modify: `internal/listingkit/workflow_platform_summary_phase.go`
- Modify: `internal/listingkit/workflow_platform_finalize_phase.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Write the failing tests for summary completion ownership**

Extend `workflow_assets_test.go` with a direct seam-level test that locks summary completion as a durable completion concern, independent of review-stage derivation:

```go
func TestPlatformSummaryPhaseCompletesSummaryAndPreviewSync(t *testing.T) {
\tt.Parallel()

\ttask := &Task{ID: "listingkit-task-summary-complete"}
\tfinal := &ListingKitResult{
\t\tAssetBundle: &asset.Bundle{
\t\t\tAssets: []asset.Asset{{
\t\t\t\tID:   "asset-main",
\t\t\t\tKind: asset.KindMainImage,
\t\t\t\tURL:  "https://cdn.example.com/main.jpg",
\t\t\t}},
\t\t},
\t\tShein: &SheinPackage{
\t\t\tInspection: &SheinInspection{
\t\t\t\tNeedsReview: true,
\t\t\t\tSummary:     []string{"manual review"},
\t\t\t},
\t\t\tImageBundle: &common.PublishImageBundle{
\t\t\t\tPlatform: "shein",
\t\t\t\tMain: &common.BundleSlot{
\t\t\t\t\tKey:     "main",
\t\t\t\t\tAssetID: "asset-main",
\t\t\t\t\tURL:     "https://cdn.example.com/main.jpg",
\t\t\t\t},
\t\t\t},
\t\t},
\t\tSummary: &GenerationSummary{Warnings: []string{"existing warning"}},
\t}

\tresult := buildPlatformSummaryPhase().run(task, final)

\tif result.Summary == nil || result.Summary.ReviewCount == 0 || result.Summary.IssueCount == 0 {
\t\tt.Fatalf("summary = %+v, want finalized counts", result.Summary)
\t}
\tif len(result.PlatformAssetRenderPreviews) == 0 {
\t\tt.Fatalf("platform previews = %+v, want synced previews", result.PlatformAssetRenderPreviews)
\t}
}
```

Expected before the refactor: FAIL because `buildPlatformSummaryPhase().run(task, final)` does not yet represent the seam in that narrowed form.

- [ ] **Step 2: Run focused summary-completion verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(PlatformSummaryPhaseCompletesSummaryAndPreviewSync|RunWorkflowFinalizesSummaryAfterPlatformDispatch|RunWorkflowAppliesSheinPlatformFinalizationDecorations)" -count=1
```

Expected: FAIL before the refactor because the seam contract is still broader.

- [ ] **Step 3: Narrow `workflow_platform_summary_phase.go` to durable completion**

Refactor so this file becomes the home of:

- `FinalizeSummary()`
- preview synchronization
- final finalization logging

and no longer acts as the main home of review-stage issue-derivation compatibility.

`workflow_platform_finalize_phase.go` should keep the already-validated orchestration order, but the summary seam itself should read more clearly as “completion semantics,” not “review compatibility adapter.”

- [ ] **Step 4: Re-run focused summary-completion verification**

Run:

```powershell
go test ./internal/listingkit -run "Test(PlatformSummaryPhaseCompletesSummaryAndPreviewSync|RunWorkflowFinalizesSummaryAfterPlatformDispatch|RunWorkflowAppliesSheinPlatformFinalizationDecorations)" -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/workflow_platform_summary_phase.go internal/listingkit/workflow_platform_finalize_phase.go internal/listingkit/workflow_assets_test.go
git commit -m "refactor: narrow listingkit summary completion seam"
```

## Task 4: Lock summary/review seam ownership boundaries

**Files:**
- Create: `internal/listingkit/phase6b_summary_review_boundary_test.go`
- Modify: `internal/listingkit/phase6a_platform_finalize_boundary_test.go`
- Modify: `internal/listingkit/workflow_assets_test.go`

- [ ] **Step 1: Add boundary guardrails**

Create a new source-boundary test file that locks three things:

1. review-stage warning merge and issue derivation no longer primarily live in `workflow_platform_summary_phase.go`
2. `workflow_platform_summary_phase.go` remains the durable completion seam
3. `workflow_platform_finalize_phase.go` keeps the validated ordering between review prep, coverage guard, asset dispatch, and completion

Suggested checks:

```go
func TestWorkflowPlatformSummaryPhaseFileOwnsCompletionNotReviewCompatibility(t *testing.T) {
\tt.Parallel()

\tsrc, err := os.ReadFile("workflow_platform_summary_phase.go")
\tif err != nil {
\t\tt.Fatalf("ReadFile(workflow_platform_summary_phase.go) error = %v", err)
\t}
\tcontent := string(src)

\tfor _, needle := range []string{
\t\t"FinalizeSummary()",
\t\t"syncAssetRenderPreviews(final)",
\t} {
\t\tif !strings.Contains(content, needle) {
\t\t\tt.Fatalf("workflow_platform_summary_phase.go should contain %q", needle)
\t\t}
\t}

\tfor _, needle := range []string{
\t\t"withSheinVariantCoverageReviewSuppressed(",
\t\t"addSheinReviewWorkflowIssues(",
\t} {
\t\tif strings.Contains(content, needle) {
\t\t\tt.Fatalf("workflow_platform_summary_phase.go should not contain %q", needle)
\t\t}
\t}
}
```

Also update the existing finalization boundary test only as needed so it stays aligned with the new seam split.

- [ ] **Step 2: Run full ListingKit verification**

Run:

```powershell
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/listingkit/phase6b_summary_review_boundary_test.go internal/listingkit/phase6a_platform_finalize_boundary_test.go internal/listingkit/workflow_assets_test.go
git commit -m "test: lock listingkit summary review boundaries"
```

## Self-Review

### Spec coverage

This plan intentionally covers one bounded hotspot:

- review-stage warning merge
- review issue derivation semantics
- coverage-guard compatibility ownership
- durable summary completion boundary
- source and behavior guardrails for the new split

It does not mix in asset-dispatch mutation contract cleanup, generic workflow abstractions, or HTTP/runtime work.

### Reuse check

This slice explicitly reuses mature local patterns already present in ListingKit:

- keep feature-owned bounded seams
- keep `workflow_platform_finalize_phase.go` as the orchestration entry
- keep behavior-first regression tests as the main protection against silent sequencing bugs

It does not invent a generic review engine or workflow framework.

### Root-cause check

The problem being addressed is not just “too many methods in one file.” The real problem is that:

- `workflow_platform_summary_phase.go`

currently hides a fragile compatibility rule through temporary slice mutation, even though that rule is about review-stage issue derivation rather than generic summary completion.

The plan therefore focuses on:

- making review-stage semantics more explicit
- preserving current ordering behavior
- reducing opaque temporary suppression
- locking the new ownership with narrow source and behavior guardrails

### Scope discipline

This is a bounded slice:

- no deferred asset-dispatch contract redesign
- no workflow framework work
- no business-policy change
- no return to postprocess seam extraction

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-30-task-processor-framework-phase6b-summary-review-semantics.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**

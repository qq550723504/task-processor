# ListingKit SDS Batch Production Closure Implementation Plan

> **For agentic workers:** Use test-driven development. Complete tasks in order unless a task explicitly says it may run in parallel. Keep changes narrow and preserve the current item/attempt async generation architecture.

## Status

Ready for implementation.

## Requirement Source

- `docs/superpowers/specs/2026-06-20-listingkit-sds-batch-production-closure-requirements.md`

## Goal

Close the remaining production gaps in the SDS batch flow by delivering:

- correct `design x selection` task fan-out
- durable batch-to-ListingKit-task ownership
- candidate-level idempotency
- complete backend final gating
- strict SDS baseline reuse
- compatibility-aware generation grouping
- structured created/reused/rejected/failed results
- real task/submission projection
- regeneration protection
- partial batch submission

## Architecture

Keep the current backbone:

```text
batch -> item -> attempt -> materialized design
```

Add a durable task-ownership boundary:

```text
batch task candidate -> durable batch task link -> ListingKit task
```

The implementation should keep `internal/listingkit` as the current product orchestration surface while isolating the new policy and persistence logic into focused files. Do not start a broad package migration as part of this plan.

## Definition of Done

The plan is complete when all of the following are true:

```text
3 compatible selections x 2 approved designs creates or reuses 6 tasks
session-less batches preserve created tasks after refresh
concurrent duplicate task creation produces one ListingKit task
backend blocks invalid baseline/store/ownership combinations
shared generation groups by compatibility fingerprint rather than dimensions alone
frontend displays rejected and failed candidates separately
new tasks display task_created rather than draft_saved
regeneration cannot orphan already-created tasks
batch submission continues after one task fails
full backend and targeted frontend tests pass
one real SDS -> ListingKit -> SHEIN draft run is recorded
```

---

## Task 0: Establish the Verification Baseline

**Files:**

- No production files required
- Optional update: `docs/product/validation/runs/README.md`

- [ ] **Step 1: Record the starting commit**

Record the current `master` SHA in the implementation PR description.

- [ ] **Step 2: Run the existing backend baseline**

```bash
go test ./internal/listingkit -count=1
go test ./internal/listing/studio -count=1
go test ./internal/listing/submission -count=1
```

Expected: PASS before implementation changes.

- [ ] **Step 3: Run the targeted frontend baseline**

```bash
cd web/listingkit-ui
npm test -- \
  src/lib/api/shein-studio-batches.test.ts \
  src/lib/shein-studio/grouped-sds-create.test.ts \
  src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx \
  src/components/listingkit/shein-studio/shein-batch-publish-gate.test.tsx
npm run typecheck
```

Expected: existing tests pass. If a listed test file does not exist, add it in the task that owns that behavior rather than silently skipping coverage.

- [ ] **Step 4: Commit only if baseline documentation changed**

```bash
git commit -m "docs: record SDS batch closure baseline"
```

---

## Task 1: Fix Candidate Fan-Out

**Files:**

- Modify: `internal/listingkit/task_studio_batch_candidate_support.go`
- Modify: `internal/listingkit/task_studio_batch_task_execute_adapter.go`
- Modify: `internal/listingkit/studio_batch_service_test.go`
- Test: `internal/listingkit/studio_batch_service_test.go`

- [ ] **Step 1: Add failing fan-out tests**

Add tests covering:

```text
TestServiceCreateStudioBatchTasks_FansOutEachDesignToEveryCompatibleSelection
TestBuildStudioBatchTaskCandidates_PerProductRequiresOneSelection
TestBuildStudioBatchTaskCandidates_SharedMismatchReturnsStructuredRejection
TestBuildStudioBatchTaskCandidates_DoesNotFallbackToAllBatchSelectionsForOwnedItem
```

The main test should create:

- one shared item
- three selection IDs owned by the item
- two approved designs

Expected result:

```text
6 created task candidates
6 distinct selection_id values across design/selection tuples
no task silently uses only the first selection
```

- [ ] **Step 2: Run the tests and verify failure**

```bash
go test ./internal/listingkit -run "TestServiceCreateStudioBatchTasks_FansOut|TestBuildStudioBatchTaskCandidates" -count=1
```

Expected: FAIL because the current builder returns one chosen candidate per design.

- [ ] **Step 3: Replace single-candidate selection with candidate slices**

Refactor the candidate builder so each design returns:

```go
[]studioBatchTaskCandidate
```

Rules:

- resolve only `item.SelectionIDs`
- `per_product` requires exactly one resolved selection
- shared mode requires all resolved selections to have one complete fingerprint
- append one candidate per selection
- append one structured rejection per rejected design/item relationship

Do not use `studioBatchAllGroupedSelections(batch)` as a normal fallback when an item has explicit ownership.

- [ ] **Step 4: Preserve deterministic order**

Candidate order must be stable:

```text
design request order
then item selection order
```

This keeps API results and tests deterministic.

- [ ] **Step 5: Update task execution adapter**

Ensure the execute adapter creates one execute item for each candidate, not one per design.

- [ ] **Step 6: Run targeted tests**

```bash
go test ./internal/listingkit -run "TestServiceCreateStudioBatchTasks_FansOut|TestBuildStudioBatchTaskCandidates|TestServiceCreateStudioBatchTasks_RejectsCompatibilityMismatch" -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/listingkit
git commit -m "fix: fan out SDS batch tasks by selection"
```

---

## Task 2: Add Durable Batch Task Links

**Files:**

- Create: `internal/listingkit/studio_batch_task_link_model.go`
- Create: `internal/listingkit/studio_batch_task_link_repository.go`
- Create: `internal/listingkit/studio_batch_task_link_repository_mem.go`
- Create: `internal/listingkit/studio_batch_task_link_repository_gorm.go`
- Create: `internal/listingkit/studio_batch_task_link_repository_test.go`
- Modify: `internal/listingkit/httpapi/builders_repository_schema.go`
- Modify: `internal/listingkit/service.go`
- Modify: `internal/listingkit/service_collaborators.go`
- Test: `internal/listingkit/studio_batch_task_link_repository_test.go`

- [ ] **Step 1: Add failing repository tests**

Cover:

```text
create and load link by candidate key
list links by batch
tenant isolation
unique candidate key
unique batch/item/design/selection tuple
update projection status
```

Recommended model:

```go
type StudioBatchTaskLinkRecord struct {
    ID                       string
    TenantID                 string
    UserID                   string
    BatchID                  string
    ItemID                   string
    DesignID                 string
    SelectionID              string
    CompatibilityFingerprint string
    SheinStoreID             int64
    ListingKitTaskID         string
    CandidateKey             string
    Status                   string
    ReasonCode               string
    Message                  string
    CreatedAt                time.Time
    UpdatedAt                time.Time
}
```

- [ ] **Step 2: Run repository tests and verify failure**

```bash
go test ./internal/listingkit -run "Test.*StudioBatchTaskLink" -count=1
```

Expected: FAIL with missing model/repository types.

- [ ] **Step 3: Implement the repository contract**

Required methods:

```go
GetStudioBatchTaskLinkByCandidateKey
CreateStudioBatchTaskLink
UpdateStudioBatchTaskLink
ListStudioBatchTaskLinksByBatchID
```

Add a reservation/claim method if needed for concurrency:

```go
ClaimStudioBatchTaskCandidate
```

- [ ] **Step 4: Implement in-memory behavior**

The in-memory repository must enforce the same uniqueness semantics as GORM tests.

- [ ] **Step 5: Implement GORM behavior and migration**

Add indexes for:

```text
tenant_id + candidate_key unique
tenant_id + batch_id + item_id + design_id + selection_id unique
batch_id
listingkit_task_id
```

Wire migration through `internal/listingkit/httpapi/builders_repository_schema.go`.

- [ ] **Step 6: Wire the repository into the service**

Use an explicit collaborator. Do not hide it behind an unrelated session repository.

- [ ] **Step 7: Run tests**

```bash
go test ./internal/listingkit -run "Test.*StudioBatchTaskLink" -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/listingkit
git commit -m "feat: persist SDS batch task ownership"
```

---

## Task 3: Add Candidate Idempotency and Durable Reuse

**Files:**

- Modify: `internal/listingkit/task_studio_batch_candidate_support.go`
- Modify: `internal/listingkit/task_studio_batch_task_existing_support.go`
- Modify: `internal/listingkit/task_studio_batch_task_execute_adapter.go`
- Modify: `internal/listingkit/task_studio_batch_task_flow_support.go`
- Modify: `internal/listingkit/task_studio_batch_detail_support.go`
- Modify: `internal/listingkit/studio_batch_service_test.go`

- [ ] **Step 1: Add failing idempotency tests**

Add:

```text
TestServiceCreateStudioBatchTasks_ReusesDurableLinkedTaskWithoutSession
TestServiceCreateStudioBatchTasks_ConcurrentRequestsCreateOneTask
TestStudioBatchDetail_LoadsCreatedTasksFromDurableLinks
TestServiceCreateStudioBatchTasks_RecoversReservedCandidate
```

- [ ] **Step 2: Add deterministic candidate key**

Build from normalized:

```text
tenant_id|batch_id|item_id|design_id|selection_id|store_id
```

Use SHA-256 or another stable repository-standard hash.

- [ ] **Step 3: Reserve candidate ownership before creating the task**

Recommended states:

```text
reserved
creating
created
failed
```

The repository must ensure only one request owns the create operation.

- [ ] **Step 4: Reconcile existing tasks**

Reuse order:

1. durable link with valid task
2. legacy session `CreatedTasks` compatibility lookup
3. create new task

When a legacy task is reused, write a durable link so the next request no longer depends on session state.

- [ ] **Step 5: Persist task metadata**

Populate created task/link fields:

```text
item_id
selection_id
compatibility_fingerprint
status = task_created
```

- [ ] **Step 6: Load created tasks from links in batch detail**

The batch detail path must:

- read durable links first
- optionally merge legacy session records not yet linked
- deduplicate by ListingKit task ID

- [ ] **Step 7: Run tests**

```bash
go test ./internal/listingkit -run "TestServiceCreateStudioBatchTasks_Reuses|TestServiceCreateStudioBatchTasks_Concurrent|TestStudioBatchDetail_LoadsCreatedTasks|TestServiceCreateStudioBatchTasks_RecoversReserved" -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/listingkit
git commit -m "feat: make SDS batch task creation idempotent"
```

---

## Task 4: Implement the Complete Backend Final Gate

**Files:**

- Create: `internal/listingkit/studio_batch_task_gate.go`
- Create: `internal/listingkit/studio_batch_task_gate_test.go`
- Modify: `internal/listingkit/task_studio_batch_candidate_support.go`
- Modify: `internal/listingkit/service_studio_batch_wiring_support.go`
- Modify: `internal/listingkit/studio_batch_service.go`
- Test: `internal/listingkit/studio_batch_task_gate_test.go`

- [ ] **Step 1: Define gate collaborators**

Use narrow interfaces for:

```go
type StudioBatchBaselineReadinessChecker interface { ... }
type StudioBatchStoreValidator interface { ... }
```

Do not import concrete management or HTTP client types into the policy object.

- [ ] **Step 2: Add table-driven failing tests**

Cover reason codes:

```text
design_not_found
design_target_mismatch
design_not_approved
design_image_missing
selection_not_in_batch
selection_not_in_item
selection_identity_incomplete
selection_variant_incompatible
store_missing
store_invalid
store_not_available
baseline_missing
baseline_not_ready
baseline_invalid
baseline_stale
baseline_check_unavailable
compatibility_incomplete
compatibility_mismatch
```

Also cover one fully eligible candidate.

- [ ] **Step 3: Implement candidate-level evaluation**

Return a structured result rather than a generic error:

```go
type studioBatchTaskGateResult struct {
    Eligible   bool
    ReasonCode string
    Message    string
}
```

- [ ] **Step 4: Make creation continue after rejection**

For each candidate:

- eligible -> execute or reuse
- rejected -> append `RejectedTasks`
- operational error -> append `FailedTasks`

Do not return early for expected business rejections.

- [ ] **Step 5: Batch shared checks**

Cache repeated checks during one request:

- baseline identity readiness
- store validation
- compatibility fingerprint

Avoid one remote lookup per design when candidates share selection/store identity.

- [ ] **Step 6: Run tests**

```bash
go test ./internal/listingkit -run "TestStudioBatchTaskGate|TestServiceCreateStudioBatchTasks_.*Reject" -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/listingkit
git commit -m "feat: enforce SDS batch final task gate"
```

---

## Task 5: Make Baseline Reuse Strict

**Files:**

- Modify: `internal/listingkit/sds_baseline_service.go`
- Modify: `internal/listingkit/sds_baseline_validator.go`
- Modify: `internal/listingkit/sds_baseline_readiness_support.go`
- Modify: `internal/listingkit/sds_baseline_readiness_test.go`
- Modify: `internal/listingkit/sds_baseline_validator_test.go`
- Modify: `internal/listingkit/workflow_studio_sds_metadata_test.go`

- [ ] **Step 1: Add failing strict-reuse tests**

Add:

```text
TestSDSBaselineGetCachedBaseline_RejectsBlockedValidation
TestSDSBaselineGetCachedBaseline_RejectsUnknownValidation
TestValidateSDSBaselineRemote_CredentialBootstrapErrorIsNotReady
TestRunStandardProductWorkflow_DoesNotReuseBlockedSDSBaseline
```

- [ ] **Step 2: Centralize reusable-readiness policy**

Create one helper used by both:

- `GetCachedBaseline`
- final batch task gate

Required conditions:

```text
usable cache status
valid canonical payload
validation_status == ready
supported version
```

- [ ] **Step 3: Fix credential bootstrap semantics**

A missing credential/bootstrap error must return:

```text
blocked or unknown
stable reason_code
```

It must not return `ready`.

- [ ] **Step 4: Add baseline version policy**

Use the existing `Version` field and define the currently supported version in one constant.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/listingkit -run "TestSDSBaseline|TestRunStandardProductWorkflow_DoesNotReuseBlocked" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit
git commit -m "fix: require ready SDS baseline validation"
```

---

## Task 6: Group Generation by Compatibility Fingerprint

**Files:**

- Modify: `internal/listingkit/studio_batch_generation_request_support.go`
- Modify: `internal/listingkit/studio_batch_compatibility.go`
- Modify: `internal/listingkit/studio_batch_generation_test.go`
- Modify: `web/listingkit-ui/src/lib/shein-studio/grouped-image-mode.ts`
- Modify: `web/listingkit-ui/src/lib/shein-studio/grouped-sds-create.test.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-model.test.ts`

- [ ] **Step 1: Add backend grouping tests**

Cover:

```text
same size + different mask -> separate items
same size + different template -> separate items
identical full fingerprint -> one shared item
incomplete fingerprint -> per-product fallback or explicit block
```

- [ ] **Step 2: Replace the actual shared group key**

Use:

```text
compat:<fingerprint>
```

Keep dimensions only in the human-readable label.

- [ ] **Step 3: Add frontend compatibility helper**

The frontend should either:

- use a backend-provided fingerprint, or
- compute the same normalized raw compatibility key before hashing

Do not keep width/height-only grouping as the execution key.

- [ ] **Step 4: Preserve legacy batch reads**

Older `size:*` items remain readable. New generation graphs use compatibility keys.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/listingkit -run "Test.*Compatibility|TestExpandStudioBatchItems" -count=1

cd web/listingkit-ui
npm test -- \
  src/lib/shein-studio/grouped-sds-create.test.ts \
  src/components/listingkit/shein-studio/shein-studio-workbench-model.test.ts
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit web/listingkit-ui/src
git commit -m "fix: group SDS generation by compatibility"
```

---

## Task 7: Complete API and Frontend Result Projection

**Files:**

- Modify: `internal/listingkit/studio_session_model.go`
- Modify: `internal/listingkit/studio_batch_service.go`
- Modify: `internal/listingkit/studio_batch_status_groups.go`
- Modify: `internal/listingkit/task_studio_batch_detail_support.go`
- Modify: `web/listingkit-ui/src/lib/types/shein-studio.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batches.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batches.test.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-task-creation-actions.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-created-tasks-list.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-batch-task-tracker.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`

- [ ] **Step 1: Add frontend parser tests**

Require parsing for:

```text
item_id
selection_id
compatibility_fingerprint
status
submission_state
last_submission_action
reason_code
message
rejected_tasks
reused_tasks
```

- [ ] **Step 2: Expand frontend types**

Add explicit task lifecycle union:

```text
task_created
needs_review
ready_to_submit
draft_saved
published
submit_failed
unknown
```

Add rejected and reused result types.

- [ ] **Step 3: Populate backend created-task fields**

Every new task result must include:

```text
status = task_created
item_id
selection_id
compatibility_fingerprint
```

- [ ] **Step 4: Project real task/submission state**

Resolve current state from the actual ListingKit task/readiness/submission projection and update the durable link projection.

Do not infer state from title text.

- [ ] **Step 5: Remove empty-state draft assumption**

Update `studioBatchCreatedTaskGroup`:

```text
empty/unknown -> task_created or unknown
never -> draft_saved
```

Keep title-based inference only behind an explicit legacy migration branch if absolutely required, and mark it for retirement.

- [ ] **Step 6: Display structured outcomes**

The UI must show:

```text
created
reused
rejected with reason
failed with message
```

A response with zero created and one rejected must not display a success message saying zero tasks were created.

- [ ] **Step 7: Show estimated task count**

Before creation, display:

```text
approved designs x eligible selections = estimated candidates
```

- [ ] **Step 8: Run tests**

```bash
go test ./internal/listingkit -run "TestBuildStudioBatchStatusGroups|TestStudioBatchDetail" -count=1

cd web/listingkit-ui
npm test -- \
  src/lib/api/shein-studio-batches.test.ts \
  src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx
npm run typecheck
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/listingkit web/listingkit-ui/src
git commit -m "feat: expose SDS batch task outcomes and states"
```

---

## Task 8: Protect Existing Tasks from Destructive Regeneration

**Files:**

- Modify: `internal/listingkit/task_studio_batch_retry_support.go`
- Modify: `internal/listingkit/studio_batch_service.go`
- Modify: `internal/listingkit/studio_batch_service_test.go`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-panel.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-generation-panel.test.tsx`

- [ ] **Step 1: Add failing retry protection tests**

Add:

```text
TestRetryStudioBatchItems_RejectsItemWithCreatedTaskLinks
TestRetryStudioBatchItems_AllowsFailedItemWithoutTaskLinks
```

- [ ] **Step 2: Add task-link check to retry policy**

Phase-one rule:

```text
item has created/reused task links -> normal retry rejected
reason_code = tasks_already_created
```

- [ ] **Step 3: Preserve old materialized designs**

Do not delete or replace historical design rows connected to task links.

- [ ] **Step 4: Update the UI**

Disable ordinary regenerate for linked items and direct the operator to existing ListingKit tasks.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/listingkit -run "TestRetryStudioBatchItems_" -count=1

cd web/listingkit-ui
npm test -- src/components/listingkit/shein-studio/shein-studio-generation-panel.test.tsx
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit web/listingkit-ui/src
git commit -m "fix: protect linked SDS designs from regeneration"
```

---

## Task 9: Make Batch SHEIN Submission Partially Successful

**Files:**

- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-batch-publish-gate.tsx`
- Create or modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-batch-publish-gate.test.tsx`
- Optional modify: `web/listingkit-ui/src/lib/api/submit.ts`

- [ ] **Step 1: Add failing partial-submission tests**

Cover:

```text
second task fails but third task is still submitted
success and failure counts are displayed
retry targets failed tasks only
successful tasks are not resubmitted
```

- [ ] **Step 2: Replace fail-fast loop**

Use per-task error capture, either sequentially or with controlled concurrency.

Recommended result:

```ts
{
  succeeded: TaskResult[];
  failed: TaskResult[];
  skipped: TaskResult[];
}
```

- [ ] **Step 3: Preserve stable idempotency for retries**

Ensure retries do not accidentally create a second remote submission attempt for already successful tasks.

- [ ] **Step 4: Refresh every attempted task**

Refresh task result and preview independently after each attempt.

- [ ] **Step 5: Run tests**

```bash
cd web/listingkit-ui
npm test -- src/components/listingkit/shein-studio/shein-batch-publish-gate.test.tsx
npm run typecheck
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/listingkit-ui/src
git commit -m "fix: continue SHEIN batch submit after failures"
```

---

## Task 10: Backfill Legacy Task Ownership

**Files:**

- Create: `internal/listingkit/studio_batch_task_link_backfill.go`
- Create: `internal/listingkit/studio_batch_task_link_backfill_test.go`
- Optional modify: `internal/listingkit/httpapi/builders_repository_schema.go`
- Optional create: admin/debug command under the existing repository conventions

- [ ] **Step 1: Add backfill tests**

Cover:

```text
legacy session CreatedTasks -> durable links
missing task -> unresolved record or skip with report
existing link -> no duplicate
cross-tenant task -> rejected
legacy style ID -> linked using selection match
```

- [ ] **Step 2: Implement idempotent backfill**

Backfill should be safe to run more than once.

- [ ] **Step 3: Produce a reconciliation summary**

Return/log:

```text
sessions scanned
links created
links already present
missing tasks
unresolved selection ownership
errors
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/listingkit -run "Test.*StudioBatchTaskLinkBackfill" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit
git commit -m "feat: backfill SDS batch task links"
```

---

## Task 11: Regression and Concurrency Verification

**Files:**

- Modify tests only as needed
- Update: `docs/product/validation/runs/`

- [ ] **Step 1: Run targeted race tests**

```bash
go test -race ./internal/listingkit -run "TestServiceCreateStudioBatchTasks_Concurrent|Test.*StudioBatchTaskLink" -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full backend tests**

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 3: Run full frontend verification**

```bash
cd web/listingkit-ui
npm run lint
npm run typecheck
npm test
npm run build
```

Expected: PASS.

- [ ] **Step 4: Run candidate matrix tests**

At minimum verify:

```text
1 design x 1 product
2 designs x 3 compatible products
2 same-size incompatible products
mixed ready/blocked baselines
mixed valid/invalid stores
one task-creator operational failure
repeat request
concurrent repeat request
session-less batch refresh
legacy session-backed batch
```

- [ ] **Step 5: Commit test and documentation updates**

```bash
git add .
git commit -m "test: verify SDS batch production closure"
```

---

## Task 12: Real Environment Validation

**Files:**

- Create: `docs/product/validation/runs/YYYY-MM-DD-shein-sds-batch-production-closure.md`
- Update: `docs/product/validation/unknown-state-and-blocker-tracker.md`

- [ ] **Step 1: Run a real compatible fan-out batch**

Use at least:

```text
2 compatible SDS selections
2 approved designs
expected 4 ListingKit tasks
```

Record:

```text
batch_id
item_ids
design_ids
selection_ids
candidate keys
task IDs
store IDs
```

- [ ] **Step 2: Verify refresh and duplicate request**

After creation:

- refresh the batch
- confirm all tasks remain visible
- repeat create request
- confirm no duplicate tasks

- [ ] **Step 3: Run a controlled rejection**

Use one of:

```text
blocked baseline
invalid store
compatibility mismatch
```

Confirm the UI shows the structured reason and unrelated candidates still succeed.

- [ ] **Step 4: Save one real SHEIN draft**

Confirm:

```text
task_created -> ready_to_submit -> draft_saved
```

State must come from the real submission projection.

- [ ] **Step 5: Run partial submission failure**

Trigger or simulate one task failure and confirm later tasks are still attempted.

- [ ] **Step 6: Update trackers**

Record every unknown state, unknown reason code, empty error, or UI path without a next action.

- [ ] **Step 7: Commit validation evidence**

```bash
git add docs/product/validation
git commit -m "docs: record SDS batch closure validation"
```

---

## Recommended PR Sequence

To keep review size manageable, split implementation into these PRs:

### PR 1: Candidate correctness and durable ownership

```text
Task 1
Task 2
Task 3
```

### PR 2: Final business gate and baseline strictness

```text
Task 4
Task 5
```

### PR 3: Compatibility grouping and frontend result projection

```text
Task 6
Task 7
```

### PR 4: Regeneration and submission recovery

```text
Task 8
Task 9
Task 10
```

### PR 5: Validation

```text
Task 11
Task 12
```

## Review Checklist

Every implementation PR must answer:

```text
Does this change preserve one task per design/selection candidate?
Can it create a duplicate under retry or concurrency?
Does it depend on SheinStudioSession for new ownership?
Can a frontend path bypass the backend gate?
Does a cached-but-blocked baseline pass?
Can incompatible selections share generation work?
Is task creation still distinguishable from SHEIN submission?
Can one failure stop unrelated candidates?
Can regeneration orphan an existing task?
Are tenant scopes enforced on every new repository query?
```

## Scope Control

Do not include the following in this implementation cycle:

- broad ListingKit package relocation
- new marketplace support
- a full Studio UI rewrite
- merging several SDS products into one SHEIN listing
- changing the approved-by-default decision
- redesigning the generic ListingKit submission state machine

The objective is production correctness and recoverability, not structural perfection.

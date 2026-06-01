# Task Processor Framework Phase 14 ListingKit Generation Task Read Ownership Plan

## Goal

Reduce the remaining generation-task-read ownership complexity in ListingKit by making task snapshot acquisition and task-page shaping flow through explicit feature-owned seams instead of remaining clustered inside `GetTaskGenerationTasks(...)`.

## Architecture

Reuse the same bounded-seam pattern already established in `Phase 13`. Do **not** invent a generic list-read or query framework. Instead, split the current generation-task read path in [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:60) into two ListingKit-owned local seams:

1. shared generation-task read snapshot acquisition
2. generation-task page assembly and shaping

Keep business behavior unchanged and preserve current filtering, sorting, paging, and task-summary semantics.

## Scope

This phase is limited to the ListingKit generation-task read path.

Primary hotspots:

- [internal/listingkit/task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:60)
- [internal/listingkit/generation_task_list.go](/D:/code/task-processor/internal/listingkit/generation_task_list.go:1)
- [internal/listingkit/service_generation_tasks_test.go](/D:/code/task-processor/internal/listingkit/service_generation_tasks_test.go:1)
- [internal/listingkit/task_generation_service_test.go](/D:/code/task-processor/internal/listingkit/task_generation_service_test.go:1)

Allowed outcomes:

- new feature-local helper/seam files under `internal/listingkit/`
- tighter service orchestration in generation-task read
- behavior tests and source-boundary guardrails

Not in scope:

- queue-read seam changes
- review session / preview seam changes
- navigation/action/retry/submit-path changes
- studio-session worktree changes
- generic paging/filtering/query abstractions
- HTTP/bootstrap/runtime changes

## Working Rules

1. Preserve task-read behavior first; ownership clarity second.
2. Keep all new abstractions feature-local to ListingKit.
3. Prefer seams that reflect ownership pressure, not mechanical helper splitting.
4. Keep this phase isolated from the active unrelated worktree changes.
5. Add or update tests before the implementation that depends on them.

## Task Breakdown

This phase should land as three small commits:

1. `refactor: extract listingkit generation task read snapshot seam`
2. `refactor: extract listingkit generation task read page seam`
3. `test: lock listingkit generation task read boundaries`

Keep them in that order so each ownership split stays reviewable.

---

## Task 1: Extract generation-task read snapshot seam

**Files:**
- Create: `internal/listingkit/task_generation_tasks_read_snapshot.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify tests if needed:
  - `internal/listingkit/service_generation_tasks_test.go`
  - `internal/listingkit/task_generation_service_test.go`

- [ ] **Step 1: Write the failing task-snapshot tests**

Add focused coverage that locks:

1. generation-task reads still derive from one current task + persisted task snapshot
2. snapshot load errors still surface unchanged
3. task metadata and task list come from the same acquisition handoff

Suggested seam shape:

```go
type taskGenerationTasksReadSnapshot struct {
	task  *Task
	tasks []assetgeneration.Task
}

type taskGenerationTasksReadSnapshotPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationTasksReadSnapshotPhase(service *taskGenerationService) *taskGenerationTasksReadSnapshotPhase

func (p *taskGenerationTasksReadSnapshotPhase) run(ctx context.Context, taskID string) (*taskGenerationTasksReadSnapshot, error)
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationTasksReadSnapshot.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the snapshot seam**

Create `task_generation_tasks_read_snapshot.go` so the seam owns:

- `repo.GetTask(...)`
- `listAssetGenerationTasks(...)`
- snapshot handoff of the current `Task`
- snapshot handoff of persisted generation tasks

Important:

- preserve current error behavior
- do not move filtering/sorting/paging into this seam
- keep it feature-local and narrow

- [ ] **Step 4: Route the service method through the snapshot seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:60) so `GetTaskGenerationTasks(...)` delegates snapshot acquisition instead of loading task/tasks inline.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationTasksReadSnapshot.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_tasks_read_snapshot.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_tasks_test.go internal/listingkit/task_generation_service_test.go
git commit -m "refactor: extract listingkit generation task read snapshot seam"
```

---

## Task 2: Extract generation-task read page seam

**Files:**
- Create: `internal/listingkit/task_generation_tasks_read_page.go`
- Modify: `internal/listingkit/task_generation_service.go`
- Modify tests if needed:
  - `internal/listingkit/service_generation_tasks_test.go`
  - `internal/listingkit/task_generation_service_test.go`

- [ ] **Step 1: Write the failing task-page tests**

Add focused coverage that locks:

1. empty task-page response shape stays unchanged
2. filtering / sorting / paging still behave the same
3. task summary is still built from the filtered task set before paging

Suggested seam shape:

```go
type taskGenerationTasksReadPagePhase struct{}

func buildTaskGenerationTasksReadPagePhase() *taskGenerationTasksReadPagePhase

func (p *taskGenerationTasksReadPagePhase) run(
	snapshot *taskGenerationTasksReadSnapshot,
	query *GenerationTaskQuery,
) *GenerationTaskPage
```

- [ ] **Step 2: Run focused failing verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationTasksReadPage.*" -count=1
```

Expected: FAIL until the seam exists.

- [ ] **Step 3: Add the task-page seam**

Create `task_generation_tasks_read_page.go` so the seam owns:

- empty task-page response shaping
- `filterGenerationTasks(...)`
- `sortGenerationTasks(...)`
- `paginateGenerationTasks(...)`
- `buildGenerationTaskPage(...)`

Important:

- preserve current page / pageSize defaults
- preserve summary shaping from the filtered task set
- do not absorb task snapshot acquisition back into this seam

- [ ] **Step 4: Route the service method through the new seam**

Update [task_generation_service.go](/D:/code/task-processor/internal/listingkit/task_generation_service.go:60) so generation-task page shaping becomes a single seam handoff.

- [ ] **Step 5: Re-run focused verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationTasksReadPage.*|TestGetTaskGenerationTasks.*|TestTaskGenerationServiceGetTaskGenerationTasks.*" -count=1
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/task_generation_tasks_read_page.go internal/listingkit/task_generation_service.go internal/listingkit/service_generation_tasks_test.go internal/listingkit/task_generation_service_test.go
git commit -m "refactor: extract listingkit generation task read page seam"
```

---

## Task 3: Lock generation-task read ownership guardrails

**Files:**
- Create: `internal/listingkit/phase14_generation_tasks_read_boundary_test.go`
- Modify if needed:
  - `internal/listingkit/service_generation_tasks_test.go`
  - `internal/listingkit/task_generation_service_test.go`
- Verify:
  - `internal/listingkit/task_generation_tasks_read_snapshot.go`
  - `internal/listingkit/task_generation_tasks_read_page.go`
  - `internal/listingkit/task_generation_service.go`

- [ ] **Step 1: Add failing ownership guardrails**

Create boundary tests that lock:

1. `GetTaskGenerationTasks(...)` delegates to snapshot/page seams in order
2. snapshot seam owns task/task-list acquisition, but not page shaping
3. page seam owns filtering/sorting/paging/page assembly, but not snapshot acquisition

- [ ] **Step 2: Run focused boundary verification**

Run:

```powershell
go test ./internal/listingkit -run "TestTaskGenerationTasksRead.*Boundary" -count=1
```

Expected: FAIL until the guardrails reflect the final seam split.

- [ ] **Step 3: Keep the guardrails low-fragility**

Anchor the ownership tests on:

- helper names
- occurrence counts
- explicit forbidden helper calls
- responsibility-level signals

Avoid fragile dependence on:

- local variable names
- exact conditional layout
- whitespace-sensitive snippets

- [ ] **Step 4: Run final generation-task read verification**

Run:

```powershell
go test ./internal/listingkit -run "TestGetTaskGenerationTasks.*|TestTaskGenerationServiceGetTaskGenerationTasks.*|TestTaskGenerationTasksRead.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/phase14_generation_tasks_read_boundary_test.go internal/listingkit/service_generation_tasks_test.go internal/listingkit/task_generation_service_test.go internal/listingkit/task_generation_tasks_read_snapshot.go internal/listingkit/task_generation_tasks_read_page.go internal/listingkit/task_generation_service.go
git commit -m "test: lock listingkit generation task read boundaries"
```

---

## Verification Checklist For The Whole Phase

At the end of the full phase, run:

```powershell
go test ./internal/listingkit -run "TestGetTaskGenerationTasks.*|TestTaskGenerationServiceGetTaskGenerationTasks.*|TestTaskGenerationTasksRead.*" -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

If unrelated working-tree changes are still present, do **not** silently broaden this phase to fix them. Record that broader verification may still be noisy for out-of-slice reasons.

# ListingKit Core Model Ownership Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `internal/listingkit/core` the sole owner of ListingKit task-lifecycle errors and status types, remove the duplicate root API, and migrate every repository caller.

**Architecture:** The foundational core package retains the canonical sentinels and `TaskStatus`. Root models and all callers import core directly; an AST boundary test prevents the root package from declaring the shared names again. The repository-wide API break is delivered as one atomic implementation commit.

**Tech Stack:** Go 1.26.5, standard `errors`, `go/ast`, `go/parser`, `gofmt -r`, `gopls imports`, PowerShell.

## Global Constraints

- Do not retain root-package type aliases, variable aliases, or constant aliases for the removed symbols.
- Preserve all error strings and all serialized `TaskStatus` values exactly.
- Preserve root-only errors in `internal/listingkit/model.go`.
- Do not migrate unrelated task status or task error definitions in other domains.
- Use `core` as the import name for `task-processor/internal/listingkit/core`.
- Observe the ownership regression test fail before removing root declarations.
- Do not commit an intermediate revision that fails to compile or test. Task 1's expected-red test is staged for review but remains uncommitted until Task 2 makes it pass.
- Use `gofmt -r` and `gopls imports`; do not add a custom source-rewriting utility.

---

### Task 1: Add the Ownership Regression Test

**Files:**
- Create: `internal/listingkit/core/model_ownership_test.go`

**Interfaces:**
- Consumes: root-package non-test Go declarations under `internal/listingkit/*.go`.
- Produces: `TestRootPackageDoesNotDeclareCoreTaskLifecycleSymbols`, which fails while the duplicate root declarations exist and passes only when core is the sole owner.

- [ ] **Step 1: Write the AST ownership test**

Create `internal/listingkit/core/model_ownership_test.go`:

```go
package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestRootPackageDoesNotDeclareCoreTaskLifecycleSymbols(t *testing.T) {
	t.Parallel()

	forbidden := map[string]struct{}{
		"ErrTaskNotFound": {}, "ErrTaskNotPending": {},
		"ErrTaskNotRecoverable": {}, "ErrTaskRecoveryUnavailable": {},
		"ErrTaskRequeueUnavailable": {}, "ErrTaskRequeueInvalidRequest": {},
		"ErrGenerationTaskNotFound": {}, "ErrGenerationTaskNotRetryable": {},
		"ErrGenerationActionNotFound": {}, "ErrChildTaskRetryInvalidRequest": {},
		"ErrChildTaskNotFound": {}, "ErrChildTaskNotRetryable": {},
		"ErrChildTaskRetryConflict": {}, "TaskStatus": {},
		"TaskStatusPending": {}, "TaskStatusProcessing": {},
		"TaskStatusCompleted": {}, "TaskStatusNeedsReview": {},
		"TaskStatusFailed": {}, "TaskStatusBlockedRetryable": {},
	}

	paths, err := filepath.Glob(filepath.Join("..", "*.go"))
	if err != nil {
		t.Fatalf("glob root listingkit files: %v", err)
	}
	fset := token.NewFileSet()
	var duplicates []string
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				switch typed := spec.(type) {
				case *ast.TypeSpec:
					if _, found := forbidden[typed.Name.Name]; found {
						duplicates = append(duplicates, filepath.Base(path)+":"+typed.Name.Name)
					}
				case *ast.ValueSpec:
					for _, name := range typed.Names {
						if _, found := forbidden[name.Name]; found {
							duplicates = append(duplicates, filepath.Base(path)+":"+name.Name)
						}
					}
				}
			}
		}
	}
	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		t.Fatalf("root listingkit package redeclares core task lifecycle symbols: %s", strings.Join(duplicates, ", "))
	}
}
```

- [ ] **Step 2: Run the new test and verify RED**

Run:

```powershell
go test ./internal/listingkit/core -run TestRootPackageDoesNotDeclareCoreTaskLifecycleSymbols -count=1
```

Expected: FAIL listing the 13 duplicate sentinels, `TaskStatus`, and six status constants from `model.go`. A parser or compile failure is not the expected RED state.

- [ ] **Step 3: Stage the test for review without committing a red revision**

Run:

```powershell
git add -- internal/listingkit/core/model_ownership_test.go
git diff --cached --check
```

Leave this staged test in the worktree. Task 2 will make it pass and commit it together with the production migration.

---

### Task 2: Remove Root Declarations and Migrate Production Code

**Files:**
- Modify: `internal/listingkit/model.go`
- Modify: `internal/listingkit/export_builder.go`
- Modify: `internal/listingkit/model_task.go`
- Modify: `internal/listingkit/model_task_requeue.go`
- Modify: `internal/listingkit/preview_builder.go`
- Modify: `internal/listingkit/preview_builder_stages.go`
- Modify: `internal/listingkit/preview_domain_adapter.go`
- Modify: `internal/listingkit/preview_model_shell.go`
- Modify: `internal/listingkit/processor.go`
- Modify: `internal/listingkit/processor_state_machine.go`
- Modify: `internal/listingkit/review_reason_presentation.go`
- Modify: `internal/listingkit/sds_repair.go`
- Modify: `internal/listingkit/service_child_task_retry_helpers.go`
- Modify: `internal/listingkit/service_child_task_retry_logic.go`
- Modify: `internal/listingkit/service_process_persistence_helper.go`
- Modify: `internal/listingkit/service_process_runner_helper.go`
- Modify: `internal/listingkit/service_sds_child_retry.go`
- Modify: `internal/listingkit/service_task_generation_support_helpers.go`
- Modify: `internal/listingkit/service_task_layer_processing_helpers.go`
- Modify: `internal/listingkit/service_task_layers_logic.go`
- Modify: `internal/listingkit/shein_admin_service.go`
- Modify: `internal/listingkit/studio_batch_task_link_backfill.go`
- Modify: `internal/listingkit/task_generation_action_target_resolution.go`
- Modify: `internal/listingkit/task_generation_navigation_dispatch_entry.go`
- Modify: `internal/listingkit/task_generation_navigation_dispatch_primary.go`
- Modify: `internal/listingkit/task_generation_navigation_dispatch_step_execution.go`
- Modify: `internal/listingkit/task_lifecycle_service_support.go`
- Modify: `internal/listingkit/task_list_item_support.go`
- Modify: `internal/listingkit/task_recovery_backfill.go`
- Modify: `internal/listingkit/task_recovery_service.go`
- Modify: `internal/listingkit/task_requeue_adapter.go`
- Modify: `internal/listingkit/task_requeue_service.go`
- Modify: `internal/listingkit/task_result_support.go`
- Modify: `internal/listingkit/task_revision_service.go`
- Modify: `internal/listingkit/task_studio_batch_detail_support.go`
- Modify: `internal/listingkit/task_studio_batch_task_existing_support.go`
- Modify: `internal/listingkit/workflow_result.go`
- Modify: `internal/listingkit/workflow_sds_sync.go`
- Modify: `internal/listingkit/workflow_sds_sync_remote_support.go`
- Modify: `internal/listingkit/workflow_sds_sync_stage_support.go`
- Modify: `internal/listingkit/workflow_standard_canonical_phase.go`
- Modify: `internal/listingkit/workflow_standard_media_phase.go`
- Modify: `internal/listingkit/api/export_handler.go`
- Modify: `internal/listingkit/api/generation_navigation_dispatch_handler.go`
- Modify: `internal/listingkit/api/generation_tasks_handler.go`
- Modify: `internal/listingkit/api/handler_tasks.go`
- Modify: `internal/listingkit/api/history_detail_handler.go`
- Modify: `internal/listingkit/api/history_handler.go`
- Modify: `internal/listingkit/api/preview_handler.go`
- Modify: `internal/listingkit/api/revision_handler.go`
- Modify: `internal/listingkit/api/revision_validate_handler.go`
- Modify: `internal/listingkit/api/sds_repair_handler.go`
- Modify: `internal/listingkit/api/shein_category_search_handler.go`
- Modify: `internal/listingkit/api/shein_customer_flow_handler.go`
- Modify: `internal/listingkit/api/shein_image_regeneration_handler.go`
- Modify: `internal/listingkit/api/shein_resolution_cache_handler.go`
- Modify: `internal/listingkit/api/store_profile_handler.go`
- Modify: `internal/listingkit/api/studio_async_jobs_handler_runner.go`
- Modify: `internal/listingkit/api/submit_handler.go`
- Modify: `internal/listingkit/api/task_recovery_handler.go`
- Modify: `internal/listingkit/api/task_requeue_handler.go`
- Modify: `internal/listingkit/store/mem_store.go`
- Modify: `internal/listingkit/store/sds_child_retry_mem_repo.go`
- Modify: `internal/listingkit/store/sds_retirement_repo.go`
- Modify: `internal/listingkit/store/shein_pod_image_lookup_backfill.go`
- Modify: `internal/listingkit/store/task_repo_listing.go`
- Modify: `internal/listingkit/store/task_repo_shein_pod_image_lookup_index.go`
- Modify: `internal/listingkit/store/task_repo_status.go`
- Modify: `internal/productenrich/httpapi/sourcea1688/handler.go`

**Interfaces:**
- Consumes: canonical `core.Err...`, `core.TaskStatus`, and status constants.
- Produces: production packages with no dependency on the removed root symbols and no change to error text, status strings, JSON, GORM, or service behavior.

- [ ] **Step 1: Remove only the shared declarations from the root model**

In `internal/listingkit/model.go`, delete the 13 duplicated `Err...` variables, the `TaskStatus` type, its constant block, and the stale `ErrSubmitInProgress moved` comment. Keep the `errors` import and these root-owned variables unchanged:

```go
var ErrUnsupportedSubmitPlatform = errors.New("unsupported submit platform")
var ErrSubmitBlocked = errors.New("submit blocked by readiness")
var ErrInvalidSheinResolutionCacheKind = errors.New("invalid shein resolution cache kind")
var ErrInvalidSheinCategorySearchQuery = errors.New("invalid shein category search query")
```

- [ ] **Step 2: Apply the exact root-package AST rewrites**

For the direct production files listed above, replace each unqualified identifier with its `core.` form:

```text
TaskStatus -> core.TaskStatus
TaskStatusPending -> core.TaskStatusPending
TaskStatusProcessing -> core.TaskStatusProcessing
TaskStatusCompleted -> core.TaskStatusCompleted
TaskStatusNeedsReview -> core.TaskStatusNeedsReview
TaskStatusFailed -> core.TaskStatusFailed
TaskStatusBlockedRetryable -> core.TaskStatusBlockedRetryable
ErrTaskNotFound -> core.ErrTaskNotFound
ErrTaskNotPending -> core.ErrTaskNotPending
ErrTaskNotRecoverable -> core.ErrTaskNotRecoverable
ErrTaskRecoveryUnavailable -> core.ErrTaskRecoveryUnavailable
ErrTaskRequeueUnavailable -> core.ErrTaskRequeueUnavailable
ErrTaskRequeueInvalidRequest -> core.ErrTaskRequeueInvalidRequest
ErrGenerationTaskNotFound -> core.ErrGenerationTaskNotFound
ErrGenerationTaskNotRetryable -> core.ErrGenerationTaskNotRetryable
ErrGenerationActionNotFound -> core.ErrGenerationActionNotFound
ErrChildTaskRetryInvalidRequest -> core.ErrChildTaskRetryInvalidRequest
ErrChildTaskNotFound -> core.ErrChildTaskNotFound
ErrChildTaskNotRetryable -> core.ErrChildTaskNotRetryable
ErrChildTaskRetryConflict -> core.ErrChildTaskRetryConflict
```

Use one `gofmt -r '<old> -> core.<old>' -w <files>` pass per identifier. Longer constant/error identifiers must be rewritten before `TaskStatus`. Do not run these unqualified rewrites recursively outside the direct `internal/listingkit` package.

- [ ] **Step 3: Apply the exact child-package rewrites**

In the 27 child-package production files listed above, replace only `listingkit.<shared identifier>` with `core.<shared identifier>`. Retain the root `listingkit` import wherever the file still uses root models or services.

- [ ] **Step 4: Repair imports with gopls and format**

For every changed Go file, run:

```powershell
gopls imports -w <file>
gofmt -w <file>
```

Expected: each file imports `task-processor/internal/listingkit/core` as package `core`; unused root imports are removed.

- [ ] **Step 5: Verify the production migration**

Run:

```powershell
go test ./internal/listingkit/core -run TestRootPackageDoesNotDeclareCoreTaskLifecycleSymbols -count=1
go build ./...
```

Expected: the ownership test and the production-only repository build PASS. Test packages may still fail to compile until Task 3; do not commit yet.

---

### Task 3: Migrate Tests and Source-Text Assertions

**Files:**
- Modify: `internal/listingkit/canonical_product_cache_test.go`
- Modify: `internal/listingkit/generation_conditional_state_test.go`
- Modify: `internal/listingkit/generation_navigation_conditional_targets_test.go`
- Modify: `internal/listingkit/generation_scene_preset_summary_test.go`
- Modify: `internal/listingkit/phase5a_process_boundary_test.go`
- Modify: `internal/listingkit/pod_execution_test.go`
- Modify: `internal/listingkit/preview_builder_test.go`
- Modify: `internal/listingkit/processor_process_test.go`
- Modify: `internal/listingkit/processor_state_machine_test.go`
- Modify: `internal/listingkit/review_reason_presentation_test.go`
- Modify: `internal/listingkit/sds_repair_test.go`
- Modify: `internal/listingkit/service_child_task_retry_test.go`
- Modify: `internal/listingkit/service_generation_actions_test.go`
- Modify: `internal/listingkit/service_generation_navigation_dispatch_test.go`
- Modify: `internal/listingkit/service_generation_queue_test.go`
- Modify: `internal/listingkit/service_generation_retry_test.go`
- Modify: `internal/listingkit/service_generation_tasks_test.go`
- Modify: `internal/listingkit/service_generation_test.go`
- Modify: `internal/listingkit/service_layers_test.go`
- Modify: `internal/listingkit/service_preview_test.go`
- Modify: `internal/listingkit/service_process_status_test.go`
- Modify: `internal/listingkit/service_revision_test.go`
- Modify: `internal/listingkit/service_revision_validate_test.go`
- Modify: `internal/listingkit/service_sds_child_retry_test.go`
- Modify: `internal/listingkit/service_submit_lifecycle_test.go`
- Modify: `internal/listingkit/service_submit_test.go`
- Modify: `internal/listingkit/service_task_test.go`
- Modify: `internal/listingkit/service_test.go`
- Modify: `internal/listingkit/service_wiring_test.go`
- Modify: `internal/listingkit/studio_batch_service_test.go`
- Modify: `internal/listingkit/studio_batch_task_link_backfill_test.go`
- Modify: `internal/listingkit/task_generation_service_test.go`
- Modify: `internal/listingkit/task_lifecycle_retryable_test.go`
- Modify: `internal/listingkit/task_lifecycle_service_test.go`
- Modify: `internal/listingkit/task_recovery_backfill_test.go`
- Modify: `internal/listingkit/task_recovery_model_test.go`
- Modify: `internal/listingkit/task_recovery_service_test.go`
- Modify: `internal/listingkit/task_requeue_service_test.go`
- Modify: `internal/listingkit/task_result_projection_test.go`
- Modify: `internal/listingkit/task_revision_service_test.go`
- Modify: `internal/listingkit/task_source_reference_projection_test.go`
- Modify: `internal/listingkit/workflow_assets_test.go`
- Modify: `internal/listingkit/workflow_studio_sds_metadata_test.go`
- Modify: `internal/app/httpapi/e2e_listingkit_sds_live_test.go`
- Modify: `internal/app/httpapi/e2e_listingkit_sds_test.go`
- Modify: `internal/app/httpapi/e2e_test.go`
- Modify: `internal/compatibility/listingkit/preview_adapter_test.go`
- Modify: `internal/listingkit/api/child_task_retry_handler_test.go`
- Modify: `internal/listingkit/api/generation_tasks_handler_test.go`
- Modify: `internal/listingkit/api/sds_repair_handler_test.go`
- Modify: `internal/listingkit/api/task_recovery_handler_test.go`
- Modify: `internal/listingkit/api/task_requeue_handler_test.go`
- Modify: `internal/listingkit/store/sds_retirement_repo_test.go`
- Modify: `internal/listingkit/store/shein_pod_image_lookup_backfill_test.go`
- Modify: `internal/listingkit/store/source_reference_persistence_test.go`
- Modify: `internal/listingkit/store/task_repo_retryable_test.go`
- Modify: `internal/listingkit/store/task_repo_shein_pod_image_lookup_test.go`
- Modify: `internal/listingkit/store/task_repo_test.go`
- Modify: `internal/listingkit/store/tenant_test.go`
- Modify: `internal/listingkit/temporal/workflow_publish_integration_test.go`
- Modify: `internal/product/sourcehandoff/a1688/httpapi/handler_test.go`
- Modify: `internal/productenrich/httpapi/sourcea1688/handler_test.go`
- Modify: `tests/a1688_source_to_task_flow_test.go`

**Interfaces:**
- Consumes: the migrated production API from Task 2.
- Produces: tests and source-boundary expectations that use core directly and preserve all existing behavioral assertions.

- [ ] **Step 1: Migrate direct root-package tests**

Apply the same unqualified-to-`core.` identifier map from Task 2 only to the 43 direct root test files listed above. In string assertions that intentionally describe production source, update exact signatures and expressions, including:

```text
func deriveProcessTerminalStatus(result *ListingKitResult) core.TaskStatus {
func applyProcessTerminalResult(result *ListingKitResult, status core.TaskStatus) *ListingKitResult {
if task.Status != core.TaskStatusPending {
```

- [ ] **Step 2: Migrate child-package tests**

In the 20 child-package test files listed above, replace only `listingkit.<shared identifier>` with `core.<shared identifier>`. Do not rewrite unrelated domain statuses.

- [ ] **Step 3: Repair imports and format all changed tests**

Run `gopls imports -w <file>` and `gofmt -w <file>` for every changed test file.

- [ ] **Step 4: Verify focused packages**

Run:

```powershell
go test ./internal/listingkit/core ./internal/listingkit ./internal/listingkit/api ./internal/listingkit/store ./internal/listingkit/temporal -count=1
go test ./internal/app/httpapi ./internal/compatibility/listingkit ./internal/product/sourcehandoff/a1688/httpapi ./internal/productenrich/httpapi/sourcea1688 ./tests -count=1
```

Expected: all packages PASS, including source-text boundary tests.

---

### Task 4: Prove Single Ownership and Commit Atomically

**Files:**
- Verify: all files modified in Tasks 1-3.
- Commit: implementation and tests only after every gate passes.

**Interfaces:**
- Consumes: the complete repository migration.
- Produces: one clean, buildable commit with a single core model authority and no old root call sites.

- [ ] **Step 1: Scan for removed qualified root API references**

Run:

```powershell
$removed = 'ErrTaskNotFound|ErrTaskNotPending|ErrTaskNotRecoverable|ErrTaskRecoveryUnavailable|ErrTaskRequeueUnavailable|ErrTaskRequeueInvalidRequest|ErrGenerationTaskNotFound|ErrGenerationTaskNotRetryable|ErrGenerationActionNotFound|ErrChildTaskRetryInvalidRequest|ErrChildTaskNotFound|ErrChildTaskNotRetryable|ErrChildTaskRetryConflict|TaskStatus|TaskStatusPending|TaskStatusProcessing|TaskStatusCompleted|TaskStatusNeedsReview|TaskStatusFailed|TaskStatusBlockedRetryable'
rg -n --glob '*.go' "listingkit\.($removed)\b" internal tests
```

Expected: exit code 1 and no matches.

- [ ] **Step 2: Re-run ownership, build, static, and diff checks**

Run:

```powershell
go test ./internal/listingkit/core -run TestRootPackageDoesNotDeclareCoreTaskLifecycleSymbols -count=1
go build ./...
go vet ./internal/listingkit/... ./internal/app/httpapi ./internal/compatibility/listingkit ./internal/product/sourcehandoff/a1688/httpapi ./internal/productenrich/httpapi/sourcea1688 ./tests
git diff --check
```

Expected: all commands exit 0 with no diagnostics.

- [ ] **Step 3: Run the full backend suite**

Run from a command invocation that permits at least ten minutes:

```powershell
go test -count=1 -timeout=5m ./...
```

Expected: PASS. Capture the exact package and output for any failure or timeout and do not report the suite as passing.

- [ ] **Step 4: Review scope and mechanical integrity**

Run:

```powershell
git status --short
git diff --stat HEAD
git diff -- internal/listingkit/model.go internal/listingkit/core/model.go internal/listingkit/core/model_ownership_test.go
git diff --word-diff=porcelain HEAD | Select-String -Pattern 'TaskStatus|ErrTask|ErrGeneration|ErrChildTask'
```

Expected: `internal/listingkit/core/model.go` is behaviorally unchanged, the root declarations are deleted, all other symbol changes are direct qualification/import changes, and only task files are present.

- [ ] **Step 5: Commit the implementation intentionally**

Run:

```powershell
git add -- internal/listingkit internal/app/httpapi internal/compatibility/listingkit internal/product/sourcehandoff/a1688/httpapi internal/productenrich/httpapi/sourcea1688 tests/a1688_source_to_task_flow_test.go
git diff --cached --check
git diff --cached --stat
git commit -m "refactor: centralize ListingKit task lifecycle model"
```

Expected: the commit succeeds and `git status --short` contains only the separately authored design/plan files if they were not committed earlier.

# ListingKit Submission Inventory

> Status: active Phase 3.1 inventory for the submission consolidation track described in [project-wide-execution-plan.md](./project-wide-execution-plan.md) and [listingkit-refactoring-plan.md](./listingkit-refactoring-plan.md).

## 1. Purpose

This document inventories the current submission-related code under `internal/listingkit` so later refactoring slices can move one coherent boundary at a time.

It is intentionally descriptive first:

- what files currently participate in submit, retry, recovery, requeue, and Temporal submission flows,
- which files are mostly facade and wiring,
- which files already hold reusable submission mechanics,
- which files still mix orchestration with SHEIN-specific rules.

Observed against the repository state on 2026-06-09 after the preview refactoring first wave.

## 2. Current Submission Shape

Submission behavior currently spans four layers:

1. root `package listingkit` facade methods on `service`,
2. root `package listingkit` collaborator services such as `taskSubmissionService`,
3. root `package listingkit` SHEIN-specific state, readiness, payload, and remote-diagnosis helpers,
4. the `internal/listingkit/submission` package for reusable submission primitives.

This means the direction is clearer than before, but not finished:

- generic mechanics like locks, retry delay, events, and transition helpers already have a home in `submission/`,
- service field sprawl is reduced by `submissionCollaborators`,
- `submissionCollaborators` now includes task recovery, task requeue, submission orchestration, recovery, execution, state, direct submit, Temporal adapter, and submit locks,
- root `listingkit` still owns most high-level submit orchestration and most SHEIN submission rules,
- Temporal submission logic is separated by collaborator but still depends heavily on root-side helpers and models.

## 3. Inventory By Responsibility

### A. Root service facade and collaborator wiring

These files mainly expose `service` methods, lazy collaborator accessors, or collaborator construction:

- `internal/listingkit/service_submission_collaborators.go`
- `internal/listingkit/service_submit.go`
- `internal/listingkit/service_submit_direct.go`
- `internal/listingkit/service_submit_recovery.go`
- `internal/listingkit/service_submit_temporal_adapter.go`
- `internal/listingkit/service_submit_wiring.go`
- `internal/listingkit/task_requeue_service.go`

Current role:

- keep `Service` interface compatibility,
- group submission collaborators behind `s.submission.*`,
- translate service-owned dependencies into collaborator configs.

Assessment:

- mostly facade or wiring,
- good candidates to stay thin,
- should avoid accumulating more business logic.

### B. Root collaborator services

These files contain the main internal submission-oriented service objects:

- `internal/listingkit/task_submission_service.go`
- `internal/listingkit/task_direct_submission_service.go`
- `internal/listingkit/task_submission_recovery_service.go`
- `internal/listingkit/task_submission_execution_service.go`
- `internal/listingkit/task_submission_state_service.go`
- `internal/listingkit/task_temporal_submission_adapter.go`
- `internal/listingkit/task_recovery_service.go`
- `internal/listingkit/task_requeue_service.go`

Current role:

- `taskSubmissionService`: entry orchestration for submit and remote refresh,
- `taskDirectSubmissionService`: direct SHEIN submit path,
- `taskSubmissionRecoveryService`: submit lease, stale-submit recovery, remote confirmation recovery,
- `taskSubmissionExecutionService`: build/prepare/upload/pre-validate/remote-submit execution steps,
- `taskSubmissionStateService`: persist phases, success, and failure state,
- `taskTemporalSubmissionAdapter`: workflow-facing adapter for publish activities,
- `taskRecoveryService`: blocked-retryable recovery flow; it owns recover-and-submit semantics and is not just a repository helper,
- `taskRequeueService`: pending-task requeue flow.

Assessment:

- this is the primary current consolidation seam,
- these files are the best place for additional root-level slimming before any deeper package move,
- `taskRecoveryService` and `taskRequeueService` are now part of the submission collaborator cluster, but their semantics are broader than the SHEIN publish path.

### C. Runtime context and settings resolution

These files resolve store, tenant, warehouse, and runtime API context needed by submission flows:

- `internal/listingkit/service_submit_context_resolver.go`
- `internal/listingkit/service_submit_store_context.go`
- `internal/listingkit/service_submit_runtime_context.go`
- `internal/listingkit/service_submit_settings_resolution.go`
- `internal/listingkit/service_submit_default_action.go`

Current role:

- derive store selection,
- derive warehouse code,
- build authenticated API runtime context,
- apply store/task settings overlays,
- resolve default submit action.

Assessment:

- not pure submit-state logic,
- better treated as “submission runtime context” than as generic submission primitives,
- likely should remain root-side or move only after model and interface seams are clearer.

### D. SHEIN-specific readiness, payload, and state helpers in root ListingKit

These files are strongly platform-specific and still encode SHEIN business rules:

- `internal/listingkit/shein_submit_payload.go`
- `internal/listingkit/shein_submit_readiness.go`
- `internal/listingkit/shein_submit_retry.go`
- `internal/listingkit/shein_submit_state.go`
- `internal/listingkit/shein_submission_events.go`
- `internal/listingkit/shein_submit_debug.go`
- `internal/listingkit/shein_submit_images.go`
- `internal/listingkit/submit_readiness_gate_shein.go`
- `internal/listingkit/submit_readiness_projection_shein.go`
- `internal/listingkit/submit_freshness_shein.go`
- `internal/listingkit/submit_attribute_freshness_evaluation_shein.go`
- `internal/listingkit/submit_attribute_freshness_issue_state_shein.go`
- `internal/listingkit/submit_attribute_freshness_message_shape_shein.go`
- `internal/listingkit/submit_sale_attribute_freshness_evaluation_shein.go`
- `internal/listingkit/submit_sale_attribute_freshness_message_shape_shein.go`
- `internal/listingkit/submit_sale_attribute_freshness_resolution_repair_shein.go`
- `internal/listingkit/submission_projection_shein.go`
- `internal/listingkit/shein_workspace_submit_bridge.go`

Current role:

- build submit payload snapshots,
- enforce submit readiness and freshness rules,
- encode retry and remote event behavior,
- mutate SHEIN submission state structures,
- bridge SHEIN workspace/final-review semantics into submit flows.

Assessment:

- these are not generic ListingKit submission mechanics,
- they are the biggest reason a true `internal/listingkit/submission` package move is still risky,
- future moves here should favor marketplace-owned placement when a safe target exists rather than moving everything into generic `submission/`.

### E. Generic submission primitives already extracted

These files already live under `internal/listingkit/submission`:

- `submission/doc.go`
- `submission/submit_lock.go`
- `submission/submit_errors.go`
- `submission/state.go`
- `submission/transitions.go`
- `submission/events.go`
- `submission/confirm_remote.go`
- `submission/source_facts.go`
- `submission/enqueue_retry.go`

Current role:

- lock management,
- common submit-related error types,
- state/transition/event helpers,
- remote confirmation event parts,
- source facts,
- enqueue retry and bounded backoff logic.

Assessment:

- this is the cleanest current target home for shared submission mechanics,
- recent retry extraction into `enqueue_retry.go` was aligned with this direction,
- additional helpers can move here only when they do not require root `listingkit` models or create import cycles.

### F. API and runtime assembly surface

These files expose submission capabilities at API/runtime boundaries:

- `internal/listingkit/api/submit_handler.go`
- `internal/listingkit/api/task_recovery_handler.go`
- `internal/listingkit/api/task_requeue_handler.go`
- `internal/listingkit/httpapi/bootstrap_submit_module.go`
- `internal/listingkit/httpapi/bootstrap_temporal_module.go`
- `internal/listingkit/httpapi/temporal_runtime.go`

Assessment:

- mostly assembly or transport translation,
- should remain thin,
- should not become the place where submission rules are repaired.

### G. Temporal package and contracts

These files define the workflow-side runtime rather than root business orchestration:

- `internal/listingkit/submit_temporal_contract.go`
- `internal/listingkit/layer_temporal_contract.go`
- `internal/listingkit/temporal/*.go`

Assessment:

- already form a meaningful runtime layer,
- still rely on the root adapter and root models,
- should not be the first place to move business rules until the adapter seam gets narrower.

## 4. Latest Code Validation Notes

Latest code inspection confirms:

- `service` now stores a single `submission submissionCollaborators` field rather than separate direct fields for each submission service.
- `submissionCollaborators` includes `taskRecovery`, `taskRequeue`, `taskSubmission`, `taskSubmissionRecovery`, `taskSubmissionExecution`, `taskSubmissionState`, `taskDirectSubmission`, `taskTemporalSubmissionAdapter`, and `sheinSubmitLocks`.
- `taskRecoveryService` depends on `Repository`, `TaskSubmitter`, and time only. It is submission-adjacent and owns recover-and-submit behavior for blocked retryable tasks.
- `Repository.BulkRecoverBlockedTasks(...)` is explicitly documented as persistence-only; `TaskRecoveryService` owns authoritative recovery semantics.
- `taskSubmissionService`, `taskDirectSubmissionService`, `taskSubmissionRecoveryService`, `taskSubmissionExecutionService`, `taskSubmissionStateService`, and `taskTemporalSubmissionAdapter` still depend on root models and SHEIN-specific packages, so they should not be moved into `internal/listingkit/submission` yet.

## 5. Facade vs. Rule Ownership

### Mostly facade or assembly

- `service_submit*.go`
- `service_submission_collaborators.go`
- `task_requeue_service.go` service accessor/wiring portion
- `api/*submit*`
- `api/*recovery*`
- `api/*requeue*`
- `httpapi/*submit*`
- `httpapi/*temporal*`

These should stay thin and delegate.

### Mixed orchestration and platform rules

- `task_submission_service.go`
- `task_direct_submission_service.go`
- `task_submission_recovery_service.go`
- `task_submission_execution_service.go`
- `task_temporal_submission_adapter.go`

These are the main “mixed” files where orchestration and SHEIN-specific concerns still meet.

### Predominantly SHEIN business rules

- `shein_submit_*`
- `submit_*_shein.go`
- `submission_projection_shein.go`
- `shein_workspace_submit_bridge.go`

These should not be treated as generic submission internals just because they are part of the submit flow.

## 6. Boundary Observations

1. `submissionCollaborators` is now the right root-side consolidation seam for submit, recovery, direct submit, temporal adapter, state, requeue, and submit locks.
2. `taskRecoveryService` is submission-adjacent and now belongs in the same collaborator cluster, but it should not move to generic `submission/` because it still owns task-level recover-and-submit semantics over root task/repository models.
3. `taskRequeueService` is grouped with submission collaborators, which makes requeue/retry/recovery easier to reason about as one cluster.
4. The `submission/` package is viable for shared mechanics, but not yet for most orchestrators because those orchestrators still depend on root `listingkit` models, repository interfaces, and SHEIN package structures.
5. The biggest structural risk is not service wiring anymore; it is the remaining mix of generic orchestration and SHEIN-specific rules inside the collaborator services.

## 7. Recommended Migration Order

Recommended next slices after this inventory:

1. Keep shrinking root service submission surface by grouping adjacent collaborators and accessors consistently.
2. Prefer extracting model-light helper groups into `internal/listingkit/submission` when they depend only on generic submit mechanics.
3. Split collaborator internals by responsibility before attempting package moves:
   - entry orchestration,
   - runtime context resolution,
   - state persistence,
   - remote confirmation recovery,
   - direct/temporal execution paths.
4. Delay true package extraction for SHEIN-heavy helpers until there is a safe marketplace-owned target or a narrower shared model seam.

## 8. Concrete Candidate Files For Near-Term Refactors

Low-risk next candidates:

- `internal/listingkit/task_recovery_service.go`
  - keep in the submission collaborator cluster,
  - do not move to generic `submission/`,
  - consider extracting durability/restore helper functions only if they are model-light.
- `internal/listingkit/task_submission_service.go`
  - continue reducing central branching and inline remote-refresh result mutation.
- `internal/listingkit/task_submission_recovery_service.go`
  - identify helper groups that can be isolated without moving marketplace rules.
- `internal/listingkit/service_submit_recovery.go`
  - keep facade-only and move any remaining construction details into explicit builders if needed.

Avoid as an early package-move target:

- `shein_submit_*`
- `submit_*_shein.go`
- `task_temporal_submission_adapter.go`

These are still tightly coupled to root models and SHEIN-specific behavior.

## 9. Success Criterion For Phase 3.1

This inventory is complete when:

- submission-related files are grouped by concept,
- facade versus business-rule ownership is explicit,
- the current `submission/` package role is distinguished from root `listingkit` orchestrators,
- later refactoring slices can cite this document instead of re-inventing the map.

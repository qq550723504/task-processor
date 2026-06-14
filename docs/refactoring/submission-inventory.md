# ListingKit Submission Inventory

> Status: historical Phase 3.1 inventory snapshot for the submission consolidation track described in [project-wide-execution-plan.md](./project-wide-execution-plan.md) and [listingkit-refactoring-plan.md](./listingkit-refactoring-plan.md).
>
> Current authority note: this document remains useful for file-group history and local terminology, but the current approved target direction for submission and Temporal structure is defined by [project-wide-refactoring-plan.md](./project-wide-refactoring-plan.md), [project-wide-execution-plan.md](./project-wide-execution-plan.md), and [listingkit-boundary-checkpoint.md](./listingkit-boundary-checkpoint.md). Where this inventory conflicts with those newer checkpoints, follow the newer checkpoints.

## 1. Purpose

This document inventories the current submission-related code under `internal/listingkit` so later refactoring slices can move one coherent boundary at a time.

It is intentionally descriptive first:

- what files currently participate in submit, retry, recovery, requeue, and Temporal submission flows,
- which files are mostly facade and wiring,
- which files already hold reusable submission mechanics,
- which files still mix orchestration with SHEIN-specific rules.

Observed against the repository state on 2026-06-09 after the preview refactoring first wave and the submission execution/direct-submit/recovery/Temporal-adapter/task-recovery/requeue file-group splits.

## 2. Current Submission Shape

Submission behavior currently spans four layers:

1. root `package listingkit` facade methods on `service`,
2. root `package listingkit` collaborator services such as `taskSubmissionService`,
3. root `package listingkit` SHEIN-specific state, readiness, payload, and remote-diagnosis helpers,
4. generic submission/domain helpers under `internal/listing/submission`, while root `shein_submit_state.go` now acts as the remaining ListingKit-owned SHEIN transition sequencer instead of a separate `internal/listingkit/submission` package.

This means the direction is clearer than before, but not finished:

- generic mechanics like locks, retry delay, event shaping, confirm-remote state, refresh guards, and other model-light policies should prefer `internal/listing/submission`,
- service field sprawl is reduced by `submissionCollaborators`,
- Temporal host behavior is no longer represented by a standalone adapter layer; it is split across dedicated lifecycle, flow, persistence, and refresh collaborators,
- root `listingkit` still owns most high-level submit orchestration and most SHEIN submission rules,
- Temporal submission logic is separated by collaborator but still depends heavily on root-side helpers and models.

## 3. Inventory By Responsibility

### A. Root service facade and collaborator wiring

These files mainly expose `service` methods, lazy collaborator accessors, or collaborator construction:

- `internal/listingkit/service_submission_collaborators.go`
- `internal/listingkit/service_submit.go`
- `internal/listingkit/service_submit_direct.go`
- `internal/listingkit/service_submit_direct_prepare.go`
- `internal/listingkit/service_submit_direct_remote.go`
- `internal/listingkit/service_submit_recovery.go`
- `internal/listingkit/service_submit_temporal_adapter.go`
- `internal/listingkit/service_submit_wiring.go`
- `internal/listingkit/task_requeue_service.go`

Current role:

- keep `Service` interface compatibility,
- group submission collaborators behind `s.submission.*`,
- translate service-owned dependencies into collaborator configs,
- keep direct submit facade/accessor separate from direct submit preparation and remote-submit details.

Assessment:

- mostly facade or wiring,
- good candidates to stay thin,
- should avoid accumulating more business logic,
- direct submit helper files are still root `service` methods because they bridge multiple collaborators and root models.

### B. Root collaborator services

These files contain the main internal submission-oriented service objects:

- `internal/listingkit/task_submission_service.go`
- `internal/listingkit/task_submission_refresh_service.go`
- `internal/listingkit/task_submission_refresh_selection.go`
- `internal/listingkit/task_submission_refresh_mutation.go`
- `internal/listingkit/task_direct_submission_service.go`
- `internal/listingkit/task_submission_recovery_service.go`
- `internal/listingkit/task_submission_recovery_lease.go`
- `internal/listingkit/task_submission_recovery_remote.go`
- `internal/listingkit/task_submission_execution_service.go`
- `internal/listingkit/task_submission_execution_product.go`
- `internal/listingkit/task_submission_execution_images.go`
- `internal/listingkit/task_submission_execution_normalize.go`
- `internal/listingkit/task_submission_execution_remote.go`
- `internal/listingkit/task_submission_state_service.go`
- `internal/listingkit/task_temporal_submission_activity_support.go`
- `internal/listingkit/task_temporal_submission_flow_service.go`
- `internal/listingkit/task_temporal_submission_lifecycle_service.go`
- `internal/listingkit/task_temporal_submission_persistence_service.go`
- `internal/listingkit/task_temporal_submission_refresh_service.go`
- `internal/listingkit/task_recovery_service.go`
- `internal/listingkit/task_recovery_durability.go`
- `internal/listingkit/task_recovery_backfill.go`
- `internal/listingkit/task_requeue_service.go`
- `internal/listingkit/task_requeue_helpers.go`

Current role:

- `taskSubmissionService`: entry orchestration for submit attempts,
- `taskSubmissionRefresh*`: refresh/status remote confirmation, selection/request building, mutation, validation, and confirmation application,
- `taskDirectSubmissionService`: direct SHEIN submit path orchestration,
- `serviceSubmitDirectPrepare`: direct submit product preparation, image upload phase, and pre-validation bridge,
- `serviceSubmitDirectRemote`: direct remote submit, response persistence, sensitive-word retry bridge, and finish semantics,
- `taskSubmissionRecoveryService`: recovered submit routing, local-vs-remote recovery decision, recovered success/failure persistence, and finalization,
- `taskSubmissionRecoveryLease`: begin/clear submit lease, in-flight state validation, replay detection, and start-failure marking,
- `taskSubmissionRecoveryRemote`: remote refresh, missing supplier-code fallback, probe logging/state application, shared remote-status request handling, `SubmissionConfirmRemoteUpdate`-based confirmation wiring, and remote status callback bridge,
- `taskSubmissionRecoveryRemote`: remote refresh, missing supplier-code fallback, probe logging/state application, shared remote-status request handling, shared remote-refresh execution request handling, `SubmissionConfirmRemoteUpdate`-based confirmation wiring, and remote status callback bridge,
- `taskSubmissionExecutionService`: execution collaborator shell, Product API construction, and submit runtime resolution,
- `taskSubmissionExecutionProduct`: submit-product preparation, translation API selection, and pre-validation,
- `taskSubmissionExecutionImages`: submit image upload runtime/API construction and upload-cache persistence,
- `taskSubmissionExecutionNormalize`: package normalization, pricing application, final-draft confirmation, sale-attribute repair, and final image/variant guards,
- `taskSubmissionExecutionRemote`: remote publish/save-draft call and remote response logging,
- `taskSubmissionStateService`: persist phases, success, and failure state,
- `taskTemporalSubmissionLifecycle`: workflow-facing lifecycle/readiness entry orchestration, preview bridge, and workflow-start helpers,
- `taskTemporalSubmissionFlow`: prepare, upload, pre-validate, remote submit, and submit-flow orchestration,
- `taskTemporalSubmissionPersistence`: workflow success/failure persistence and state completion support,
- `taskTemporalSubmissionRefresh`: remote status refresh, refresh phase switching, and refresh completion handling,
- `taskRecoveryService`: blocked-retryable recovery flow, recover-now, sweep, submit recovered task, and service facade,
- `taskRecoveryDurability`: submit-failure durability restoration plus root retryable-block rollback/adapter helpers,
- `taskRecoveryBackfill`: historical failed-task listing/repository bridge for the submission-domain retryable-backfill runner,
- `taskRequeueService`: pending-task requeue flow and service facade,
- `taskRequeueHelpers`: request task-id normalization and retry enqueue helper.

Assessment:

- this is the primary current consolidation seam,
- these files are the best place for additional root-level slimming before any deeper package move,
- `taskRecoveryService` and `taskRequeueService` are now part of the submission collaborator cluster, but their semantics are broader than the SHEIN publish path,
- execution, direct-submit, submission-recovery, Temporal lifecycle/flow/persistence/refresh, task-recovery, and requeue responsibilities are now separated by file group, but still live in root `package listingkit` because they depend on root models and SHEIN-specific helpers.

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
- they are the biggest reason a true generic package move is still risky,
- future moves here should favor marketplace-owned placement when a safe target exists rather than moving everything into generic `submission/`.

### E. Generic submission primitives already extracted

These helpers should now prefer `internal/listing/submission` as their canonical home. The old `internal/listingkit/submission` package has been retired from production code, and root `shein_submit_state.go` now holds the remaining ListingKit-owned transition sequencing:

- `submission/action_record*.go`
- `submission/attempt_*.go`
- `submission/confirm_remote_state*.go`
- `submission/event_*.go`
- `submission/inflight_state*.go`
- `submission/refresh_*.go`
- `submission/remote_*.go`
- `submission/result_state*.go`
- `submission/submit_error*.go`

Current role:

- lock management,
- common submit-related error types,
- state/transition/event helpers,
- remote confirmation event parts,
- source facts,
- enqueue retry and bounded backoff logic.

Assessment:

- this is the cleanest current target home for shared submission mechanics,
- newer extractions should prefer `internal/listing/submission` rather than rebuilding a large generic surface under `internal/listingkit/submission`,
- additional helpers can move here only when they do not require root `listingkit` models or create import cycles.
- recovery selection now projects both `StartedAt` and `SupplierCode` from `internal/publishing/shein`, so ListingKit recovery refresh wiring no longer has to re-read those fields from the selected record.
- recovered submission root-state wiring now also carries the publishing-side recovery selection snapshot directly, so ListingKit no longer mirrors recovery report/record/supplier-code fields separately across local recovery, remote confirmation, and refresh request assembly.
- recovery remote confirm-state application now also routes through `internal/publishing/shein`, so ListingKit no longer keeps local wrappers for missing-supplier fallback or confirm-remote state mutation before runner-owned event append/persistence steps.
- refresh fallback-message rule lookup now also routes through `internal/publishing/shein`, so ListingKit refresh confirmation no longer imports marketplace publishing policy directly just to override remote-confirmation fallback text.
- refresh selection/request assembly now also reuses publishing-side snapshot types directly, so ListingKit no longer mirrors refresh action/record/supplier-code or trimmed request-id/remote-input payload fields in local compatibility DTOs before building refresh state.
- Temporal persistence state now also drops unused supplier-code and snapshot fields after publishing-side persistence input application, so ListingKit no longer carries stale ownership signals on the root-side success/failure persistence shell.
- Temporal flow support now also drops the prepared-state payload-stage forwarding wrapper plus the dead execution-state guard helper, so ListingKit no longer keeps single-hop support helpers once the execution-state stage-context seam is the only active consumer.
- Temporal facade upload entrypoint now also delegates directly into flow service, so ListingKit no longer keeps prepared-payload validation or no-upload branching in the facade once flow already owns that decision seam.
- direct-submit and Temporal persistence owners now also wire `submissiondomain.NewResultPersistenceService(...)` directly, so ListingKit no longer keeps a separate result-persistence support file just to map generic result input into success/failure inputs for those two owner services.

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
- `submissionCollaborators` includes task recovery, task requeue, submission orchestration, recovery, execution, state, direct submit, Temporal lifecycle/flow/persistence/refresh collaborators, and submit locks.
- `taskRecoveryService` depends on `Repository`, `TaskSubmitter`, and time only. It is submission-adjacent and owns recover-and-submit behavior for blocked retryable tasks.
- `Repository.BulkRecoverBlockedTasks(...)` is explicitly documented as persistence-only; `TaskRecoveryService` owns authoritative recovery semantics.
- `taskSubmissionService`, `taskDirectSubmissionService`, `taskSubmissionRecovery*`, `taskSubmissionExecution*`, `taskSubmissionStateService`, and `taskTemporalSubmission*` still depend on root models and SHEIN-specific packages, so they should not be moved into generic `internal/listing/submission` yet.
- `taskSubmissionRecoveryRemote` has already shed its pure confirm-remote branch resolution into `internal/publishing/shein`; what remains root-side is probe orchestration plus ListingKit task-state persistence.
- `taskSubmissionRecoveryRemote` has also shed its raw on-way/record/inventory probe helpers into `internal/publishing/shein`; what remains root-side is logging hooks, probe orchestration entrypoints, and ListingKit task-state persistence.
- refresh and recovery now also share one root-side remote-status request model; root `internal/listingkit` no longer duplicates refresh-only confirm-remote request DTO assembly before remote-status resolution.
- temporal refresh and recovery now also share one root-side remote-refresh execution request model; root `internal/listingkit` no longer duplicates long positional executor wiring before remote refresh orchestration.
- refresh runtime state now also reuses that shared remote-status request model; root `internal/listingkit` no longer maintains a second refresh-state field group for action/request/lookup/fallback/API inputs before remote-status resolution and mutation persistence.
- remote-status request construction now also reuses shared root-side builders; root `internal/listingkit` no longer hand-composes parallel `sheinRemoteStatusRequest` field groups across refresh-state assembly, recovery remote-refresh orchestration, and task-id cloning.
- SHEIN refresh-selection and recovery-remote lookup-input builders now also live in `internal/publishing/shein`; root `internal/listingkit` no longer combines default-confirmed plus fallback-message lookup rules inline before building remote-status requests.
- SHEIN submission refresh request projection now also lives in `internal/publishing/shein`; root `internal/listingkit` no longer normalizes refresh request-id plus remote lookup payload fields inline after selecting a submission record.
- SHEIN recovered submission selection now also carries started-at projection from `internal/publishing/shein`; root `internal/listingkit` no longer re-reads recovery record timing fields separately after selecting the recovered submission state.
- SHEIN submission refresh mutation validation now also reuses one publishing-side read-only validation result; root `internal/listingkit` no longer evaluates availability, action match, and request match as separate local checks before mapping them to existing errors.
- remote refresh execution now also reuses one root-side execution state; root `internal/listingkit` recovery and Temporal refresh no longer hand-assemble separate supplier-code plus refresh-started-at wrappers before invoking shared remote refresh orchestration.
- remote refresh orchestration now also reuses `internal/listing/submission`; root `internal/listingkit` recovery and Temporal refresh no longer duplicate the persist-phase, execute-remote, append-event, and success-vs-failure finish skeleton around those shared execution states.
- recovered submission route dispatch now also reuses `internal/listing/submission`; root `internal/listingkit` no longer duplicates the accepted-response local-completion vs remote-confirmation branch skeleton inside recovered submit routing.
- recovered submit lease-acquire dispatch now also reuses `internal/listing/submission`; root `internal/listingkit` no longer duplicates replay-preview vs remote-recovery vs blocked-missing-package branching after lease acquisition.
- workflow-start failure cleanup now also reuses `internal/listing/submission`; root `internal/listingkit` no longer duplicates failure-record persistence, lease cleanup, and returned-error priority resolution after workflow-start failure.
- recovered remote-recovery routing now also reuses one root-side state object; root `internal/listingkit` no longer threads task/package/action/request/response fields separately across route selection, local completion, remote confirmation, and success/failure completion.
- remote confirmation/refresh success and failure tails now also reuse one root-side completion support layer; root `internal/listingkit` recovery and Temporal refresh no longer hand-assemble duplicate complete/fail plus remember/persist-success/save-result sequences after remote confirmation.
- Temporal publish success/failure entrypoints now also reuse one root-side persistence state; root `internal/listingkit` Temporal persistence no longer loads task/package and reapplies supplier-code/response/snapshot input separately across success and failure paths before routing into persistence tails.
- Temporal payload/remote-submit flow now also reuses one root-side execution state; root `internal/listingkit` Temporal flow no longer reloads task/package and rebuilds payload-stage or remote-submit input context separately across prepare/upload/prevalidate/submit-remote entrypoints.
- Temporal readiness/payload preparation now also reuses one root-side prepared publish state; root `internal/listingkit` Temporal lifecycle and prepare-payload entrypoints no longer rebuild activity request plus submit-package normalization separately before readiness gates or payload-stage entry.
- Temporal upload/prevalidate/submit-remote continuation now also reuses one root-side prepared-payload resume state; root `internal/listingkit` Temporal flow no longer re-validates payload shape, reloads task/package, and rebuilds payload-stage context separately across resumed prepared-payload entrypoints.
- Temporal SHEIN service entrypoints now also reuse one root-side facade; root `internal/listingkit` no longer routes publish lifecycle, payload flow, persistence, and remote-refresh entrypoints through four separate workflow collaborators at the service boundary.
- Temporal SHEIN collaborator config assembly now also reuses one root-side wiring bundle; root `internal/listingkit` no longer rebuilds the same submission assembly plus orchestrator binding set separately across Temporal lifecycle/flow/persistence/refresh config builders.
- Temporal submit facade construction now also reuses one explicit root-side config builder; root `internal/listingkit` no longer hand-assembles lifecycle, flow, persistence, and refresh collaborators directly inside the lazy facade accessor.
- Temporal workflow collaborators now also reuse one shared root-side ensure seam plus collaborator wiring bundle; root `internal/listingkit` no longer repeats lifecycle/flow/persistence/refresh lazy-construction steps across `initializeSubmitWorkflowCollaborators()` and the Temporal accessor set.
- Temporal workflow ensure wiring now also resolves one collaborator bundle before assignment; root `internal/listingkit` no longer hand-orders persistence/lifecycle/flow/refresh/facade construction inside that ensure seam.
- direct submit and refresh config assembly now also reuse one root-side managed-submission wiring bundle; root `internal/listingkit` no longer rebuilds the same submission assembly plus callback set across those builders, while recovery config remains the constructor stop-line because it participates in the orchestrator's own recovery dependency.
- managed submission collaborators now also reuse one shared root-side ensure seam plus collaborator wiring bundle; root `internal/listingkit` no longer repeats recovery/direct/refresh/submission lazy-construction steps across `initializeSubmitOrchestratorCollaborators()` and the managed-submission accessor set.
- managed submission ensure wiring now also resolves one collaborator bundle before assignment; root `internal/listingkit` no longer hand-orders recovery/direct/refresh/submission construction inside that ensure seam.
- retryable failure classification plus blocked-task reblock policy now also reuse `internal/listing/submission`; root `internal/listingkit` no longer keeps generic reason-code matching or next-retry scheduling rules in its blocked-recovery and failed-task backfill flows.
- retryable failure persistence branching plus failed-task backfill block construction now also reuse `internal/listing/submission`; root `internal/listingkit` no longer keeps blocked-vs-failed persistence branching or backfill retry metadata assembly inline before repository writes.
- failed-task retryable backfill orchestration now also reuses `internal/listing/submission`; root `internal/listingkit` no longer loops through failed tasks or applies created-after filtering inline before historical blocked-state backfill writes.
- submit result persistence dispatch now also reuses `internal/listing/submission`; root `internal/listingkit` no longer repeats success/failure branching, original-error return behavior, or fallback tail orchestration across direct-submit completion and Temporal publish persistence.
- submit collaborator config builders now also reuse shared root-side submitter/support/assembly/temporal wiring bundles within each ensure seam; root `internal/listingkit` no longer rebuilds identical dependency snapshots across recovery/requeue, submission core, managed submission, and Temporal collaborator constructor chains.
- submission assembly/support builder paths now also reuse one shared root-side base wiring bundle; root `internal/listingkit` no longer rebuilds the same assembly plus support view separately across submission core, managed submission, and Temporal config/collaborator entrypoints.
- managed submission ensure wiring now also reuses one per-resolve managed wiring snapshot for direct/refresh construction; root `internal/listingkit` no longer rebuilds the same managed submit dependency view multiple times during one collaborator resolution pass.
- submission recovery and Temporal config builders now also reuse shared base/temporal wiring helpers; root `internal/listingkit` no longer rebuilds standalone assembly or temporal wiring views separately across recovery, lifecycle, persistence, flow, and refresh config paths.
- submission service, execution, and state config assembly now also reuse one root-side support wiring bundle; root `internal/listingkit` no longer re-resolves repository access, runtime resolver callbacks, pricing rule lookup, or remember-submitted hooks separately across those builders.
- submission core ensure wiring now also resolves one collaborator bundle before assignment; root `internal/listingkit` no longer hand-orders execution/state construction inside that ensure seam.
- submission core collaborators now also reuse one shared root-side ensure seam plus collaborator wiring bundle; root `internal/listingkit` no longer repeats execution/state lazy-construction steps across `initializeSubmitStateCollaborators()` and the core accessor pair.
- submission task-recovery collaborators now also reuse one shared root-side ensure seam plus collaborator wiring bundle; root `internal/listingkit` no longer repeats recovery/requeue lazy-construction steps across `initializeSubmitTaskRecoveryCollaborators()` and the task-recovery accessor pair.
- submission task-recovery ensure wiring now also resolves one collaborator bundle before assignment; root `internal/listingkit` no longer hand-orders recovery/requeue construction inside that ensure seam.
- requeue task-status eligibility policy now also lives in `internal/listing/submission`; root `internal/listingkit` no longer formats nil/non-pending requeue rejection reasons inline and keeps only its local pending-status mapping.
- `taskSubmissionExecutionService` is now a thin shell for constructor/runtime/Product API wiring, while product preparation, image upload, normalization, and remote submit are split into dedicated execution files.
- `taskSubmissionRefresh*` is now split into main flow, selection/request building, and mutation/validation file groups.
- `serviceSubmitDirect*` is now split into facade/accessor, direct product preparation, and direct remote submit file groups.
- `taskSubmissionRecovery*` is now split into recovered-route/finalization, lease management, and remote confirmation file groups.
- `taskTemporalSubmission*` is now split into lifecycle/readiness, submit-flow, persistence, and remote-refresh activities.
- Temporal host entrypoints now route directly from `service` to those dedicated collaborators; the old extra adapter layer no longer represents the target shape.
- Temporal facade construction now also routes through an explicit config builder instead of directly wiring four collaborators inside `taskTemporalSubmissionOrDefault()`.
- Temporal collaborator lazy initialization now also routes through one shared ensure seam instead of repeating component construction across each accessor and the workflow initializer.
- managed submission collaborator lazy initialization now also routes through one shared ensure seam instead of repeating recovery/direct/refresh/submission construction across each accessor and the orchestrator initializer.
- submission core collaborator lazy initialization now also routes through one shared ensure seam instead of repeating execution/state construction across each accessor and the state initializer.
- submission task-recovery collaborator lazy initialization now also routes through one shared ensure seam instead of repeating recovery/requeue construction across each accessor and the task-recovery initializer.
- `taskRecovery*` is now split into recovery flow, durability/reblock helpers, and historical backfill.
- `taskRequeue*` is now split into requeue flow/facade and helper functions.
- `serviceStudioBatchRun*` wiring now also shares one root-side batch-run wiring bundle, so studio batch-run service/coordinator/executor builders no longer rebuild the same repo pair and domain-runner assembly separately.
- `serviceStudioSession*` wiring now also shares one root-side session wiring bundle, so studio session and batch-draft builders no longer rebuild the same session repository and domain-runner assembly separately.
- `serviceStudioBatch*` wiring now also shares one root-side batch-service wiring bundle, so studio batch service builders no longer keep prebuilt batch detail/review runners inline on the config builder path.
- studio batch collaborators now also reuse one shared root-side ensure seam plus collaborator wiring bundle, so root `listingkit` no longer repeats batch-generation/batch-service lazy-construction steps across `initializeTaskStudioBatchCollaborators()` and the studio batch accessor pair.
- studio session collaborators now also reuse one shared root-side ensure seam plus collaborator wiring bundle, so root `listingkit` no longer repeats session/batch-draft/media lazy-construction steps across `initializeTaskStudioSessionCollaborators()` and the studio session accessor set.
- studio batch-run collaborators now also reuse one shared root-side ensure seam plus collaborator wiring bundle, so root `listingkit` no longer repeats batch-run/executor/coordinator lazy-construction steps across `initializeTaskStudioBatchCollaborators()`, the batch-run accessor set, and startup recovery bootstrap.
- studio batch retry preparation now also reuses `internal/listing/studio`; root `internal/listingkit` no longer open-codes detail-load, retry-item selection, reset, and final detail reload inside the prepare-only retry path before handing off to batch generation.
- studio batch task preparation now also reuses `internal/listing/studio`; root `internal/listingkit` no longer open-codes pending-task session updates, batch status persistence, and final detail-result reload inside the prepare-only task-creation path before task execution begins.
- studio batch task resume finalization now also reuses `internal/listing/studio`; root `internal/listingkit` no longer open-codes pending-task cleanup, created/failed task persistence, batch completion status updates, and final detail-result reload after resumed task creation finishes.

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
- `task_submission_refresh_*.go`
- `task_direct_submission_service.go`
- `service_submit_direct_prepare.go`
- `service_submit_direct_remote.go`
- `task_submission_recovery_*.go`
- `task_submission_execution_*.go`
- `task_temporal_submission_*.go`
- `task_recovery_*.go`
- `task_requeue_*.go`

These are the main “mixed” files where orchestration and SHEIN-specific or task-recovery/requeue concerns still meet, even though several of them are now narrower file groups.

### Predominantly SHEIN business rules

- `shein_submit_*`
- `submit_*_shein.go`
- `submission_projection_shein.go`
- `shein_workspace_submit_bridge.go`

These should not be treated as generic submission internals just because they are part of the submit flow.

## 6. Boundary Observations

1. `submissionCollaborators` is now the right root-side consolidation seam for submit, recovery, direct submit, Temporal lifecycle/flow/persistence/refresh, state, requeue, and submit locks.
2. `taskRecoveryService` is submission-adjacent and now belongs in the same collaborator cluster, but it should not move to generic `submission/` because it still owns task-level recover-and-submit semantics over root task/repository models.
3. `taskRequeueService` is grouped with submission collaborators, which makes requeue/retry/recovery easier to reason about as one cluster.
4. `internal/listing/submission` is viable for shared mechanics, but not yet for most orchestrators because those orchestrators still depend on root `listingkit` models, repository interfaces, and SHEIN package structures.
5. The biggest structural risk is not service wiring anymore; it is the remaining mix of generic orchestration and SHEIN-specific rules inside the collaborator services.
6. File-group splitting has reduced per-file density, but it has not changed ownership: root `listingkit` still owns compatibility/orchestration while SHEIN-specific behavior awaits a safer marketplace-owned target.

## 7. Recommended Migration Order

Recommended next slices after this inventory:

1. Keep shrinking root service submission surface by grouping adjacent collaborators and accessors consistently.
2. Prefer extracting model-light helper groups into `internal/listing/submission` when they depend only on generic submit mechanics.
3. Split collaborator internals by responsibility before attempting package moves:
   - entry orchestration,
   - runtime context resolution,
   - state persistence,
   - remote confirmation recovery,
   - direct/temporal execution paths.
4. Delay true package extraction for SHEIN-heavy helpers until there is a safe marketplace-owned target or a narrower shared model seam.

## 8. Concrete Candidate Files For Near-Term Refactors

Low-risk next candidates:

- `internal/listingkit/service_submit_wiring.go`
  - keep config builders readable,
  - avoid adding large inline closures,
  - move repeated loader/building behavior into named helpers.
- `internal/listing/submission/*.go`
  - only consider model-light helper extraction when root orchestrators no longer need root models.

Avoid as an early package-move target:

- `shein_submit_*`
- `submit_*_shein.go`
- `task_temporal_submission_*.go`

These are still tightly coupled to root models, workflow contracts, and SHEIN-specific behavior.

## 9. Success Criterion For Phase 3.1

This inventory is complete when:

- submission-related files are grouped by concept,
- facade versus business-rule ownership is explicit,
- the current `internal/listing/submission` role is distinguished from root `listingkit` orchestrators and the remaining root-side SHEIN transition sequencing in `shein_submit_state.go`,
- later refactoring slices can cite this document instead of re-inventing the map.

Current status: Phase 3.1 has met this success criterion at the file-group and inventory level. Future work should use this inventory before attempting package moves.

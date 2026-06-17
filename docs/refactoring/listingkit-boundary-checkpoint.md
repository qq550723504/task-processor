# ListingKit Boundary Checkpoint

> Status: current checkpoint for the recent ListingKit slimming and boundary-guard wave.

## Purpose

This checkpoint records the current small-loop refactor state so the next phase does not keep extracting details from `internal/listingkit` without a clear ownership gain.

This wave was intentionally not a broad migration. It tightened existing target packages, added guardrails, and moved only small orchestration seams that already had stable behavior.

## Completed Seams

### `internal/listing/studio`

Current extracted seams:

- session ensure/get flow,
- session async-job sync flow,
- session generation metadata patch flow,
- session review/task metadata patch flow,
- session general metadata patch orchestration,
- batch draft default-name sequencing,
- batch draft upsert policy: default design type, create-time generation-job sanitization, and batch-name resolution,
- batch detail status aggregation and status-preservation policy,
- batch-run completion rules: cancel unfinished items, count item statuses, resolve final run status.
- studio batch top-level facade routing now also lives in `internal/listing/studio`; `internal/listingkit` keeps only request normalization adapters, repository callbacks, and orchestration wiring into the shared batch runners.
- generic studio batch repository contract plus unknown-item/ownership-conflict sentinel errors now live in `internal/listing/studio`; `internal/listingkit` keeps only concrete record/detail types plus repository implementation and wiring bridges.
- `internal/listingkit` studio batch-run service/coordinator/executor config assembly now also reuses one shared root-side wiring bundle, so the compatibility shell no longer rebuilds the same batch-run repo pair and domain-runner assembly separately across those three builders.
- `internal/listingkit` studio session and batch-draft config assembly now also reuse one shared root-side session wiring bundle, so the compatibility shell no longer rebuilds the same studio-session repository plus domain-runner set separately across those builders.
- `internal/listingkit` studio batch service config assembly now also reuses one shared root-side batch-service wiring bundle, so the compatibility shell no longer stores prebuilt batch detail/review runners directly in the config builder path.

### `internal/listingkit` submission/root wiring

Current state:

- submission service, execution, and state config assembly now also reuse one shared root-side support wiring bundle, so the compatibility shell no longer re-resolves repository access, runtime resolver callbacks, pricing rule lookup, and remember-submitted hooks separately across those builders.
- submission core collaborators now also reuse one shared root-side ensure seam plus collaborator wiring bundle, so the compatibility shell no longer repeats execution/state lazy-construction steps across the state initializer and the core accessor pair.
- submission task-recovery collaborators now also reuse one shared root-side ensure seam plus collaborator wiring bundle, so the compatibility shell no longer repeats recovery/requeue lazy-construction steps across the task-recovery initializer and the task-recovery accessor pair.
- requeue task-status eligibility policy now also lives in `internal/listing/submission`, so the compatibility shell no longer formats nil/non-pending rejection reasons inline and keeps only its local pending-status mapping.
- managed submission collaborators now also reuse one shared root-side ensure seam plus collaborator wiring bundle, so the compatibility shell no longer repeats recovery/direct/refresh/submission lazy-construction steps across the orchestrator initializer and the managed-submission accessor set.
- Temporal submit facade construction is now removed, and the remaining collaborator config assembly now lives beside collaborator wiring support, so the compatibility shell no longer hand-assembles lifecycle, flow, persistence, and refresh collaborators inside lazy accessors or through a separate root wrapper file.
- Temporal workflow collaborators now also reuse one shared root-side ensure seam plus collaborator wiring bundle, so the compatibility shell no longer repeats lifecycle/flow/persistence/refresh lazy-construction steps across the workflow initializer and the Temporal accessor set.

`internal/listingkit` still owns:

- API shell DTOs,
- repository implementations and adapters,
- expected-updated-at conflict checks,
- field assignment adapters for mixed studio session updates,
- concrete batch run executor loop,
- generation resume and task creation behavior,
- logging and legacy error translation.

Guardrail:

- `internal/listing/studio` must not import `internal/listingkit`, SHEIN marketplace/workspace/publishing packages, or runtime/integration wiring.

### `internal/listing/preview`

Current state:

- preview package already owns generic preview read/service skeletons,
- `listingkit` task preview delegates through `previewdomain.TaskPreviewService`,
- preview package owns render-preview metadata summary extraction, while `listingkit` still owns asset/platform DTO adapters,
- preview package owns platform render-preview summary aggregation over neutral slot inputs,
- preview package owns render-preview capability mapping and raster-preview fallback rules, while legacy generation packages keep compatibility wrappers,
- preview domain remains independent from `listingkit` and SHEIN-specific packages.

Guardrail:

- `internal/listing/preview` must not import `internal/listingkit`, `internal/marketplace/shein`, `internal/publishing/shein`, or `internal/workspace/shein`.

### `internal/marketplace/shein/publishing`

Current state:

- new canonical SHEIN marketplace publishing helpers should land here,
- pricing policy is already represented in the marketplace package,
- legacy `internal/publishing/shein.PricingPolicy` is a compatibility alias over the marketplace pricing policy, guarded by a bridge contract test,
- `internal/publishing/shein` remains a legacy compatibility/model package for now.

Guardrail:

- `internal/marketplace/shein/publishing` must not import `internal/listingkit` or root runtime/integration wiring.

### `internal/marketplace/shein/workspace`

Current state:

- workspace package already owns inspection, status overview, success messaging, revision helpers, and other SHEIN workspace-facing presentation rules,
- SHEIN preview-card status, summary, and needs-review rules now also live in the marketplace workspace package,
- SHEIN preview review-summary plus final-review image/SKU projection rules now also live in the marketplace workspace package, while `listingkit` keeps the preview payload shell and canonical/source-product wiring,
- SHEIN preview resolution-cache summary plus image-upload preflight aggregation rules now also live in the marketplace workspace package; `listingkit` keeps only runtime upload/cache classifiers and preview payload shell wiring,
- SHEIN source-product summary projection from canonical product now also lives in the marketplace workspace package; `listingkit` keeps only preview/final-review payload assembly that references it,
- SHEIN store-resolution summary DTO/value projection now also lives in the marketplace workspace package; `listingkit` still keeps selection/task/preview context fallback logic and submission-store-resolution conversion,
- SHEIN submission-event store-resolution attachment and submission store-resolution DTO construction now also live in the marketplace workspace package; `listingkit` keeps only task/snapshot extraction and root DTO adaptation entrypoints,
- SHEIN remote-submit persistence input preparation now also lives in `internal/publishing/shein`; root `internal/listingkit` no longer resolves supplier-code fallback or snapshot persistence inline before remote submit attempts,
- SHEIN temporal/direct submit snapshot and remote-response persistence now call `internal/publishing/shein` mutations directly; root `internal/listingkit` no longer keeps pass-through setter wrappers for those pure submission-state writes,
- Temporal submit persistence is now owned by a dedicated collaborator instead of living inline in the Temporal host surface; success/failure state completion concentrates in a separate service,
- Temporal submit payload-stage plus remote-submit flow is now owned by a dedicated collaborator instead of living inline in the Temporal host surface; prepare/upload/pre-validate/remote-submit flow concentrates in a separate service,
- Temporal submit lifecycle entry responsibilities are now owned by a dedicated collaborator instead of living inline in the Temporal host surface; begin/readiness/workflow-start/preview flow concentrates in a separate service,
- Temporal submit remote-refresh orchestration is now owned by a dedicated collaborator instead of living inline in the Temporal host surface; refresh phase switching, remote refresh, and completion handling concentrate in a separate service,
- Temporal SHEIN activity host entrypoints now route through one dedicated Temporal submit collaborator, while lifecycle, flow, persistence, and refresh remain internal sub-services behind that seam; the extra workflow adapter layer has been removed,
- SHEIN task-list work-queue and action-queue derivation rules now also live in the marketplace workspace package,
- SHEIN inspection review-reason extraction plus cookie-unavailable review-note detection/cleanup rules now also live in the marketplace workspace package,
- SHEIN workflow/work-queue/action-queue taxonomy definitions and display descriptors now also live in the marketplace workspace package, while `listingkit` keeps only task-list facet DTO adaptation plus blocker/warning descriptors tied to local issue codes,
- SHEIN submit template-freshness evaluation rules for category, attribute, and sale-attribute drift now also live in the marketplace workspace package; `listingkit` keeps only online template loading, readiness check assembly, and persistence/orchestration glue,
- `listingkit` keeps only platform-card DTO assembly and cross-platform queue/preview enrichment.

Guardrail:

- `internal/marketplace/shein/workspace` must not import `internal/listingkit` or root runtime/integration wiring.

## Legacy Exceptions

These exceptions are intentional for the current checkpoint:

- `internal/publishing/shein` may still be imported by existing ListingKit submission/model flows.
- `internal/publishing/shein` may still depend on legacy OpenAI infra helpers, but production code must not import `internal/listingkit` or root runtime packages.
- `internal/workspace/shein` may still exist as a compatibility shell over `internal/marketplace/shein/workspace`.
- root `internal/listingkit` may still own facade composition, API-facing DTOs, and adapter glue.

These exceptions should get thinner over time, but they are not blockers for this checkpoint.

## Phase Closeout

This boundary wave is now a checkpointed phase, not an open invitation to keep shaving helpers.

Current stop lines:

- do not keep splitting `internal/listing/studio` unless the seam removes real root-object ownership; field assignment adapters, generation resume, task creation, and batch-run execution should stay in `listingkit` for now,
- do not keep moving `asset_render_preview_groups.go` platform DTO composition into `internal/listing/preview`; preview now owns neutral render metadata, summary, and capability rules, while platform image-bundle adapters remain legacy DTO glue,
- do not ban `internal/publishing/shein` imports from `listingkit` yet; existing submission/model flows still depend on it as a compatibility package.
- do not recreate an `internal/listingkit/submission` compatibility package now that the last production transition sequencer has been folded back into `shein_submit_state.go`; new generic submit primitives should land in `internal/listing/submission` directly.

Good next candidates:

- `internal/listing/submission`: continue with small read-only policy seams that do not touch Temporal determinism or platform submit side effects,
- `internal/marketplace/shein/publishing`: continue guard-backed migration of new SHEIN publishing rules, not legacy model relocation,
- `internal/product/sourcing`: only add source normalization seams when crawler/runtime adapters can remain thin.

## Next Direction

Do not continue extracting studio or preview internals unless a new seam clearly reduces root `listingkit` ownership.

Preferred next areas:

- `internal/listing/submission`: only continue if a seam reduces duplicate orchestration without touching Temporal determinism or platform submit semantics.
- `internal/marketplace/shein/publishing`: keep new marketplace publishing rules out of root `listingkit`.
- `internal/product/sourcing`: consolidate product source request/result normalization and source identity only when a new crawler/source seam appears.
- `internal/integration/crawler/*`: keep crawler adapters focused on raw source execution; boundary guards prevent dependencies on `listingkit`, marketplace/workspace/publishing packages, or `product/sourcing`.

Recommended next slice:

- evaluate another minimal `internal/listing/submission` read-only policy seam or SHEIN marketplace publishing guard-backed rule seam before extracting more studio/preview helpers.

Current submission stop line:

- `shein_submit_state.go` is now the direct root-side stop-line for SHEIN submit transition sequencing. Generic readiness, retry, locking, response-error, lease, and event DTO primitives should not route through a recreated `internal/listingkit/submission` compatibility package anymore.
- remaining root-side submission owners should now stay narrow:
  - `shein_submit_state.go` keeps only transition sequencing that binds generic submission state flow to SHEIN event ordering and task-owned persistence callers,
  - `task_submission_refresh_service.go` and `task_submission_refresh_mutation.go` keep only task/repository mutation entrypoints plus refresh-runner adaptation around publishing-side selection/validation helpers,
  - `task_submission_state_service.go`, `task_submission_state_persistence_support.go`, and `task_temporal_submission_persistence_service*.go` keep only task/result persistence callbacks, Temporal/direct fallback routing, and event append ordering around submission-domain runners,
  - new submission work should prefer `internal/listing/submission` or `internal/publishing/shein` unless it truly requires those root task/repo/event ordering responsibilities.

Completed submission slices:

- source-facts readiness policy for 1688-derived facts now lives in `internal/listing/submission`; the old `internal/listingkit/submission` compatibility wrapper has been removed.
- in-process submit lock manager now lives in `internal/listing/submission`; the old `internal/listingkit/submission` compatibility alias has been removed.
- enqueue retry/backoff policy for queue-full submit retries now lives in `internal/listing/submission`; the old `internal/listingkit/submission` compatibility wrapper has been removed.
- response outcome policy for save-draft success and publish response errors now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN response adapters.
- phase detail mapping policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN phase labels.
- failure-state fallback policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN report adapters.
- remote-recovery lease expiry policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN phase/report adapters.
- active attempt lease policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN report adapters.
- in-flight clearing match policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN report mutation.
- submit-in-progress error shape now lives in `internal/listing/submission`; the old `internal/listingkit/submission` compatibility alias has been removed.
- submission event history policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN event model adaptation.
- attempt result status policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN record/event DTO shaping.
- submission event outcome policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN event DTO assembly and response pointer wiring.
- phase event policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN phase-event DTO assembly.
- remote record id normalization policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN confirm-remote DTO assembly and record mutation.
- confirm-remote state policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN confirm-remote DTO assembly.
- refresh mutation guard policy now lives in `internal/listing/submission`; `internal/listingkit` keeps SHEIN report/record loading and error translation.
- refresh selection policy now lives in `internal/listing/submission`; `internal/publishing/shein` now owns SHEIN report/record/supplier-code projection for refresh, while `internal/listingkit` keeps only validation error translation.
- refresh request-id normalization now lives in `internal/listing/submission`; `internal/listingkit` keeps refresh request DTO assembly.
- retryable failure classification and blocked-task reblock policy now live in `internal/listing/submission`; `internal/listingkit` keeps task-owned `RetryableBlock` persistence and rollback wiring.
- retryable failure persistence branching and failed-task backfill block construction now also live in `internal/listing/submission`; `internal/listingkit` keeps only repository callbacks plus root `RetryableBlock` adaptation for those paths.
- failed-task retryable backfill orchestration now also lives in `internal/listing/submission`; `internal/listingkit` keeps only failed-task listing and blocked-state persistence callbacks for the historical backfill path.
- submit result persistence dispatch now also lives in `internal/listing/submission`; `internal/listingkit` keeps only SHEIN task/result/package adaptation plus local fallback callbacks while generic success-vs-failure routing and original-error return policy moved out.
- submit collaborator config builders now also retain shared root-side submitter/support/assembly/temporal wiring bundles within each ensure seam, so `internal/listingkit` no longer re-snapshots the same dependency graph across recovery/requeue, submission core, managed submission, and Temporal collaborator constructors.
- submission assembly/support builder paths now also converge on one shared root-side base wiring bundle, so `internal/listingkit` no longer rebuilds the same assembly plus support view separately across submission core, managed submission, and Temporal config/collaborator entrypoints.
- managed submission ensure wiring now also reuses one per-resolve managed wiring snapshot for direct/refresh construction, so `internal/listingkit` no longer rebuilds the same managed submit dependency view multiple times inside one collaborator resolution pass.
- submission recovery and Temporal config builders now also converge on shared base/temporal wiring helpers, so `internal/listingkit` no longer rebuilds standalone assembly or temporal wiring views separately across recovery, lifecycle, persistence, flow, and refresh config entrypoints.
- recovered submission state now also carries one publishing-side recovery selection snapshot, so `internal/listingkit` no longer mirrors recovery report/record/supplier-code fields separately before local-vs-remote recovery and remote-refresh routing.
- refresh remote policy now lives in `internal/listing/submission`; `internal/listingkit` keeps SHEIN lookup-code/SPU-name enrichment.
- action-record state policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN package/report DTO adaptation while generic action-slot selection and last-state synchronization moved out.
- action-record query policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN record DTO views while success checks and completed-record lookup moved out.
- action-record mutation policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN field assignment details while request-id-guarded slot mutation moved out.
- remote-sync policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN remote field assignment while report-level remote status/check-time sync and guarded record mutation moved out.
- attempt-record fallback policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN record construction details while matching-request reuse and in-flight timing/attempt fallback moved out.
- in-flight state policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN report/record field mapping while begin/advance state updates for action, phase, lease, and attempt count moved out.
- attempt finalize policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN record field assignment while final status/error/finished-at resolution moved out.
- attempt record draft policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN record DTO assembly while minimal draft status/error/submitted-at shaping moved out.
- event draft policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN submission-event DTO assembly while generic attempt/phase event field shaping moved out.
- generic submit lock manager ownership now lives in `internal/listing/submission`; service collaborator/config callsites now use the new package directly, while `internal/listingkit/submission` keeps only a compatibility alias.
- generic source-facts, enqueue-retry, response-error, and in-flight TTL primitives now live in `internal/listing/submission`; direct service/readiness/requeue/remote-submit callsites use the new package directly, while `internal/listingkit/submission` remains for SHEIN-specific event/state adaptation.
- generic requeue task-id normalization now lives in `internal/listing/submission`; `internal/listingkit` no longer keeps a duplicate trim/dedupe helper for requeue requests.
- generic `SubmitInProgressError` ownership now lives in `internal/listing/submission`; direct API/service/Temporal callsites use the new package, while `internal/listingkit/submission` stays a compatibility shell for SHEIN-specific submission helpers.
- SHEIN remote record classification rules now live in `internal/marketplace/shein/publishing`; `internal/listingkit` keeps remote lookup orchestration and submission-state mutation only.
- SHEIN remote confirmation fallback/default-confirmed policy now lives in `internal/marketplace/shein/publishing`; `internal/listingkit` keeps refresh/recovery orchestration only.
- SHEIN remote record selection rules now live in `internal/marketplace/shein/publishing`; `internal/listingkit` no longer decides preferred SPU match vs latest-create-time fallback after remote record queries.
- SHEIN remote response parsing rules now live in `internal/marketplace/shein/publishing`; `internal/listingkit` no longer interprets on-way/record/inventory response DTO success semantics directly.
- SHEIN submission response acceptance, remote lookup SPU resolution, and remote lookup-code collection now live in `internal/publishing/shein`; `internal/listingkit` no longer derives those identities from package state inline during refresh/recovery orchestration.
- SHEIN remote refresh/recovery lookup input projection now also lives in `internal/publishing/shein`; `internal/listingkit` no longer keeps separate root-side structs for lookup codes, SPU name, and fallback-policy payload wiring before remote confirmation orchestration.
- SHEIN remote confirmation payload shape now reuses `internal/publishing/shein.SubmissionConfirmRemoteUpdate`; root `internal/listingkit` no longer keeps a separate `sheinRemoteConfirmation` DTO or refresh-side duplicate confirm-remote apply path.
- SHEIN remote refresh/recovery confirm-remote decision tree now also lives in `internal/publishing/shein`; root `internal/listingkit` remote recovery keeps only SHEIN API probing, task-state mutation, and callback/orchestration glue.
- SHEIN remote refresh/recovery probe helper now also lives in `internal/publishing/shein`; root `internal/listingkit` no longer issues on-way/record/inventory probe requests inline, and keeps only logging hooks, task-state mutation, and callback/orchestration glue.
- SHEIN remote refresh/recovery request wiring now routes through one shared root-side request object; `internal/listingkit` no longer keeps a refresh-only confirm-remote request DTO or ten-argument remote-status callback signatures across refresh/recovery wiring.
- SHEIN remote refresh execution now also routes through one shared root-side refresh request object; `internal/listingkit` temporal refresh and recovery no longer feed long positional argument lists into the shared remote-refresh executor.
- root submit action validation and unsupported-action error call sites now use `internal/listing/submission` directly; `service_submit_shared.go` no longer keeps duplicate submit-action or unused workflow request-id policy wrappers.
- SHEIN recovery-remote confirm state application now also routes through `internal/publishing/shein`; `internal/listingkit` recovery remote no longer keeps local wrappers for missing-supplier fallback or confirm-remote state mutation, and runner-owned event ordering remains separate from model updates.
- SHEIN submission refresh state now stores the shared remote-status request directly instead of mirroring its fields; `internal/listingkit` refresh flow no longer keeps a second copy of action/request/lookup/fallback/API state beside the shared request boundary.
- SHEIN refresh remote-fallback rule lookup now also routes through `internal/publishing/shein`; `internal/listingkit` refresh service no longer imports marketplace publishing policy directly just to override refresh confirm-remote fallback messaging.
- SHEIN refresh selection/request assembly now also reuses `internal/publishing/shein` snapshot types directly; `internal/listingkit` no longer mirrors refresh action/record/supplier-code or trimmed request-id/remote-input payload fields in local compatibility DTOs before state assembly.
- SHEIN remote-status request construction now also routes through shared root-side builders; `internal/listingkit` refresh-state assembly, recovery remote-refresh orchestration, and task-id cloning no longer hand-compose parallel `sheinRemoteStatusRequest` field groups at each callsite.
- SHEIN remote refresh execution now also reuses one shared root-side execution state; `internal/listingkit` recovery and Temporal refresh no longer hand-assemble separate supplier-code plus refresh-started-at request wrappers before invoking shared remote refresh orchestration.
- SHEIN remote refresh orchestration now also routes through `internal/listing/submission`; `internal/listingkit` recovery and Temporal refresh keep only phase/request/event/persistence callbacks while the generic persist-phase, execute, record-event, and finish dispatch skeleton moved out.
- SHEIN recovered submission route dispatch now also routes through `internal/listing/submission`; `internal/listingkit` keeps only the accepted-response predicate plus local/remote handlers while the generic local-vs-remote recovery branch skeleton moved out.
- SHEIN recovered submit lease-acquire dispatch now also routes through `internal/listing/submission`; `internal/listingkit` keeps only begin-lease, replay-preview, remote-recovery, and blocked-error adapters while the generic sentinel-error branch skeleton moved out.
- SHEIN workflow-start failure cleanup now also routes through `internal/listing/submission`; `internal/listingkit` keeps only failure-record and lease-clear callbacks while the generic record-failure, clear-lease, and returned-error priority skeleton moved out.
- SHEIN recovered remote-recovery routing now also runs on one shared root-side state object; `internal/listingkit` recovery flow no longer threads task/package/action/request/response fields separately across local recovery, remote confirmation, and success/failure completion paths.
- SHEIN remote confirmation/refresh success and failure tails now also reuse one shared root-side completion support layer; `internal/listingkit` recovery and Temporal refresh no longer hand-assemble duplicate complete/fail plus remember/persist-success/save-result sequences after remote confirmation.
- SHEIN Temporal publish success/failure entrypoints now also reuse one shared root-side persistence state; `internal/listingkit` Temporal persistence no longer loads task/package and reapplies supplier-code/response/snapshot input separately across success and failure paths before routing into persistence tails.
- SHEIN Temporal persistence state now also narrows to completion plus failure-tail fields after publishing-side persistence input application; `internal/listingkit` no longer keeps unused supplier-code or snapshot fields on that root-side state shell once model mutation has already been applied.
- SHEIN Temporal payload/remote-submit flow now also reuses one shared root-side execution state; `internal/listingkit` Temporal flow no longer reloads task/package and rebuilds payload-stage or remote-submit input context separately across prepare/upload/prevalidate/submit-remote entrypoints.
- SHEIN Temporal flow support now also drops prepared-state payload-stage forwarding wrappers and the dead execution-state guard helper; `internal/listingkit` no longer keeps single-hop support helpers once the execution-state stage-context builder is the only real seam still consumed.
- SHEIN Temporal upload entrypoint now also delegates directly into flow service; `internal/listingkit` no longer keeps prepared-payload validation or no-upload branching in a root compatibility shell once flow already owns that decision seam.
- direct-submit and Temporal persistence owners now also wire `submissiondomain.NewResultPersistenceService(...)` directly; `internal/listingkit` no longer keeps a separate result-persistence support file just to map generic result input into success/failure inputs for those two owner services.
- SHEIN Temporal readiness/payload preparation now also reuses one shared root-side prepared publish state; `internal/listingkit` Temporal lifecycle and prepare-payload entrypoints no longer rebuild activity request plus submit-package normalization separately before readiness gates or payload-stage entry.
- SHEIN Temporal upload/prevalidate/submit-remote continuation now also reuses one shared root-side prepared-payload resume state; `internal/listingkit` Temporal flow no longer re-validates payload shape, reloads task/package, and rebuilds payload-stage context separately at each resumed prepared-payload entrypoint.
- SHEIN Temporal service entrypoints now also delegate straight to lifecycle, flow, persistence, and refresh owners; `internal/listingkit` service no longer routes publish workflow calls through an extra aggregate Temporal facade just to forward into those same four collaborators.
- SHEIN Temporal collaborator config assembly now also converges on one shared root-side wiring bundle; `internal/listingkit` service wiring no longer rebuilds the same submission assembly plus orchestrator binding set separately in each Temporal lifecycle/flow/persistence/refresh config builder.
- SHEIN Temporal ensure wiring now also resolves one collaborator bundle before assignment; `internal/listingkit` service no longer hand-orders persistence/lifecycle/flow/refresh construction inside the ensure seam itself.
- SHEIN direct-submit and refresh config assembly now also converges on one shared root-side managed-submission wiring bundle; `internal/listingkit` service wiring no longer rebuilds the same submission assembly plus callback set in those builders, while recovery config stays as the constructor stop-line because it participates in the orchestrator's own recovery dependency.
- SHEIN managed-submission ensure wiring now also resolves one collaborator bundle before assignment; `internal/listingkit` service no longer hand-orders recovery/direct/refresh/submission construction inside the ensure seam itself.
- submission core ensure wiring now also resolves one collaborator bundle before assignment; `internal/listingkit` service no longer hand-orders execution/state construction inside the ensure seam itself.
- submission task-recovery ensure wiring now also resolves one collaborator bundle before assignment; `internal/listingkit` service no longer hand-orders recovery/requeue construction inside the ensure seam itself.
- SHEIN submission workflow-status derivation plus latest-outcome/primary-record selection now live in `internal/publishing/shein`; root `internal/listingkit` submission projection keeps readiness fallback wiring and DTO assembly, but no longer interprets submission event/report precedence inline.
- remaining SHEIN submission projection state merge for latest status/error and remote summary now also lives in `internal/publishing/shein`; root `internal/listingkit` no longer reads submission-state fallback branches or remote-record checked-at precedence inline when shaping task-list DTO fields.
- SHEIN action-aware response acceptance now lives in `internal/publishing/shein`; `internal/listingkit/submission` no longer keeps a save-draft success compatibility helper for recovered submit routing.
- `internal/listingkit/submission` event helpers no longer keep unused response-error or record-draft compatibility wrappers; direct response-error policy stays in `internal/listing/submission`, while the adapter package keeps only active SHEIN event DTO assembly.
- SHEIN submission event history mutation now lives in `internal/publishing/shein`; root `internal/listingkit` no longer imports `internal/listingkit/submission` just to append events, and the adapter package no longer re-exports event-history append logic.
- SHEIN refresh confirm-remote running-event assembly and event-backed confirm-remote application now live in `internal/publishing/shein`; `task_submission_refresh_mutation.go` no longer depends on `internal/listingkit/submission` for those model-layer mutations.
- SHEIN confirm-remote update construction now also lives in `internal/publishing/shein`; `task_submission_recovery_remote.go` no longer depends on `internal/listingkit/submission`, and the old `submission/confirm_remote.go` compatibility shell has been removed.
- sensitive-word retry phase-event assembly no longer routes through `internal/listingkit/submission`; `shein_submit_retry.go` now uses the generic phase-event draft policy directly because it only needs a custom retry detail message, not the full SHEIN event adapter surface.
- SHEIN submission event DTO assembly now lives in `internal/publishing/shein`; direct lifecycle, lease, recovery-test, and Temporal persistence callsites no longer import `internal/listingkit/submission` just to build attempt/phase/confirm-remote events, while `internal/listingkit/submission` keeps transition composition helpers.
- SHEIN lease-start event assembly no longer routes through `internal/listingkit/submission`; `task_submission_recovery_lease.go` now reuses the root `beginSheinSubmitAttempt(...)` transition helper plus `internal/publishing/shein` event DTO construction directly.
- SHEIN submission report initialization, action-slot record selection, completed-record lookup, success checks, and in-flight clearing helpers now live in `internal/publishing/shein`; `internal/listingkit/submission` keeps transition assembly while pure model helpers move down to the compatibility/model layer.
- SHEIN submission report/record mutation helpers now live in `internal/publishing/shein`; `internal/listingkit/submission` keeps transition orchestration while direct supplier-code, remote-response, submit-snapshot, and remote-record mutations no longer live in the adapter layer.
- SHEIN active submission attempt and remote-recovery state checks now live in `internal/publishing/shein`; `internal/listingkit/submission` keeps transition assembly while stale-attempt and recoverability checks no longer live in the adapter layer.
- `internal/listingkit/submission` no longer re-exports those pure SHEIN report/record query or mutation helpers; root `internal/listingkit` callsites now use `internal/publishing/shein` directly, leaving the adapter package focused on transition assembly, event shaping, and confirm-remote glue.
- SHEIN transition-plus-event composition no longer leaks into recovery or state persistence services; those callsites now route through `shein_submit_state.go`, so direct production imports of `internal/listingkit/submission` are reduced to the single SHEIN state adapter entrypoint.
- obsolete `internal/listingkit/submission/transitions.go` compatibility exports have been removed; failure-state resolution now happens at the `shein_submit_state.go` entrypoint, while `internal/listingkit/submission` itself stays focused on pure SHEIN transition state mutation.
- SHEIN submission in-flight state projection, action-record slot matching, and attempt-record reuse helpers now live in `internal/publishing/shein`; `internal/listingkit/submission/state.go` keeps transition sequencing while no longer owning the repeated report/record slot plumbing.
- SHEIN submission response-outcome mapping and attempt finalize field assignment now live in `internal/publishing/shein`; `internal/listingkit/submission/state.go` resolves generic finalize state, then delegates pure record mutation back to the model layer.
- SHEIN running-attempt record construction and attempt-seed record assembly now also live in `internal/publishing/shein`; `internal/listingkit/submission/state.go` no longer instantiates `SubmissionRecord` inline during begin/reuse flows.
- `internal/listingkit/submission/state.go` is now treated as a stop-line transition sequencer. Guard tests should fail if pure SHEIN record literals or local `ResponseOutcome` shaping reappear there.
- obsolete generic compatibility files for source-facts readiness, enqueue retry, submit lock, and submit-in-progress error have been deleted from `internal/listingkit/submission`; the package surface now tracks its actual remaining SHEIN-only responsibility more closely.
- empty historical subdirectories under `internal/listingkit/submission/` have also been removed; the package is now physically flat around `doc.go`, `state.go`, and boundary/state tests, matching its reduced ownership.
- SHEIN remote submit error shaping now reuses `internal/publishing/shein.SubmissionResponseOutcome(...)`; root `internal/listingkit` no longer keeps duplicate response-outcome mappers in remote-attempt or sensitive-word retry paths.
- SHEIN remote-response-persisted checks now live in `internal/publishing/shein`; recovery lease orchestration no longer keeps that report/record query inline in `internal/listingkit`.
- SHEIN confirmed remote-check response shaping now lives in `internal/publishing/shein`; Temporal persistence no longer keeps the action-aware confirmed-response DTO helper in root `internal/listingkit`.
- SHEIN submit started-at and response lookup queries now live in `internal/publishing/shein`; Temporal persistence no longer keeps those package/report read helpers inline in root `internal/listingkit`.
- SHEIN refresh action/record/supplier-code selection now lives in `internal/publishing/shein`; root `internal/listingkit` no longer keeps that package/report projection inline before refresh orchestration.
- SHEIN refresh action/request match queries now live in `internal/publishing/shein`; root `internal/listingkit` keeps only changed-state error translation during refresh mutation guards.
- SHEIN submission-state availability/canonicalization query for refresh flows now lives in `internal/publishing/shein`; root `internal/listingkit` no longer duplicates `NormalizePackageSemanticFields + SubmissionState != nil` checks before refresh selection/mutation error translation.
- SHEIN lease remote-recovery query now lives in `internal/publishing/shein`; root `internal/listingkit` no longer decides inline whether same-request non-remote phases, persisted remote responses, or stale submit-remote attempts require remote recovery before lease replay handling.
- SHEIN preview-payload availability/canonicalization query now lives in `internal/publishing/shein`; recovery-lease and Temporal submit loaders no longer duplicate `NormalizePackageSemanticFields + PreviewPayload != nil` checks before translating missing-package errors in `internal/listingkit`.
- submit target normalization and started-request replay predicates now live in `internal/listing/submission`; root `internal/listingkit` keeps only SHEIN-specific unsupported-platform/action error translation around that generic policy seam.
- submit request-id normalization and workflow request-id derivation now live in `internal/listing/submission`; root `internal/listingkit` keeps only `SubmitTaskRequest` field adaptation before routing into that generic policy seam.
- submit attempt planning now lives in `internal/listing/submission`; root `internal/listingkit` keeps only `SubmitTaskRequest`/task attachment adaptation while request-id resolution, workflow-start request-id derivation, and use-workflow skeleton moved out.
- supported submit-action classification now lives in `internal/listing/submission`; root `internal/listingkit` keeps only platform-facing unsupported-action error wording and remote submit switch handling.
- started-workflow replay classification now lives in `internal/listing/submission`; root temporal lifecycle code calls the domain replay policy directly instead of keeping a submit-target wrapper.
- preferred submit-action selection now lives in `internal/listing/submission`; root `internal/listingkit` keeps only SHEIN task/settings adaptation before choosing draft-mode or settings fallback action candidates.
- source-facts readiness classification now lives in `internal/listing/submission`; root readiness checks pass SHEIN package metadata directly instead of keeping a source-facts wrapper in status support.
- submit readiness gate skeleton now lives in `internal/listing/submission`; root `internal/listingkit` keeps only SHEIN readiness snapshot/freshness adapters while base blocked-message selection and freshness gate sequencing moved out.
- result-persistence success/failure input mapping now lives in `internal/listing/submission`; root `internal/listingkit` direct and Temporal persistence owners keep only SHEIN-specific fallback/persistence callbacks while common ResultPersistenceInput-to-success/failure shaping moved out.
- SHEIN workflow-start failure record mutation now lives in `internal/publishing/shein`; recovery-lease cleanup no longer rewrites failed submission record fields inline in root `internal/listingkit`, and submission-state lease loading now reuses the shared shein submission-state availability query.
- recovered SHEIN submission-state loading now reuses the shared shein submission-state availability query, and recovery-state response fallback now reuses `internal/publishing/shein.SubmissionResponseForAction(...)`; root `internal/listingkit` no longer opens report/record fallback branches inline for that recovery path.
- recovered SHEIN submission-state projection now lives in `internal/publishing/shein`; root `internal/listingkit` no longer assembles report/record/request-id/response fallback tuples inline before recovery orchestration, and keeps only the orchestration-local timestamp wrapper.
- Temporal remote-refresh state projection now lives in `internal/publishing/shein`; root `internal/listingkit` no longer assembles started-at, response fallback, and remote-status tuples inline across Temporal refresh entry and success-result shaping.
- Temporal persistence input mutation now lives in `internal/publishing/shein`; root `internal/listingkit` no longer batches snapshot, supplier-code, and remote-response writes inline before Temporal success/failure persistence routing.
- Temporal and direct submit result-tail routing now also reuse the shared submission result-persistence runner; root `internal/listingkit` no longer duplicates success/failure dispatch scaffolding before delegating to SHEIN-specific state/event fallbacks.
- Temporal success/failure persistence support now routes attempt transition-plus-event assembly through `shein_submit_state.go`; fallback success and remote-refresh completion/failure paths no longer hand-compose duplicate complete/fail + event sequences inline in root `internal/listingkit`.
- the last production `internal/listingkit/submission` transition sequencer has now been folded into `shein_submit_state.go`; the compatibility package has been removed from production code entirely, and the boundary guard now sits on the root state-entry file instead.
- SHEIN submit payload attribute-readiness checks and sale-attribute repair now live in `internal/publishing/shein`; root `internal/listingkit` submission execution keeps only submit-request normalization, platform-facing unsupported-action wording, and orchestration glue around those payload helpers.
- action-aware SHEIN submission response acceptance now lives in `internal/marketplace/shein/publishing`; `internal/publishing/shein` keeps only `SubmissionResponse` field adaptation before delegating publish/save-draft response interpretation.
- SHEIN submit phase default detail wording now lives in `internal/marketplace/shein/publishing`; `internal/publishing/shein` keeps only event DTO assembly and delegates publish/save-draft phase wording to the marketplace publishing policy.
- SHEIN remote-confirmation SPU-name precedence now lives in `internal/marketplace/shein/publishing`; `internal/publishing/shein` keeps only the `SubmissionRemoteResolution` adapter wrapper before delegating on-way/record/fallback SPU selection.
- SHEIN remote-confirmation update-message selection now lives in `internal/marketplace/shein/publishing`; `internal/publishing/shein` keeps only confirm-remote update/event DTO construction while record-query-error and record-not-found message selection is delegated.
- SHEIN submission projection workflow-status mapping now lives in `internal/marketplace/shein/publishing`; `internal/publishing/shein` keeps only event/report model adaptation before invoking the generic submission projection engine.
- SHEIN submission refresh fallback-message selection now calls `internal/marketplace/shein/publishing` directly from root refresh orchestration; the obsolete `internal/publishing/shein` refresh-fallback wrapper has been removed.
- SHEIN remote-confirmation SPU-name selection now calls `internal/marketplace/shein/publishing` directly from confirm-update construction; the obsolete internal `RemoteResolutionSPUName` wrapper has been removed from `internal/publishing/shein`.
- the obsolete actionless `internal/publishing/shein.SubmissionResponseAccepted` wrapper has been removed; the remaining action-aware response acceptance entrypoint stays only as `SubmissionResponse` field adaptation before marketplace policy evaluation.
- the obsolete internal `internal/publishing/shein.SubmissionRecordResult` getter has been removed; remote publish-accepted adaptation now reads the selected submission record response directly before calling marketplace policy.
- the obsolete private `internal/publishing/shein.submissionPhaseDetail` forwarding wrapper has been removed; phase-event assembly now calls the marketplace publishing phase-detail policy directly.
- submit base/support wiring now reuses the submission assembly's repository and runtime resolver snapshots; managed and Temporal collaborator wiring flow through the shared assembly completion path, `buildTaskSubmissionSupportWiringWithAssembly` no longer rebuilds support dependencies before overriding them, and core collaborator wiring is explicitly kept off the base assembly path because base assembly bindings resolve core collaborators.
- retryable failure reason-code and default task recovery-scope ownership now live in `internal/listing/submission`; root `internal/listingkit` references those submission-domain metadata constants directly instead of keeping root-side retry metadata aliases.
- SHEIN task-list projection now reuses one shared submission projection snapshot for status and remote summary fields, so `internal/listingkit` no longer rebuilds the same normalized package/readiness/projection state twice in one task-list item assembly.
- SHEIN task-list readiness state now also reuses one shared readiness projection snapshot for blocker keys, warning keys, and status overview fields, so the task-list item assembly no longer rebuilds identical readiness/checklist/status state through three helper paths.
- SHEIN revision success apply/restore result assembly now reuses one shared readiness projection snapshot for status summary and follow-up checklist data, while standalone status/checklist helpers remain as compatibility delegates.
- SHEIN preview payload entrypoints now share one preview-input assembly helper, so direct preview and result-backed preview construction no longer duplicate readiness projection, repair-center, and workspace-overview wiring.
- SHEIN task-list submission projection now accepts the already-computed readiness result, so task-list item assembly no longer reruns readiness solely to decide ready-vs-pending workflow status.
- asset-generation preview/export decoration now applies the shared generation projection through dedicated preview/export adapters, so field assignment no longer drifts separately across read surfaces.
- ListingKit preview/export read-surface projection now applies DTO fields through dedicated preview/export adapters, so projection-to-legacy-field mapping no longer lives inline in the read entrypoints.

Completed sourcing slices:

- `SourceIdentity` and normalized `SourceRequest` fields now live in `internal/product/sourcing`, with Amazon crawl request planning consuming that normalization.
- Amazon batch result alignment now lives in `internal/product/sourcing`, preserving source identity for each requested product ID.
- Amazon source batch fetch now guards configured sources only when execution is required, while empty batches stay side-effect free.
- 1688 URL/result identity normalization now lives in `internal/product/sourcing`, while crawler execution remains in `internal/integration/crawler/a1688` and legacy `internal/crawler/alibaba1688`.
- 1688 scraped-data normalization now trims and drops empty specs/details, falls back to title when details are blank, and normalizes image lists before enrichment handoff.
- crawler integration packages now have a boundary guard that prevents dependencies on `listingkit`, marketplace/workspace/publishing packages, or `product/sourcing`.

Current sourcing stop line:

- do not keep shaving individual crawler field cleanup unless it prevents real downstream identity, enrichment, or catalog pollution; prefer the next structural seam over more one-off source cleanup.

## Verification Matrix

Use this focused matrix after edits in this boundary area:

```powershell
go test ./internal/listing/studio
go test ./internal/listing/preview
go test ./internal/listingkit
go test ./internal/marketplace/shein/publishing
go test ./internal/marketplace/shein/workspace
go test ./internal/workspace/shein
```

Latest checkpoint verification:

```powershell
go test ./internal/listing/studio ./internal/listing/preview ./internal/product/sourcing ./internal/marketplace/shein/publishing ./internal/marketplace/shein/workspace ./internal/workspace/shein
go test ./internal/listingkit -run 'Test.*Boundary|Test.*Guard|Test.*Preview|Test.*Studio|Test.*Source|Test.*Crawler'
```

For narrower iterations, use package-specific `-run` filters, but always rerun the affected package without a filter before claiming the package is fully verified.

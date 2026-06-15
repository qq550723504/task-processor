# ListingKit Refactoring Plan

> Status: active only as a ListingKit-specific supplement. For architecture authority and implementation order, follow [`project-wide-refactoring-plan.md`](./project-wide-refactoring-plan.md), [`project-wide-execution-plan.md`](./project-wide-execution-plan.md), and [`listingkit-boundary-checkpoint.md`](./listingkit-boundary-checkpoint.md) first.

## 1. Purpose

This document narrows the project-wide refactoring program down to the `internal/listingkit` area.

It should help us:

- reduce root `internal/listingkit` complexity,
- keep `listingkit` focused on orchestration and compatibility,
- move platform-specific behavior out of the root package,
- sequence small, testable, behavior-preserving PRs.

It should not be treated as a competing source of truth.

## 2. Current Position

Observed in the local workspace on 2026-06-09:

Recent boundary checkpoint:

- see [`listingkit-boundary-checkpoint.md`](./listingkit-boundary-checkpoint.md) for the current studio/preview/SHEIN publishing guard state and the recommended next direction toward `product/sourcing`.

| Metric | Current snapshot |
| --- | ---: |
| Root `internal/listingkit` Go files excluding tests | 304 |
| Root `internal/listingkit` Go files including tests | 512 |
| `internal/listingkit/core` | already exists |
| `internal/listingkit/service` | already exists |
| `internal/listingkit/submission` | retired from production code; remaining SHEIN transition sequencing now sits directly in `shein_submit_state.go` |

Implications:

- Older goals based on root-file-count `532` are no longer accurate.
- Early extraction work has already started, so future work should continue from the current package shape rather than recreate it.
- The next meaningful work is boundary tightening, preview modularization, submission consolidation, and service slimming, not broad directory creation.

## 3. ListingKit Target Role

Per the project-wide boundary rules, `internal/listingkit` should converge toward:

- task lifecycle and orchestration,
- workflow entrypoints,
- request normalization,
- persistence coordination,
- preview and export aggregation,
- revision and history facade behavior,
- API-facing shell models,
- cross-platform listing task concepts.

Avoid adding new long-lived business rules here when they belong elsewhere, especially:

- SHEIN category, attribute, pricing, or publishing rules,
- SHEIN workspace, repair, editor, and revision UX rules,
- new marketplace-specific behavior that can live in marketplace-owned packages,
- concrete infrastructure client behavior that should sit behind interfaces.

## 4. Non-goals

The following are out of scope for the first-pass ListingKit refactor:

- broad package-tree renames,
- microservice extraction,
- combining feature delivery with file moves,
- moving files solely to satisfy arbitrary file-count targets,
- promoting advisory dependency checks to CI before legacy exceptions are documented.

## 5. Required Baseline Before More Moves

Before starting the next substantial code move, use the same baseline flow defined by the project-wide execution plan.

Commands:

```powershell
./scripts/analyze-project-deps.ps1 6>&1 | Tee-Object -FilePath docs/refactoring/dependency-baseline-output.txt
go test ./internal/listingkit/... -count=1
go test ./internal/app/httpapi/... -count=1
go test ./... -count=1
```

Then update:

- [`dependency-baseline.md`](./dependency-baseline.md)

Minimum baseline fields to refresh:

- root `internal/listingkit` Go file count,
- largest ListingKit files,
- packages importing `internal/listingkit*`,
- advisory boundary violations,
- known legacy exceptions,
- unstable or slow test packages that affect refactoring cadence.

If `go test ./...` is too slow or flaky, record that explicitly and use focused test commands per PR.

## 6. Execution Principles

All ListingKit refactoring work should follow these rules:

1. Keep each PR behavior-preserving unless the PR is explicitly a feature change.
2. Keep one primary purpose per PR.
3. Prefer extracting internal file groups before forcing real subpackage moves.
4. Stop when ownership becomes unclear and document the ambiguity.
5. Keep rollback simple by limiting cross-package movement per PR.
6. Add tests before moves when semantics are not already locked.

## 7. Phase Alignment

This document follows the same order as the project-wide execution plan.

### Phase 0: Baseline and Guardrails

Goal:

- make ListingKit pressure measurable before further moves.

Preferred outputs:

- [`dependency-baseline.md`](./dependency-baseline.md)
- `docs/refactoring/dependency-baseline-output.txt`
- optional focused test baseline notes for ListingKit-heavy packages

Acceptance criteria:

- baseline is filled with real data,
- known exceptions are documented,
- no behavior changes are mixed into the baseline update.

Stop conditions:

- if the dependency scan reveals unclear ownership,
- if test instability makes later PR validation unreliable.

### Phase 1: Preview First Cut

Goal:

- reduce hardcoded platform branching inside preview assembly without changing API behavior.

Recommended work slices:

1. keep `preview_builder.go` thin and delegate to smaller helpers,
2. split per-platform preview assembly into dedicated file-group helpers,
3. add targeted tests for selected-platform semantics and missing payload cases.

Candidate files:

- `internal/listingkit/preview_builder.go`
- `internal/listingkit/preview_header.go`
- `internal/listingkit/preview_platform_sections.go`
- `internal/listingkit/preview_builder_shein.go`
- preview-related tests

Acceptance criteria:

- central preview entrypoint becomes shorter and easier to review,
- platform-specific preview logic is no longer mixed together in one large function,
- preview behavior remains unchanged,
- focused preview tests pass.

Stop conditions:

- if helper extraction starts requiring broad model churn,
- if a real subpackage move would create import cycles before interfaces are ready.

### Phase 2: Preview Package Extraction

Goal:

- group preview aggregation into a bounded module or file cluster without forcing premature package churn.

Recommended work slices:

1. finish internal preview file grouping while staying in `package listingkit` if necessary,
2. introduce package-private adapter-like interfaces where they reduce central branching,
3. evaluate whether a real `preview` subpackage is viable without cycles.

Acceptance criteria:

- logical ownership of preview code is clearer,
- import pressure is lower or at least better understood,
- no cycles are introduced,
- public behavior is unchanged.

Decision rule:

- if package extraction increases coupling or requires widespread model duplication, keep the preview grouping inside `package listingkit` and record that decision.

### Phase 3: Submission Consolidation

Goal:

- gather submit, retry, recovery, lock, and Temporal-adjacent coordination into a tighter submission-oriented surface.

Current checkpoint:

- `internal/listing/submission` now owns nine small generic orchestration seams:
  - refresh status (`RefreshStatus`)
  - task requeue (`RequeueTasks`)
  - immediate recovery (`RecoverNow`)
  - batch recovery (`RecoverBatch`)
  - direct-submit phase flow (`DirectSubmit`)
  - prepared-payload stage flow (`Prepare/Upload/PreValidate`)
  - remote-submit attempt flow (`prepare state -> execute attempt -> shape result`)
  - post-success persistence flow (`persist result/phase -> complete attempt -> remember -> persist success`)
  - failure-record persistence flow (`record failure event/state`)
- `internal/listingkit` submit/recovery services remain compatibility adapters that still own ListingKit DTO mapping, repository contracts, and root retryable-block persistence/rollback hooks.
- pricing cache support is now split by helper family, so root `listingkit` keeps submission-facing cache entrypoints while cache-key/SKU fact helpers and review/logging helpers live in dedicated support files.
- SHEIN submit SKU normalization support is now split by helper family, so root `listingkit` keeps the normalization entrypoint while variant-matching/base-SKU helpers and pricing/style alias helpers live in dedicated support files.
- manual sale-attribute revision support is now split by helper family, so root `listingkit` keeps the revision resolution entrypoints while assignment/backfill helpers and source-value/compare helpers live in dedicated support files.
- `internal/listingkit` direct submit now delegates phase sequencing to the submission-domain runner while still owning SHEIN readiness gates, state persistence hooks, and remote-submit error semantics.
- Temporal payload preparation/upload/pre-validate steps now also delegate to a submission-domain payload-stage runner.
- direct submit and Temporal now also share a submission-domain remote-submit attempt runner while keeping post-attempt persistence semantics separate.
- direct submit and Temporal success tails now also share a submission-domain success-persistence runner, while failure-return semantics remain adapter-specific.
- direct submit and Temporal failure recording now also share a submission-domain failure-persistence runner, while error-return contracts remain adapter-specific.
- remote refresh/recovery confirmation models now reuse `internal/publishing/shein.SubmissionConfirmRemoteUpdate`, so root `listingkit` no longer maintains a parallel confirm-remote DTO just to bridge refresh and recovery flows.
- remote refresh/recovery confirm-remote branch resolution now also reuses a pure `internal/publishing/shein` helper, so root `listingkit` no longer assembles on-way/record/inventory/fallback confirm-remote updates inline after SHEIN API probes.
- remote refresh/recovery probe execution now also reuses a `internal/publishing/shein` helper, so root `listingkit` no longer builds record-query requests or inventory/on-way probes inline before confirm-remote resolution.
- remote refresh/recovery callback wiring now shares one request object across refresh and recovery, so root `listingkit` no longer maintains a refresh-specific confirm-remote request DTO or long positional callback signatures for remote-status resolution.
- remote refresh execution now also shares one request object across temporal refresh and recovery, so root `listingkit` no longer maintains a long positional executor signature for shared remote-refresh orchestration.
- refresh runtime state now also embeds the shared remote-status request boundary, so root `listingkit` no longer maintains duplicate refresh-state fields for action/request/lookup/fallback/API inputs beside the shared request model.
- retryable failure classification plus blocked-task reblock policy now also live in `internal/listing/submission`, so root `listingkit` no longer owns generic reason-code matching or next-retry scheduling decisions for blocked recovery and failed-task backfill.
- retryable failure persistence branching plus failed-task backfill block construction now also route through `internal/listing/submission`, so root `listingkit` no longer decides blocked-vs-failed persistence or assembles backfill retry metadata inline before repository writes.
- failed-task retryable backfill orchestration now also routes through an `internal/listing/submission` runner, so root `listingkit` no longer owns failed-task iteration, created-after filtering, or retryable backfill control flow beyond repository callbacks.
- submit result persistence dispatch now also routes through an `internal/listing/submission` runner, so root `listingkit` no longer open-codes success-vs-failure branching, original-error return policy, or fallback tail persistence separately across direct-submit finish and Temporal persistence entrypoints.
- submit collaborator config assembly now also lives directly beside shared root-side submitter/support/assembly bundles within each ensure path, and the remaining wiring surface is split into shared, managed, and Temporal support files, so root `listingkit` no longer rebuilds the same dependency snapshots or funnel every submit collaborator constructor through one catch-all wiring file.
- remote-status request construction now also shares root-side builders across refresh-state assembly, recovery remote-refresh orchestration, and task-id cloning, so root `listingkit` no longer hand-composes parallel `sheinRemoteStatusRequest` field groups at each seam.
- remote refresh execution now also shares one root-side execution state across recovery and Temporal refresh, so root `listingkit` no longer hand-assembles separate supplier-code plus refresh-started-at wrappers before invoking shared remote refresh orchestration.
- remote refresh orchestration now also routes through an `internal/listing/submission` runner, so root `listingkit` no longer open-codes the persist-phase, execute-remote, append-event, and success-vs-failure finish skeleton separately across recovered remote confirmation and Temporal refresh entrypoints.
- recovered submission route dispatch now also routes through an `internal/listing/submission` runner, so root `listingkit` no longer open-codes accepted-response local-completion vs remote-confirmation branching inside the recovered submit path.
- recovered submit lease-acquire dispatch now also routes through an `internal/listing/submission` runner, so root `listingkit` no longer open-codes replay-preview vs remote-recovery vs blocked-missing-package branching after lease acquisition.
- task submission recovery support is now split by concern, so the compatibility adapter keeps runner wiring and local completion flow while recovered-remote helpers and workflow-start-failure helpers live in dedicated support files.
- workflow-start failure cleanup now also routes through an `internal/listing/submission` runner, so root `listingkit` no longer open-codes failure-record persistence, lease cleanup, and returned-error priority resolution after publish workflow start fails.
- recovered remote-recovery routing now also shares one root-side state boundary across route selection, local completion, remote confirmation, and success/failure finish paths, so root `listingkit` no longer threads task/package/action/request/response fields separately through that recovery chain.
- remote confirmation/refresh success and failure tails now also share one root-side completion support layer across recovery and Temporal refresh, so root `listingkit` no longer hand-assembles duplicate complete/fail plus remember/persist-success/save-result sequences after remote confirmation.
- Temporal publish success/failure entrypoints now also share one root-side persistence state across task load, persistence-input application, and tail routing, so root `listingkit` no longer duplicates task/package load plus supplier-code/response/snapshot input application before success vs failure persistence paths.
- Temporal payload/remote-submit flow now also shares one root-side execution state across prepare/upload/prevalidate/submit-remote entrypoints, so root `listingkit` no longer reloads task/package and rebuilds payload-stage or remote-submit input context separately at each flow step.
- Temporal readiness/payload preparation now also shares one root-side prepared publish state across readiness validation and payload preparation entrypoints, so root `listingkit` no longer rebuilds activity request plus submit-package normalization separately before readiness gates or payload-stage entry.
- SHEIN submit readiness home construction now retains only the root builder, guidance-resolver seam, and summary-shaping seam, while readiness types, guidance helpers, and image/price/status predicates live in dedicated support files, so root `listingkit` no longer mixes all submit-readiness helper families into one broad home file.
- SHEIN submit payload home construction now retains only the root submit-product assembly plus collection/extra/transport shims, while site/SKU defaults, image normalization, and supplier-code plus payload-validation helpers live in dedicated support files, so root `listingkit` no longer mixes every submit-payload helper family into one broad home file.
- generation conditional state home construction now retains only the root conditional/apply seams, while response descriptor builders and panel-update merge/minimize helpers live in dedicated support files, so root `listingkit` no longer mixes every conditional projection helper family into one broad state file.
- generation review session home construction now retains only the root session assembly seam, while section/review-state helpers and slot/selection/context helpers live in dedicated support files, so root `listingkit` no longer mixes every review-session helper family into one broad sections file.
- workflow SDS sync home construction now retains only the root SDS sync orchestration and fallback seams, while uploaded-image/local-file helpers and variant aggregation helpers live in dedicated support files, so root `listingkit` no longer mixes every SDS sync helper family into one broad workflow file.
- Temporal upload/prevalidate/submit-remote continuation now also shares one root-side prepared-payload resume state across resumed prepared-payload entrypoints, so root `listingkit` no longer re-validates payload shape, reloads task/package, and rebuilds payload-stage context separately at each continuation step.
- Temporal SHEIN service entrypoints now also delegate straight to lifecycle, payload flow, persistence, and remote-refresh owners, so root `listingkit` no longer inserts an aggregate Temporal facade between the service-entry seam and those four workflow collaborators.
- Temporal SHEIN collaborator config builders now also share one root-side wiring bundle across lifecycle, flow, persistence, and refresh service construction, so root `listingkit` no longer rebuilds the same submission assembly plus orchestrator wiring separately in each Temporal config builder.
- Temporal submit facade construction is now retired entirely, so root `listingkit` no longer hand-assembles a four-part Temporal aggregate inside the lazy collaborator accessor just to forward back into lifecycle, flow, persistence, and refresh owners.
- Temporal workflow collaborators now also initialize through one shared root-side ensure seam plus collaborator wiring bundle, so root `listingkit` no longer repeats lifecycle/flow/persistence/refresh lazy-construction steps across the workflow stage initializer and each Temporal accessor.
- Temporal workflow ensure wiring now also resolves one collaborator bundle before assignment, so root `listingkit` no longer hand-orders persistence/lifecycle/flow/refresh construction inside the ensure seam itself.
- direct submit and refresh config builders now also share one root-side managed-submission wiring bundle across shared assembly, callbacks, and orchestrator access, while recovery config remains a constructor stop-line because it participates in the orchestrator's own recovery dependency.
- managed submission collaborators now also initialize through one shared root-side ensure seam plus collaborator wiring bundle, so root `listingkit` no longer repeats recovery/direct/refresh/submission lazy-construction steps across the orchestrator-stage initializer and the managed-submission accessor set.
- managed submission ensure wiring now also resolves one collaborator bundle before assignment, so root `listingkit` no longer hand-orders recovery/direct/refresh/submission construction inside the ensure seam itself.
- submission service, execution, and state config builders now also share one root-side support wiring bundle across repository access, runtime resolver callbacks, pricing rule lookup, and remember-submitted hooks, so root `listingkit` no longer re-resolves those base submission dependencies separately across those builders.
- submission core ensure wiring now also resolves one collaborator bundle before assignment, so root `listingkit` no longer hand-orders execution/state construction inside the ensure seam itself.
- submission core collaborators now also initialize through one shared root-side ensure seam plus collaborator wiring bundle, so root `listingkit` no longer repeats execution/state lazy-construction steps across the state-stage initializer and the core accessor pair.
- submission task-recovery collaborators now also initialize through one shared root-side ensure seam plus collaborator wiring bundle, so root `listingkit` no longer repeats recovery/requeue lazy-construction steps across the task-recovery stage initializer and the task-recovery accessor pair.
- submission task-recovery ensure wiring now also resolves one collaborator bundle before assignment, so root `listingkit` no longer hand-orders recovery/requeue construction inside the ensure seam itself.
- requeue task-status eligibility policy now also lives in `internal/listing/submission`, so root `listingkit` no longer formats nil/non-pending requeue rejection reasons inline and keeps only its local pending-status mapping.
- studio batch-run service/coordinator/executor config builders now also share one root-side wiring bundle across batch-run repositories plus domain-runner assembly, so root `listingkit` no longer rebuilds the same batch-run repo pair and runner construction separately across those builders.
- studio session and batch-draft config builders now also share one root-side session wiring bundle across session repository and domain-runner assembly, so root `listingkit` no longer rebuilds the same studio-session runner set separately across those builders.
- studio batch service config builders now also share one root-side batch-service wiring bundle across batch/session repositories, graph-resume callbacks, and domain-runner assembly, so root `listingkit` no longer stores prebuilt batch detail/review runners directly in the config builder path.
- studio batch collaborators now also initialize through one shared root-side ensure seam plus collaborator wiring bundle, so root `listingkit` no longer repeats batch-generation/batch-service lazy-construction steps across the studio batch initializer and accessor set.
- studio session collaborators now also initialize through one shared root-side ensure seam plus collaborator wiring bundle, so root `listingkit` no longer repeats session/batch-draft/media lazy-construction steps across the studio session initializer and accessor set.
- studio batch-run collaborators now also initialize through one shared root-side ensure seam plus collaborator wiring bundle, so root `listingkit` no longer repeats batch-run/executor/coordinator lazy-construction steps across the studio batch initializer, accessor set, and recovery bootstrap path.
- The next preferred slice is to stop and reassess whether more submit extraction still shrinks `listingkit`, rather than automatically extracting every leftover helper.

### Phase 3.5: Studio Skeleton Extraction

Goal:

- establish `internal/listing/studio` with low-risk internal services before moving broader studio orchestration.

Current checkpoint:

- `internal/listing/studio` now owns its first real service skeleton:
  - studio batch-run service (`create/get/list/cancel`)
- `internal/listing/studio` now also owns a new read-only studio seam:
  - studio batch-detail read flow (`read graph -> fallback -> ensure graph -> project detail`)
- `internal/listing/studio` now also owns a small batch review seam:
  - studio batch review flow (`ensure batch -> replace reviews -> reload detail`)
- `internal/listing/studio` now also owns a second low-risk studio seam:
  - studio batch-draft read/delete flow (`gallery/list/get/delete`)
- `internal/listing/studio` now also owns a third low-risk studio seam:
  - studio session ensure/get flow (`ensure/get`)
- `internal/listing/studio` now also owns a fourth low-risk studio seam:
  - studio session async-job sync flow (`sync async job -> persist session state`)
- `internal/listing/studio` now also owns a fifth low-risk studio seam:
  - studio session generation-metadata patch flow (`status/job/error` metadata-only updates)
- `internal/listing/studio` now also owns a sixth low-risk studio seam:
  - studio session review/task-metadata patch flow (`approved_design_ids/created_tasks` metadata-only updates)
- `internal/listing/studio` now also owns a seventh low-risk studio seam:
  - studio session general-metadata patch flow (`load session -> apply adapter patch -> persist`)
- `internal/listing/studio` now also owns an eighth low-risk studio seam:
  - studio batch-run completion flow (`cancel unfinished items -> count item statuses -> resolve final run status`)
- `internal/listing/studio` now also owns a ninth low-risk studio seam:
  - studio batch retry-prepare flow (`load detail -> select retryable items -> reset items -> reload detail`)
- `internal/listing/studio` now also owns a tenth low-risk studio seam:
  - studio batch task-prepare flow (`persist pending task state -> mark batch creating -> reload detail result`)
- `internal/listing/studio` now also owns an eleventh low-risk studio seam:
  - studio batch task-resume finalize flow (`clear pending task state -> persist created/failed tasks -> mark batch done -> reload detail result`)
- `internal/listingkit` studio batch-run service is now a compatibility adapter that keeps API shell types, repository/session adapters, and error translation (`ErrStudioSessionNotFound`).
- `internal/listingkit` batch detail flow now delegates read/fallback/projection orchestration to `internal/listing/studio`, while keeping batch/session models, repositories, graph materialization implementation, and task-creation execution in the compatibility shell.
- `internal/listingkit` batch review flow now delegates approval/reload orchestration to `internal/listing/studio`, while keeping request normalization, repositories, and batch detail/task execution models in the compatibility shell.
- `internal/listingkit` batch draft service now delegates stable read/delete behavior to `internal/listing/studio`, while keeping `UpsertStudioBatch` and ListingKit-specific request shaping in the compatibility shell.
- `internal/listingkit` session service now delegates stable ensure/get behavior and async-job status synchronization to `internal/listing/studio`, while broader metadata patch/update semantics remain in the compatibility shell.
- `internal/listingkit` session service now also delegates pure generation-metadata updates to `internal/listing/studio`, while mixed-field update requests still remain in the compatibility shell.
- `internal/listingkit` session service now also delegates pure review/task metadata updates and mixed-field general metadata update orchestration to `internal/listing/studio`, while field assignment, expected-updated-at conflict checks, logging, and error translation stay in the compatibility shell.
- `internal/listingkit` batch-run executor now delegates completion bookkeeping rules to `internal/listing/studio`, while `executeOne`, generation resume, task creation, repository records, and the executor loop remain in the compatibility shell.
- `internal/listingkit` batch retry preparation now delegates detail-load/select/reset/reload orchestration to `internal/listing/studio`, while draft execution-config synchronization, retry continuation, repository/session models, and generation execution remain in the compatibility shell.
- `internal/listingkit` studio batch generation service now delegates start/prepare/resume/retry orchestration to `internal/listing/studio`, while graph refresh/resume helpers, repository/session adapters, generation execution, and task-creation semantics remain in the compatibility shell.
- studio batch generation helpers are now split by family, so root `listingkit` no longer mixes recovery/retry policy helpers and item-request/grouping projection helpers into the same generation home file.
- `internal/listingkit` studio batch task creation now also delegates resume orchestration to `internal/listing/studio`, while prepare-state assembly, approved-design validation, grouped-selection request shaping, existing-task reuse checks, and concrete ListingKit task creation semantics remain in the compatibility shell.
- `internal/listingkit` studio batch task creation execute flow now also delegates reuse/create/finalize orchestration to `internal/listing/studio`, while candidate preparation, approved-design validation, grouped-selection request shaping, and concrete ListingKit task creation payload semantics remain in the compatibility shell.
- studio batch task support helpers are now split by family, so root `listingkit` no longer mixes task-resume gating, existing-task reuse matching, and request/SDS payload shaping inside one broad task-creation support file.
- studio batch repository support is now split by implementation, so root `listingkit` keeps the shared repository contract/errors while in-memory and Gorm persistence live in dedicated repository files.
- studio batch-run repository support is now split by implementation, so root `listingkit` keeps run models/contracts plus scope helpers while in-memory and Gorm persistence live in dedicated repository files.
- generation overview support is now split by responsibility, so root `listingkit` keeps overview decisioning plus local filter-mutation rules while action target, impact, and preview-capability action helpers live in a dedicated support file.
- `internal/listingkit` batch task preparation now delegates pending-task state persistence plus result reload orchestration to `internal/listing/studio`, while design validation, session/batch loading, task creation execution, repository/session models, and resume continuation remain in the compatibility shell.
- `internal/listingkit` batch task resume now delegates completion-state persistence plus result reload orchestration to `internal/listing/studio`, while pending-design selection, session/batch loading, task creation execution, repository/session models, and resume entrypoint control stay in the compatibility shell.
- `internal/listing/studio` now has a boundary guard that keeps it independent from `internal/listingkit`, SHEIN marketplace/workspace/publishing packages, and root runtime/integration wiring.

Recommended work slices:

1. continue with small studio services that already look like `load/check/build result` flows,
2. keep listingkit-owned repositories and shell models behind adapters until multiple studio services settle,
3. avoid moving batch-generation orchestration until batch-run/session seams are proven.

Acceptance criteria:

- `internal/listing/studio` owns at least one real service, not just a placeholder README,
- `listingkit` service/root collaborators get thinner through adapters rather than copied orchestration,
- existing studio API and repository behavior remain unchanged.

Stop conditions:

- if extraction requires copying large listingkit model trees into the new package,
- if batch executor/coordinator behavior starts leaking into the first studio skeleton.

Recommended work slices:

1. inventory submit, retry, recovery, lock, and direct-submit flows,
2. reduce root service field sprawl behind a submission-focused facade or coordinator,
3. keep platform-specific submit rules outside the root package when possible,
4. expand tests around state transitions and lock behavior before moving sensitive logic.

Candidate areas:

- `internal/listing/submission/` for model-light generic mechanics, while root `shein_submit_state.go` remains the stop-line for the small amount of SHEIN transition sequencing still owned by ListingKit
- `internal/listingkit/task_requeue_service.go`
- task submission and retry services in root `listingkit`
- submission-related API handlers and tests

Acceptance criteria:

- root service has fewer direct submission dependencies,
- submit and retry flows remain behavior-compatible,
- lock semantics and state transitions are covered by focused tests,
- no platform-specific logic drifts back into generic ListingKit code.

Progress signal:

- if a new submission-domain seam only duplicates ListingKit logic without shrinking a root service or compatibility file, stop and regroup before adding more skeletons.

Stop conditions:

- if marketplace-specific submit rules cannot be separated cleanly yet,
- if Temporal-facing behavior and domain behavior are still too entangled to move safely.

### Phase 4: Service Object Slimming

Goal:

- reduce root ListingKit service constructor and dependency sprawl.

Recommended work slices:

1. identify clusters such as preview, submission, revision/history, and studio coordination,
2. introduce private facade structs for coherent clusters,
3. make service construction easier to read and test without changing public API.

Acceptance criteria:

- root service object has fewer direct fields,
- constructor wiring is clearer,
- tests still pass without broad fixture rewrites,
- ownership of each dependency cluster is easier to explain.

Stop conditions:

- if slimming starts to hide unclear boundaries instead of clarifying them,
- if constructors are being rearranged without reducing responsibility.

### Phase 5: Runtime Assembly Cleanup

Goal:

- keep `internal/app/httpapi` and other runtime assembly layers focused on wiring only.

Recommended work slices:

1. remove business-rule leakage from app assembly code,
2. keep route and worker registration thin,
3. move business branching back into ListingKit or marketplace-owned packages.

Acceptance criteria:

- app-layer code is mostly construction, registration, and wiring,
- no new business logic is added to runtime assembly packages.

### Phase 6: Marketplace Boundary Normalization

Goal:

- keep marketplace-specific behavior in marketplace-owned packages and out of generic ListingKit surfaces.

Recommended work slices:

1. identify SHEIN-specific rules still living under root ListingKit,
2. move marketplace-owned logic to the appropriate package when a safe target already exists,
3. leave compatibility facades behind when needed to avoid broad breakage.

Current checkpoint:

- `internal/marketplace/shein/{publishing,workspace}` exists as the target SHEIN directory shape.
- Stable low-risk helpers can start moving there first, while `internal/publishing/shein` and `internal/workspace/shein` shrink into compatibility shells.
- When a helper is structurally independent, prefer moving the implementation first and letting the old package re-export it, instead of doing another large in-place cleanup in `listingkit`.
- The current boundary posture is guard-first: keep `internal/publishing/shein` as a legacy compatibility/model package for now, while preventing new `internal/marketplace/shein/publishing` logic from depending on `listingkit` or root runtime wiring.
- Current migrated examples:
  - `publishing`: pricing policy
  - `workspace`: state helpers, dirty hints, editor progress, editor recommendations/effects, readiness/checklist/guidance helpers, repair center/plan/session projection helpers, editor context/revision models, editor context builder, editor revision skeleton, editor revision-from-context projection, minimal revision pruning, revision diff/applied-changes/history-compare helpers, restore draft/request/preview helpers, history detail/restore detail/presentation projections, revision field validation, revision validation/success payloads, overview projection, inspection payload/build helpers

Acceptance criteria:

- new marketplace logic no longer lands in root ListingKit by default,
- existing moves reduce cross-domain ambiguity rather than merely shifting files.

### Phase 7: Infrastructure Interface Cleanup

Goal:

- hide concrete external clients behind narrow interfaces where ListingKit currently depends on implementation details.

Recommended work slices:

1. identify direct infrastructure coupling in service construction and task flows,
2. shrink interfaces to what ListingKit actually uses,
3. keep interface introduction incremental and tied to real boundaries.

Acceptance criteria:

- ListingKit depends on narrower abstractions,
- external-client behavior remains unchanged,
- tests or fakes become easier to write where coupling was reduced.

## 8. Candidate Backlog by Area

This is a planning backlog, not a mandatory move list.

### Preview

- central preview builder branching,
- per-platform preview section assembly,
- selected-platform validation behavior,
- preview header and overview composition.

### Submission

- direct submit flow,
- retry and recovery orchestration,
- submission locks,
- task requeue coordination,
- Temporal-facing submit adapters.

Current narrow target:

- extract the smallest reusable orchestration inside direct submit before moving any platform-owned SHEIN rules

### Revision and History

- revision history queries and identifiers,
- restore and validation coordination,
- preview-facing revision presentation seams.

### Studio and Workspace Coordination

- studio batch and session orchestration that still lives in root ListingKit,
- bridge code that may belong in marketplace workspace packages,
- compatibility shims that can stay in ListingKit temporarily.

## 9. PR Template for ListingKit Refactoring

Each refactoring PR should answer:

1. What single boundary or responsibility is being improved?
2. What behavior is intentionally unchanged?
3. Which focused tests were run?
4. Did the change reduce root ListingKit pressure, dependency sprawl, or ownership ambiguity?
5. Did it introduce any temporary compatibility seam or known exception?

## 10. Success Metrics

Measure progress using trend lines, not arbitrary promises.

Primary signals:

- root `internal/listingkit` file pressure trends downward,
- largest root files become smaller or more isolated,
- more responsibilities have an obvious owning package,
- fewer marketplace-specific rules remain in generic ListingKit code,
- fewer direct dependencies hang off the root service object.

Secondary signals:

- test setup becomes easier,
- targeted test suites get faster and more reliable,
- reviewers can explain package ownership with less ambiguity.

## 11. Maintenance Rule

Update this document when:

- the project-wide execution order changes,
- a major ListingKit boundary decision is made,
- a phase is completed and the next bottleneck becomes clearer,
- current metrics materially change.

Do not leave stale hardcoded file counts, branch names, or package-creation steps here after the codebase has moved on.

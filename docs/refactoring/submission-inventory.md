# ListingKit Submission Inventory

> Status: historical inventory snapshot.
>
> Original observation window: 2026-06-09 through the later submission consolidation wave.
>
> Current authority: use `listingkit-boundary-checkpoint.md` for current stop lines, `current-refactoring-status.md` for Now / Next / Later, and `next-phase-plan.md` for immediate execution.

## 1. Why this document still exists

This document is no longer the active submission migration plan. It remains as a compact historical inventory of why submission work was separated into generic submission primitives, root ListingKit orchestration, SHEIN publishing rules, and Temporal/runtime collaborators.

Do not append long validation logs here. If fresh evidence is needed, keep generated or command output under `.local/` and summarize only the decision-relevant result in a dated validation note.

## 2. Current submission shape

Submission behavior currently spans these layers:

1. root `internal/listingkit` facade methods and service collaborators;
2. root `internal/listingkit` task/result persistence ordering and DTO adaptation;
3. root `internal/listingkit` SHEIN transition sequencing where task-owned persistence order still matters;
4. generic submission primitives under `internal/listing/submission`;
5. SHEIN-specific publishing/model helpers under `internal/publishing/shein` and `internal/marketplace/shein/publishing`;
6. workflow/runtime boundaries under ListingKit Temporal and HTTPAPI assembly packages.

The current direction is clearer than before, but not finished:

- generic mechanics should prefer `internal/listing/submission`,
- SHEIN publishing rules should prefer marketplace/publishing-owned packages,
- root ListingKit should keep only orchestration, compatibility, DTO adaptation, persistence ordering, and legacy error translation,
- Temporal and direct-submit paths should not be moved into generic packages while they still depend on root task models, runtime callbacks, and persistence order.

## 3. Responsibility map

### Root ListingKit facade and collaborator wiring

Representative files:

```text
internal/listingkit/service_submit*.go
internal/listingkit/service_submission_collaborators.go
internal/listingkit/task_submission_*service*.go
internal/listingkit/task_temporal_submission_*service*.go
internal/listingkit/task_recovery*.go
internal/listingkit/task_requeue*.go
```

Current role:

- preserve public service/API compatibility,
- coordinate root task/package persistence ordering,
- assemble submission collaborators,
- bridge repository callbacks and runtime support,
- translate legacy/root DTOs and errors.

Stop line:

```text
Do not move these files into internal/listing/submission unless the moved seam no longer needs root ListingKit models, repository callbacks, Temporal/direct-submit ordering, or SHEIN runtime side effects.
```

### Generic submission domain

Preferred home:

```text
internal/listing/submission
```

Owns generic, platform-neutral mechanics such as:

- lock and lease policy,
- retry/backoff policy,
- attempt and result state helpers,
- event-history and event outcome policy,
- request/record matching with neutral inputs,
- remote confirmation state policy,
- recovery route and refresh skeletons when they do not need root models,
- submit error shapes and retryable classification.

Stop line:

```text
Do not recreate internal/listingkit/submission as a compatibility package.
New generic submit primitives should land directly in internal/listing/submission.
```

### SHEIN publishing and marketplace rules

Preferred homes:

```text
internal/marketplace/shein/publishing
internal/publishing/shein during compatibility migration
```

Own:

- SHEIN remote response interpretation,
- default-confirmed/fallback rules,
- remote record selection and projection,
- supplier/lookup normalization with neutral inputs,
- publish-specific rules that do not require root ListingKit persistence ordering.

Stop line:

```text
Do not keep new SHEIN publishing policy in root internal/listingkit when it can live behind a marketplace/publishing-owned seam.
```

### SHEIN readiness/workspace rules

Preferred home:

```text
internal/marketplace/shein/workspace
```

Own:

- operator-facing inspection summaries,
- review and repair presentation rules,
- task-list queue taxonomy,
- template freshness/readiness descriptors where they are marketplace/workspace-facing.

Root ListingKit may still assemble payload shells and bridge task/snapshot fields when the data still belongs to ListingKit task orchestration.

### API/runtime assembly

Representative areas:

```text
internal/listingkit/api
internal/listingkit/httpapi
internal/listingkit/temporal
```

Current role:

- transport translation,
- module/bootstrap/runtime assembly,
- Temporal workflow/runtime contracts,
- adapter construction.

Stop line:

```text
API/runtime assembly must not become the place where submission policy is repaired.
```

## 4. Current safe next slices

Good candidates:

1. Move one read-only submission decision predicate to `internal/listing/submission`.
2. Move one SHEIN response/fallback/record-selection rule to `internal/marketplace/shein/publishing` or keep it in `internal/publishing/shein` if still compatibility-owned.
3. Add or tighten a guard proving target packages do not import root `internal/listingkit`.
4. Document missing validation evidence instead of continuing to split root files.

Bad candidates:

- remote submit execution,
- repository mutations,
- Temporal activity/workflow orchestration,
- root task/result persistence callbacks,
- broad DTO moves,
- package renames for consistency only.

## 5. Historical lessons

The long submission consolidation wave produced these durable lessons:

- file splitting helps only when it exposes ownership seams;
- root ListingKit still has legitimate orchestration and persistence-ordering work;
- generic submission policy can move safely when inputs are neutral and side effects are absent;
- SHEIN-specific rules should move toward marketplace/publishing packages instead of generic submission;
- Temporal determinism and activity retry behavior require separate review before any move;
- generated dependency/package baselines are evidence, not architecture authority.

## 6. Current review checklist

Before another submission refactor, check:

```text
[ ] The target package does not import root internal/listingkit.
[ ] The moved logic has neutral inputs or marketplace-owned inputs.
[ ] No persistence callback, remote submit side effect, or Temporal determinism changes are mixed into the move.
[ ] Root ListingKit keeps only compatibility, DTO adaptation, orchestration, or persistence ordering.
[ ] A package or import-boundary test protects the new direction.
[ ] Generated outputs are local evidence only unless summarized in a dated validation note.
```

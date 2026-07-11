# ListingKit Boundary Checkpoint

> Status: current checkpoint for the ListingKit slimming and boundary-guard wave.
>
> Last reviewed: 2026-07-11.

## Purpose

This checkpoint records the current stop lines for ListingKit boundary work. It is intentionally concise: long completed-slice inventories should stay in dated historical notes, not in the active checkpoint.

The current direction is:

```text
Do not keep extracting helpers from internal/listingkit unless the move reduces real ownership or dependency pressure.
Keep new stable rules in their owning listing/product/marketplace packages.
Keep root internal/listingkit as facade, orchestration, DTO adaptation, persistence ordering, and compatibility glue.
```

## Current extracted target packages

### `internal/listing/studio`

Owns platform-neutral studio/batch seams that no longer need root ListingKit ownership, including stable batch/session orchestration rules and repository contracts.

Guardrail:

- `internal/listing/studio` must not import `internal/listingkit`, SHEIN marketplace/workspace/publishing packages, or runtime/integration wiring.

Root `internal/listingkit` still owns API-facing DTO adaptation, repository implementations, generation resume, task creation behavior, batch-run execution, logging, and legacy error translation when those paths still require root task/repository ordering.

### `internal/listing/studio/referenceanalysis`

Owns platform-neutral interpretation and safety policy for Studio reference-image analysis, including structured/malformed result parsing, reusable style abstraction, protected-identity filtering, and sanitized brief/prompt construction.

Guardrail:

- `internal/listing/studio/referenceanalysis` uses only the Go standard library and must not import ListingKit, marketplace, runtime, infrastructure, HTTP, SDS, or external SDK packages.

Root `internal/listingkit` retains request validation, image URL and upload resolution, AI invocation, compatibility DTOs, warning text, and public error translation.

### `internal/listing/preview`

Owns platform-neutral preview rules, capability mapping, and neutral preview summaries.

Guardrail:

- `internal/listing/preview` must not import `internal/listingkit`, `internal/marketplace/shein`, `internal/publishing/shein`, or `internal/workspace/shein`.

Root `internal/listingkit` may still keep legacy preview facade behavior, platform DTO shells, and task-result aggregation during migration.

### `internal/listing/submission`

Owns generic submission primitives such as readiness, retry/backoff, lock/lease, event-history, attempt-state, recovery, refresh selection, remote state policy, and generic request/record matching rules.

Guardrail:

- Generic submit primitives should land directly in `internal/listing/submission`.
- Do not recreate an `internal/listingkit/submission` compatibility package.
- Root `internal/listingkit` may keep SHEIN task/result/package adaptation, repository callbacks, persistence ordering, Temporal/direct fallback routing, and root DTO/error translation when those paths still require task-owned state transitions.

### `internal/product/sourcing/sdspod`

Owns deterministic, platform-neutral SDS POD normalization into canonical product facts, including trusted title, SDS identity attributes, Studio style metadata, rendered mockup normalization, variant image assignment, and canonical field traces.

Guardrail:

- `internal/product/sourcing/sdspod` may import only the Go standard library and `internal/catalog/canonical`; it must not import ListingKit, marketplace or publishing packages, SDS runtime/client packages, app/runtime, infra, HTTP, Temporal, or external SDKs.

Root `internal/listingkit` retains legacy SDS DTO adaptation, historical decorated supplier-SKU lookup compatibility, task/workflow orchestration, and changed-result propagation.

The anonymous successful variant union in `product.Images` is a human-approved intentional refinement; all other image precedence and per-variant lookup/fallback behavior remains unchanged.

### `internal/marketplace/shein/publishing`

Owns canonical SHEIN marketplace publishing rules that are stable and do not require root runtime wiring.

Current examples include remote record classification, remote confirmation fallback/default-confirmed policy, remote record selection, and remote response parsing.

Guardrail:

- `internal/marketplace/shein/publishing` must not import `internal/listingkit` or root runtime/integration wiring.

### `internal/marketplace/shein/workspace`

Owns SHEIN workspace-facing presentation and repair rules such as inspection summaries, preview status, review summaries, action/work queue descriptors, template freshness evaluation, and related operator-facing classification rules.

Guardrail:

- `internal/marketplace/shein/workspace` must not import `internal/listingkit` or root runtime/integration wiring.

## Current legacy exceptions

These exceptions are intentional for this checkpoint and should continue to shrink over time:

- `internal/publishing/shein` may still be imported by existing ListingKit submission/model flows as a compatibility/model package.
- `internal/publishing/shein` may still depend on legacy OpenAI infra helpers, but production code must not import `internal/listingkit` or root runtime packages.
- `internal/workspace/shein` may still exist as a compatibility shell over `internal/marketplace/shein/workspace`.
- Root `internal/listingkit` may still own facade composition, API-facing DTOs, repository callbacks, task/result persistence ordering, and adapter glue.
- The concrete `internal/infra/clients/management` package may still exist as a legacy retirement target, but new runtime semantics should not reintroduce a broad management service dependency.

## Stop lines

Do not:

- keep splitting `internal/listingkit` files only to reduce file size;
- move behavior that still requires root task/repository/event ordering;
- route generic submission primitives through a recreated `internal/listingkit/submission` package;
- add new SHEIN, TEMU, Amazon, or Walmart platform policy to root `internal/listingkit`;
- ban all `internal/publishing/shein` imports from ListingKit until the remaining compatibility flows are explicitly migrated;
- use stale generated package maps or dependency baselines as evidence for current ownership.

## Good next candidates

Prefer one small, guard-backed seam at a time:

1. `internal/listing/submission`: read-only policy seams that do not touch Temporal determinism or platform submit side effects.
2. `internal/marketplace/shein/publishing`: new SHEIN publishing rules that do not require runtime wiring or legacy model relocation.
3. `internal/product/sourcing`: source identity and normalization seams where crawler adapters can remain thin.
4. `internal/integration/crawler/*`: raw source execution adapters that do not depend on ListingKit, marketplace publishing, or workspace packages.

## Current execution rule

Before approving another ListingKit boundary PR, check:

```text
[ ] The target package does not import root internal/listingkit.
[ ] The moved logic is a stable rule or policy, not runtime wiring.
[ ] ListingKit keeps only compatibility, DTO adaptation, orchestration, or persistence callbacks.
[ ] Behavior changes are separated from file moves.
[ ] A boundary test or narrow package test protects the new direction.
[ ] Generated baselines are freshly regenerated or intentionally left out of the commit.
```

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
- `internal/publishing/shein` remains a legacy compatibility/model package for now.

Guardrail:

- `internal/marketplace/shein/publishing` must not import `internal/listingkit` or root runtime/integration wiring.

## Legacy Exceptions

These exceptions are intentional for the current checkpoint:

- `internal/publishing/shein` may still be imported by existing ListingKit submission/model flows.
- `internal/workspace/shein` may still exist as a compatibility shell over `internal/marketplace/shein/workspace`.
- root `internal/listingkit` may still own facade composition, API-facing DTOs, and adapter glue.

These exceptions should get thinner over time, but they are not blockers for this checkpoint.

## Next Direction

Do not continue extracting studio internals unless a new seam clearly reduces root `listingkit` ownership.

Preferred next areas:

- `internal/product/sourcing`: consolidate product source request/result normalization and source identity.
- `internal/integration/crawler/*`: keep crawler adapters focused on raw source execution.
- `internal/listing/submission`: only continue if a seam reduces duplicate orchestration without touching Temporal determinism or platform submit semantics.

Recommended next slice:

- inspect current crawler/source boundaries and identify one small normalization seam for `product/sourcing`.
- current first sourcing seam: `SourceIdentity` and normalized `SourceRequest` fields now live in `internal/product/sourcing`, with Amazon crawl request planning consuming that normalization.
- current second sourcing seam: Amazon batch result alignment now lives in `internal/product/sourcing`, preserving source identity for each requested product ID.
- current third sourcing seam: 1688 URL/result identity normalization now lives in `internal/product/sourcing`, while crawler execution remains in `internal/integration/crawler/a1688` and legacy `internal/crawler/alibaba1688`.

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

For narrower iterations, use package-specific `-run` filters, but always rerun the affected package without a filter before claiming the package is fully verified.

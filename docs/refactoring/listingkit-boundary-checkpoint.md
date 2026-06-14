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
- legacy `internal/publishing/shein.PricingPolicy` is a compatibility alias over the marketplace pricing policy, guarded by a bridge contract test,
- `internal/publishing/shein` remains a legacy compatibility/model package for now.

Guardrail:

- `internal/marketplace/shein/publishing` must not import `internal/listingkit` or root runtime/integration wiring.

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

Completed submission slices:

- source-facts readiness policy for 1688-derived facts now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps a compatibility wrapper.
- in-process submit lock manager now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps a compatibility alias.
- enqueue retry/backoff policy for queue-full submit retries now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps a compatibility wrapper.
- response outcome policy for save-draft success and publish response errors now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN response adapters.
- phase detail mapping policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN phase labels.
- failure-state fallback policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN report adapters.
- remote-recovery lease expiry policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN phase/report adapters.
- active attempt lease policy now lives in `internal/listing/submission`; `internal/listingkit/submission` keeps SHEIN report adapters.

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

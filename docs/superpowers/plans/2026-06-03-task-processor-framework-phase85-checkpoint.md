# Phase 85 checkpoint

## Theme

`ListingKit shared asset-generation projection contract`

## What changed

- introduced shared bundle seam in `internal/listingkit/asset_generation_projection.go`
- centralized the common outward projection contract for:
  - cloned generation tasks
  - asset generation summary
  - generation queue
  - asset generation overview
- rewired three production consumers to delegate to the shared seam:
  - `decorateListingKitResultGeneration(...)`
  - `GetTaskPreview(...)`
  - `GetTaskExport(...)`

## Why this is framework-level

- this seam is consumed by result, preview, and export flows
- it removes repeated `tasks + summary + queue + overview` assembly across multiple outward adapters
- it reduces drift risk between result-local decoration and preview/export payload decoration

## Guardrails

- behavior coverage:
  - `internal/listingkit/asset_generation_projection_test.go`
- boundary coverage:
  - `internal/listingkit/phase85_asset_generation_projection_boundary_test.go`

## Validation

- `go test ./internal/listingkit -run "TestBuildAssetGenerationProjectionBuildsSharedBundle|TestAssetGenerationProjectionBoundary" -count=1`
- `go test ./internal/listingkit -run "TestBuildAssetGenerationProjectionBuildsSharedBundle|TestAssetGenerationProjectionBoundary|TestBuildListingKitExportPopulatesSemanticFields|TestBuildSheinPreviewPayloadPopulatesSemanticFields|TestBuildRevisionSuccessFollowUpDataBuildsSharedPayload|TestBuildGenerationWorkQueue.*" -count=1`
- `go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1`

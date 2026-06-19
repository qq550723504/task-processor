# Listing Preview Boundaries

This note captures the current preview-related boundary for the project-wide refactor.

Use `docs/architecture/project-boundaries.md` as the repository-wide default
for ownership and dependency direction. This document only narrows that
baseline for preview extraction and ListingKit preview migration work; broader
package-placement rules should still be resolved through the main project
boundary entrypoint first.

It complements:

- [`project-boundaries.md`](./project-boundaries.md)
- [`listingkit-refactor-status.md`](./listingkit-refactor-status.md)
- [`project-wide-refactoring-plan.md`](../refactoring/project-wide-refactoring-plan.md)

## Why This Matters

The project-wide plan calls preview aggregation the best next bounded extraction from the legacy ListingKit facade.

That extraction has already started, but the codebase is now in a mixed state:

- `internal/listing/preview` already owns several platform-neutral preview concepts.
- `internal/listingkit` still owns task/result projection and platform payload composition.

Without a written boundary, it is easy to keep adding more preview behavior to root `listingkit` and lose the benefit of the new `internal/listing/preview` package.

## Current Stable Ownership

## `internal/listing/preview`

This package is now the stable home for platform-neutral preview rules and shell composition.

Current responsibilities:

- preview shell construction
- preview header construction
- preview status label / status message mapping
- selected-platform normalization and validation
- platform availability / selection rules
- supported preview platform registry

Representative files:

- `internal/listing/preview/shell.go`
- `internal/listing/preview/header.go`
- `internal/listing/preview/status.go`
- `internal/listing/preview/platform.go`
- `internal/listing/preview/platforms.go`
- `internal/listing/preview/sections.go`
- `internal/listing/preview/errors.go`

This package should remain platform-neutral. It should not start depending on marketplace-specific draft models, repository state, or HTTP wiring.

Current guardrail:

- `TestListingPreviewPackageStaysPlatformNeutral` prevents `internal/listing/preview` from importing the ListingKit facade or marketplace-specific Amazon, SHEIN, TEMU, publishing, or workspace implementation packages.
- `TestProjectBoundaryDomainsDoNotImportListingKitFacade` keeps new domain-side preview logic from reintroducing root `internal/listingkit` facade dependencies as the default home for extraction work.

## `internal/listingkit`

This remains the compatibility facade and preview orchestration layer.

Current preview responsibilities that still belong here for now:

- adapting task/result state into preview inputs
- attaching catalog and asset snapshots to preview responses
- assembling revision metadata into preview responses
- dispatching to platform-specific preview builders
- assembling marketplace-specific preview payload sections

Representative files:

- `internal/listingkit/preview_result_projection.go`
- `internal/listingkit/preview_result_attachment.go`
- `internal/listingkit/preview_platform_registry.go`
- `internal/listingkit/preview_platform_amazon*.go`
- `internal/listingkit/preview_platform_shein.go`
- `internal/listingkit/preview_platform_temu.go`
- `internal/listingkit/preview_platform_walmart.go`
- `internal/listingkit/preview_builder_shein*.go`

These files are still acceptable in `listingkit` because they bridge legacy task/result models into preview-facing responses.

## Current Hotspots

### 1. Preview result projection is still facade-bound

`internal/listingkit/preview_result_projection.go` still combines:

- task/result adaptation
- catalog / asset attachments
- revision history metadata
- generation queue projection

This is the most likely next extraction seam, but only after neutral projection inputs are defined.

### 2. Platform preview assembly is split across generic and marketplace-specific helpers

The following pattern is already emerging:

- platform-neutral selection rules in `internal/listing/preview`
- marketplace-specific preview payload assembly in `internal/listingkit`

This is acceptable for now, but future moves should prefer marketplace-owned builders instead of adding more platform-specific payload logic to root `listingkit`.

### 3. SHEIN preview helpers are still numerous

The large group of `preview_builder_shein*` files means preview assembly is still one of the remaining high-density areas in ListingKit.

The immediate goal is not to move them blindly. The goal is to prevent new SHEIN domain rules from being added there unless they are truly facade-only preview composition.

## Next Extraction Rules

Use these rules for the next preview-related refactors.

### Move to `internal/listing/preview` when the code is:

- platform-neutral
- independent of `listingkit.Task` and `listingkit.TaskResult`
- about preview shell, selection, availability, status, or generic section assembly

### Keep in `internal/listingkit` when the code is:

- adapting legacy ListingKit task/result models
- composing preview payloads from repository/service state
- bridging old API response types
- still tightly coupled to ListingKit-owned facade types

### Move to marketplace packages later when the code is:

- platform-specific preview payload shaping
- marketplace-specific review or readiness presentation
- marketplace-specific asset or publishing summaries

Target direction:

- Amazon preview rules -> `internal/marketplace/amazon/...`
- SHEIN preview rules -> existing SHEIN publishing/workspace homes when the shape becomes clear
- TEMU / Walmart preview rules -> their future marketplace homes

## Recommended Next Step

The next behavior-preserving preview refactor should be:

1. Define neutral projection inputs for preview result composition.
2. Extract the generic parts of `preview_result_projection.go` behind those inputs.
3. Keep marketplace payload builders in place until each platform has a clear non-ListingKit owner.

Do not start by renaming or moving every preview file.

The desired outcome is:

- `internal/listing/preview` owns generic preview domain rules
- `internal/listingkit` becomes a thinner preview adapter/facade
- marketplace packages eventually own marketplace-specific preview sections

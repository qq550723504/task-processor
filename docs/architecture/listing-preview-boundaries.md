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

- `internal/listing/preview` owns preview shell/status concepts and compatibility wrappers.
- `internal/listing/platformsection` owns reusable platform-section selection and dispatch.
- `internal/listingkit` still owns task/result projection and platform payload composition.

Without a written boundary, it is easy to keep adding more preview behavior to root `listingkit` and lose the benefit of the new `internal/listing/preview` package.

## Current Stable Ownership

## `internal/listing/preview`

This package is now the stable home for platform-neutral preview rules and shell composition.

Current responsibilities:

- preview shell construction
- preview header construction
- preview status label / status message mapping
- compatibility wrappers around listing platform selection helpers

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

## `internal/listing/platformsection`

This package owns platform-neutral section selection and builder dispatch that
can be reused by preview/export adapters without naming that shared framework
as preview-only.

Current responsibilities:

- platform name normalization
- supported listing platform registry
- selected-platform validation
- selected-platform matching
- single section availability checks
- ordered platform-section builder execution
- nil builder skipping
- stop-on-first-error dispatch

Representative files:

- `internal/listing/platformsection/sections.go`

## `internal/listingkit`

This remains the compatibility facade and preview orchestration layer.

Current preview responsibilities that still belong here for now:

- adapting task/result state into preview inputs
- attaching catalog and asset snapshots to preview responses
- assembling revision metadata into preview responses
- adapting platform-specific preview/export builders into neutral section dispatch
- assembling marketplace-specific preview payload sections

Representative files:

- `internal/listingkit/preview_result_attachment.go`
- `internal/listingkit/preview_result_adapter.go`
- `internal/listingkit/preview_task_read_model_adapter.go`
- `internal/listingkit/preview_platform_registry.go`
- `internal/listingkit/preview_platform_amazon*.go`
- `internal/listingkit/preview_platform_shein.go`
- `internal/listingkit/preview_platform_temu.go`
- `internal/listingkit/preview_platform_walmart.go`
- `internal/listingkit/preview_builder_shein*.go`

These files are still acceptable in `listingkit` because they bridge legacy task/result models into preview-facing responses.

## Current Hotspots

### 1. Preview result projection adapter is still facade-bound

The former `internal/listingkit/preview_result_projection.go` has been retired.
The remaining facade-owned projection responsibilities now live in:

- `internal/listingkit/preview_result_adapter.go`
- `internal/listingkit/preview_task_read_model_adapter.go`

Those files still bridge:

- task/result adaptation
- catalog / asset attachments
- revision history metadata
- generation queue projection

This is still a useful extraction area, but it now has neutral projection inputs
and a narrower adapter boundary. Future work should avoid reintroducing a
root projection file that mixes domain projection, legacy DTO adaptation, and
attachment application.

### 2. Platform preview assembly is split across generic and marketplace-specific helpers

The following pattern is already emerging:

- platform-neutral selection, validation, and section dispatch in `internal/listing/platformsection`
- preview shell/status and compatibility wrappers in `internal/listing/preview`
- marketplace-specific preview payload assembly in `internal/listingkit`

This is acceptable for now, but future moves should prefer marketplace-owned builders instead of adding more platform-specific payload logic to root `listingkit`.

### 3. SHEIN preview helpers are still numerous

The large group of `preview_builder_shein*` files means preview assembly is still one of the remaining high-density areas in ListingKit.

The immediate goal is not to move them blindly. The goal is to prevent new SHEIN domain rules from being added there unless they are truly facade-only preview composition.

## Next Extraction Rules

Use these rules for the next preview-related refactors.

### Move to `internal/listing/preview` when the code is:

- preview-specific
- independent of `listingkit.Task` and `listingkit.TaskResult`
- about preview shell, status, or compatibility wrappers

### Move to `internal/listing/platformsection` when the code is:

- platform-neutral
- independent of `listingkit.Task` and `listingkit.TaskResult`
- about platform normalization, supported-platform validation, selection, availability, or ordered builder dispatch rather than preview payload shape

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

1. Continue shrinking the ListingKit preview adapter surface behind neutral projection inputs.
2. Keep marketplace payload builders in place until each platform has a clear non-ListingKit owner.
3. Add guard coverage before moving any platform-specific preview payload logic.

Do not start by renaming or moving every preview file.

The desired outcome is:

- `internal/listing/preview` owns generic preview domain rules
- `internal/listingkit` becomes a thinner preview adapter/facade
- marketplace packages eventually own marketplace-specific preview sections

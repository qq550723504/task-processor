# Listing Preview

Owns platform-neutral listing preview rules and preview-facing shell composition as
the project migrates away from the legacy `internal/listingkit` facade.

Current stable ownership:

- preview shell construction
- platform-neutral preview attachment fields (`catalog`, `assets`, `asset_inventory`)
- preview header construction
- preview status mapping
- selected-platform normalization and validation
- supported platform selection rules
- platform section availability checks
- read-model projection seam for legacy preview adapters
- task preview read orchestration seam (`GetTaskPreview` style load/build/finalize flow)
- render-preview metadata summary extraction for asset preview DTO adapters
- platform render-preview summary aggregation over neutral slot inputs

Does not own yet:

- ListingKit task/result projection
- marketplace-specific preview payload builders
- concrete repository implementations and legacy preview DTO adapters

Those responsibilities still live in `internal/listingkit` until neutral preview
projection inputs are introduced and marketplace-specific preview builders have
clear owners.

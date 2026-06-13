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

Does not own yet:

- ListingKit task/result projection
- marketplace-specific preview payload builders
- repository/service attachment logic

Those responsibilities still live in `internal/listingkit` until neutral preview
projection inputs are introduced and marketplace-specific preview builders have
clear owners.

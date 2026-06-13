# Product Sourcing

Owns normalized sourcing pipelines for external product-data inputs.

It should consume crawler outputs and normalize them into reusable product facts, assets, and enrichment inputs.

Expected upstream adapters:

- `internal/integration/crawler/amazon`
- `internal/integration/crawler/a1688`

Owns:

- source-result normalization
- source-to-product handoff contracts
- enrichment-ready intermediate models
- handoff into catalog, asset, and image domains

Does not own:

- crawler runtime details
- marketplace-specific publishing rules
- legacy ListingKit compatibility shims

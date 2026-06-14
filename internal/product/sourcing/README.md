# Product Sourcing

Owns normalized sourcing pipelines for external product-data inputs.

It should consume crawler outputs and normalize them into reusable product facts, assets, and enrichment inputs.

Expected upstream adapters:

- `internal/integration/crawler/amazon`
- `internal/integration/crawler/a1688`

Legacy sources to drain over time:

- `internal/crawler/amazon`
- `internal/crawler/alibaba1688`
- source-normalization code currently stranded in mixed marketplace or ListingKit flows

Owns:

- source-result normalization
- source identity normalization
- source-to-product handoff contracts
- source platform to crawler platform mapping
- source request inheritance for variants
- enrichment-ready intermediate models
- handoff into catalog, asset, and image domains

Does not own:

- crawler runtime details
- marketplace-specific publishing rules
- legacy ListingKit compatibility shims

Boundary guard:

- this package may normalize crawler outputs, but must not depend on `internal/listingkit`, marketplace packages, or runtime/platform wiring.

If a change starts from Amazon or 1688 source data, a good default split is:

1. source access and raw extraction in `internal/integration/crawler/*`
2. normalization and reusable product handoff in `internal/product/sourcing`
3. downstream listing or marketplace use in `internal/listing/*` or `internal/marketplace/*`

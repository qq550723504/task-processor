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
- source-result alignment with source identities
- 1688 URL/result identity normalization
- source identity normalization
- source-to-product handoff contracts
- source platform to crawler platform mapping
- source request inheritance for variants
- Amazon batch fetch source-configuration and empty-batch semantics
- Amazon source domain, language, and URL planning used by crawl requests
- Amazon source default-zipcode policy used by crawl request planning
- enrichment-ready intermediate models
- 1688 scraped-data normalization for specs, descriptions, and image lists
- handoff into catalog, asset, and image domains

Does not own:

- crawler runtime details
- marketplace-specific publishing rules
- legacy ListingKit compatibility shims

Boundary guard:

- this package may normalize crawler outputs, but must not depend on `internal/listingkit`, marketplace packages, or runtime/platform wiring.
- this package may consume raw crawler DTO/model packages when they are the handoff format, but must not import legacy crawler runtime packages directly.

Current stop line:

- do not keep shaving individual 1688 scraped fields unless the change prevents downstream identity, enrichment, or catalog pollution; crawler execution and marketplace usage remain outside this package.

If a change starts from Amazon or 1688 source data, a good default split is:

1. source access and raw extraction in `internal/integration/crawler/*`
2. normalization and reusable product handoff in `internal/product/sourcing`
3. downstream listing or marketplace use in `internal/listing/*` or `internal/marketplace/*`

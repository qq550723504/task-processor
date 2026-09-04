# Product Sourcing

Owns the provider-neutral source boundary for product facts.

Source adapters create `SourceEnvelope` values. This package validates and
normalizes source identity, warning codes, raw evidence, lineage, capture time,
and verifiable source metadata. `ToSnapshot` deterministically projects an
envelope into `internal/product/catalog.ProductSnapshot`.

Owns:

- `SourceIdentity`, `SourceEnvelope`, raw references, warnings, and trace data
- strict source identity validation and deterministic fingerprints
- provider-neutral source request/result contracts
- `Normalize(SourceEnvelope)` and `ToSnapshot(SourceEnvelope)`

Does not own:

- Amazon, A1688, SDS, or other provider-specific DTOs and conversion rules
- crawler clients, request planning, fetch orchestration, or runtime details
- asset/image conversion, storage, or workflow orchestration
- ProductEnrich, ListingKit, marketplace payloads, or compatibility shims

Concrete source adapters currently live in:

- `internal/integration/crawler/amazon`
- `internal/integration/crawler/a1688`
- `internal/sds/adapter/product_source`

Boundary guard:

- production code may depend on the standard library and pure product-domain
  packages such as `internal/product/catalog`
- production code must not import `internal/model`, legacy crawler packages,
  `internal/integration`, `internal/productenrich`, legacy `internal/asset`,
  ListingKit, marketplace, or runtime/platform wiring

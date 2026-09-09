# Product Sourcing

Owns the provider-neutral source boundary for product facts.

Source adapters create `SourceEnvelope` values. This package validates and
normalizes source identity, warning codes, raw evidence, lineage, capture time,
and verifiable source metadata. `ToSnapshot` deterministically projects an
envelope into `internal/product/catalog.ProductSnapshot`.

Owns:

- `SourceIdentity`, `SourceEnvelope`, raw references, warnings, and trace data
- explicit missing facts, admitted producer kind/version, and immutable source
  publication identity
- strict source identity validation and deterministic fingerprints
- provider-neutral source request/result contracts
- `Normalize(SourceEnvelope)` and `ToSnapshot(SourceEnvelope)`
- the in-process `InternalProducer` contract and its exact read-only evidence
  projection

## Atomic source publication

`internal/app/productsourcing.NewInternalProducer` is the admitted in-process
composition for `controlled_snapshot/v1`. It does not register an HTTP route or
call a provider. Every publish, replay, verify, and read refreshes live
Organization roles and requires `product_sourcing.write`; Organization and actor
come only from verified context.

`internal/integration/persistence/product/sourcing` stores the normalized
envelope and an operation receipt. In one PostgreSQL transaction it delegates
snapshot/version/head writes to Catalog's transaction writer, then persists the
exact Catalog version binding. `(organization_id, publication_id)` is the
operation key. The canonical input hash covers producer kind/version,
`SourceIdentity`, the normalized envelope, ProductKey, and expected base
version. Same-key/same-hash calls replay; a different hash conflicts.

`InternalProducer.Read` is the downstream evidence contract: after fresh
authorization it returns the immutable envelope, derived snapshot, and receipt
bound to one ProductKey and CatalogVersion. `Verify` additionally re-derives the
canonical hash from the original command and is the read-only recovery path for
response loss or unknown COMMIT outcome.

Envelope and derived snapshot encodings are each capped at 2 MiB; the complete
operation is capped at 10 seconds. Schema setup is explicit via the composed
`productsourcing.InstallSchema`; ordinary calls never execute DDL. Source
images remain Catalog source candidates and do not create ApprovedAsset facts.

Does not own:

- Amazon, A1688, SDS, or other provider-specific DTOs and conversion rules
- crawler clients, request planning, fetch orchestration, or runtime details
- asset/image conversion, storage, or workflow orchestration
- ProductEnrich, ListingKit, marketplace payloads, or compatibility shims
- public/browser routes, provider acquisition, source-account ownership, or
  ApprovedAsset approval

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

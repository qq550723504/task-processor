# SHEIN DRAFT-S1 local records — Issue #376

This package owns the immutable local result of the first SHEIN draft slice.
The frozen architecture authority is Issue #372 R2 plus its PA-1 acceptance;
Issue #376 is the bounded implementation slice. The route is deliberately not
part of default production composition.

## Admitted call chain

`POST /api/listing/shein-records` runs only through verified identity and
effective-Organization middleware, `LiveWrite`, and
`listingkit.admin.write`. The body is strict JSON, at most 1 KiB, with exactly:

```json
{
  "product_key": "product-key",
  "snapshot_version": 1,
  "store_id": "11111111-1111-4111-8111-111111111111",
  "country": "US",
  "language": "en",
  "action": "save_draft"
}
```

`action` is `save_draft` or `publish`; the initial locale is exactly `US/en`.
One canonical `Idempotency-Key` (1–128 bytes) is required. Organization,
owner, Package, diagnostic, freshness, trust and evidence are server-owned.
The end-to-end deadline is 10 seconds and never extends an earlier deadline.

After authorization, the service reads these exact current facts:

1. SHEIN Store reference by effective Organization plus canonical Store UUID;
2. immutable Product Snapshot by Organization, product key and nonzero version;
3. ApprovedAsset inventory by Organization, product key, `shein`, and the same
   Product version.

The Product persistence adapter projects an exact payload only when its
database-side byte count is within the limit and reports oversize separately
from corrupt/hash-invalid state. The Asset adapter measures both the largest
raw Asset row and the complete inventory envelope in PostgreSQL before
transferring or JSON decoding matching payloads. There is no latest/unversioned
Asset fallback. Missing exact approval is `422 not_ready`; missing Product/Store
is `404 not_found`. Product JSON,
ApprovedAsset inventory JSON, generated Package, and persisted diagnostic are
independently bounded to 2 MiB. Dependency/corrupt-source failure is 503,
size rejection 413, idempotency conflict 409, and cancellation/deadline 504.

## Pure draft and diagnostic

The draft builder reuses `catalog.ProjectCanonical`, the existing SHEIN
assembler, and strict persisted-Package decoder. It replaces every Product and
variant source image with the exact ApprovedAsset inventory before assembly.
It has no network/provider/AI resolver, goroutine, cache, retry loop or hidden
database write.

The current offline evaluator is reused. The exact ApprovedAsset read discharges
only `approved_asset_provenance_and_consent`; template freshness, Store remote
authorization, cookie, POD, human review and submission remain explicitly
`not_evaluated`. The stored result is diagnostic-only, never submit authority.

## Durable idempotency and transaction

`listing_shein_records` contains immutable source/input hashes, exact Asset
version/hash, Store/action, rule and policy revisions, Package hash/bytes,
diagnostic hash/status/bytes and ownership. A separate
`listing_shein_record_operations` receipt owns the unique
`(organization_id, operation_id)` idempotency identity.

The PostgreSQL adapter writes receipt plus immutable record in one transaction.
The receipt foreign key is deferred so concurrent contenders can claim the
operation first; a loser loads the committed original only when owner and the
complete canonical input hash match. The hash binds Organization, owner,
Product coordinates/hash, exact ApprovedAsset hash, Store, options/action and
rule/policy revisions. Same key with any changed fact conflicts. Unknown COMMIT
is not compensated: retry reauthorizes and resolves the durable receipt.
Cancellation before commit rolls back both tables; no recovery worker, Saga,
outbox, fallback, dual read/write or second fact source exists.

The schema is a greenfield operator-applied schema for an explicitly admitted
database. Runtime code never migrates it. There is no old-row migration,
legacy wrapper, compatibility DTO, historical Catalog admission or Task-first
workflow.

## Read surfaces

The existing owner/org-scoped reader, offline diagnostic GET, and keyset
collection GET read the immutable record. Collection items include record ID,
Product/version, Store/action, locale and creation time; they never expose
payload, diagnostic, hashes, operation, owner or Organization. Tenant-admin
owner bypass never bypasses Organization. GET paths do not mutate rows.

## Reproduction and rollout boundary

Point `ISSUE376_TEST_DSN` only at a fresh, task-exclusive PostgreSQL database.
Each test creates and drops its own schema. Missing DSN is an explicit SKIP and
is not PostgreSQL acceptance.

```powershell
$env:ISSUE376_TEST_DSN = 'host=127.0.0.1 port=55476 user=issue376 password=issue376-local-only dbname=issue376_draft_s1 sslmode=disable'
go test -v ./internal/listing/record ./internal/marketplace/shein/draft ./internal/marketplace/shein/validator
go test -v ./internal/app/httpapi -run 'Test(SheinRecord|DraftS1|SharedRegression)' -count=1
go test -race ./internal/app/httpapi -run 'Test(SheinRecordConcurrentAndConflictingHTTP|DraftS1ExactApprovedAssetMissDoesNotUseUnversionedHead)' -count=1
```

Controlled test URLs, identities and Store IDs are fixtures only. This slice
does not permit production/shared data, provider calls, AI calls, remote writes,
deployment, merge or Issue closure.

Legacy decision: EXTRACT. Reusable behavior: Product projection, Product/Asset
exact inventory reader, Store Center repository, SHEIN assembler/validator,
verified auth/org middleware. Current owner: Product, Product/Asset, Store
Center, Marketplace/SHEIN, Listing record, and the explicit application adapter.
Cutover/deletion condition: DRAFT-S1 has no dependency on any registered RETIRE
owner and introduces no compatibility path.

# Local SHEIN records — Issue #319

Authority: [D1-CURRENT-PRODUCT-INPUT/V1](https://github.com/qq550723504/task-processor/issues/319#issuecomment-5553109689).
Second narrow review is IMPLEMENTATION_READY for repository/application/database
integration. Final SHA, tests, CI and code review live in the Issue/PR.

`POST /api/listing/shein-records` saves an immutable **local incomplete draft**.
It does not call SHEIN, approve assets, Apply, submit, charge, or create a Task.
`NewSheinRecordApplication` in `internal/app/httpapi` builds a real HTTP server
using the existing verified identity / Organization middleware and configured
authorizer. It requires an explicitly bound current Product database and does
not open databases or migrate schemas. Default runtime composition never calls
it and cannot supply its shared historical Catalog through a feature flag.

## Allowed source and authorization

For this delivery the only allowed source is an isolated temporary PostgreSQL,
starting with empty Catalog tables. Upstream setup calls the real
`catalog.Publisher.Publish` and Catalog persistence adapter with server-owned
Organization/actor context and synthetic product content. Tests retain the
returned identity, publication and immutable version. Publisher is not IAM.

The resource identity includes the explicitly bound storage scope, verified
effective Organization, product key, and nonzero fixed version. No latest,
Task, legacy cache, tenant mapping, timestamp cutoff or trust flag is accepted.
Numeric Organization strings are valid when verified. Within this allowed
scope Product is organization-shared, irrespective of its setup author.

The route uses `LiveWrite` and `listingkit.admin.write`. The use case separately
checks `listingkit.admin.read` and `listingkit.admin.write` **before Product SQL**.
Only roles of the selected Organization apply; Home is not the resource scope.
Every replay reauthorizes and reads/checks the exact Product version. Neither
permission implies the other. Existing grant expiration/revocation behavior is
unchanged. Other Product consumers do not inherit this limited access policy.

Request header: exactly one `Idempotency-Key` (1–128 bytes). Strict JSON body,
at most 1 KiB, no query parameters:

```json
{"product_key":"product-key","snapshot_version":1,"country":"US","language":"en"}
```

The initial finite option set is explicitly **US/en only**, with no defaults.
Unknown/duplicate/case-mismatched body fields fail. Tenant/user/Package,
freshness and admission fields cannot be submitted. The server and handler
bound body reads and work to five seconds without extending earlier deadlines;
the Catalog consumer uses the existing 8 MiB bounded version reader.
Response is `201 {"record_id":"UUID"}` (also on same-operation replay).
Invalid requests return 400, permissions 403, unreadable source/record 404,
operation conflicts 409, size rejection 413, dependency/unknown commit 503,
and cancellation/deadline 504. Errors never return payload, digest or SQL.

## Ownership, persistence and retry

Listing owns `Service`, `Prepared`, input/record contracts and the reader port.
The concrete PostgreSQL adapter and explicit `schema.sql` live in
`internal/app/listingrecordstore`. Current guards prohibit GORM in Listing and
Listing imports in generic integration packages; the narrow application adapter
implements the Listing port without relaxing either guard. Existing authz owns
admin policy. No domain imports this application adapter.

Only `Service` can populate a nonzero `Prepared` value. The concrete adapter has
one INSERT, no public arbitrary-row constructor, no UPDATE/UPSERT mutation or
DELETE API. `listing_shein_records` binds org, actor, operation, source key and
version, fixed options and 1..2 MiB payload in one transaction. The unique key
is `(organization_id, owner_user_id, operation_id)`. No Product/Asset/Task write
occurs. The schema is explicitly applied only in isolated tests, never at runtime.

Concurrent inserts use that exact unique constraint and a scoped conflict read.
The same request returns the original record without rebuilding or replacing
bytes; different source/options conflict. Unknown COMMIT outcome or a lost
HTTP response causes no compensation or deletion: a reauthorized retry locates
the durable operation. Restart requires no in-memory lock or recovery worker.
Cancellation before commit rolls back; cancellation after commit may leave the
record committed and the retry protocol still applies. No Saga/Outbox is needed.

`ReadOfflinePackage(ctx, listingtask.Actor, recordID)` returns detached bytes,
exact org/owner/source identity, creation time and read time. The caller must
supply its verified effective actor (future #315 uses `CachedRead`). SQL filters
ID + org + nonempty owner and owner equality unless the configured authorizer
grants the existing org-local admin bypass. Admin never bypasses org. The same
SELECT uses `octet_length`/CASE before transmitting payload, followed by identity
and scope checks. Unknown Task IDs, absent rows and denied owners are unreadable.

## Package and #315 handoff

`catalog.ProjectCanonical` EXTRACTs the existing pure snapshot projection;
existing ListingKit call sites now invoke that owner. The SHEIN draft builder
clears both product and variant source images and constructs the existing
`NewAssembler(AssemblerConfig{})` without external resolvers, model, pricing,
size or brand authorization context. Missing assets/templates remain blockers.

Encoding uses existing `json.Marshal(Package)` and then #318
`DecodePersistedPackageStrict` admission (2 MiB / 64 levels). No second encoder,
hash, normalization rule or evaluator exists here. Stored bytes are exactly the
admitted output, including Package evidence fields. This builder produces no
external freshness evidence; #315 must explicitly use `not_evaluated` with
`no_authoritative_package_freshness`. CreatedAt/ReadAt are not ObservedAt/expiry.
Future evidence-producing builders require their own reviewed contract; this
path cannot silently import stale cached templates or pretend to approve images.

#315's agreed route is
`GET /api/listing/shein-records/{record_id}/offline-diagnostic`; it is not mounted
here. It consumes this reader and passes the same bytes to #318
`DiagnosticValidator.Validate(BoundRequest[[]byte])`. The PostgreSQL HTTP test
already exercises that computation and asserts real blockers/unknown freshness.

## Evidence and rollout

Run `go test ./internal/app/httpapi -run TestSheinRecord -count=1` with
`ISSUE319_TEST_DSN` naming a **task-isolated PostgreSQL**. Each test creates and
removes its own random schema. Without this variable the PG tests explicitly
skip; this is not database acceptance. Add `-race` for concurrent verification.
Driver-level fault injection tests the real transaction before/after COMMIT;
it does not simulate a real network partition. The normal creation path never
seeds the Listing table. Negative constraint tests mutate only isolated rows
previously created via HTTP. Production default-route tests run without this DSN.

The authorized online/shared historical Catalog set is empty. Real source
binding, writer isolation and customer enablement remain an explicit future
environment rollout gate (#30/#307 coordination), not a prerequisite for this
repository-level delivery. No production data, migration, deployment, merge or
Issue closure is authorized.

Legacy decision: EXTRACT. Reusable behavior: pure Product projection and current
SHEIN assembler. Current owner: Product projection, Marketplace draft builder,
Listing records, Application persistence/HTTP. Cutover: old projection calls
switch in this slice; no new legacy dependency or historical record import.

# SRC-2B1 public 1688 acquisition

Contract: `src2b-acquisition-v1`, Issue #398. This is anonymous public acquisition,
not Browser Capture, Source Account management, a Connection, or a login flow.
The parser accepts only the explicitly supported static `window.context` JSON
shape. The checked-in HTML is a synthetic fixture, not evidence that the current
1688 site is available. Real 1688 network acceptance is NOT_RUN.

## Ownership and lifecycle

- `internal/integration/acquisition/a1688` fetches bounded anonymous public HTML
  and produces untrusted `AcquisitionEvidence` only.
- `internal/product/sourcing` validates/maps evidence. Original decimals and
  missing facts remain visible; images remain candidates, never Approved Assets.
- `internal/app/productsourcing` coordinates existing live authorization,
  operation staging, and the explicitly admitted `public_acquisition/v1` producer.
- `product_acquisition_operations` is bounded recovery input, not published facts.
- SRC-1 owns published evidence/receipts. Catalog alone owns ProductSnapshot.

One scope `(organization, actor, Idempotency-Key)` retains the original intent.
Equivalent source URLs have the same fingerprint. A different source with the
same key conflicts. Canonical identity is `crawler:1688:<offerID>`.

`acquiring -> prepared -> publishing -> published` freezes the complete original
PublicationCommand before publication. Only an explicitly successful first claim
may Publish. Once publishing starts, all uncertain results use original-command
Verify/Read only. A missing receipt is still unknown, not permission to republish.
Only expired acquiring operations without a prepared command may obtain a new
fenced GET lease. There is no background scheduler, fallback, rebase, TTL or key GC.

Limits: 256 retained operations per organization, 32 active operations (including
prepared/publishing/unknown), 2 MiB command, 8 KiB strings, 256 items per collection,
1024 aggregate items, 20-second operation deadline, 8-second GET, 30-second GET
lease. Compressed and expanded provider bodies are each limited to 2 MiB.

## Explicit empty-database initialization

Provision a dedicated PostgreSQL database and the login
`source_acquisition_runtime` separately, with its credential supplied through the
deployment's existing secret process. The runtime role must not be owner, superuser,
role/database creator, replication/BYPASSRLS role, or member of another role.

An owner creates a private JSON file (absolute path, not a symlink; mode 0600 on
Unix). Do not commit it or put its credential in command arguments:

```json
{
  "schemaVersion": 1,
  "database": {
    "host": "127.0.0.1",
    "port": 5432,
    "user": "product_schema_owner",
    "password": "REPLACE_WITH_PRIVATE_OWNER_CREDENTIAL",
    "database": "product_acquisition"
  }
}
```

```text
go run ./cmd/product-acquisition-init -config <absolute-private-manifest> -confirm-empty-database product_acquisition
```

This explicit action installs the five current Product tables and narrowly grants
CONNECT/USAGE; staging SELECT/INSERT/UPDATE; SRC-1 and Catalog versions SELECT/INSERT;
Catalog heads SELECT/INSERT/UPDATE. It removes PUBLIC database/schema privileges
in this dedicated database. It refuses a nonempty database, including a previously
initialized database, and never migrates, deletes business data, provisions a
database, or changes login credentials. Never run it against a shared database.

## RUN-1 and BFF wiring

Add optional `productAcquisitionDatabase` to the existing private RUN-1 manifest,
using the same database fields as the other pools, with user
`source_acquisition_runtime` and `maxConnections` between 1 and 8. It must identify
a different database than the Source Account and commercial pools. Omission keeps
the existing ten-route application unchanged. Ordinary startup only verifies
schema/actual privileges and opens existing pools; it never invokes initialization.

The three added backend routes are:

```text
POST /api/v1/workbench/sourcing/1688/acquisitions
POST /api/v1/workbench/sourcing/1688/acquisitions/verify
GET  /api/v1/workbench/sourcing/1688/acquisitions/:operation_id
```

POST bodies contain exactly `{"source":"<offer ID or canonicalizable URL>"}`
and a single canonical UUID `Idempotency-Key`. Every action requires verified
identity, Effective Organization, and live `product_sourcing.write`. No caller
org/actor/roles/base version/evidence/envelope is accepted.

The existing Auth.js BFF exposes corresponding `/api/workbench/...` paths. It
uses its server token, the existing organization selection cookie, and explicit
`X-Expected-Organization-ID` / `X-Expected-User-ID` assertions. POST requires the
configured trusted application origin. No browser Authorization/Cookie is forwarded
to 1688. `LISTINGKIT_SERVICE_API_BASE` must point to the RUN-1 `/api/v1` root and
the existing public-app-origin configuration must match the browser origin.

`src/lib/api/product-acquisition.ts` accepts an explicit immutable operation
`{key, source, userId, organizationId}`. The caller retains it across retries.
The client never generates a replacement key or automatically retries a mutation.
After response loss use `verify1688` with that original operation. Catalog version
is a decimal string; `published` requires the exact verified publication binding.
Unknown remains HTTP 503 `OUTCOME_UNKNOWN`. GET projects publishing as
`outcome_unknown` without changing state. Error bodies never expose SQL/raw HTML.

## Verification and limitations

Use only task-owned PostgreSQL for `ISSUE398_TEST_DSN`; integration helpers require
loopback and `issue398_owner`, create unique databases, close pools and drop only
those databases. `ISSUE378_TEST_DSN` enables the existing SRC-1 regression fixture.
No positive test seeds final Product/SRC-1 tables. Fault injection commits real
PostgreSQL writes before deliberately losing the acknowledgement.

```text
go test -race -p 1 ./internal/product/sourcing ./internal/integration/acquisition/a1688 ./internal/integration/persistence/product/acquisition ./internal/app/productsourcing
go test -race ./internal/app/httpapi -run "TestAcquisitionMounted|TestProductAcquisitionHTTP|TestCurrentApplicationMountsExplicitAcquisition"
go test ./internal/app/runtime/currentapplication ./cmd/current-application ./cmd/product-acquisition-init
```

The mounted tests exercise real current HTTP middleware with fixture identity and
live-grant adapters, real fixture HTTP and real PostgreSQL. They do not claim a
production IdP login, real 1688 acceptance, deployment or browser-capture ingress.
Browser payload ingestion and the #399 handoff are separately planned work.

Legacy decision: EXTRACT. Reused behavior is the static public field projection;
its current owners are the new adapter and Sourcing mapper. No old service,
browser/profile, task/queue/worker or compatibility dependency remains in this path.
Removal of unrelated registered legacy code is not part of this slice.

## Explicit operational wrapper

`scripts/product-acquisition-init.ps1 -Config <absolute-private-manifest> -ConfirmEmptyDatabase <exact-empty-database>` delegates to the initializer and propagates its exit code. Both arguments are mandatory; it does not provision a database/role, load default credentials or initialize implicitly.

The exact command/import/API registrations are admitted by Issue #398 comment 5643032970 and authorized to this unique writer by PM task `01a076b0-cad0-7c90-a02d-7213378d82e9`. Historical importer ceilings remain 21/8; the two precisely named CURRENT edges are independently guarded, not a CI or scope-threshold waiver.

# Completed local preparation work

This read-only projection was introduced by Issue #340 and consumed by the task
center in #328. Issue #36 updates its current Store attribution contract to v2;
the bounded design is in `docs/architecture/issue-36-completed-work-store-scope.md`.
It is not a BusinessTask, AgentRun or generic producer registry.

## Source and meaning

The explicit SHEIN record application is the only producer. Its real Listing
Create service and PostgreSQL transaction insert a record with a unique
`(organization_id, owner_user_id, operation_id)` key. Retrying that operation
returns the same committed record. The isolated collection has no historical
imports or backfills. Within that approved source, a visible record proves
`local_record_committed`; no extra operation lookup, payload read or table is
needed. This does not prove review approval, publishability, remote publication
or completion of a broader business process. The validated immutable Input
includes the historical Store ID and `save_draft` or `publish` preparation
action; neither action is a remote submission receipt. Product title Review
and Apply are a separate Product Review/Catalog flow sharing the task-center
navigation. Production admission is unchanged.

## Request and trust boundary

`GET /api/workbench/completed-work?source=listing-local-preparation&limit=20`
accepts only that source plus the existing opaque `cursor` and bounded `limit`.
Unknown statuses, search, Store filters and statistics are unsupported (400),
not successful empty results. Raw query is limited to 1 KiB and checked before
removing the source parameter; remaining raw bytes go to the original validator.

The shared SHEIN GET handler retains Auth.js server session/token, effective
Organization cookie/expected-org assertion, fixed origin/path, header allowlist,
manual redirects, no-store, strict JSON and current Go read authorization.
Go scopes each page and cursor anchor by organization/owner; admin only has its
existing organization scope. CachedRead semantics and revocation behavior are
unchanged. Non-200 responses, including cleared cookies, are returned unchanged.
The historical Store ID already in authorized Listing metadata does not grant
`workbench.store.read`; this projection does not fetch Store name/current status.

The same 15-second BFF deadline covers authentication, upstream and projection;
there is no reset for mapping. Abort propagates throughout. Both input and mapped
JSON have a 128 KiB actual-byte bound. No background work, writes, recovery,
compensation, new idempotency key or transaction is added. Repeated reads project
the same committed records; pagination remains the original keyset contract.

## Consumer contract

`src/lib/api/completed-work.ts` exports `CompletedWorkItem`, `CompletedWorkList`,
`CompletedWorkFailure` and `parseCompletedWorkList`.
`src/lib/api/completed-work-client.ts` exports `fetchCompletedWork` and
`CompletedWorkError` (an alias of the existing `SheinRecordListError` class).

`fetchCompletedWork({organizationId, limit?, cursor?, signal?})` returns
`Promise<CompletedWorkList>`. Client input schemas, strict JSON and failure
decoding are reused. Errors expose `status`, `code`, `payload`; a server timeout
is HTTP 504 / `DEADLINE_EXCEEDED` (existing lower-level deadline errors retain
their existing code). Client cancellation retains the signal reason, usually
AbortError, and does not become an empty list. No new client timer is installed.

The list has `projection_version: "2"`,
`coverage: "listing-local-preparation-only"`, `items`, and `next_cursor`.
Empty pages still carry coverage. Items contain only the original metadata
(with `source_record_id` and exact string `snapshot_version`), fixed source
kind/type, title/summary, SHEIN platform, `work_scope: "store"`, canonical
`store_id`, `action: "save_draft" | "publish"`, and completion basis. Missing or
invalid Store/action facts fail closed; there is no `general` fallback. There is
no task_id, Store name/status, remote submission result, count or progress.
`result.href` must equal `/workbench/shein-records/{source_record_id}/diagnostic`
for the same validated UUID. Opening that result always reauthorizes; appearing
in the list grants no future access. The mistaken draft URL has no compatibility
route. #328 owns pages, navigation and selection; this module owns DTO/BFF/client.

## Verification and isolated consumer fixture

From repository root, after frozen frontend dependency install:

```text
node web/listingkit-ui/scripts/shein-diagnostic-fixture.mjs --completed-work
node web/listingkit-ui/scripts/shein-diagnostic-fixture.mjs --serve --completed-work
```

The first command performs original collection/diagnostic checks plus projection
assertions. The second serves for consumer verification and prints a manifest
path; it does not itself claim those acceptance assertions passed. To run a
consumer checkout, add `--web-dir <absolute-consumer-web-directory>` only after
it includes the actual projection implementation. Go is built from the launcher's
repository, and the manifest identifies the selected frontend directory.

The launcher creates its own loopback Docker PostgreSQL and random schema, builds
real Catalog Publisher + POST records (including same-operation retry, multiple
owners/orgs and >1 page), then checks unchanged business tables/xmin on exit.
Auth.js session issuance and external identity/grant providers are synthetic;
Auth.js decryption, server token, middleware, Go/SQL and diagnostic are real.
The manifest contains synthetic session material; do not publish it in PRs.
Create `stop-fixture` in its `controlDirectory` for normal verified cleanup, or
send SIGINT. Maximum serving lifetime is 30 minutes. A stopped fixture has no
live URL. No real IAM, production schema/wiring/data, model or deployment is used.

The original #340 narrow review reached IMPLEMENTATION_READY; the current #36
Store projection design was separately reviewed and admitted. Query preservation,
shared deadline, revocation cookies, actual byte bounds, source/href binding and
real PG read-only/replay proofs are IMPLEMENTATION_TEST requirements, verified
in unit/route tests and the shared launcher. Original modes remain available.

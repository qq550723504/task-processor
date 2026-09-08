# Source Account / 1688 Organization Ownership Cutover

Baseline: main `2fd42cc06`; related issues: #30, #301, #300.
Status: IMPLEMENTATION_READY (A, independent review round 1, frozen after correction);
B–D require their implementation evidence before rollout.

## B1 prepared-only transaction contract (Issue #362)

Status: IMPLEMENTATION_READY (independent review round 2). This increment narrows the first part of B to
the source-account PostgreSQL transaction kernel. It does not reopen A and does not
authorize a real migration, reader cutover, route change, deployment or data operation.
The clean-slate decision in
[`docs/product/issue30-clean-slate-cutover.md`](../../product/issue30-clean-slate-cutover.md)
supersedes the older B text below wherever it requires Product, Asset, Task or Redis
result migration. B1 preserves source accounts and profile/login-state references only.

### Narrow inventory and ownership

- `internal/sourceaccount/gorm_repository.go` is the only production model for
  `source_account`. Repository code reads 1688 rows and the two schema entry points
  only call its `AutoMigrateRepository`; the repository contains no production
  account writer. No second platform writer for this table was found in the current
  tracked Go/SQL tree. Non-1688 rows are nevertheless treated as protected shared
  rows because the table has a platform discriminator.
- The live 1688 repository, profile resolver, HTTP service and old #30 handoff still
  use numeric `TenantID`. B1 does not change those callers, their interfaces or their
  runtime wiring. `internal/tenantbridge` gains no consumer.
- Source Account remains the one business owner. The A receipt and the B1 database
  receipt are migration evidence, not repositories and not authorization facts.
  The legacy `source_account` representation is migration input only; the new target
  does not extend, wrap or share its numeric ownership schema.
- The tracked tree has no second platform writer, but that does not make the legacy
  row the correct target: its model and every live caller are defined by numeric
  `TenantID`. Adding a dormant Organization column there would let the retired reader
  constrain the new design and create a compatibility-shaped dual representation.
  B1 therefore writes `public.organization_source_accounts`, the dedicated eventual
  1688 Source Account persistence owner. Its frozen columns are `id BIGINT PRIMARY
  KEY`, nonempty `organization_id VARCHAR(128)`, canonical `platform VARCHAR(32)`
  constrained to `1688`, nullable `label VARCHAR(128)`, nonempty original
  `profile_ref VARCHAR(256)`, nonempty verified `profile_directory VARCHAR(1024)`,
  nullable `proxy_ref VARCHAR(256)` and `login_url TEXT`, non-null `status SMALLINT`
  and `deleted SMALLINT`, nullable `last_verified_at TIMESTAMPTZ`, and non-null
  `created_at`/`updated_at TIMESTAMPTZ`. It has no numeric tenant column. The exact
  A-verified directory is a target account field so C never derives a path from the
  retired tenant ID and never treats the migration receipt as a runtime repository.
  The scoped reader index is
  `(organization_id, id, status, deleted)`; the primary key prevents one legacy
  account identity from being copied under two Organizations. It is not a mirror or
  fallback table. Until C it is prepared but unreachable from runtime; C performs one
  reader switch and D retires the legacy table/path after its approved cutover. There
  is no dual read, dual write, bidirectional synchronization or request-time conversion.
- The target schema is 1688-specific because the legacy table is platform-shaped and
  other-platform rows are protected. B1 reads only normalized platform `1688` from
  the legacy source and never copies or alters other-platform rows. A later inventory
  finding another writer cannot be solved by adding fields or adapters to the legacy
  schema; it must stop B2 and preserve the isolated current-owner target.
- The repository has no shared cross-domain Unit of Work. B1 reuses the package's
  existing `database/sql` + pgx PostgreSQL boundary and executes target changes and
  the receipt insert in one local transaction. It does not create a generic UoW,
  Saga, journal service or runtime migration framework.

### Request, before-image and target schema

The internal B1 operation takes an explicit `(contract_version, idempotency_key)` and
one complete, valid A `preflight_only` receipt plus the explicit account-source identity
selected by the operator-facing B2 composition. That identity must exactly equal A's
`AccountObservation.SourceID`; `current_database()` must equal A's observed database,
and all B1 SQL is fully qualified to `public`. This binds the core to A's selected
database/schema evidence without adding database discovery; B2 still owns the external
environment/authority attestation. The A version, stage, non-atomic marker and digest
are revalidated without changing A's canonical digest or its filesystem rules. The
request fingerprint is a canonical SHA-256 over the B1 contract version, source
identity, A digest and ordered account mapping/profile evidence; observation timestamps
are not new identity inputs. An empty/over-128-byte key, mismatched source identity or
database, invalid A receipt, empty account set, duplicate account, missing Organization,
or more than A's `MaxRows` fails before mutation.
Before hashing the A receipt, B1 measures its exact canonical JSON size one bounded
record at a time and rejects more than 64 MiB; JSON escaping is therefore included
without first materializing the whole encoded receipt. After taking the source lock,
it asks PostgreSQL for the 1688 row count, largest variable-width field and aggregate
variable-width bytes before selecting any text value. A field over 64 KiB or raw source
text over 64 MiB fails without entering process memory. Once selected, source and target
records are likewise measured one at a time and an encoded snapshot over 128 MiB fails
before whole-snapshot digest marshaling. Prepared-result JSON has the same 128 MiB
pre-marshal bound. Its INSERT returns `octet_length(result_json::text)` and the receipt
reader checks that identical persisted JSONB metric before selecting the value, so an
accepted receipt cannot reject its own replay because of a different size convention.

Inside the PostgreSQL transaction B1 locks and rereads the complete current 1688
inventory, ordered by account ID. Its before-image includes every current non-secret
column: ID, legacy tenant, platform, label, `ProfileRef`, proxy reference, login URL,
status, deleted flag, last verification time and creation/update times. The set and
the A-captured fields must exactly equal the supplied A evidence. Added, removed or
changed source rows fail closed. B1 computes and persists a canonical full-source
digest; it never reads credentials, cookies or profile contents.

Each target row contains account ID, mapped Organization, fixed platform `1688`, label,
opaque `ProfileRef`, exact A-verified profile directory, proxy reference, login URL,
status, deleted flag, last verification time and original creation/update times. It
deliberately has no legacy tenant column.
Account ID is globally unique in the target and `(organization_id, id)` is the
reader scope. A pre-existing target without this operation's matching committed receipt
is a conflict even when its fields equal the desired values; B1 never adopts an
unexplained partial copy. The target digest covers the Organization and complete
preserved non-secret account fields. The locked legacy source rows are not updated.

The dedicated schema artifact creates `public.organization_source_accounts` and
`public.source_account_ownership_migration_receipts`. The receipt table has one
immutable terminal row per `(contract_version, idempotency_key)`, enforced by a
composite primary key. It stores stage `prepared_only`, request fingerprint, A digest,
source identity/database/schema, source digest, target digest, account count, canonical
JSON result and transaction timestamp. Checks require version `1`, a 1–128 byte key,
stage `prepared_only`, 64 lowercase-hex digests, `public` schema and an account count
between 1 and A's `MaxRows`. The concrete columns are `contract_version SMALLINT`,
`idempotency_key VARCHAR(128)`, `stage VARCHAR(32)`, `request_sha256 CHAR(64)`,
`preflight_sha256 CHAR(64)`, `source_id VARCHAR(256)`, `source_database VARCHAR(128)`,
`source_schema VARCHAR(63)`, `source_sha256 CHAR(64)`, `target_sha256 CHAR(64)`,
`account_count INTEGER`, `result_json JSONB` and `prepared_at TIMESTAMPTZ`. There is
no externally visible
`pending` or `failed` receipt: all target inserts and the prepared receipt commit
together, or neither is visible. Schema creation is an explicit internal/test call;
it is not added to startup `AutoMigrate`, CLI, HTTP or worker assembly.
The installer creates and validates the schema in one transaction, and B1 revalidates
the exact target/receipt columns, named constraints, primary keys and scoped reader
index while holding table locks. A pre-existing same-name table with any extra or
missing column—including a nullable numeric `tenant_id`—fails closed; it is never
adopted as a compatibility-shaped target.

### Idempotency, concurrency and failure semantics

1. B1 requires a caller deadline with at most ten minutes remaining, keeps the A
   100,000-row cap, enforces the fixed request/source/receipt byte budgets above, and
   sets transaction-local PostgreSQL lock/statement timeouts. Cancellation is checked
   before the transaction and throughout bounded row work.
2. Lock order is fixed. The transaction first takes PostgreSQL `SHARE` on
   `public.source_account`, which conflicts with legacy INSERT/UPDATE/DELETE and closes
   the phantom-row gap before the complete ordered 1688 reread. It then takes
   `SHARE ROW EXCLUSIVE` on `public.organization_source_accounts` and
   `public.source_account_ownership_migration_receipts` in that order before deciding
   first execution versus replay. The latter lock self-conflicts, so concurrent B1
   writers serialize; target account and receipt primary keys remain final guards.
   A legacy writer already holding a conflicting lock completes first, after which B1
   rereads and rejects the changed A set. A writer arriving later waits until B1 has
   committed or rolled back. B2 still must keep the external freeze after commit.
3. A committed receipt with the same key and fingerprint is returned only after the
   current source and target digests/rows still match it. No rows are rewritten.
   Same key with a different fingerprint is an idempotency conflict. A different key
   encountering a prepared target is a target-ownership conflict, not another receipt.
4. A source-set/field difference from A or the committed receipt is source drift. A
   changed/missing Organization or changed preserved field after preparation is target
   drift. Either blocks replay; a fresh A digest cannot bypass an existing target.
5. Any validation error, guarded-update miss, injected mid-transaction failure,
   deadline or cancellation before successful COMMIT rolls back both target and
   receipt. B1 has no automatic transaction retry because an ambiguous COMMIT result
   must not be converted into a blind second mutation attempt.
6. A successful COMMIT is the state boundary. Cancellation observed afterward does
   not turn durable prepared facts into rollback. If the caller loses the COMMIT
   acknowledgement or an injected post-commit response fails, it opens a fresh
   context and calls the receipt read API with the same identity/fingerprint. That
   API validates the persisted receipt plus source/target equality and determines the
   outcome without writing. Missing evidence means unknown/not prepared; it does not
   authorize an automatic replay.

The receipt lifecycle ends at `prepared_only`. It is not rollout-ready, does not
certify current ZITADEL projection freshness, writer/admission freeze, multi-host
profile agreement or live account authorization, and is not consumed by production.
B2 must supply those operation-gate facts and explicit environment authority before
invoking this kernel. C remains responsible for Organization reader/access semantics
and exact profile-directory reuse; B1 only preserves `ProfileRef` and the A-verified
directory evidence without filesystem I/O.

### Verification allocation and invariants

The implementation must first demonstrate RED tests, then GREEN against a disposable,
task-exclusive PostgreSQL instance. The real database suite must prove:

- enabled, disabled and deleted 1688 inputs produce Organization-only target rows with
  every non-secret field preserved and no numeric tenant; the legacy rows remain byte
  for byte unchanged, the A `ProfileRef` and verified profile directory remain exact
  evidence, and B1 performs no filesystem read/write/create/delete operation;
- non-1688 accounts, unrelated 1688 accounts in rejected requests, and unrelated
  tables remain unchanged;
- invalid/missing/ambiguous A evidence, source drift, unexplained target ownership,
  same-key/different-payload and committed target drift all reject without partial
  changes;
- same-key/same-payload replay returns the same persisted JSON/timestamp without a
  second update, while concurrent identical calls produce at most one legal prepared
  result and every other successful call reads that result;
- a fault after at least one target insert, a pre-commit cancellation and a database
  error roll back the whole transaction; a post-commit injected response loss is
  resolved by a new service/read context and the durable receipt;
- schema installation is repeatable only for the exact frozen schema; missing or extra
  columns/constraints, a changed primary key/index or a legacy numeric owner column
  reject before account/receipt mutation;
- real concurrent legacy INSERT, UPDATE and DELETE transactions either complete before
  the B1 table lock and make the A set fail closed, or wait until B1 finishes; no
  phantom row can be omitted from a committed receipt;
- PostgreSQL rejects an over-limit source field and over-limit aggregate source text
  before target/receipt writes; escape-heavy A/source/target JSON also rejects before
  a whole digest/result marshal, and the persisted receipt limit is identical on write
  and read rather than relying on the row cap or raw text length as a memory bound;
- normal and failure paths drop their task-specific schema/database and stop/remove
  disposable PostgreSQL resources. Mock, SQLite, compile-only and in-memory tests do
  not count as this evidence.

Adjacent regression checks must show no changes to the production Source Account
reader, 1688/Amazon shared task protocol, #30 route/handoff, A digest/`preflight_only`
semantics, startup migration list or tenantbridge/Legacy guards.

Legacy decision: EXTRACT. Reusable behavior is A's validated mapping, source-account
before-image/status/profile evidence and exact replay/conflict semantics. Current owner
is the Organization-only 1688 Source Account target plus its offline PostgreSQL
ownership-migration adapter. The cutover condition is a separately authorized B2
operation gate followed by C's one-time reader switch and D numeric-owner retirement;
B1 adds no compatibility path and new code never depends on the legacy model.

## Authority and scope

Read AGENTS.md, #30/#301/#300, legacy-hard-cut-policy.md, legacy-register.md,
and approved `2026-09-01-internal-target-architecture-phase3-product-design.md`.
ProductSnapshot and ApprovedAsset remain Product facts; this work introduces no
Product route, handoff, approval, marketplace submission, IAM, UI or other crawler change.
Legacy decision: EXTRACT -> RETIRE. Extract access checks, profile reuse and valid
crawler result semantics into Organization identity, source-account and 1688 owners.
Retire numeric ownership and 1688 bridge imports at reader cutover.

Must: effective verified Organization ownership; preserve durable data and existing
profiles; fail closed on missing/ambiguous mapping; repeatable migration receipts;
account disabled/deleted and cross-Organization checks. No numeric equality inference,
request resolver, fallback, permanent dual read/write or new tenantbridge consumer.
Threat model: wrong ownership, stale/removed Organization metadata, profile aliasing,
concurrent migration/writers, process loss and old job replay. No new IAM or generic
recovery framework. No assumption that production contains no old jobs.

## Repository inventory

| Production path | Current ownership / persistence | Cutover owner/action |
| --- | --- | --- |
| `internal/sourceaccount/account.go`, `repository.go`, `gorm_repository.go` | int64 TenantID; `source_account` SQL rows, scoped access; disabled/deleted checks | sourceaccount OrganizationID-only contract and exact scoped SQL |
| `internal/sourceaccount/bootstrap/repository.go`, `internal/listingkit/schema/runtime.go` | legacy table AutoMigrate, two schema entry points | explicit migration, no startup backfill |
| `internal/crawler/alibaba1688/account_profile.go` | tenant/account derived directory; ProfileRef only checked nonempty | verified opaque runtime directory; never derive directory from OrganizationID |
| `browser_manager.go`, `processor.go`, `worker_processor.go` in same package | profile directory passed to browser; tenant/account process lock | preserve exact directory; reject absent/changed reference; no implicit creation on migrated-account path |
| `api_service.go` | tenantbridge plus bootstrap; identity.TenantID conversion | verified EffectiveOrganizationID, remove both imports/configuration |
| `crawler_service.go`, `worker_processor.go` | shared numeric task/result; process worker queue; result saved before enqueue; updates can use unscoped path | 1688-owned versioned Organization task/result protocol, scoped update/read/delete, fail closed |
| `internal/crawler/shared/{task,result,base_service,job_handler}.go` | shared with Amazon; memory map and Redis `crawler:1688:task-result:tenant:<id>:<task>`; unscoped keys also possible; default TTL 6h | extract only required 1688 behavior; leave Amazon/shared contracts unchanged |
| `internal/infra/httpx/crawler_1688_handler.go`, `base_crawler_handler.go`, `crawler_service.go` | numeric resolver and shared handler/service interface | 1688-only Organization handler/service contract; shared Amazon path unchanged |
| `internal/app/httpapi/{crawler_1688_module,composition_builder,feature_module_builders}.go` | repository assembly and validator type assertions | switch construction once C ready |
| `internal/compatibility/listingkit/sourcehandoff/a1688/command.go` | numeric source-account validator and tenantbridge | #30 owns retirement; cannot accept new numeric wrapper to keep this consumer working |
| `internal/platform/workerpool/pool.go` | process-local queue/execution, not a durable inventory | freeze/drain each live instance before rollout, retain terminal evidence |

Inventory commands: `rg -n 'sourceaccount|tenantbridge' internal -g '*.go'`;
`rg -n 'TenantID|ProfileRef|ProfileDir' internal/crawler internal/sourceaccount`.
At baseline 16 production files import tenantbridge or its bootstrap (including
tenantbridge/bootstrap itself). A adds zero consumers; C must remove 1688 imports.
#300 owns the global import guard; no parallel baseline rewrite here.

## Contract and target schema

Business account: `ID int64`, `OrganizationID string`, existing platform/status/deleted
and non-secret metadata, opaque `ProfileRef`, and the verified runtime profile directory
carried by the current owner rather than derived from legacy ownership. Numeric source ownership exists only
in migration input/receipt, never in the new business contract. Account identity is
OrganizationID + account ID. Repository Get/Validate require both, recheck the returned
owner, reject empty Organization, unavailable/deleted/wrong owner and disabled accounts.
Verified effective Organization comes from existing authidentity/authz, never a header
or parse of TenantID. Worker checks current account access immediately before browser use.
No account/Organization ownership cache; revocation uses current checks.

C target SQL is `public.organization_source_accounts`, the isolated target frozen by
the B1 contract above. It contains nonempty Organization ownership and the preserved
non-secret account/profile fields, but no numeric tenant. The legacy
`public.source_account` row is read-only migration input and is never extended to keep
its reader working. C can only switch once to the isolated target; D then retires the
numeric table/path. Do not apply speculative live DDL in A. A defines the migration
receipt schema in Go/JSON; it is audit input, not a second source-account repository or
runtime authority.

1688 version-2 tasks/results carry OrganizationID and source account ID; Redis scope
uses a collision-free encoded Organization component. Numeric legacy JSON must be
rejected, including jobs with both old and new ownership fields. No payload upgrade
on the request/worker path. Account-free public jobs still require Organization scope.

## Safe independently verifiable slices

**A (this PR): contract and read-only migration preflight.** A standalone command reads
`source_account` and `projections.org_metadata2` via two explicitly configured connections,
each in its own repeatable-read read-only transaction (business DB and ZITADEL DB differ),
bounded to 100,000 rows per collection, with a deadline. It uses the key `yudao_tenant_id`
directly, without importing tenantbridge. Duplicate metadata for one Organization is
ambiguous (including conflicting/removed rows); multiple live Organizations for the same
legacy value are ambiguous; malformed metadata blocks. Removed owners never map.
No numeric equality inference. Query all 1688 rows, including disabled/deleted, preserving
their flags. Verify each existing derived profile directory using the operator-specified
actual runtime root; reject absent directories, symlinks/aliases and shared resolved paths.
Keep old ProfileRef and actual verified absolute directory separately in the receipt.
Never read cookies/profile contents or create/move/delete browser directories.
Receipt records operator-specified source identities, database names and observation times;
it explicitly declares the two snapshots non-atomic. Its deterministic digest covers the
source identities, database names and sorted mapping/rows/profile references, excluding
observation timestamps so the same evidence can be compared across restarts.
Receipt has version,
and explicitly says `preflight_only`, not rollout-ready. Rerun is a fresh read; crash
has no database effect. Publish receipt only after all validations; an existing receipt
file is not overwritten. Different input gives a different digest. This command does
not establish projection freshness or certify all runtime volumes: B must do that.

**B: account preparation + operation gate.** B1 is exactly the prepared-only PostgreSQL
kernel frozen above: legacy account rows are locked/read, Organization-only target rows
and one terminal receipt commit atomically, and no runtime reader changes. B2, in a
separate task, must freeze source-account writes and Organization mapping changes; verify
the explicitly selected authoritative ZITADEL instance, projection watermark and current
Organization existence/removal; reread A evidence under that freeze; and prove all browser
hosts/volumes agree with the receipt. Same key/different payload, changed source/target
and unexplained target rows fail closed; lost response is resolved by the committed
receipt rather than an upsert or blind retry. Filesystem access remains read-only.

Per `PD-ISSUE30-CLEAN-SLATE-2026-09-05`, B does **not** migrate, export, checksum, CAS,
retain or replay old Product/Asset/Task/Redis terminal results and does not build a SQL/
Redis journal. Old-job admission/replay rejection and writer quiescence remain later
cutover prerequisites where required by C/#30, not B1 persistence work.

**C: reader cutover.** Requires validated B receipt, zero active legacy work with every
instance attested, and no old writers/retry owners. Frozen admissions stay frozen until
all instances run the new version. Old-job submission/replay fails before browser or result
effects. Unknown in-flight outcome blocks rollout; do not delete or silently resubmit jobs.
Retain old terminal evidence through its retention window. #30 handoff retirement must
be coordinated with its owning PR: if it still requires numeric Repository, C is blocked,
not solved by a wrapper. The #30 route is untouched by A/B. C changes only 1688 consumers,
not shared Amazon types. Before reopening, test wrong org, revoked identity, disabled/deleted
account, result collision/isolation, legacy job rejection, and exact existing-profile reuse.
Migrated profiles missing at use time fail closed; do not create an empty replacement.

**D: retirement.** Remove unused numeric 1688 fields/keys/schema entry points and artifacts
after validation/retention approval. No physical deletion of unrelated rows or profiles.
Rollback after C cannot start old binaries against new writes: freeze and recover forward
from receipts/backup; rollback before C can discard staged migration outputs under freeze.

## Findings and stop conditions

Independent review round 1 found a BLOCKER for A (core happy path cannot complete):
metadata bootstrap selects a distinct `zitadel_auth` / `zitadel` database, so a single
business-DB transaction cannot read both tables. Corrected to explicit two connections,
separate read-only snapshots and non-atomic evidence. No automatic database discovery.
Reviewer confirmed this correction permits A to reach IMPLEMENTATION_READY.

Finding: ProfileRef is not the actual runtime path today.
Product requirement affected: existing profile reuse.
Classification: BLOCKER (data loss/unsafe migration).
Reason: replacing the derived path with the stored placeholder could launch a fresh profile.
Action: A records both references and verifies the actual directory; C reuses it unchanged.

Finding: Redis alone does not inventory process-local work; enqueue can fail after result save.
Product requirement affected: safe old-job cutover.
Classification: BLOCKER (rollout cannot safely complete).
Action: B/C need per-instance quiescence plus results reconciliation; A receipt cannot clear it.

Finding: Old #30 validator consumes numeric Repository.
Product requirement affected: no compatibility layer and preserve old handoff in this task.
Classification: BLOCKER for C (core path/rollout cannot complete independently).
Action: safely split A; coordinate C with #30 owner instead of adding an adapter.

Finding: SQL snapshot cannot prove external projection freshness or filesystem atomicity.
Classification: IMPLEMENTATION_TEST for A; mandatory B rollout evidence.
Action: label preview only; B freeze/current-authority and multi-host checks are required.

## Validation allocation

A TDD: no numeric fallback, missing/ambiguous/removed/malformed mapping, duplicate accounts,
stable digest after restart/reordering, changed mapping changes digest, disabled/deleted
flags retained, exact profile path reuse and no filesystem mutation, missing/alias paths
rejected; read-only transaction/rollback and row limits. Command only emits a receipt on success.
B1: PostgreSQL target/receipt atomicity, full source/target digests, idempotency conflicts,
source/target drift, legacy-writer and B1 concurrency, cancellation and lost-response readback.
C: wrong Organization, disabled/deleted, authorization revocation, cross-Organization result
isolation, legacy job rejection, real browser profile reuse, no bridge imports; Amazon regression.
D: retirement/import guards, retained business assets and migration receipts.
Production migration/acceptance is not claimed by local A tests.

## Slice A implementation evidence (2026-09-05)

Independent review verified the two-database correction and implementation; no new
A BLOCKER. Mapping/profile tests, snapshot transaction tests and receipt publication
tests each failed before their corresponding implementation, then passed.
Windows junction rejection was exercised successfully (the original symlink-only
test required privileges and was replaced with the junction case on Windows).
Concurrent publishers produce exactly one complete receipt; tampered/partial receipts
are refused. Process-kill/power-loss injection is not claimed.

A local disposable PostgreSQL 16 container with separate `source_business` and
`organization_authority` databases verified the real CLI: one disabled/deleted 1688
record retained, Amazon excluded, identical digest on restart, and an added ambiguous
Organization mapping rejected without a final receipt. Container stopped afterward;
no production database, browser or worker was accessed.

Focused migration tests, `go vet` for migration/CLI and existing sourceaccount,
alibaba1688 and shared crawler tests passed. Full `go test ./tests -count=1`
reported three existing documentation guard failures, reproduced independently
in a clean detached worktree at the same main baseline:

- `TestPhase2ClosureDocumentsRuntimeOwnershipAndDeferredDebt`
- `TestCommerceToolBoundaryDocumentsDefineNeutralRegistryOwnership`
- `TestCommerceToolCanonicalInspectionGovernanceIsRecorded`

CI follow-up: these assert omitted but still-valid text in `module-target-mapping.md`
and `project-target-architecture.md`. Reclassified IMPLEMENTATION_TEST for CI closure:
restore current owner and Commerce Tool governance documentation without restoring a
compatibility layer. The three focused guards pass after restoration.

CI also exposed `TestCmdContainsOnlyOfficialEntrypoints`: the new command was not
registered as an operational entrypoint. Classification: IMPLEMENTATION_TEST. Register
it in both command/category and operational-owner checks and maintain
`scripts/source-account-ownership-preflight.ps1`. This guard scans Git-tracked files,
so pre-staging tests missed the newly added command. Follow-up verification stages new
files before running the guards. All four failing guards and the operational-owner
guard now pass; script argument forwarding, error propagation and cwd restoration
were verified independently.

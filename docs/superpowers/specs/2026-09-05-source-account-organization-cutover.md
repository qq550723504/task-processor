# Source Account / 1688 Organization Ownership Cutover

Baseline: main `2fd42cc06`; related issues: #30, #301, #300.
Status: IMPLEMENTATION_READY (A, independent review round 1, frozen after correction);
B–D require their implementation evidence before rollout.

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
and non-secret metadata, opaque `ProfileRef`. Numeric source ownership exists only
in migration input/receipt, never in the new business contract. Account identity is
OrganizationID + account ID. Repository Get/Validate require both, recheck the returned
owner, reject empty Organization, unavailable/deleted/wrong owner and disabled accounts.
Verified effective Organization comes from existing authidentity/authz, never a header
or parse of TenantID. Worker checks current account access immediately before browser use.
No account/Organization ownership cache; revocation uses current checks.

C target SQL is the existing source-account record with nonempty organization_id,
tenant_id removed for migrated 1688 ownership, and an Organization/platform index.
Because other-platform records must not be changed and old handoff cannot be wrapped,
B/C must choose an isolated 1688 target table if inventory finds other platform writers.
Do not apply speculative live DDL in A. A defines the migration receipt schema in Go/JSON;
it is audit input, not a second source-account repository or runtime authority.

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

**B: backfill + validation.** Before mutation, freeze source-account writes and Organization
mapping changes; verify the correct authoritative ZITADEL instance, projection watermark
and current Organization existence/removal. Re-read A evidence under the freeze. All browser
hosts/volumes must agree with the receipt. Freeze 1688 admissions and drain every instance
while keeping existing workers alive. Enumerate pending/running/retry/in-memory work and
Redis results, including unscoped keys. Unscoped/unknown owner is BLOCKER, never guessed.
Backfill SQL and receipt in one transaction (existing GORM transaction or database/sql),
keyed by migration version + account ID + source digest. Same key/different payload fails.
Persist checkpoints and per-row before/after counts/digests, including disabled/deleted.
Restart reuses committed receipts and verifies target equality; no upsert overwrites a
changed owner. SQL rollback is sufficient before commit; lost response re-reads receipt.
Across SQL/Redis use a bounded operator-driven migration journal, not a new Saga service:
export/checksum retained terminal results; CAS each exact source payload into version-2
namespace with remaining TTL (never extend retention), verify receipt, then retire old
key after C switch. No application dual write/read. Failed compare or expired evidence
requires a fresh inventory; no blind retry. Filesystem is read-only throughout.

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
B: transactional rollback/lost-response/restart, CAS conflicts, metadata drift, counts/checksums.
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

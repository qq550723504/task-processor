# AI工作台个人报告 v1

Execution: [#628](https://github.com/qq550723504/task-processor/issues/628), parent #137.
Status: IMPLEMENTATION_READY / frozen baseline (independent review, 2026-10-10).

## 1. Product outcome and authority

Current internal trial users can manually save an authorized real business result,
read that historical version, favorite/unfavorite it and download a reusable file.
The user confirmed on 2026-10-10: reports and favorites are private to the actor
within the current enterprise; saving captures the version at that time, read-only.
The 2026-10-09 stage decision admits previously placeholder menus after design.
It does not grant arbitrary analysis engines or external provider calls.

Figma Product Authority is `tg48P46SSXl6TBy9lZwg63`, page `31:463`, current visible
non-archived frames: overview `427:5695`/`427:5900`, recent `662:481`/`662:641`,
favorites `579:4677`/`579:4900`, all `579:5123`/`579:5346`.
These nodes were read live, including screenshot/context of `427:5900`.
See `docs/product/final-ui-ia-authority.md` and greenfield-no-legacy-migration.md.

Must:
- Private current-enterprise reports and favorites; no admin bypass of actor ownership.
- Explicitly save an exact authorized source version; immutable content and provenance.
- Recent / favorites / all, type filter, bounded search, detail and download.
- Truthful state at capture, source links, saved timestamp and historical labels.
- Retried writes recover with the same key and intent; conflicts cannot change content.
- Native empty installation and retained persistence; no serving DDL.

Out of scope: team sharing, new market/store analysis generation, BI, new Agent
execution, editing historical reports, arbitrary upload, Office/PDF conversion,
image material generation, generic files platform, migration/compatibility and new
acceptance infrastructure. Figma examples such as five shops, 42 reports, XLSX or
PDF are not data or promised formats. Current unavailable types say unavailable.
No newly accepted security risks. Scope is current authenticated internal trial.

## 2. Owner and existing paths

`reportcenter` owns only immutable presentation snapshots, private favorite metadata
and command receipts. It cannot be a Product/Review/Task/Listing/Validator owner.
Reports are historical exports, never used as current approval, title, readiness,
authorization or execution input. A source link enters the original owner page,
where existing freshness, permissions and explicit actions still apply.

First supported sources:
1. TITLE_REVIEW: `review.Service.Get(ctx,id)` and `review.ValidateView`; expected
   version is canonical `revision:state` (positive decimal revision and one of
   pending/accepted/rejected/applied), `View.Owner == actor`. Apply preserves the
   Review revision while adding its terminal state/receipt, so revision alone is
   not an immutable capture identity. Verify both fields; do not change Review.
   Capture a whitelist of title before /
   after, proposal state, product/base version, quality/unresolved/decisions and
   Apply receipt. Exclude knowledge content, raw model output and provider metadata.
2. SHEIN_RECORD: `record.Reader.ReadOfflinePackage(ctx, actor,id)`; exact stored
   `InputHash`, ID/org/OwnerUserID match; keep Product key/version, store/country,
   action, original rule/policy/package/diagnostic hashes and the validated saved
   DiagnosticResult. This is the persisted package summary and save-time offline
   diagnostic, not a new diagnosis or store operating report. Do not copy images,
   complete payload, credentials or externally hosted artifacts.

Source selectors reuse existing authorized pending-review collection, BusinessTask
review links and saved-record collection. A terminal Review remains saveable from
its exact detail even though the pending collection does not list terminal rows.
Capture widgets belong to the report page; existing task/Review pages need no new
mutation or permanent dependency on Report Center.

No changes to original owner contracts are necessary. New app adapter consumes
existing public narrow readers, not SQL from their databases or root legacy Service.

## 3. Contract to consumer

`reportcenter.SourceReader.Read(ctx,scope,kind,id)` -> app whitelist adapter
over original services -> `reportcenter.Service.Save` -> independent Report Store
transaction -> protected report HTTP routes -> feature-local same-origin BFF ->
Figma report pages. `SourceRef` is kind / canonical UUID / exact version only.
Clients cannot submit report title/body/owner/organization/content/provider URL.

Domain ports: Authorize(ctx,scope,read|manage); SourceReader.Read;
Repository.LookupCommand / Save / Read / List / SetFavorite / Summary.
Source checks use current original permission on capture. Saved historical content
is owned by the actor and readable through current report permission plus current
membership; source deletion/later changes do not destroy an authorized export.
Reports never confer access to the current source. Exported files contain no bearer,
private signed URLs or credentials. Owner provenance is validated at capture.

HTTP base `/api/v1/workbench/reports`: GET list, POST save; GET summary;
GET `/sources/:kind/:id` returns only the server-owned exact ref/title/Product/store
metadata after Report read, original source permission and actor/org checks, without
exposing the report body. Save re-reads via the same adapter and compares the full
selected ref, so a stale preview cannot silently capture a different version.
GET `/:id`; POST `/:id/favorite`. Every route uses VerifiedIdentity plus explicit
CachedRead or LiveWrite organization access. All saves/favorites are LiveWrite;
source capture receives the fresh identity produced by that boundary.
BFF `/api/report-center/[...path]` is local to this feature, reuses Auth.js server
session, trusted current-app origin and existing organization resolver/protocol.
Browser supplies expected org; BFF rejects context drift and strips browser identity.
Only allowlisted paths/queries/body/headers reach upstream, redirect=manual/no-store.

Permissions are `workbench.report.read` and `workbench.report.manage`; reports module
grant never grants LocalAgentWrite, listing permissions, models or external effects.
Shared authz owner registers them and retains the existing enterprise role policy.
Application factory takes source readers and report authorization explicitly;
missing Report DB fails startup when enabled, absent source disables that source.
Original sources remain optional and a saved report can be read without configured
model/Agent. Shared owner integrates the factory, not a parallel runtime.

## 4. Persistence and transaction

Dedicated logical Report database/pool with schema `report_center`, owner native
installer and non-owner runtime login. Serving checks expected tables/privileges,
does not run DDL. No SQL access to Product, Listing, Workbench or other owners.
Fixed qualified table names, transaction/request context deadlines, bounded reads.

`saved_reports`: UUID, org, owner actor, kind, source UUID, source version, title,
product key, optional store, source time (only when owner exposes one), captured
timestamp, schema-v1 JSON content, SHA-256 content digest. Unique org/actor/kind/id/
version prevents duplicate snapshots for the same exact source. Immutable: runtime
SELECT/INSERT only. Header/metadata max lengths and JSON content <=128 KiB.
`favorites`: compound FK to report ID/org/owner, bool target and timestamp.
`commands`: primary org/actor/idempotency UUID, operation, fingerprint, report ID,
favorite target where applicable; FK binds report scope. Immutable receipts.
Report/favorite/receipt are committed in the same transaction. No asynchronous job.
Every Save initializes the scope-bound favorites row in the same transaction.
Row locking of that favorites row (not immutable saved_reports) serializes favorite
set-target changes. Runtime gets UPDATE only on favorite/updated_at; PostgreSQL row
locks require UPDATE permission, which must not be granted on snapshot content. A duplicate
save sees the same source identity and must validate the content digest; one ID wins.
Use PostgreSQL conflict handling, not preflight-only duplicate checks.

Save protocol:
1. Validate scope/ref/key and live report manage authorization.
2. Lookup command under that scope. Same key/fingerprint returns saved record without
   recapturing a changed source; different intent returns 409. Authorization still
   executes on every replay; no source writes occur.
3. On new command, original source freshly authorizes, validates private ownership
   and returns bounded server-owned presentation content. Compare its full ref with
   the selected exact version before saving; mismatch returns 409.
4. Recheck report manage authorization as the transaction starts; persist snapshot,
   default favorite=false if first saved, and receipt atomically. An existing same
   version with divergent content is unavailable/conflict, never silently overwritten.
5. Return exact report and replay flag only after successful commit. Lost commit
   response remains unknown to BFF and is recovered by repeating the original key.

Favorite is set-target (not toggle), with UUID idempotency key and exact report ID.
Lock its scope-bound favorites row, scope-filter record, lookup command, update
favorite and append receipt.
Repeated old key never reapplies a prior target after a newer command; response reads
the current favorite. No lifecycle/delete/archive in v1, no automatic expiry.

Source read and report commit are separate boundaries without cross-db transaction.
Source version validity is checked on capture; later change does not invalidate that
historical snapshot. No latest/head comparison is needed at commit because the user
requested that version. Authorization is fresh at original read and report commit;
there is no cached write authorization or copied runtime status.

## 5. Failure, retry, restart and concurrency

- Denied/wrong org/actor: no capture/save/metadata; scope filters on all SQL paths.
- Missing source/version changed: 404/409 and no report/receipt. No fake empty result.
- Source unavailable/oversized/malformed: 503/413, no snapshot; saved reports survive.
- Source changes after capture: report retains the captured version and historical label.
- DB failure before/inside commit: transaction rollback. Response/commit loss: preserve
  original idempotency key, expose unknown and repeat it; do not allocate a new intent.
- Concurrent same key: PostgreSQL serializes uniqueness; one receipt/report, conflicting
  payload loses with 409. Concurrent distinct keys same source: one report identity.
- Favorites concurrent: row-lock serialization; last committed target is current.
- Restart/cancel: committed facts read normally; incomplete transactions roll back;
  no queued effect/reconciler/resend/new UNKNOWN owner is introduced.
- Token expiry/revocation/org switch: existing resolver and fresh write policies apply;
  client clears scoped data and commands, old scope recovery only when selected again.
- Report storage cannot write/update domain facts or report content; non-owner grants
  forbid DDL, schema create, memberships, destructive and cross-owner privileges.

## 6. UI, download and bounds

Reuse Console Shell/tokens, PageHeader/summary/three feature cards and existing table
components; preserve current Figma hierarchy/light and dark styles. Summary counts
come from scope-filtered stored reports/favorites (never current page length); labels
say saved, not generated, when the owner does not expose a generation timestamp.
Recent uses capture time and 30-day default. All/favorite lists sort capture time/id
and use keyset pagination, page <=50, no false global full-list claims.
Search title/source Product key (<=128 UTF-8 bytes); exact source-type filters.
Only actual saved store references contribute to distinct-shop summary; project
association is unavailable until the Project owner supplies its own projection.

Detail shows exact captured state, source/version/time and typed content. Downloads
are deterministic UTF-8 JSON and a plain text presentation of the same historical
document, using sanitized UUID-based filenames; client Blob download, no HTML or
macros, MIME nosniff. No native PDF/XLSX/DOCX/ZIP availability claims.
Source link is constructed from kind and canonical UUID, never stored arbitrary URL.
Open source uses the original business page; v1 reuse is download or source navigation,
never implicit reexecution. Labels/loading/error/empty/denied/unavailable remain real.

Request JSON <=2 KiB; strict duplicate/unknown fields, content type/encoding validation.
Reject bodies on GET before reads. Query <=1 KiB, only known keys, singleton values.
Timeout <=10 seconds including sources and DB. Body/detail response <=160 KiB, list
<=128 KiB, title <=256 bytes. No body content in list/summary. BFF uses the existing
bounded strict JSON reader and zod shape validation; mutations whose result is
unverified return outcome unknown and preserve original key/intent in browser state.

## 7. Shared ownership and rollout

Writer chat 01a11f6e-73ff-7692-a26f-6ca0251912b0, branch codex/ai-my-reports,
managed worktree ai-my-reports/task-processor, initial main 6db1bbc5529433d37b49708828b7d2b621bf9bc2.
Shared navigation/authz/catalog/currentapplication/initializer edits were requested
in #628/#137 and await coordination. Feature-local production code may proceed after
independent admission; shared paths remain untouched until ownership is confirmed.
One main PR. Native schema init and normal startup/handoff document are part of it;
runtime enablement is conditional on explicit config and owner injection.

Legacy decision: N/A. Qualified existing current owners are reused; root ListingKit,
legacy tasks/workspaces/wrappers, fallback, dual reads/writes and migrations are not
introduced. New installation only; no data operations against retained/shared DBs.

## 8. Verification and acceptance

TDD tests capture cross actor/org/denied source, exact version mismatch, content
immutability, digest bounds, idempotency conflict/replay and favorite set-target.
Real disposable PostgreSQL tests use existing testcontainers for atomic save/
concurrent keys/different source intents/favorite replay/reopen and least privileges.
Existing HTTP/BFF harness tests enforce auth/path/body/query/size/deadline/org drift
and unknown response loss. UI tests exercise manual save/detail/favorite/download,
empty/failed sources and org switching; reuse current libraries, no new test runner.
Compile/typecheck and focused checks, then exact-head CI on stable candidate.
Independent review checks new persistence/privacy boundary before code and final
complete feature once, with finding classifications per AGENTS (normal design review
max two rounds). Product/user acceptance and real-provider operation are NOT_RUN
unless explicitly executed by the user/designated verifier.

## Architecture review decision

Round 1: IMPLEMENTATION_READY; no unresolved BLOCKER. Review Apply capture identity
uses revision:state; favorites row locking preserves immutable snapshot privileges.
IMPLEMENTATION_TEST obligations: actor ownership despite original admin permissions,
atomic concurrent save/receipts, old favorite replay, restricted-role persistence and
complete wiring. Implementation and user acceptance remain NOT_RUN until executed.

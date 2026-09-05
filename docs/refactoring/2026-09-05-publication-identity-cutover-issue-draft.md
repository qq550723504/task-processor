# Draft prerequisite: Publication Identity Cutover

> **SUPERSEDED — historical analysis, not an execution prerequisite.**
> The draft below assumed retention of old publication/version/receipt history.
> A dedicated [Issue #307](https://github.com/qq550723504/task-processor/issues/307)
> was subsequently created; its current decision and
> [PD-ISSUE30-CLEAN-SLATE-2026-09-05](../product/issue30-clean-slate-cutover.md)
> cancel historical mapping, stream merging and cross-cutover old-version replay.
> The original “no dedicated Issue” statement and analysis below describe the
> original checkpoint only. Do not dispatch the cancelled migration work from it.
> Current scope is new import → new Catalog snapshot → explicit asset approval →
> readiness, with old execution isolation and separately authorized cutover.
> Account/profile, IAM, Store and financial/effect evidence remain protected.
> This replacement is a product decision, not a claim that migration was fixed,
> cutover completed, or real data deletion was authorized.

## Original draft (historical, retained verbatim)

Status: DRAFT FOR CONFIRMATION; not a created Issue and not migration approval.
Refs #30; preparation PR #304. Separate from source-account ownership #301/#303.

Repository issues (open and closed) were searched on 2026-09-05 for publication,
identity and cutover. No dedicated Publication Identity Cutover Issue was found.

## Problem and classification

BLOCKER: rollout/migration cannot safely complete. A no-SourceRunID retry crossing
the old/new handoff boundary gets a new publication ID for an identical snapshot.
Catalog correctly treats a distinct ID as a new publication/version. Lack of
production wiring isolates the prepared code but does not fix persisted identity.

`TestIssue30PublicationIdentityCutoverRequiresMigration` persists the historical
`source-snapshot:<raw checksum>` ID, closes/reopens file-backed SQLite, proves old
replay stays at version 1, then proves the new canonical-snapshot hash appends
version 2 with identical facts. Its initial safe-replay assertion failed (1 != 2).
The committed test characterizes the unresolved hazard; passing is NOT safe
cutover acceptance. Historical version 1 remains readable and new-ID replay is
stable at version 2.

## Bounded proposed scope

Owner: Product sourcing/Catalog cutover; application rollout owner coordinates
the one-time switch. Catalog remains the single durable Product fact authority.

1. Read-only inventory of persisted publication/product identities, immutable
   versions, source lineage, original payload availability and in-flight requests.
   Establish exact Organization ownership without numeric inference.
2. Freeze and independently review the identity mapping and conflict policy.
   Current approved preparation algorithm uses canonical snapshot plus lineage
   when no run is supplied. Do not restore runtime checksum fallback. Explicit
   runs are keys whose changed payloads must conflict.
3. Cover no-run with/without raw checksum, short/long explicit run, versioned
   source keys, long/Unicode product keys and missing evidence. The historical
   long-run formula hashes the run alone; the prepared helper hashes the prefixed
   key. Versioned product keys also need inventory. Do not assume all explicit
   runs or product keys survive cutover unchanged.
4. Define ambiguous/many-to-one/changed-payload conflict handling, immutable
   history/reference preservation, restart and cancellation checkpoints, concurrent
   writers, response loss and pre/post-switch retries. Missing reconstructible
   evidence blocks automatic rewriting.
5. Supply a separately approved, bounded migration/cutover plan with dry-run
   receipts, reconciliation counts, backup/restore evidence, writer drain and
   rollback/abort conditions. Only that later approval can authorize execution.

## Required acceptance evidence before production wiring

- Exact identity matrix and representative persisted-data inventory; explicit
  disposition of missing/ambiguous evidence and existing duplicates.
- Replay spanning old commit/new request after restart returns the original
  immutable publication/version; same key with changed facts or lineage conflicts.
- Concurrent old/new in-flight requests, response loss, cancellation and retry
  cannot allocate duplicate logical publications or cross Organization boundaries.
- Migration dry-run and restart/fault injection prove completeness, conflict
  handling, retained history/references, and reconciliation/rollback safety.
- Independent approval of cutover and source-account prerequisites, then controlled
  production-composition acceptance and replacement of the temporary no-wiring
  guard by legacy route/import absence guards.

Out of scope: source-account B/C/D, profile/ownership migration implementation,
new Importer, live DB mutations, automatic Catalog identity rewriting, runtime
numeric fallback, tenantbridge consumers, dual-read/dual-write or second authority.

Legacy decision: EXTRACT (deterministic identity and replay requirements).
Reusable behavior: stable publication, immutable history, Catalog conflict checks.
Current owner: Product sourcing/Catalog and application cutover.
Cutover/deletion condition: reviewed migration evidence plus account prerequisite
and controlled acceptance; retire old handoff only in the authorized switch.

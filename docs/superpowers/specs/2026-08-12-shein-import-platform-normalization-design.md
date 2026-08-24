# SHEIN Import Platform Normalization Design

## Goal

Prevent platform spelling and case mismatches from hiding import tasks while keeping service startup safe in databases that still contain historical mixed-case rows. Preserve source and target platform semantics: a source marketplace such as `amazon` must not be collapsed into a target marketplace such as `shein`.

## Scope

- Normalize platform values at import-task creation, persistence, read projection, runtime mapping, and control-plane query boundaries.
- Make platform predicates case-insensitive while historical rows are being cleaned up.
- Provide an explicit, audited recovery command for the known store-986 pending-task cohort.
- Keep RabbitMQ publication and task status changes inside the existing control-plane workflow.
- Keep production Kubernetes concurrency tuning separate from data recovery and require its own runtime evidence.

## Startup and database policy

Application startup must not mutate or reject existing mixed-case import-task rows. The repository must therefore not install a database case constraint or replace the unique index during ordinary application `AutoMigrate` execution.

The recovery command is the only data-mutating path. It defaults to dry-run and requires all of the following before it can execute:

- an explicit execution confirmation;
- `store_id=986`;
- a pending, non-terminal task status allowlist;
- expected legacy and canonical platform values;
- a transaction that verifies the selected row count and rejects case-folded duplicate keys before mutation.

The command normalizes `platform`, `source_platform`, and `target_platform` in-place without changing status or publishing messages. It reports selected, updated, skipped, and conflicting IDs. A future, separately approved schema-hardening migration may add a case constraint only after production evidence proves there are no violations.

## Data flow

1. API batch creation canonicalizes `platform`, `source_platform`, and `target_platform` to trimmed lowercase strings.
2. Persistence applies the same normalization before writes and preserves a missing legacy source as the canonical legacy platform.
3. Runtime and consumer projections carry source and target independently; consumer routing prefers canonical target and falls back only to legacy platform.
4. Listing-admin dispatch, pause, quota, and queued-count queries compare canonicalized request values with case-folded stored values.
5. The explicit recovery command updates only its pre-audited cohort. Existing control-plane claim/publish logic then handles eligible tasks normally.

## Failure handling

- A dry-run never writes.
- A requested cohort with unexpected row count, a non-pending status, a wrong store, or a case-folded duplicate aborts the transaction.
- No application startup path performs recovery, status changes, queue publication, or broad data normalization.
- Database constraints are deliberately deferred; write-boundary normalization plus query compatibility protect new and historical traffic until an audited hardening rollout.

## Tests and verification

- RED/GREEN tests cover write normalization, legacy fallback, case-insensitive dispatch queries, runtime target projection, and command dry-run/transactional rejection behavior.
- PostgreSQL-specific tests cover the query semantics and verify ordinary auto-migration does not reject an existing mixed-case row.
- Run focused packages, architecture checks, the recovery command in dry-run mode against test data, `git diff --check`, and the relevant broader Go suite.

## Non-goals

- No production data mutation or RabbitMQ publishing in this code change.
- No automatic conversion of historical rows at application startup.
- No database constraint rollout in this PR.

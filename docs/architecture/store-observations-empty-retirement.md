# Authorized empty Store observation retirement

Design Basis: Independent Architecture. Admission: IMPLEMENTATION_READY.
The existing independent reviewer approved design commit
`c851bffa1bff9758955f9b99186234babf2a58e3` without a current Must blocker.
This freezes the design; the actual private operation and fixture evidence still
need the final review before retained-instance execution.
Execution: [#691](https://github.com/qq550723504/task-processor/issues/691).

## Product outcome and authority

The retained installation owner can replace the specifically authorized, empty
`shein_observations` namespace with the current `store_observations` schema and
continue the five-agent installation. The user explicitly approved this exact
empty-schema retirement on 2026-10-14; [#137](https://github.com/qq550723504/task-processor/issues/137)
records the current decision. The original cold backups, current fifteen-database
business/ACL/role baselines, and private Store disposition evidence are reused.
Their existence is a prerequisite, not evidence of a completed installation.

There is no new UI or IA: Figma N/A. Existing Store and agent product authorities
remain applicable to the subsequent installation. Reuse the current
[Store observation architecture](store-center-platform-observations-v1.md),
[preparation runtime](store-observations-preparation-runtime.md), and the actual
current schema-owner implementation. Historical SHEIN namespace references in
those documents are not permission to relax the current serving guard.

Scope: one native PostgreSQL operation for the named retained instance, executed
by its original installation owner. Out of scope: nonempty schema retirement,
fact migration, renaming, ID mapping, compatibility, new CLI/runtime APIs, serving
DDL, role recreation, other permissions, new queues, models, providers, and
general migration or validation frameworks.

## Owner and operation path

Store and the current observation repository remain the sole fact owners. The
same `store_center_owner` executes the operation; the application never acquires
an owner connection. Use native `psql`, the canonical SQL in
`internal/integration/persistence/storeobservations/schema.sql`, and the exact
four-table privileges from its existing `GrantRuntime` implementation.

Existing private owner connection and retained-instance identity -> explicit
single-transaction `psql` operation -> exclusive locks and repeated admission ->
four authorized empty tables and empty namespace retired with `RESTRICT` ->
canonical current schema installed and original narrow runtime grants applied ->
commit -> existing runtime-role verifier -> original installer resumes serving
and the existing worker after all source/main CI gates pass.

The original installer prepares a bounded private SQL operation only after this
design reaches IMPLEMENTATION_READY. The source Writer reviews that exact file,
its hash, the canonical source hashes, and synthetic PostgreSQL results. No new
production command is needed. Do not introduce a legacy installer or a second
schema SQL source; include the frozen current canonical schema verbatim.

## Admission and atomic persistence boundary

Before execution, keep serving, initializers, workers, and provider access stopped.
Use the existing PostgreSQL cluster/volume and private owner credentials. Check
the exact expected database, current role, old namespace OID, the four table OIDs,
and their owner against the approved private disposition evidence; never select
a target from an unbounded directory or a namespace-prefix search. New
`store_observations` must be absent. Different identity or metadata stops the
operation for inspection, without attempting to repair it.

Use one PostgreSQL transaction with `ON_ERROR_STOP`, a bounded lock timeout and
statement timeout. Acquire ACCESS EXCLUSIVE locks on `public.workbench_stores`
and all four old tables before the final zero-row checks. Recheck identity,
ordinary-table shape and ownership, the approved metadata/ACLs, no inheritance,
and zero rows in Store and each table under these locks. Require exactly the
four authorized tables and their expected internal indexes/types/constraints;
unexpected objects, external dependencies, or user functions referencing the
namespace stop the operation. Retain the private inventory and compare it rather
than replacing a changed baseline with a newly accepted one.

Drop the four tables together using `RESTRICT`, then drop the empty namespace
with `RESTRICT`. Never use `CASCADE`, drop other objects to make this succeed,
or use a superuser to bypass owner/permission checks. PostgreSQL dependency
checks are the last transactional protection for external references even after
the explicit inventory checks. A concurrent conflicting DDL/metadata change
must either block within the bounded timeout or fail these checks/RESTRICT;
never silently widen the retirement set.

Install the current canonical schema and apply exactly the native grants in this
same transaction. Only `store_center_runtime` gains USAGE on the new namespace;
commands gains SELECT/INSERT, syncs/heads/records SELECT/INSERT/UPDATE. No CREATE,
DELETE, TRUNCATE, REFERENCES, TRIGGER, role or membership change. Keep public
Store grants and every unrelated object/data/ACL unchanged. Check the resulting
new schema ownership, tables, required variant constraint and grants before
commit. Failure anywhere, including new-schema or grant failure, rolls back the
entire retirement and installation. Do not run the ordinary fresh initializer
on a separate connection between retirement and installation.

## Retry, response loss, and runtime boundary

This is an explicitly authorized empty-object retirement, not a business
migration or a recovery state machine. No durable marker or new receipt owner.
Re-execution never automatically accepts an absent old schema or an existing new
schema. After a lost connection/commit response, only read actual old/new
namespace, table structure/ownership/ACLs and the existing runtime verifier:

- Old exact schema still present, new absent: the operation did not commit;
  recheck the original backup/inventory and all admission before a deliberate retry.
- Old absent, exact current new schema and grants verified: record the completed
  operation; do not replay DDL.
- Any other or uncertain state: stop installation for inspection; do not guess,
  recreate, rename, compensate, or run providers.

The existing fresh `Install`, retained Store admission, enabled/disabled serving
preflight, tenant authorization, preparation-only empty repository check and
worker lifecycle remain unchanged. A successful transaction alone does not mean
the worker, UI, real models or user acceptance passed. Compare unrelated facts,
roles and ACL baselines after execution. Missing external configuration stays
NOT_RUN; no automatic enabling or authorization occurs.

After a committed retirement, the former SHEIN-namespace runtime cannot be
restored merely by switching back to its old image. If the new application
combination fails, stop affected serving/worker processes, retain the current
schema and all data, and repair the admitted current combination forward. Do
not automatically drop the new schema, recreate the old one, restore an entire
database, or add a fallback. Reverse-schema or disaster-recovery operations
require a separate concrete decision and are outside this authorization.

## Legacy and necessary verification

Legacy decision: RETIRE. Retire only the specifically authorized zero-fact old
namespace. Reusable behavior: canonical current schema SQL, GrantRuntime,
VerifyRuntime and original installation/backup tooling. No old runtime owner,
data extraction, aliases or compatibility remain.

Use an isolated synthetic PostgreSQL instance and native psql, without shared
business data. Check the same operation for: successful exact empty retirement
and current verifier; nonempty Store/each observation table rejection; external
dependency/unexpected-object rejection; grant/install failure preserving the old
schema atomically; and unrelated facts/ACLs/roles retained. Verify exclusive
locks prevent a write from crossing the locked zero-row check, using native SQL
where needed. These are targeted implementation checks, not a new acceptance
runner or exhaustive platform.

The existing read-only reviewer performs architecture admission, then final
review of the actual bounded operation and its source/fixture evidence. Findings
follow the repository classification and stopping rules. Only real new blockers
reopen the design. Any repository documentation/code change follows applicable
exact-head CI; a private operation receives explicit source/hash review and the
above PostgreSQL checks, without pretending it was executed by PR CI.

## Delivery and stopping point

The original installer alone executes the frozen operation after all five source
merges and the relevant exact push/main CI pass. Record private operation hash,
frozen source SHA, transaction result, runtime-role verification and unchanged
baselines in #691/#137. Then resume the already authorized original installation,
page/four-mode/worker checks. Preserve all named volumes/backups and original
configs/history; no Issue closure, new product work or paid/provider execution.
Hand off the actual retained URL and known limitations after those checks.

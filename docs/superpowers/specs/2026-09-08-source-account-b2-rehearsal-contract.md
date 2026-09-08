# Source Account B2 isolated rehearsal contract

Issue: [#364](https://github.com/qq550723504/task-processor/issues/364)
Status: **IMPLEMENTATION_READY**
Allowed completion: `B2_SOFTWARE_VERIFIED (rehearsal-only)` / `ISOLATED_REHEARSAL_PASS`

Independent architecture review reached `IMPLEMENTATION_READY` in round 2 after
closing the round-1 live-parent admission blocker. Implementation tests remain the
required evidence for static-copy rejection, unknown/readback recovery, storage
admission, permissions, and cleanup; they do not reopen the frozen architecture.

This contract narrows B2 to one disposable local rehearsal. It does not create a
production operation mode or prove a real Organization authority, mapping freeze,
writer drain, profile fleet, deployment, or cutover. The already merged A and B1
contracts remain authoritative and are composed rather than reimplemented.

## Hard-Cut decision

```text
Legacy decision: RETIRE
Reusable behavior: the already extracted A evidence validation and B1
  Organization-only prepare/receipt transaction are reused at their current owner.
Current owner: Source Account's Organization-only persistence and the offline
  ownershipmigration operation.
Cutover/deletion condition: after a separately authorized C/#30 one-time reader and
  protocol cutover, D removes the numeric 1688 reader/writer/runtime dependencies.
  This issue performs neither cutover nor deletion.
```

The concrete retirement inventory is:

| RETIRE object | Why it is legacy | Current owner / replacement | Exit condition |
| --- | --- | --- | --- |
| numeric `TenantID` as 1688 Source Account owner | Organization is the current ownership boundary | `public.organization_source_accounts.organization_id` and the future C Source Account repository | C switches the 1688 reader once; D removes the numeric path |
| `public.source_account` as the 1688 runtime repository | its ownership and profile path derivation are numeric-tenant shaped | Organization-only Source Account target | it remains read-only migration input through B/C; D stops the 1688 runtime dependency after approved cutover |
| current numeric Source Account service/repository reader and its 1688 runtime callers | they require the retired owner and cannot constrain the new contract | future C Organization-scoped Source Account reader | all 1688 callers switch once and old jobs are rejected |
| `internal/tenantbridge` use by the 1688 Source Account path | it resolves the retired numeric bridge | verified Organization identity plus current domain persistence | C removes existing 1688 consumers; B2 adds none |
| numeric-tenant profile-directory derivation | it can point a migrated account at the wrong or empty browser state | exact A-verified `profile_directory` persisted by B1 | C reuses that exact directory; D deletes the derivation path |
| old 1688 task/result/route/handoff ownership protocol | it permits old execution identity to reach the new path | C/#30 Organization protocol and one-time route cutover | C/#30 reject old jobs and switch; D retires the old path |

The legacy table is not dropped here because it contains protected migration input and
may contain other-platform rows. A/B1 reading and locking that table for a bounded
one-time migration is not a new runtime consumer. B2 must not import the production
legacy repository or service to do so.

Forbidden dependencies and mechanisms: `internal/tenantbridge`, legacy Source Account
service/repository wrappers, request/worker/HTTP/startup wiring, fallback, dual read,
dual write, synchronization, receipt-as-runtime-repository, a second ownership fact,
production authority/freeze plug-ins, registries, abstract factories, remote collectors,
or a generic migration framework. Product/Asset/Task/Redis result migration is not part
of this operation.

## Exact composition and ownership

The only new executable is `cmd/source-account-ownership-rehearsal`. Its only positive
write path is a fixed synthetic rehearsal whose PostgreSQL container and temporary
profile/evidence directory are created and retained by that same parent invocation.
It has no flag, environment variable, config file, or API for a caller-supplied DSN,
database, host, Organization, account, profile root, fixture payload, image, or real
environment mode.

```text
operator
  |
  +-- source-account-ownership-rehearsal inspect   [default; no allocation => deny]
  |
  +-- source-account-ownership-rehearsal rehearsal --yes
        |
        +-- Rehearsal owner
        |     create postgres:18-alpine + synthetic source/metadata + temp profiles
        |     retain live Testcontainers handle and parent-only allocation state
        |     create anonymous control/output pipes per child stage
        |     finally terminate that exact container and remove only its temp tree
        |
        +-- child process: inspect
        |     one-shot online admission by the still-running parent
        |     -> ownershipmigration.ReadSnapshot(source RO, metadata RO)
        |     -> ownershipmigration.Preflight(profile metadata only)
        |     -> optional explicit A receipt publication outside profile root
        |
        +-- child process: install-schema            [explicit DDL stage]
        |     admission -> ownershipmigration.InstallPreparedSchema(source schema role)
        |
        +-- child process: prepare                   [explicit DML stage]
        |     admission -> acquire concrete source + metadata SHARE locks
        |     -> ReadSnapshot -> Preflight -> compare the pinned A digest
        |     -> ownershipmigration.NewPreparer(source writer)
        |     -> Preparer.Prepare(same contract/key/payload)
        |           |
        |           +-- one PostgreSQL transaction/UoW
        |                 SHARE public.source_account
        |                 SHARE ROW EXCLUSIVE public.organization_source_accounts
        |                 SHARE ROW EXCLUSIVE public.source_account_ownership_migration_receipts
        |                 reread full 1688 source -> validate exact schema/target
        |                 insert Organization target + immutable receipt -> COMMIT
        |
        +-- fresh child process: receipt-read
              admission -> reacquire concrete source + metadata SHARE locks
              -> ReadSnapshot -> Preflight -> compare the same pinned A digest
              -> ownershipmigration.NewPreparer(source read role)
              -> Preparer.ReadPreparedReceipt(same contract/key/payload)
              -> validate source + target + durable receipt; no mutation
```

| Owner | Existing/new API | Reads | Writes | Must not own |
| --- | --- | --- | --- | --- |
| `ownershipmigration` A | existing `ReadSnapshot`, `Preflight`, optional `ValidateReceiptTarget`/`WriteReceipt` | full 1688 source rows, Organization metadata, filesystem metadata | only an explicitly requested A evidence file | authority freshness, freeze, runtime ownership |
| `ownershipmigration` B1 | existing `InstallPreparedSchema`, `NewPreparer`, `Prepare`, `ReadPreparedReceipt` | exact schema, full source/target, receipt | explicit DDL; or target plus receipt in one DB transaction | resource admission, production freeze, runtime reader |
| rehearsal coordinator | new concrete command-local sequencing; no provider interface | parent allocation facts and A/B1 results | fixed synthetic fixture, explicit evidence, child process launch and exact owned-resource cleanup | business repository, alternate writer, generic environment support |
| PostgreSQL B1 UoW | existing B1 transaction | locked complete source and target/receipt state | target and receipt atomically | filesystem, ZITADEL database, process admission |
| future C/D owners | no B2 API | none in this issue | none in this issue | B2 must not anticipate their runtime contracts |

No new package may import the legacy Source Account model/service to build a second
DTO. The `ownershipmigration.Receipt`, `PrepareRequest`, and `PreparedReceipt` are the
single A/B1 operation contracts. Any small command coordinator lives under
`internal/app/runtime/sourceaccountownershiprehearsal` and calls those concrete APIs;
it is not an extension point.

## Observable commands and write boundaries

The command uses fixed actions. `inspect` is the default when no action is supplied.
The public command never accepts a DSN or static allocation manifest. Internal stage
children are spawned only by the live `rehearsal --yes` parent. For each stage,
the parent creates fresh anonymous stdin/stdout pipes, validates
the exact `exec.Cmd` child PID and its still-owned Testcontainers handle, and then performs
a bounded challenge/response. After the one-shot grant, the still-open control pipe is
the dedicated parent-liveness channel. The response binds the parent invocation, child PID, stage,
one-shot nonce, immutable container ID, cluster/database identity and fixed role. Only
that response carries the stage DSN/credential. It is sent through the anonymous pipe,
never through arguments, environment, files or output, and is not persisted by the child.
The child also verifies that its OS-reported parent is the same executable instance
identified by the handshake. Parent exit or any control/liveness-pipe loss cancels the
stage; a nonce or stage cannot be replayed by another child.

| Action | Preconditions | Permitted effects | Result |
| --- | --- | --- | --- |
| `inspect` (default) | a live parent-owned allocation and successful online one-shot handshake; otherwise admission denial | bounded reads of Docker/container identity, PostgreSQL identity, A tables and profile metadata; no DDL/DML/profile/evidence write by default | redacted A digest/count and explicit `rehearsal_only` classification |
| `inspect --receipt <owned path>` | same, plus a new path in the allocation evidence directory | one exclusive A receipt publication through existing A APIs | pinned A input for later stages |
| `install-schema` | explicit rehearsal parent action, `--yes`, re-admission, schema role | B1 schema installer only | exact schema installed or rejected; never implies Prepare success |
| `prepare` | explicit rehearsal parent action, `--yes`, pinned A receipt, re-admission, live freeze locks and exact reread | B1 target/receipt transaction only | prepared/replayed/definite rejection/outcome unknown |
| `receipt-read` | active live parent, fresh one-shot handshake, original contract/key/pinned A payload and fixed read role | read-only B1 recovery transaction | prepared, not found, drift/conflict, or unverifiable |
| `rehearsal --yes` | Docker available; no external resource input | allocates the fixed fixture, invokes the stages above as separate processes, then cleans up | `ISOLATED_REHEARSAL_PASS` only after fresh-process readback and cleanup |

Starting the binary, asking for help, an invalid action, missing input, default inspect,
and receipt-read cannot allocate a container, install schema, call `Prepare`, change a
freeze, or create a profile/evidence file. `rehearsal` without `--yes` also stops before
allocation. Schema installation is never hidden in connection setup, inspect, prepare,
or receipt-read.

`inspect`, `install-schema`, `prepare`, and `receipt-read` are child-stage names in the
same executable, not independently authorized environment operations. Directly invoking
one, copying every printed/static allocation field, supplying a static environment or
manifest, or replaying an old pipe payload fails before opening a SQL writer. The parent
has no responder mode exposed to callers: it answers a stage only for the exact child
process it just spawned while its concrete container handle remains live.

Final user-facing exit classes are frozen as: `0` success, `2` invalid/bounded input,
`10` resource admission denied or unsupported environment, `20` outcome remained unknown
after the parent attempted fresh-process readback while the allocation was still active,
`21` readback confirmed not prepared, `22` B1 conflict or drift, and `30` dependency/
permission/timeout/cancellation failure. Secret-bearing dependency errors are projected
to these classes without printing connection strings. Internal stage status `unknown`
is intercepted by the parent and is not itself a final instruction to the operator.
After cleanup, output never tells the operator to reuse the destroyed allocation.

## Task-owned resource admission

Positive admission requires all of the following facts and the online decision of the
still-running parent. No label, loopback address, database name, `--yes`, static
manifest/environment, copied nonce, or caller-signed token is authority:

1. The public `rehearsal --yes` parent created the container during this invocation
   through the repository's existing Testcontainers dependency and still holds its
   live handle in process memory. The image is fixed to `postgres:18-alpine`.
2. The child has an anonymous one-shot control/liveness channel created by that parent.
   A fresh child challenge is answered only after the parent matches the exact spawned
   process handle/PID, expected stage, unused nonce, live Testcontainers handle and
   current container inspection. No DSN or credential exists in a caller-provided input.
3. Docker inspection matches the immutable container ID, image, parent-created random
   allocation label, running state, and one loopback-only published PostgreSQL endpoint
   retained by the parent. The child cannot propose or replace an endpoint.
4. Live PostgreSQL identity queries match the captured cluster system identifier plus
   exact source and metadata database names/OIDs. Source read, schema/write and receipt
   connections must resolve to the same source database identity. This rejects a
   same-named database on another cluster as well as a wrong database on one cluster.
5. Roles are fixed fixture roles. Inspect has only the required reads; schema install,
   prepare and receipt-read use distinct least-privilege grants and are tested by
   revoking each required permission.
6. The synthetic Organization metadata and accounts match the fixed bounded fixture;
   the one supported profile host is the current child host and every profile path is
   inside the parent's new temporary tree. A performs the actual path/alias/content-free
   validation. Any host, root or allocation-field mismatch rejects.
7. The allocation has not expired or been marked cleaning/cleaned, and exactly one
   live parent owns it. Parent/channel loss, direct child invocation, multi-host, remote
   storage, external DSNs, pre-existing containers, shared databases, and unknown
   ownership are unsupported and reject before DDL/DML.

Before starting PostgreSQL or any DDL/DML, the parent creates only disposable candidate
directories and asks the existing A `Preflight`/receipt-target checks to validate the
actual local storage. Linux tries only fixed local candidates and may use supported
tmpfs; overlay, remote and unknown mounts reject. Windows requires a supported local
volume. If no candidate passes A unchanged, final exit is `10` and the directories are
removed. This is a fixed rehearsal check, not a new filesystem provider or an override
of A's supported storage policy.

The allocation capability is an isolated-test provenance mechanism, not Organization
authority. The synthetic `projections.org_metadata2` data is real PostgreSQL evidence
for the algorithm but is an external-authority substitute, not ZITADEL acceptance.
There is intentionally no real-environment positive path in this issue.

## Evidence, freeze, state, and failure semantics

The rehearsal parent is the only fixture/admission owner. For prepare and receipt-read,
the concrete coordinator holds `SHARE` locks on the synthetic
`public.source_account` and `projections.org_metadata2` tables across the fresh A
snapshot/preflight comparison and B1 call. These locks test writer exclusion for this
single controlled fixture; they do not claim production writer, worker, fleet, or
projection freeze.

State is observable but not a second durable ledger:

```text
unallocated -> allocated -> inspected -> schema_installed -> prepared -> verified
                    \             \             \              \
                     +------------- failure/cancel/unknown ------> cleanup
```

The state names are coordinator progress only. Durable preparation truth remains the
B1 receipt. A failure after schema install reports `schema_installed` and does not claim
multi-stage rollback. A failure before B1 COMMIT leaves no target/receipt. COMMIT is the
B1 state boundary.

After `Prepare` is invoked, every child/process/transport/database failure not proven to
have happened before COMMIT is internally `unknown`. The parent keeps the allocation
active, preserves exactly the original contract version, idempotency key, source ID,
A receipt and request fingerprint, performs no automatic Prepare call, and immediately
starts one fresh `receipt-read` child through a new one-shot channel. A valid durable
receipt resolves prepared success; a validated empty target/receipt resolves not
prepared; read permission failure, source/target drift, cancellation, timeout or failed
channel leaves final exit `20`. Only after that attempt does the parent clean the
allocation. No final message suggests readback after cleanup. None of these outcomes
authorizes a new key, replay, adopt/upsert, delete/recreate, or compensating receipt.

Input JSON/file reads are limited before allocation and decoding; trailing JSON and
unknown fields reject. The operation retains A's `MaxRows=100000` and B1's ten-minute,
64 KiB field, 64 MiB raw/request, and 128 MiB encoded target/receipt limits. The new
command provides no override that raises them and caps its redacted output/log sizes.

Cleanup is owned by the original Testcontainers handle in a `defer`/`finally` path and
targets only the immutable container ID and temporary tree it created. Admission is
rechecked before a child cleanup request. Normal failure, permission failure,
cancellation and timeout tests all assert container termination and temp evidence/profile
removal. A hard process kill is reported separately if not exercised; it is not called
clean cleanup. No command prunes Docker, drops an external database, deletes a real
profile, or touches another worktree.

## TDD and acceptance allocation

Coding may start only after independent `IMPLEMENTATION_READY`. TDD first records RED
tests for the command boundary and then the smallest GREEN implementation. The final
suite must include an actual built command, fixed task-created PostgreSQL 18, synthetic
profiles and a fresh child process; helper/DTO/mock-only coverage does not count.

The isolated matrix covers:

- default/help/invalid/startup zero-write; explicit receipt publication only;
- direct child invocation and a copy of every static allocation field without the live
  parent/one-shot pipe reject before SQL writer open and leave zero DDL/DML;
- pre-existing/external resource, wrong label/capability/host/profile root, wrong
  instance, same database name on another cluster, wrong database OID and expired or
  ambiguous allocation;
- missing/removed/ambiguous/stale Organization mapping, pinned-A mismatch and profile
  host/path mismatch;
- inspect, schema, prepare and receipt-read role denials independently;
- source, mapping, target and receipt drift between checks, with no fallback;
- concurrent identical prepare, same key/same payload replay, same key/different payload,
  different-key target conflict and B1/legacy synthetic writer contention;
- cancellation and deadline before writes, during locks/work and after durable commit;
- B1 commit success followed by caller response loss, internal `unknown`, no second
  Prepare, and a fresh command process reading the same durable receipt with the same
  identity before the parent returns success; failed readback remains final exit `20`;
- Linux overlay/remote/unknown temp roots reject before PostgreSQL/DDL, a supported
  local/tmpfs candidate succeeds, and Windows uses a supported local volume;
- B1 field/row/encoded budgets and bounded new input/output;
- source/non-1688/protected fixture rows and profile contents unchanged;
- normal, rejected, cancelled, timed-out and recovery-path cleanup.

The PR reports `PASS`, `SKIP`, and `NOT_RUN` separately. Real Organization authority,
real projection watermark/freshness, real source/mapping writer freeze, multi-host or
network profile storage, real worker/admission drain, production schema/Prepare,
C reader/protocol cutover, D retirement, #30 route enablement, deployment and business
acceptance are `NOT_RUN / NOT_AUTHORIZED` and cannot be inferred from this rehearsal.

Planned production files are limited to the dedicated command, its concrete rehearsal
coordinator, a thin maintained PowerShell entry, this contract, the narrow parent-design
link, and required command/Legacy architecture registrations. Tests stay adjacent. No
production Source Account, crawler, tenantbridge, HTTP, worker, startup, C, or D file is
in scope.

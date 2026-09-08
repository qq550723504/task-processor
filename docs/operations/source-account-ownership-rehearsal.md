# Source Account ownership isolated rehearsal

This command proves only the Issue #364 B2 software chain against resources that
the same invocation creates and destroys. It is not a production migration command.

## Boundary

The positive path is fixed:

- image: `postgres:18-alpine`;
- two synthetic databases in one disposable Testcontainers allocation;
- three fixed synthetic 1688 accounts and Organization mappings;
- temporary local profile/evidence directories owned by the invocation;
- separate inspect, schema-install, prepare, and receipt-read processes;
- automatic cleanup of the exact container and temporary directory.

There is no DSN, hostname, database, Organization, account, profile-root, fixture,
image, config-file, environment, or production-mode input. The command does not
connect to ZITADEL, Redis, Kubernetes, a shared database, or a real browser profile.
It does not migrate Product, Asset, Task, or result data and does not cut over a
reader, writer, worker, route, or deployment.

## Run

Docker must be available locally. The maintained PowerShell entry builds into the
ignored `.local/bin` directory and runs the explicit rehearsal action:

```powershell
./scripts/source-account-ownership-rehearsal.ps1
```

The equivalent direct commands are:

```powershell
go build -o ./.local/bin/source-account-ownership-rehearsal.exe ./cmd/source-account-ownership-rehearsal
./.local/bin/source-account-ownership-rehearsal.exe rehearsal --yes
```

Starting the binary without an action, or passing `inspect`, is intentionally denied
because no live parent-owned allocation exists. Internal stage names are not operator
commands: a direct or replayed child invocation is rejected before opening PostgreSQL.

Success ends with both of these markers:

```text
cleanup=PASS
ISOLATED_REHEARSAL_PASS real_authority=NOT_RUN real_environment=NOT_AUTHORIZED c_d=NOT_RUN
```

Do not interpret that marker as real Organization authority, mapping freshness,
production writer freeze, fleet/profile coverage, deployment health, cutover, or
business acceptance.

The command also contains a closed set of `--scenario` fault injections used by its
integration suite (permission denial, mapping/target drift, and committed response
loss). They only mutate the same synthetic allocation and do not add an external
resource or configuration path.

## Effects and recovery

The inspect child reads the fixed source/metadata fixture and publishes one A receipt
inside the task-owned evidence directory. Schema installation is a separate explicit
child. Prepare calls the existing B1 transaction; its only durable writes are the
Organization-only target and immutable receipt in the disposable source database.

If the prepare response is unavailable after invocation, the parent never calls
Prepare again. While retaining the allocation and freeze, it launches a fresh process
that reads the same B1 receipt with the same key and pinned A input. Only confirmed
prepared state succeeds; confirmed absence exits `21`, and unresolved state exits
`20`. Cleanup then destroys the allocation, so output never suggests retrying a read
against it.

## Exit classes

| Code | Meaning |
| --- | --- |
| `0` | full isolated rehearsal and cleanup passed |
| `2` | invalid or oversized command input |
| `10` | unsupported environment or resource admission denied |
| `20` | prepare outcome remained unknown after fresh-process readback |
| `21` | fresh-process readback confirmed not prepared |
| `22` | source, mapping, target, receipt, or idempotency conflict/drift |
| `30` | dependency, permission, timeout, cancellation, or cleanup failure |

Errors are projected to these classes without printing connection strings or
credentials. If cleanup does not report `PASS`, inspect Docker by the label
`task-processor.source-account-b2=issue-364-rehearsal`; do not prune unrelated
containers. A hard process kill is not claimed as clean cleanup.

## Verification

```powershell
go test ./internal/app/runtime/sourceaccountownershiprehearsal ./cmd/source-account-ownership-rehearsal
go test -tags=integration -run TestSourceAccountOwnershipRehearsalCommand -count=1 -v ./cmd/source-account-ownership-rehearsal
go test ./tests -run 'TestCmdContainsOnlyOfficialEntrypoints|TestOperationalCommandsHaveDeploymentBuildOrScriptOwner' -count=1
```

The integration test builds the real command, uses PostgreSQL 18, exercises normal
prepare and committed-response-loss recovery, checks protected fixture state, and
asserts that no task-labeled container survives.

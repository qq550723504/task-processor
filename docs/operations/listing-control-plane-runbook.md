# Listing Control Plane Runbook

> Status: active operational runbook.
>
> Last reviewed: 2026-07-09.
>
> Scope: `cmd/listing-control-plane`, dispatch/recovery control loops, validation, smoke checks, and operator troubleshooting.

## 1. Purpose

`cmd/listing-control-plane` is a current official runtime entrypoint. It coordinates Listing control-plane behavior around dispatch, recovery, leadership, and operational visibility.

This runbook defines how to build, validate, run, and troubleshoot that runtime without confusing it with historical one-off commands or crawler/debug entrypoints.

## 2. Current entrypoint

Runtime command:

```text
cmd/listing-control-plane
```

Local build:

```powershell
go build ./cmd/listing-control-plane
```

Make target:

```powershell
make build-listing-control-plane
```

Local run example:

```powershell
go run ./cmd/listing-control-plane -config=config/config-dev.yaml -log-level=info
```

Docker build path:

```text
deployments/docker/Dockerfile.listing-control-plane
```

The official `cmd/` entrypoint list is maintained in `docs/development/repository-structure.md` and guarded by repository structure tests.

## 3. Required validation before release-sensitive changes

Run the backend gate used by CI when practical:

```powershell
go test ./... -count=1
```

Run the control-plane race gate:

```powershell
go test -race ./internal/app/runtime/listingcontrol -run TestControlPlaneService -count=1
```

Run listingadmin dispatch/recovery race coverage:

```powershell
go test -race ./internal/listingadmin -run "TestConcurrentClaimForDispatchOnlyOneWorkerWins|TestConcurrentRollbackDispatchOnlyOriginalQueuedClaimIsRestoredOnce|TestConcurrentRecoveryOnlyUpdatesStillEligibleRowsOnce" -count=1
```

Build the maintained runtime commands:

```powershell
go build ./cmd/listing-control-plane
go build ./cmd/shein-listing
```

If dependency analysis is needed, keep generated output local:

```powershell
New-Item -ItemType Directory -Force .local/refactoring | Out-Null
./scripts/analyze-project-deps.ps1 6>&1 | Tee-Object -FilePath .local/refactoring/dependency-baseline-output.txt
```

Do not recommit generated package/dependency snapshots as long-lived documentation.

## 4. Runtime responsibilities

The control plane should own runtime coordination concerns such as:

- leader ownership and leader status;
- dispatch loop coordination;
- recovery loop coordination;
- work claiming / rollback / recovery orchestration;
- cycle-level operational reporting;
- health and readiness signals needed by deployment tooling.

It should not own:

- marketplace business rules;
- ListingKit task DTO shaping;
- SHEIN/TEMU/Amazon publish payload construction;
- product source normalization;
- crawler/browser extraction logic;
- broad retired management-service semantics.

Business policy should remain in the relevant domain packages, and runtime assembly should stay under `internal/app/runtime/listingcontrol`.

## 5. Smoke checklist

Before treating a control-plane change as releasable, confirm:

```text
[ ] cmd/listing-control-plane builds.
[ ] cmd/shein-listing builds.
[ ] control-plane race tests pass or failures are classified.
[ ] listingadmin dispatch/rollback/recovery race tests pass or failures are classified.
[ ] go test ./... -count=1 passes or failures are explicitly documented.
[ ] leader status is visible in logs or runtime health output.
[ ] dispatch cycle logs distinguish claimed, skipped, failed, and recovered work.
[ ] recovery cycle logs distinguish no-op, recovered, skipped, and failed work.
[ ] SHEIN listing browser startup path remains covered by the rollout smoke checklist when SHEIN runtime is part of the release.
```

## 6. Operational signals to watch

Watch for:

- repeated leader churn;
- dispatch cycles with unexpected `failed > 0`;
- recovery cycles with repeated skipped or failed rows;
- rollback attempts that do not restore eligible queued rows;
- claim conflicts that do not settle to one winner;
- control loop stalls without explicit no-op logging;
- logs that use old management-service naming instead of runtime/task/status/store/product/pricing/health capability names.

## 7. Troubleshooting guide

### Dispatch is not claiming work

Check:

1. Is this instance the active leader?
2. Are candidate rows eligible for dispatch?
3. Are rows already claimed or blocked by another worker?
4. Are capability names and runtime names used in errors instead of a retired management-service label?
5. Do listingadmin race tests still pass?

### Recovery is not restoring work

Check:

1. Are rows still eligible for recovery?
2. Was the original queued claim restored exactly once?
3. Are recovery filters too narrow?
4. Is the recovery path trying to own marketplace publish policy instead of runtime recovery orchestration?
5. Do recovery logs show skip reason, rollback reason, and final state?

### Control plane starts but does nothing

Check:

1. Config path and environment.
2. Database connectivity.
3. Redis/lock or leader election prerequisites, if enabled.
4. Scheduler/loop enablement flags.
5. Whether this environment expects another leader to be active.

### Logs mention broad management-service semantics

Treat this as a boundary regression candidate.

Expected wording should name the specific runtime capability, such as task status, store API, product facts, pricing, health, dispatch, or recovery. Do not use a generic retired management-service label as the runtime owner.

## 8. Documentation update rule

When the control plane changes runtime ownership or validation gates, update:

- this runbook;
- `docs/refactoring/current-refactoring-status.md` if Now / Next / Later changes;
- `docs/development/repository-structure.md` if official entrypoints change;
- CI workflow and structure tests if build or race gates change.

Do not add new runtime entrypoints under `cmd/` without updating the repository structure document and guard tests in the same change.

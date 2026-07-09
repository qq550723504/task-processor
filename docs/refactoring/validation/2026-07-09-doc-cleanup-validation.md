# 2026-07-09 Documentation Cleanup Validation

> Status: dated validation note.
>
> Scope: documentation cleanup, generated-baseline cleanup, current runtime/boundary documentation refresh, and validation evidence handling.

## 1. Validated change set

This note covers the documentation cleanup and alignment wave that:

- removed stale generated package/dependency baseline documents from committed docs,
- added `.gitignore` rules for generated refactoring evidence,
- aligned README, Makefile, repository structure, current refactoring status, ListingKit boundary checkpoint, and architecture boundaries with current official runtime entrypoints,
- added product-source handoff guidance,
- added Listing Control Plane runbook guidance,
- added auth/tenancy supporting architecture context,
- downgraded stale roadmap material to historical/directional context,
- condensed long submission inventory material into a historical inventory.

## 2. CI status

CI has been reported as run for this cleanup wave.

Connector note: the GitHub commit status endpoint returned no status entries for latest checked `master` commit `9661556582a15085b0fc515a84c7711fa7d9ae38`, and the workflow-run lookup available to this connector did not expose runs for that commit. Therefore, this note records that CI was run, but does not independently claim a green result from connector-visible status data.

Use GitHub Actions as the source of truth for exact job status, logs, and reruns.

## 3. Recommended validation evidence to keep visible

When a release or merge decision depends on this cleanup wave, keep these checks visible in GitHub Actions, PR notes, or a later validation note:

```powershell
go test ./... -count=1
go test -race ./internal/app/runtime/listingcontrol -run TestControlPlaneService -count=1
go test -race ./internal/listingadmin -run "TestConcurrentClaimForDispatchOnlyOneWorkerWins|TestConcurrentRollbackDispatchOnlyOriginalQueuedClaimIsRestoredOnce|TestConcurrentRecoveryOnlyUpdatesStillEligibleRowsOnce" -count=1
go build ./cmd/listing-control-plane
go build ./cmd/shein-listing
```

If dependency evidence is needed, keep it local:

```powershell
New-Item -ItemType Directory -Force .local/refactoring | Out-Null
./scripts/analyze-project-deps.ps1 6>&1 | Tee-Object -FilePath .local/refactoring/dependency-baseline-output.txt
```

Do not commit generated package/dependency snapshots as long-lived documentation.

## 4. Documentation consistency checks performed during cleanup

Repository searches after the cleanup wave did not find remaining references to:

- old Go-version badges such as `Go 1.24` or `Go 1.25`,
- retired official command entrypoints such as `amazon-crawler-api`, `amazon-listing`, `1688-crawler-api`, `productenrich-api`, `listingkit-subscription`, or `shein-address-copy`,
- removed generated refactoring baseline file names such as `dependency-baseline-output.txt`, `packages-baseline.txt`, `mod-graph-baseline.txt`, `package-map.generated`, or `dependency-baseline.generated`.

## 5. Follow-up rule

If GitHub Actions shows a failure, do not update this note to say "validated" until the failure is classified as one of:

- true refactor/doc-cleanup regression,
- stale architecture/doc test expectation,
- unrelated flaky fixture,
- unrelated legacy failure,
- environment/runtime issue.

If the full CI result is confirmed green, add a later dated note or update this one with the exact workflow run, job names, and result source.

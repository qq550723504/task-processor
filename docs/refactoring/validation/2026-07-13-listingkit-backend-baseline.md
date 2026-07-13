# ListingKit Backend Baseline Validation — 2026-07-13

## Baseline and scope

- Commit: `45b0ccd609ca2a0b60c3d93e8a3d97b55ac59010`.
- Scope: repository-wide Go test baseline after the Product Sourcing validation merge.
- Workspace note: `go.work.sum` already had uncommitted dependency-checksum changes before this validation. It was not modified as part of this check.

## Passed validation

The ListingKit package suite completed successfully:

```powershell
go test ./internal/listingkit/... -count=1
```

This includes the root facade, API and HTTP adapters, Temporal bridge, store, SHEIN synchronization, Studio batch policy, and supporting ListingKit subpackages.

The repository-wide backend suite also completed successfully:

```powershell
go test ./... -count=1
```

The command completed in approximately 295 seconds. A prior run returned exit code 1 with truncated aggregate output; the retained-context rerun above passed, so that first result is not treated as a confirmed regression. `master` advanced while the suite was running, so this evidence applies only to the commit above and must be rerun before making a claim about later commits.

The fresh dependency scan also reported no advisory boundary violations. Root `internal/listingkit` remains an orchestration, compatibility, DTO adaptation, persistence-ordering, and API-shell boundary; no package move was made because the scan did not identify a real ownership or dependency-direction improvement.

## Remaining release gates

This note records backend test evidence only. It does not claim the release baseline is fully green. The following current-plan gates remain separate:

- listing-control and listingadmin race checks;
- maintained runtime command builds (`make build-all`, or the documented Windows equivalent when GNU Make is unavailable);
- frontend install, lint, typecheck, tests, and build;
- controlled 1688 source-to-task-to-preview validation and its operational closeout evidence.

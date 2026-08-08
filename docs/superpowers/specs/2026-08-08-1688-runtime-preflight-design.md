# 1688 Runtime Preflight Design

## Goal

Make the existing local ListingKit API replay entry point runnable from the current checkout and from any caller working directory, so the next 1688 validation step can begin with a reproducible, read-only preflight.

## Root cause

`scripts/start-listingkit-api-local-replay.ps1` hard-codes the historical checkout path `D:\code\task-processor`. The script therefore fails or runs against the wrong directory when the repository is checked out elsewhere. The maintained `start-listingkit-local-api.ps1` already resolves the repository root from `$PSScriptRoot`; the replay entry point should use the same boundary.

The local port-forward entry point also used historical Kubernetes defaults
(`yudao-cloud/postgresql-v18` and `yudao-cloud/redis`). The current cluster and
the existing watcher script use `platform-data/shared-postgresql` and
`platform-data/redis`, so the maintained port-forward entry point must converge
on those service targets.

## Design

- Resolve the repository root from the replay script's own directory with `$PSScriptRoot`.
- Keep the existing fixed local database, Redis, cookie-Redis, API port, config path, and log-level settings unchanged.
- Change directory to the resolved repository root before invoking `go run`, so relative config and package paths remain valid regardless of the caller's current directory.
- Default local database forwarding to `platform-data/shared-postgresql` and Redis forwarding to `platform-data/redis`; keep the Temporal target in `temporal/temporal-frontend`.
- Do not add task creation, crawler invocation, store mutation, or platform submission to this change.
- Add Pester regression tests that parse both scripts and assert the dynamic-root contract, absence of the stale hard-coded path, and current Kubernetes service defaults.

## Validation boundary

After the script change is verified locally, runtime validation remains a separate operator action:

1. Start the existing port-forward stack.
2. Start the local API with the corrected entry point.
3. Run GET-only health, authentication, tenant, and store preflight checks.
4. Stop if credentials or runtime dependencies are unavailable; do not create a 1688 task automatically.

This work does not claim a real 1688 import or preview/readiness acceptance.

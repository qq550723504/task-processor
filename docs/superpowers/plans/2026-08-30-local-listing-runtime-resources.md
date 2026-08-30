# Local listing runtime resources: first migration slice

> **Execution note:** follow the `superpowers:executing-plans` workflow when implementing this plan. The user has explicitly requested no commit; keep all changes uncommitted.

**Goal:** Move local listing-runtime infrastructure ownership from `LocalDataProvider` to `RuntimeResources`, with explicit cleanup, while preserving current runtime behavior through a temporary compatibility facade.

**Architecture:** `RuntimeResources` owns DB/Redis creation, repository construction, and idempotent cleanup. Bootstrap retains the owner in `SharedResources`; `LocalRuntime` consumes resources instead of a provider. `LocalDataProvider` forwards to resources temporarily, so remaining adapters retain their current behavior during staged migration.

**Tech stack:** Go, GORM, go-redis, Testify, existing repository implementations.

---

### Task 1: Add resource owner with lifecycle tests

**Files:**
- Create: `internal/listingruntime/local/runtime_resources.go`
- Create: `internal/listingruntime/local/runtime_resources_test.go`
- Modify: `internal/listingruntime/local/local_data_provider.go`

1. Write a test that constructs resources from injected SQLite DB / fake or nil Redis dependencies, verifies repository accessors are initialized, and verifies `Close` is safe when called twice.
2. Run `go test ./internal/listingruntime/local -run TestRuntimeResources -count=1`; confirm it fails because the type does not exist.
3. Implement `RuntimeResources` with constructors for configuration and injected dependencies; move connection creation, repository initialization, accessors, and idempotent close from `LocalDataProvider`.
4. Convert `LocalDataProvider` into a forwarding compatibility facade backed by `RuntimeResources`; keep existing public methods and nil semantics.
5. Run `go test ./internal/listingruntime/local -run TestRuntimeResources -count=1`; confirm it passes.

### Task 2: Make bootstrap own runtime cleanup

**Files:**
- Modify: `internal/app/bootstrap/resources/shared_resources.go`
- Modify: `internal/app/bootstrap/resources/shared_resources_test.go`
- Modify: the command/runtime shutdown owners that build `SharedResources`, as identified while testing

1. Write focused tests for a `SharedResources.Close` (or equivalent explicit closer) that calls runtime-resource cleanup exactly once and reports initialization cleanup failures.
2. Run `go test ./internal/app/bootstrap/resources -run 'Test.*SharedResources.*Close' -count=1`; confirm it fails for the expected missing lifecycle API.
3. Construct `RuntimeResources` directly in bootstrap. Attach its closer to `SharedResources`, close it on construction failures after opening resources, and call it from the relevant process shutdown path(s).
4. Preserve the existing value-returning `BuildSharedResources` contract unless tests demonstrate a required compatibility change.
5. Run `go test ./internal/app/bootstrap/resources -count=1` and the affected command/runtime package tests.

### Task 3: Migrate health validation and Task RPC dependencies

**Files:**
- Modify: `internal/listingruntime/local/local_listing_runtime_health.go`
- Modify: `internal/listingruntime/local/local_task_rpc_provider.go`
- Modify: `internal/listingruntime/local/local_runtime_adapter.go`
- Modify/create: focused tests in `internal/listingruntime/local`

1. Write a test showing health validation receives `RuntimeResources` and returns the identical report fields/readiness outcome for DB-only and DB+Redis configurations.
2. Write a test showing `NewLocalTaskRPCProvider` accepts a direct `*gorm.DB` and keeps the existing nil behavior.
3. Run the focused tests and confirm they fail before migration.
4. Change health validation and Task RPC constructors to take their narrow dependencies. Update `LocalRuntime` to use `RuntimeResources` and the direct DB accessor.
5. Run `go test ./internal/listingruntime/local -count=1`.

### Task 4: Guard compatibility and validate the slice

**Files:**
- Modify: relevant local runtime tests only as required by the new dependency boundary

1. Add a structural test or targeted search assertion that production bootstrap no longer calls `NewLocalDataProvider`.
2. Add/retain tests proving the compatibility facade delegates Store API, raw JSON, and repository access without changing results.
3. Run:
   - `go test ./internal/listingruntime/local -count=1`
   - `go test ./internal/app/bootstrap/resources -count=1`
   - `go test ./internal/app/bootstrap/listingruntime -count=1`
   - `go test ./internal/app/runtime/listing -count=1`
   - `go test ./cmd/shein-login-worker -count=1`
   - `git diff --check`
4. Record only completed command results. Do not claim `go test ./...` passed unless it completes in this worktree.
5. Keep changes uncommitted and report the changed files, focused test evidence, and any remaining consumers for the next slice.

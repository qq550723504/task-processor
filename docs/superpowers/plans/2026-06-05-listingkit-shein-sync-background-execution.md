# ListingKit SHEIN Sync Background Execution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Return SHEIN sync jobs immediately from the HTTP endpoint while running the actual sync in the background.

**Architecture:** Keep the current synchronous SHEIN sync service intact and add an async wrapper used only by the HTTP runtime wiring. The wrapper returns an initial job immediately, then executes the existing sync flow in a goroutine that persists final job state.

**Tech Stack:** Go, Gin, existing `listingkit/sheinsync` repositories and job model.

---

### Task 1: Add async service tests

**Files:**
- Create: `internal/listingkit/sheinsync/async_service_test.go`
- Test: `internal/listingkit/sheinsync/async_service_test.go`

- [ ] Write tests that prove the async wrapper returns quickly with a created job and later completes background execution.
- [ ] Run `go test ./internal/listingkit/sheinsync -run Async` and confirm the new test fails first.
- [ ] Implement only the minimum async wrapper code needed for the tests to pass.
- [ ] Re-run `go test ./internal/listingkit/sheinsync -run Async` until it passes.

### Task 2: Wire async execution into HTTP runtime

**Files:**
- Modify: `internal/listingkit/httpapi/shein_sync_runtime.go`
- Test: `internal/listingkit/httpapi/shein_sync_runtime_test.go`

- [ ] Add a runtime-focused test that verifies HTTP wiring uses the async wrapper without changing endpoint contract.
- [ ] Run the focused HTTP runtime test and confirm it fails first.
- [ ] Swap runtime construction from the synchronous service to the async wrapper.
- [ ] Re-run the focused HTTP runtime test until it passes.

### Task 3: Regression verification

**Files:**
- Modify: `internal/listingkit/sheinsync/service.go`
- Modify: `internal/listingkit/sync_facade.go`

- [ ] Run `go test ./internal/listingkit/sheinsync ./internal/listingkit/api ./internal/listingkit/httpapi`.
- [ ] Restart the local API process.
- [ ] Re-test store `870` sync and confirm products remain queryable after triggering sync.

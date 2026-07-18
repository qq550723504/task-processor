# SDS Retry Timeout Budget Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give SDS multipart uploads capped exponential backoff with jitter and extend the ListingKit SDS sync budget to 180 seconds.

**Architecture:** Continue using req/v3 because it already replays multipart file bodies. Add a pure retry-delay helper to the SDS client and change only the ListingKit SDS workflow deadline. The retry count and retryable status-code rules stay unchanged.

**Tech Stack:** Go, req/v3, Go standard library testing.

## Global Constraints

- Keep `RetryCount=2`, which means three total multipart upload attempts.
- Keep retry classification: transport errors, HTTP `429`, and HTTP `5xx`.
- Use capped exponential backoff with bounded jitter from the configured 1.5-second base interval.
- Set `SDSDesignSyncTimeout` to 180 seconds; preserve the existing capped per-variant extension.

---

### Task 1: Apply retry timing and workflow-budget policy

**Files:**

- Modify: `internal/sds/client/client.go:30-51`
- Modify: `internal/sds/client/client_test.go`
- Modify: `internal/listingkit/workflow/sds_sync_policy.go:5`
- Modify: `internal/listingkit/workflow/sds_sync_policy_test.go`

**Interfaces:**

- Consumes: `Config.RetryInterval time.Duration` and req/v3 common retry callback.
- Produces: `retryIntervalWithJitter(base time.Duration, attempt int) time.Duration`.

- [ ] **Step 1: Write failing tests**

Add `TestRetryIntervalWithJitterUsesBoundedExponentialBackoff` in `internal/sds/client/client_test.go`. With a 1.5-second base, assert attempt 1 is in `[1.5s,3s)`, attempt 2 is in `[3s,6s)`, and attempt 3 is in `[6s,12s)`. Change the single-variant expectation in `internal/listingkit/workflow/sds_sync_policy_test.go` from 130 seconds to 180 seconds.

- [ ] **Step 2: Verify RED**

Run `go test ./internal/sds/client ./internal/listingkit/workflow -run 'TestRetryIntervalWithJitterUsesBoundedExponentialBackoff|TestSDSDesignSyncTimeoutForVariantCount' -count=1`. Expect the missing helper and the current 130-second timeout to fail.

- [ ] **Step 3: Implement minimal policy**

In `internal/sds/client/client.go`, replace the current `attempt * config.RetryInterval` callback with `retryIntervalWithJitter(config.RetryInterval, attempt)`. The helper normalizes attempts below one, computes `base << (attempt-1)`, then returns a value from that delay (inclusive) to twice that delay (exclusive) using `math/rand/v2`. Change `SDSDesignSyncTimeout` to `180 * time.Second`.

- [ ] **Step 4: Verify GREEN**

Run `gofmt -w internal/sds/client/client.go internal/sds/client/client_test.go internal/listingkit/workflow/sds_sync_policy.go internal/listingkit/workflow/sds_sync_policy_test.go`, then `go test ./internal/sds/client ./internal/listingkit/workflow -count=1` and `go vet ./internal/sds/client ./internal/listingkit/workflow`. Expect all checks to pass.

- [ ] **Step 5: Broaden verification and commit**

Run `go test ./internal/listingkit/... -count=1` and `git diff --check`. Commit exactly the client policy, workflow deadline, and tests with message `fix(sds): back off multipart upload retries`.

# Local-agent 1688 Contract Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the merged 1688 local-agent protocol fail closed for untrusted facts, accurately classify terminal retries, and cancel browser crawls promptly.

**Architecture:** The server admits only validated snapshot facts before it constructs a `SourceEnvelope`; URL policy is centralized in the 1688 checker and unverified derived count data is removed from every contract edge. Job records retain a terminal token digest for safe retry classification. A context-aware integration seam reaches the browser owner, where `context.AfterFunc` closes the job browser exactly once.

**Tech Stack:** Go, standard-library `net/url`, `crypto/sha256`, `crypto/subtle`, `context.AfterFunc`, `sync.OnceFunc`, Gin, Playwright-Go, Testify, Pester.

**Spec:** `docs/superpowers/specs/2026-08-22-local-agent-contract-hardening-design.md`

## Global Constraints

- This is a breaking pre-release cleanup: do not add a compatibility shim for `price_range_count` or the context-aware crawler interface.
- Decode the result wrapper and nested product snapshot with `DisallowUnknownFields`; removed or undeclared protocol fields must fail with `400 invalid_request`.
- Use only the Go standard library for URL parsing, token digests, cancellation, and synchronization; add no dependency.
- Never place a rejected URL or execution token in an error message, log, response, test failure, or retained job record.
- Preserve the existing public 1688 offer URL policy and signed media query strings.
- Do not run a live 1688 crawl, mutate tenant/account selection, deploy, or change ListingKit workflow ownership.
- Stage only the files named in each task.

---

### Task 1: Centralize untrusted snapshot URL admission

**Files:**
- Create: `internal/crawler/alibaba1688/external_url.go`
- Create: `internal/crawler/alibaba1688/external_url_test.go`
- Modify: `internal/crawler/alibaba1688/product_checker.go:99-119,244-304`
- Modify: `internal/crawler/alibaba1688/product_checker_test.go`

**Interfaces:**
- Produces: `isValidMediaURL(raw string) bool` and `isValidSupplierShopURL(raw string) bool`, both backed by `isValidExternalURL(raw string, policy externalURLPolicy) bool`.
- Consumes: `ProductChecker.validateImages` and `ProductChecker.validateSupplier` validate every URL-bearing local-agent snapshot field before `SourceEnvelope` construction.

- [ ] **Step 1: Write the failing URL-policy tests**

Add a table-driven test that builds a complete product and injects
`https://user:secret@cdn.example/image.jpg` into each of `MainImage`,
`Images`, video URL, video cover, detail image, variant image, and package
image. Require `ValidateProduct` to reject each case without including
`user:secret` in its error. Add supplier cases for credential-bearing URL,
HTTP URL, query string, and fragment, plus a media assertion that a signed URL
such as `https://cdn.example/image.jpg?signature=abc` remains accepted.

```go
func TestValidateProductRejectsCredentialBearingAssetURLs(t *testing.T) {
    for _, mutate := range credentialBearingAssetMutations() {
        product := completeProduct()
        mutate(&product)
        err := NewProductChecker().ValidateProduct(&product)
        require.Error(t, err)
        require.NotContains(t, err.Error(), "user:secret")
    }
}
```

- [ ] **Step 2: Run the focused tests to verify red**

Run: `go test ./internal/crawler/alibaba1688 -run 'TestValidateProductRejectsCredentialBearingAssetURLs|TestValidateSupplierURLPolicy' -count=1`

Expected: FAIL because `isValidMediaURL` currently accepts parsed URL user-info and image errors reflect the raw URL.

- [ ] **Step 3: Implement one explicit URL policy helper**

Create `external_url.go` with this policy shape and use it from both current
helper functions. Media allows a query string; shop facts do not.

```go
type externalURLPolicy struct {
    requireHTTPS  bool
    allowQuery    bool
    allowFragment bool
}

func isValidExternalURL(raw string, policy externalURLPolicy) bool {
    parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
    if err != nil || parsed.Hostname() == "" || parsed.User != nil {
        return false
    }
    if parsed.Scheme != "http" && parsed.Scheme != "https" {
        return false
    }
    if policy.requireHTTPS && parsed.Scheme != "https" {
        return false
    }
    return (policy.allowQuery || parsed.RawQuery == "") &&
        (policy.allowFragment || parsed.Fragment == "")
}
```

Make `validateImages` and `validateSupplier` return field/index-only
messages, for example `图片[0]URL格式无效`; do not interpolate raw values.

- [ ] **Step 4: Run focused checker tests to verify green**

Run: `go test ./internal/crawler/alibaba1688 -count=1`

Expected: PASS, including existing product validation and the new field matrix.

- [ ] **Step 5: Commit the URL-admission boundary**

```powershell
git add -- internal/crawler/alibaba1688/external_url.go internal/crawler/alibaba1688/external_url_test.go internal/crawler/alibaba1688/product_checker.go internal/crawler/alibaba1688/product_checker_test.go
git commit -m "fix: centralize 1688 snapshot URL admission"
```

### Task 2: Remove unverified price-range count from the protocol

**Files:**
- Modify: `internal/product/sourcing/a1688_snapshot.go:5-35`
- Modify: `internal/product/sourcing/a1688_source_envelope.go:213-231`
- Modify: `internal/product/sourcing/a1688_source_envelope_test.go`
- Modify: `internal/integration/crawler/a1688/legacy_product_snapshot.go:18-27`
- Modify: `internal/integration/crawler/a1688/legacy_product_snapshot_test.go`
- Modify: `internal/localagent/httpapi/handler.go:40-128`
- Modify: `internal/localagent/httpapi/handler_test.go`
- Modify: `internal/localagent/client/client.go:26-111`
- Modify: `internal/localagent/client/client_test.go`

**Interfaces:**
- Removes: `Alibaba1688ProductSnapshot.PriceRangeCount`, the HTTP `price_range_count` request field, the client payload field, and the `SupplierOrCostFacts.Facts["price_range_count"]` output.
- Produces: a source envelope whose cost facts contain only source data that the submitted snapshot can substantiate, and a closed HTTP schema that rejects undeclared wrapper and snapshot fields.

- [ ] **Step 1: Write the failing absence tests**

Change the source-envelope test to assert its facts map lacks
`price_range_count`. Change the HTTP/client serialization tests to marshal a
snapshot and assert the raw JSON has no `price_range_count` key.
Add handler tests that submit `price_range_count` inside `product_snapshot`
and `source_account_id` in the result wrapper; both must return `400` with
`invalid_request`.

```go
_, found := envelope.SupplierOrCostFacts.Facts["price_range_count"]
require.False(t, found)

encoded, err := json.Marshal(snapshotPayload(snapshot))
require.NoError(t, err)
require.NotContains(t, string(encoded), "price_range_count")
```

- [ ] **Step 2: Run targeted tests to verify red**

Run: `go test ./internal/product/sourcing ./internal/integration/crawler/a1688 ./internal/localagent/httpapi ./internal/localagent/client -run 'PriceRangeCount|SnapshotRequest|SubmitSuccessUsesSnakeCase' -count=1`

Expected: FAIL because the count is still mapped and emitted as an envelope fact,
and the existing JSON binding silently ignores both undeclared fields.

- [ ] **Step 3: Delete the field at every boundary**

Remove the field and its assignment from the domain snapshot, legacy adapter,
HTTP request translator, client payload translator, and source envelope. Do
not substitute a count limit or a default value. Replace permissive Gin JSON
binding for the result wrapper with a size-preserving standard-library decoder
that calls `DisallowUnknownFields` and rejects trailing JSON. Decode the nested
`product_snapshot` through the same strict rule. Update tests that assert the
old mapping or the formerly ignored `source_account_id` behavior.

- [ ] **Step 4: Run affected package tests to verify green**

Run: `go test ./internal/product/sourcing ./internal/integration/crawler/a1688 ./internal/localagent/httpapi ./internal/localagent/client -count=1`

Expected: PASS; no serialized or canonical fact can contain the removed field,
and removed or undeclared wire fields are rejected at admission.

- [ ] **Step 5: Commit protocol narrowing**

```powershell
git add -- internal/product/sourcing/a1688_snapshot.go internal/product/sourcing/a1688_source_envelope.go internal/product/sourcing/a1688_source_envelope_test.go internal/integration/crawler/a1688/legacy_product_snapshot.go internal/integration/crawler/a1688/legacy_product_snapshot_test.go internal/localagent/httpapi/handler.go internal/localagent/httpapi/handler_test.go internal/localagent/client/client.go internal/localagent/client/client_test.go
git commit -m "fix: remove unverified 1688 price range count"
```

### Task 3: Preserve safe terminal retry classification

**Files:**
- Modify: `internal/localagent/service.go:55-68,183-245,323-389`
- Modify: `internal/localagent/service_test.go`
- Modify: `internal/localagent/httpapi/handler_test.go`

**Interfaces:**
- Produces: `executionTokenDigest(token string) [sha256.Size]byte` and a terminal record digest populated by both `SubmitSuccess` and `SubmitFailure`.
- Preserves: `ErrTerminalJob` maps to HTTP `409 job_not_active`; a wrong token maps to `ErrInvalidClaim` and HTTP `403 claim_denied`.

- [ ] **Step 1: Write failing service and HTTP retry tests**

Complete one job, then repeat `SubmitSuccess` with its original token and
require `ErrTerminalJob`; repeat with a different token and require
`ErrInvalidClaim`. Exercise the same path through `SubmitResult` and assert
the two response-code/error-code pairs.

```go
_, err = service.SubmitSuccess(actor, job.ID, claim.ExecutionToken, snapshot)
require.ErrorIs(t, err, ErrTerminalJob)

_, err = service.SubmitSuccess(actor, job.ID, "other-token", snapshot)
require.ErrorIs(t, err, ErrInvalidClaim)
```

- [ ] **Step 2: Run retry tests to verify red**

Run: `go test ./internal/localagent ./internal/localagent/httpapi -run 'Terminal.*Retry|Duplicate.*Submission' -count=1`

Expected: FAIL because terminalization clears `executionToken` before `claimRecordLocked` reaches its terminal-state branch.

- [ ] **Step 3: Store only a terminal SHA-256 token digest**

Add a fixed-size digest field to `record`. On either terminal transition,
digest the active token before clearing it. In `claimRecordLocked`, look up
the tenant-owned record first; for a terminal record, constant-time compare
the presented token digest to the stored digest and return `ErrTerminalJob`
only on a match. Clear the digest whenever a pending record receives a new
claim.

```go
func executionTokenDigest(token string) [sha256.Size]byte {
    return sha256.Sum256([]byte(token))
}
```

Use local digest variables before slicing arrays for
`subtle.ConstantTimeCompare`, and do not retain the raw token after a terminal
transition.

- [ ] **Step 4: Run local-agent tests to verify green**

Run: `go test ./internal/localagent ./internal/localagent/httpapi -race -count=1`

Expected: PASS; original-token retries are conflict responses and invalid
tokens remain authorization failures.

- [ ] **Step 5: Commit retry semantics**

```powershell
git add -- internal/localagent/service.go internal/localagent/service_test.go internal/localagent/httpapi/handler_test.go
git commit -m "fix: classify duplicate local agent submissions"
```

### Task 4: Propagate cancellation to the browser owner

**Files:**
- Modify: `internal/integration/crawler/a1688/processor.go:15-55`
- Modify: `internal/integration/crawler/a1688/processor_test.go`
- Modify: `internal/crawler/alibaba1688/processor.go:38-61`
- Modify: `internal/crawler/alibaba1688/single_processor.go:52-118`
- Modify: `internal/crawler/alibaba1688/single_processor_test.go`
- Modify: `internal/crawler/alibaba1688/page_operator.go:25-160`
- Modify: `internal/crawler/alibaba1688/page_operator_test.go`
- Modify: `internal/crawler/alibaba1688/worker_processor.go:60-100`
- Modify: `internal/crawler/alibaba1688/worker_processor_test.go`

**Interfaces:**
- Changes: `a1688.Source.Process(context.Context, string)` and all legacy processor call sites to propagate the caller context.
- Produces: `closeOnContextDone(ctx context.Context, close func()) func()`; its returned function stops the callback and invokes `close` exactly once.
- Preserves: `Runner.submitFailure` uses `context.WithoutCancel`, so an interrupted crawl reaches terminal failure reporting.

- [ ] **Step 1: Write failing propagation and cleanup tests**

Change `stubSource1688` to record its context and require the adapter to pass
the exact live context. Add a unit test for the cleanup helper that cancels a
context, waits for one close signal, invokes the returned finalizer, and
asserts the close count remains one. Add a page-operator test proving
`waitForContext` returns `context.Canceled` instead of waiting for its full
duration. Update worker fake processors to expose the received context.

```go
func TestCloseOnContextDoneClosesOnce(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    var closes atomic.Int32
    finish := closeOnContextDone(ctx, func() { closes.Add(1) })
    cancel()
    require.Eventually(t, func() bool { return closes.Load() == 1 }, time.Second, time.Millisecond)
    finish()
    require.Equal(t, int32(1), closes.Load())
}
```

- [ ] **Step 2: Run cancellation-focused tests to verify red**

Run: `go test ./internal/integration/crawler/a1688 ./internal/crawler/alibaba1688 -run 'Context|Cancel|CloseOnce' -count=1`

Expected: FAIL or not compile because the `Source` contract does not currently
accept a context and no cancellation cleanup helper exists.

- [ ] **Step 3: Change the crawler seam and install cleanup at page ownership**

Require `Process(ctx, url)` in `a1688.Source`; propagate it through
`Processor`, `Alibaba1688Processor`, `SingleProcessor`, account-profile
processing, page-operator navigation, and worker calls. Replace all fixed
page-operation sleeps with `waitForContext(ctx, duration)` so cancellation does
not wait for a three-second navigation/readiness delay or a scroll delay. After
`CreateBrowser` returns its cleanup, install the helper below and return
`ctx.Err()` whenever cancellation caused a browser operation to end.

```go
func closeOnContextDone(ctx context.Context, close func()) func() {
    closeOnce := sync.OnceFunc(close)
    stop := context.AfterFunc(ctx, closeOnce)
    return func() {
        stop()
        closeOnce()
    }
}
```

Use `defer closeOnContextDone(ctx, cleanup)()` immediately after browser
creation. Add `ctx.Err()` checks before browser creation, after navigation,
after extraction, and before returning success. Pass the context through the
worker's public and account-profile paths.

- [ ] **Step 4: Run crawler, integration, and runner tests to verify green**

Run: `go test ./internal/crawler/alibaba1688 ./internal/integration/crawler/a1688 ./internal/localagent -race -count=1`

Expected: PASS; source contexts are propagated and cancellation cleanup occurs
once without a data race.

- [ ] **Step 5: Commit cancellation propagation**

```powershell
git add -- internal/integration/crawler/a1688/processor.go internal/integration/crawler/a1688/processor_test.go internal/crawler/alibaba1688/processor.go internal/crawler/alibaba1688/single_processor.go internal/crawler/alibaba1688/single_processor_test.go internal/crawler/alibaba1688/worker_processor.go internal/crawler/alibaba1688/worker_processor_test.go
git commit -m "fix: cancel local agent browser crawls promptly"
```

### Task 5: Verify the follow-up as one contract change

**Files:**
- Modify only if verification exposes a defect in the files named by Tasks 1-4.

**Interfaces:**
- Verifies: every local-agent source fact has passed admission, terminal retry classification is stable, and cancellation reaches browser cleanup.

- [ ] **Step 1: Run focused end-to-end contract suites**

Run:

```powershell
go test ./internal/localagent/... -race -count=1
go test ./internal/crawler/alibaba1688/... -race -count=1
go test ./internal/integration/crawler/a1688/... -race -count=1
go test ./internal/product/sourcing/... -count=1
go test ./cmd/1688-local-agent -count=1
go build ./cmd/1688-local-agent
Invoke-Pester -Path scripts/1688-local-agent-acceptance.Tests.ps1 -PassThru
```

Expected: every command exits zero; Pester reports zero failures.

- [ ] **Step 2: Run repository regression and diff checks**

Run:

```powershell
go test ./... -count=1
git diff --check master...HEAD
git status --short --branch
```

Expected: all tests pass, the diff check is clean, and only planned commits are
present on `codex/local-agent-contract-hardening`.

- [ ] **Step 3: Commit only verification-driven fixes, if any**

If focused or repository verification required a code correction, stage only
the exact affected files and commit it with a `fix:` message that names the
verified contract defect. If no correction was needed, do not create an empty
commit.

- [ ] **Step 4: Request a final code review before publication**

Use the repository review workflow to inspect the final diff against
`master`, with special attention to URL reflection, token retention,
authorization classification, cancellation races, and every changed call site
of `Process`.

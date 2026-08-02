# SHEIN Login Cancellation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Make SHEIN login cancellation real and race-safe from the UI through the Redis-backed worker, while keeping the dedicated login worker single-concurrency.

**Architecture:** Reuse the existing DELETE /accounts/:store_id/verify-code-wait route and Redis control key. Extend the worker so each attempt has one cancellation-aware context and one owner of control-message consumption; make terminal completion conditional on the attempt still being active. Add a frontend cancellation mutation and explicit cancellation controls without changing Cookie-clearing semantics or worker parallelism.

**Tech Stack:** Go, Gin, Redis Streams/lists, Playwright automation abstraction, React, TanStack Query, Vitest, miniredis.

## Global Constraints

- Preserve the existing queued, launching, waiting_verify_code, succeeded, failed, cancelled, and interrupted status values.
- Cancellation must preserve existing Cookie state; only the explicit clear-Cookie action deletes Cookie state.
- Cancellation must win races against later Worker success/failure completion.
- Keep RunWorker message consumption serial and do not change maxConcurrentLogins in this plan.
- Preserve the unrelated working-tree edit in scripts/migrate-yudao-users-to-zitadel.ps1.
- Use the existing Redis Streams/control-key design; do not add another messaging system.

---

### Task 1: Add race-safe attempt completion and idempotent cancellation

**Files:**
- Modify: internal/sheinlogin/store.go:455-465,517-611
- Modify: internal/sheinlogin/service.go:768-800
- Test: internal/sheinlogin/service_test.go:305-367

**Interfaces:**
- Produce RedisStore.CompleteLoginAttemptIfActive(ctx context.Context, attempt *LoginAttempt) (bool, error).
- Make RedisStore.CancelLoginAttempt idempotent: an already-cancelled attempt returns success without pushing another control message.
- Keep Service.CancelVerifyCodeWait(ctx context.Context, tenantID, storeID int64) (bool, error) as the HTTP-facing method.

- [ ] Step 1: Write failing tests for terminal-state protection and repeated cancellation.

Seed an attempt, cancel it, construct a stale success attempt, and assert conditional completion returns false while persisted status remains cancelled. Call cancellation twice and assert the second call adds no control-list item.

~~~go
func TestCompleteLoginAttemptIfActiveDoesNotOverwriteCancellation(t *testing.T) {
    // Seed, cancel, attempt stale completion, assert cancelled remains terminal.
}
~~~

- [ ] Step 2: Run the focused tests and verify they fail.

~~~powershell
go test ./internal/sheinlogin -run 'TestCompleteLoginAttemptIfActive|TestCancelLoginAttemptIsIdempotent' -count=1
~~~

Expected: compilation failure or assertion failure because conditional completion and idempotent cancellation are not implemented.

- [ ] Step 3: Implement Redis compare-and-set completion.

Add one Redis Lua script that reads the serialized attempt, returns false when the current status is not queued, launching, or waiting_verify_code, and otherwise writes the supplied terminal attempt, latest-attempt key, TTL, and active-lease deletion atomically.

- [ ] Step 4: Make cancellation idempotent.

Update the cancellation script to distinguish active, already-cancelled, and other terminal states. Only the active branch writes the cancellation control command. Keep tenant/store validation and session cleanup in Service.CancelVerifyCodeWait.

- [ ] Step 5: Run the focused tests and verify they pass.

~~~powershell
go test ./internal/sheinlogin -run 'TestCompleteLoginAttemptIfActive|TestCancelLoginAttemptIsIdempotent|TestCancelVerifyCodeWaitSignalsOwningWorker|TestClearCookieSignalsOwningWorker' -count=1
~~~

Expected: PASS.

- [ ] Step 6: Commit the backend state-safety slice.

~~~powershell
git add internal/sheinlogin/store.go internal/sheinlogin/service.go internal/sheinlogin/service_test.go
git commit -m "fix(shein): make login cancellation terminal and idempotent"
~~~

---

### Task 2: Make Worker browser execution cancellation-aware

**Files:**
- Modify: internal/sheinlogin/worker.go:72-183
- Modify: internal/sheinlogin/service.go:220-346,538-568
- Modify: internal/sheinlogin/store.go:557-611
- Test: internal/sheinlogin/service_test.go:266-338

**Interfaces:**
- Produce one attempt-level control consumer used for cancellation and verification-code events.
- Make processLoginAttemptMessage use an attempt child context and conditional terminal completion.
- Pass the attempt context to existing StartLogin, SubmitCode, and WaitForLogin methods.

- [ ] Step 1: Add a blocking automation stub that observes context cancellation.

Extend the existing test automation with channels. StartLogin closes started, waits for ctx.Done, closes cancelled, and returns ctx.Err.

~~~go
type blockingLoginAutomation struct {
    started   chan struct{}
    cancelled chan struct{}
}

func (a *blockingLoginAutomation) StartLogin(ctx context.Context, _ Account, _ AutomationConfig) (*AutomationResult, VerifySession, error) {
    close(a.started)
    <-ctx.Done()
    close(a.cancelled)
    return nil, nil, ctx.Err()
}
~~~

Add a test that enqueues an attempt, waits for launching, calls CancelVerifyCodeWait, and asserts the automation observed cancellation and the final status is cancelled.

- [ ] Step 2: Run the new test and verify it fails.

~~~powershell
go test ./internal/sheinlogin -run TestWorkerCancelsBrowserExecution -count=1
~~~

Expected: FAIL because current Worker code does not consume cancellation while StartLogin is executing.

- [ ] Step 3: Add one attempt-level control loop.

The loop must be the only consumer of the attempt control key and verification-code queue. It returns cancellation for the cancel command, returns a submitted code with received=true, observes parent shutdown in bounded one-second intervals, and stops when the attempt ends. Do not let two goroutines consume the same Redis control list.

- [ ] Step 4: Bind Worker execution to the attempt context.

In processLoginAttemptMessage create attemptCtx and cancelAttempt. Start the control loop before loginInline. On cancel, call cancelAttempt. Ensure all branches close the session and stop the control loop. Treat context cancellation caused by the user as cancelled, not failed.

- [ ] Step 5: Use conditional terminal completion.

Replace unconditional Worker completion writes with CompleteLoginAttemptIfActive. If it returns false, load and preserve the persisted terminal state, close local resources, and acknowledge the stream entry.

- [ ] Step 6: Run focused and package tests.

~~~powershell
go test ./internal/sheinlogin -run 'TestWorkerCancelsBrowserExecution|TestWorkerLeavesVerificationJobPendingOnShutdown|TestCancelVerifyCodeWaitSignalsOwningWorker|TestClearCookieSignalsOwningWorker' -count=1
go test ./internal/sheinlogin/... -count=1
~~~

Expected: PASS.

- [ ] Step 7: Commit the Worker cancellation slice.

~~~powershell
git add internal/sheinlogin/worker.go internal/sheinlogin/service.go internal/sheinlogin/store.go internal/sheinlogin/service_test.go
git commit -m "fix(shein): cancel active login worker executions"
~~~

---

### Task 3: Expose cancellation in the ListingKit UI

**Files:**
- Modify: web/listingkit-ui/src/lib/api/shein-login.ts:39-77
- Modify: web/listingkit-ui/src/lib/query/use-shein-login.ts:32-62
- Modify: web/listingkit-ui/src/components/listingkit/shein-login/shein-login-page.tsx:314-411,536-895
- Test: web/listingkit-ui/src/lib/api/shein-login.test.ts
- Test: create web/listingkit-ui/src/components/listingkit/shein-login/shein-login-page.test.tsx

**Interfaces:**
- Produce cancelSheinLogin(storeID: number, tenantID?: string) using DELETE /accounts/\${storeID}/verify-code-wait.
- Produce useCancelSheinLogin(tenantID?: string), reusing the existing account-query invalidation.
- Give VerifyCodeDialog separate onClose and onCancelLogin callbacks; onClose only hides the dialog.

- [ ] Step 1: Write failing API and component tests.

Mock fetch, invoke cancelSheinLogin(870, "227"), and assert the request path includes tenant_id=227 and method DELETE. Render an active account, assert the cancel control is visible, click it, and verify the mutation is called. Close the verification dialog without pressing the cancellation action and assert the mutation is not called.

~~~ts
expect(fetchMock).toHaveBeenCalledWith(
  "/api/shein-login/accounts/870/verify-code-wait?tenant_id=227",
  expect.objectContaining({ method: "DELETE" }),
);
~~~

- [ ] Step 2: Run the new UI tests and verify they fail.

From web/listingkit-ui:

~~~powershell
npm.cmd exec vitest run src/lib/api/shein-login.test.ts src/components/listingkit/shein-login/shein-login-page.test.tsx
~~~

Expected: FAIL because the API function, hook, and cancellation controls do not exist.

- [ ] Step 3: Add the API function and React Query mutation.

Implement cancelSheinLogin beside clearSheinCookie. Add useCancelSheinLogin and reuse useInvalidateSheinLoginAccounts.

- [ ] Step 4: Add explicit UI actions.

Show Cancel Login when login_in_progress is true or latest attempt status is queued, launching, or waiting_verify_code. Keep dialog Close as display-only. Add a distinct Cancel Login action that calls the mutation, shows pending/error state, closes on success, and invalidates account status.

- [ ] Step 5: Run API, component, and existing SHEIN UI tests.

~~~powershell
npm.cmd exec vitest run src/lib/api/shein-login.test.ts src/components/listingkit/shein-login/shein-login-page.test.tsx
npm.cmd exec vitest run src/lib/api/shein-login.test.ts src/components/listingkit/shein-login --passWithNoTests
~~~

Expected: PASS.

- [ ] Step 6: Commit the UI cancellation slice.

~~~powershell
git add web/listingkit-ui/src/lib/api/shein-login.ts web/listingkit-ui/src/lib/query/use-shein-login.ts web/listingkit-ui/src/components/listingkit/shein-login/shein-login-page.tsx web/listingkit-ui/src/lib/api/shein-login.test.ts web/listingkit-ui/src/components/listingkit/shein-login/shein-login-page.test.tsx
git commit -m "feat(shein): expose login cancellation in ListingKit"
~~~

---

### Task 4: Run complete focused verification

**Files:**
- Test only; no source changes unless a verification failure identifies a defect in Tasks 1-3.

- [ ] Step 1: Run complete SHEIN backend tests.

~~~powershell
go test ./internal/sheinlogin/... -count=1
~~~

- [ ] Step 2: Run complete relevant UI tests.

~~~powershell
Set-Location web/listingkit-ui
npm.cmd exec vitest run src/lib/api/shein-login.test.ts src/components/listingkit/shein-login --passWithNoTests
Set-Location ../..
~~~

- [ ] Step 3: Check formatting and working-tree scope.

~~~powershell
gofmt -w internal/sheinlogin/store.go internal/sheinlogin/service.go internal/sheinlogin/worker.go internal/sheinlogin/service_test.go
git diff --check
git status --short
~~~

Expected: only intended SHEIN files plus the pre-existing scripts/migrate-yudao-users-to-zitadel.ps1 edit are present. Do not stage or modify the unrelated script.

- [ ] Step 4: Verify the no-concurrency-change constraint.

Confirm the diff does not change RunWorker from serial message processing and does not change maxConcurrentLogins, worker.concurrency, or the Kubernetes Worker replica count.

- [ ] Step 5: Report evidence separately.

Report backend tests, frontend tests, diff hygiene, and the unchanged concurrency boundary separately. Do not claim runtime browser cancellation until the controlled automation test and a deployed manual check have both succeeded.


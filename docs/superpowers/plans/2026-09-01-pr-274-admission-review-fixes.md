# PR #274 Admission Review Fixes Implementation Plan

> Superseded implementation plan: the active policy has no architecture
> override path or review/reconciliation workflows. This file is retained as
> historical context.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the remaining PR #274 review findings while preserving the trusted development-admission authorization boundary.

**Architecture:** Keep `.github/scripts/pr-scope-guard.cjs` as the pure evaluator and `.github/workflows/development-admission.yml` as its trusted caller. Add a status-output branch for authorized overrides and a dedicated, non-privileged CI job whose path filters cover the evaluator and its workflow contract.

**Tech Stack:** Node.js 22 built-in `node:test`, GitHub Actions YAML, CommonJS.

**Spec:** `docs/superpowers/specs/2026-09-01-pr-274-admission-review-fixes-design.md`

## Global Constraints

- `.github/scripts/pr-scope-guard.cjs` remains the policy evaluator; `.github/workflows/development-admission.yml` remains the only caller that reads GitHub admission inputs and publishes the status.
- The evaluator must continue to fail closed for unstable or incomplete pull-request snapshots.
- An override still requires the exact `architecture-approved` label and an eligible maintainer/admin approval for the current head.
- A `COMMENTED` or `PENDING` review is non-decisive and must not revoke an existing approval; `CHANGES_REQUESTED` and `DISMISSED` remain decisive revocations.
- Status descriptions must distinguish ordinary allowance from allowance caused by `result.overridden === true`; status state and target SHA do not change.
- CI test wiring is non-privileged and must not alter the trusted evaluator's authorization boundary.
- No new persistence boundary, idempotency key, recovery protocol, or external write is introduced.

### Task 1: Add red regression tests for status audit output and CI wiring

**Files:**
- Modify: `.github/scripts/pr-scope-guard.test.cjs`

**Interfaces:**
- Consumes: `statusForEvaluation` from `.github/scripts/pr-scope-guard.cjs`; `.github/workflows/ci.yml` as a text contract.
- Produces: failing tests that specify the override-specific status description and the required CI trigger/job/command.

- [ ] **Step 1: Add the status regression test**

Import `node:fs` and `node:path`, then add a test next to the existing status test:

```js
test("identifies an authorized architecture override in the commit status", () => {
  assert.deepEqual(statusForEvaluation({ allowed: true, overridden: true }), {
    state: "success",
    description: "Allowed by authorized architecture override",
  });
});
```

The test must assert the public status object, not implementation details.

- [ ] **Step 2: Add the CI contract regression test**

Add this test after the status tests:

```js
test("runs admission guard tests when policy files change", () => {
  const workflowPath = path.join(__dirname, "..", "workflows", "ci.yml");
  const workflow = fs.readFileSync(workflowPath, "utf8");

  assert.equal(
    workflow.match(/- "\\.github\\/scripts\\/\\*\\*"/g)?.length,
    2,
  );
  assert.match(workflow, /development-admission-tests:/);
  assert.match(
    workflow,
    /run: node --test \\.github\\/scripts\\/pr-scope-guard\\.test\\.cjs/,
  );
  assert.match(
    workflow,
    /needs:\s*[\\s\\S]*-\\s+development-admission-tests\\b/,
  );
});
```

The two path-filter matches cover both `push` and `pull_request`; the job and notification dependency assertions prevent a silent or unreported policy-test failure.

- [ ] **Step 3: Run the focused suite and verify the tests fail for the intended reasons**

Run:

```powershell
node --test .github/scripts/pr-scope-guard.test.cjs
```

Expected: the existing tests pass, while the new status test fails because `statusForEvaluation` returns `Within admission limits`, and the CI contract test fails because `ci.yml` has no matching path filters/job yet. Do not change production code before observing these failures.

### Task 2: Implement the override-specific status description

**Files:**
- Modify: `.github/scripts/pr-scope-guard.cjs:292-302`
- Test: `.github/scripts/pr-scope-guard.test.cjs`

**Interfaces:**
- Consumes: evaluator results with `allowed` and `overridden` fields.
- Produces: the same `{ state, description }` status shape, with explicit audit text for overridden success.

- [ ] **Step 1: Add the minimal implementation**

Keep input validation and failure mapping unchanged. Change only the successful branch:

```js
function statusForEvaluation(result) {
  if (!result || typeof result.allowed !== "boolean") {
    throw new TypeError("result.allowed must be a boolean");
  }
  if (result.allowed && result.overridden === true) {
    return {
      state: "success",
      description: "Allowed by authorized architecture override",
    };
  }
  return result.allowed
    ? { state: "success", description: "Within admission limits" }
    : { state: "failure", description: "Exceeds admission limits" };
}
```

- [ ] **Step 2: Run the focused suite and verify status behavior is green**

Run:

```powershell
node --test .github/scripts/pr-scope-guard.test.cjs --test-name-pattern="status|override"
```

Expected: the new override test and all matching existing tests pass; the CI contract test remains the only failing new test until Task 3.

- [ ] **Step 3: Run the complete guard suite**

Run:

```powershell
node --test .github/scripts/pr-scope-guard.test.cjs
```

Expected: status-related tests pass and only the intentionally unmet CI contract test fails. If another test fails, stop and investigate that failure before continuing.

### Task 3: Wire the guard suite into CI and report its result

**Files:**
- Modify: `.github/workflows/ci.yml`
- Test: `.github/scripts/pr-scope-guard.test.cjs`

**Interfaces:**
- Consumes: the existing `ci.yml` push/PR path filters and `notify` job result aggregation.
- Produces: `development-admission-tests`, an independent Node test job triggered for policy-script changes and included in notification dependencies/results.

- [ ] **Step 1: Add `.github/scripts/**` to both path filters**

Insert `- ".github/scripts/**"` under both the `push.paths` and `pull_request.paths` lists, preserving the existing path filters.

- [ ] **Step 2: Add the dedicated test job**

Insert this job before `release-authority`:

```yaml
  development-admission-tests:
    name: Development Admission Policy Tests
    runs-on: ubuntu-latest
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Test pull request scope policy
        run: node --test .github/scripts/pr-scope-guard.test.cjs
```

This job has no write permissions and does not invoke the privileged `pull_request_target` workflow.

- [ ] **Step 3: Include the job in CI notification aggregation**

Add `development-admission-tests` to `notify.needs`, expose `${{ needs.development-admission-tests.result }}` as `DEVELOPMENT_ADMISSION_TEST_RESULT`, include it in the `results` list, and print its result alongside backend/frontend/code-health.

- [ ] **Step 4: Run the full guard suite and verify the CI contract turns green**

Run:

```powershell
node --test .github/scripts/pr-scope-guard.test.cjs
```

Expected: all tests pass, including the CI contract test.

### Task 4: Perform final repository verification

**Files:**
- Verify: `.github/scripts/pr-scope-guard.cjs`
- Verify: `.github/scripts/pr-scope-guard.test.cjs`
- Verify: `.github/workflows/ci.yml`

- [ ] **Step 1: Check the final diff for whitespace errors**

Run:

```powershell
git diff --check
```

Expected: exit code 0 and no output.

- [ ] **Step 2: Re-run the complete Node suite from the final tree**

Run:

```powershell
node --test .github/scripts/pr-scope-guard.test.cjs
```

Expected: every test passes with zero failures.

- [ ] **Step 3: Inspect the final diff and status**

Run:

```powershell
git diff --stat HEAD~2..HEAD
git status --short --branch
```

Confirm only the approved design/plan and the three implementation files changed, with no unrelated worktree edits.

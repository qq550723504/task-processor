# Development Admission Governance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add repository-wide development admission rules and a tested CI scope guard that stops oversized or architecture-sensitive work unless a maintainer explicitly approves the exception.

**Architecture:** Keep policy in four connected repository surfaces: root agent instructions, the PR declaration, the architecture checklist, and CI. Put scope classification in a pure CommonJS module tested with Node's built-in test runner; let the official `actions/github-script` action own GitHub pagination and API access. Run the authoritative guard from a dedicated, unfiltered `pull_request_target` workflow that checks out only the trusted default branch; keep proposed-policy tests in the ordinary CI workflow. Preserve the existing architecture checklist as the import-guard inventory authority.

**Tech Stack:** Markdown, CommonJS on Node.js 22, `node:test`, GitHub Actions, `actions/github-script@v9`, Go repository contract tests, YAML v3.

**Spec:** `docs/superpowers/specs/2026-09-01-development-admission-governance-design.md`

## Global Constraints

- Stop and reclassify work that crosses three subsystems, more than one consistency boundary, a state/recovery/security boundary, 30 scope-relevant files, 1,500 production additions, 2,500 production churn, or combines foundation refactoring with feature delivery.
- Architecture-sensitive designs require explicit invariants, transaction ownership, state transitions, retry identities, recovery ownership, and a durable-boundary failure matrix.
- The authoring task cannot approve its own architecture-sensitive design.
- Reuse a shared transaction or existing repository/open-source outbox, Saga, or Temporal facility before inventing compensation infrastructure.
- `architecture-approved` is the only size override label; it must be paired with a maintainer/admin `APPROVED` review for the current head SHA and cannot replace design evidence.
- Ordinary CI permissions remain read-only; the trusted admission workflow additionally has only `issues: read` and `checks: write` to read base-change events and publish its test-merge-SHA Check Run, and neither workflow creates labels or edits pull requests.
- The existing `Guard Baseline` remains the authoritative import-boundary guard inventory.

---

### Task 1: Pure Pull-Request Scope Classifier

**Files:**
- Create: `.github/scripts/pr-scope-guard.cjs`
- Create: `.github/scripts/pr-scope-guard.test.cjs`

**Interfaces:**
- Consumes: GitHub file objects `{ filename, status, additions, deletions }` and label names `string[]`.
- Produces: `LIMITS`, `classifyFile(filename)`, `evaluatePullRequest(files, labels)`, `formatEvaluation(result)`, and `statusForEvaluation(result)`.

- [ ] **Step 1: Write the failing tests**

Use `node:test` and `node:assert/strict`. Define a `source(filename, additions, deletions)` fixture and test:

~~~js
const test = require("node:test");
const assert = require("node:assert/strict");
const { LIMITS, classifyFile, evaluatePullRequest, formatEvaluation } = require("./pr-scope-guard.cjs");

test("classifies policy paths", () => {
  assert.equal(classifyFile("docs/rule.md").kind, "documentation");
  assert.equal(classifyFile("web/pnpm-lock.yaml").kind, "lockfile");
  assert.equal(classifyFile("internal/api/model.pb.go").kind, "generated");
  assert.equal(classifyFile("internal/store/service_test.go").kind, "test");
  assert.equal(classifyFile("internal/store/service.go").kind, "production");
});
~~~

Add exact-limit success cases, each independently exceeded limit, missing line counts, deleted files, 101 files, test-line exclusion, the wrong label, label-only rejection, stale/current-head review authorization, merge-target isolation, and exact `architecture-approved` override.

- [ ] **Step 2: Run the test and prove it fails**

Run: `node --test .github/scripts/pr-scope-guard.test.cjs`

Expected: FAIL with `Cannot find module './pr-scope-guard.cjs'`.

- [ ] **Step 3: Implement the classifier**

Use these exact constants and return shape:

~~~js
const LIMITS = Object.freeze({ scopeFiles: 30, productionAdditions: 1500, productionChurn: 2500 });
  const OVERRIDE_LABEL = "architecture-approved";

// evaluatePullRequest returns:
// { allowed, oversized, overridden, overrideLabelPresent, overrideAuthorized,
//   overrideLabel, limits,
//   metrics: { totalFiles, scopeFiles, productionAdditions, productionChurn },
//   exceeded, kinds }
~~~

Normalize slashes and case. Classification precedence is documentation, lockfile, generated, test, then production. Documentation includes `docs/**`, `*.md`, and `*.mdx`; locks include Go, npm, pnpm, Yarn, and Cargo lockfiles; generated includes a `generated` path segment, `*.pb.go`, `*.gen.go`, and generated snapshots; tests include `tests`, `test`, `__tests__`, `testdata`, and `fixtures` segments plus Go/JS test suffixes. Scope-file count excludes documentation, locks, and generated files but includes tests. Production totals exclude tests as well. Invalid non-array input throws; absent line counts become zero.

`formatEvaluation` reports measured values, limits, exceeded keys, and override status without GitHub calls.

- [ ] **Step 4: Run tests and prove they pass**

Run: `node --test .github/scripts/pr-scope-guard.test.cjs`

Expected: all tests PASS.

- [ ] **Step 5: Commit Task 1**

~~~powershell
git add -- .github/scripts/pr-scope-guard.cjs .github/scripts/pr-scope-guard.test.cjs
git commit -m "ci: add pull request scope classifier"
~~~

### Task 2: Repository Admission Contracts

**Files:**
- Create: `AGENTS.md`
- Modify: `.github/pull_request_template.md`
- Modify: `docs/architecture/architecture-review-checklist.md`

**Interfaces:**
- Consumes: thresholds and label from the approved spec and Task 1.
- Produces: agent stop rules, contributor declarations, and reviewer admission questions.

- [ ] **Step 1: Record the document mutations being guarded**

Before editing, record the failure each document change prevents:

- missing root instructions allow an agent to implement an invalid or oversized request;
- a missing PR declaration hides consistency boundaries and override rationale;
- a missing checklist section lets reviewers skip failure matrices and independent design evidence.

These are human/agent instruction documents. Do not add source-text tests that only lock wording; self-review them against the approved spec after editing.

- [ ] **Step 2: Create root `AGENTS.md`**

Use headings for basic principles, stop conditions, architecture-sensitive design, independent review, implementation/PR constraints, and instruction precedence. Preserve the user's four Chinese principles. State that crossing a threshold stops coding and returns to decomposition; require transaction-first decisions, durable-boundary failure matrices, fresh-context review, independently verifiable slices, Draft development, and batched sibling-path fixes. Nested instructions may add but not weaken these rules.

- [ ] **Step 3: Extend the PR template**

Insert `# Development Admission` before Validation and retain the existing architecture checklist. Add these fields:

~~~markdown
- Scope class: bounded / architecture-sensitive
- Affected subsystems:
- Consistency, authorization, or tenant boundaries:
- Scope metrics: files / production additions / production churn
- Design:
- Independent design review:
- Invariants and failure matrix:
- `architecture-approved` approver and split rationale (only when oversized):
~~~

Add explicit validation for lost responses, retries/restarts, concurrency, tenant/context drift, deadlines/resource bounds, or a justified `N/A`.

- [ ] **Step 4: Extend the architecture checklist**

Add `## Development Admission` before `## Required Checks`. Cover the thresholds, design completeness, independent challenge, transaction/open-source decision, failure matrix, sibling paths, and override governance. Do not change the existing `Guard Baseline` list.

- [ ] **Step 5: Self-review governance coverage**

Read the three documents side by side with the spec. Confirm identical thresholds and label name; every spec admission requirement has an owning document; nested instructions cannot weaken root rules; `Guard Baseline` remains unchanged; and the PR template requests fault-oriented evidence rather than a generic test checkbox.

- [ ] **Step 6: Commit Task 2**

~~~powershell
git add -- AGENTS.md .github/pull_request_template.md docs/architecture/architecture-review-checklist.md
git commit -m "docs: enforce development admission contracts"
~~~

### Task 3: GitHub Actions Enforcement

**Files:**
- Modify: `.github/workflows/ci.yml`
- Create: `.github/workflows/development-admission.yml`

**Interfaces:**
- Consumes: `evaluatePullRequest(files, labels, options)`, `hasAuthorizedArchitectureOverride(...)`, and `formatEvaluation(result)` from Task 1.
- Produces: a trusted, unfiltered `development-admission` job that enforces policy on `pull_request_target` events, plus a separate CI job that tests proposed policy code.

- [x] **Step 1: Inspect the existing workflow boundary**

The existing job was path-filtered and loaded its guard from the pull-request
merge revision, so it could be bypassed by omitted paths or a modified policy.
The classifier behavior is protected by unit tests; authoritative enforcement
must be separate and trusted.

- [x] **Step 2: Add workflow enforcement**

Keep the ordinary CI workflow read-only for repository contents and add the new trusted workflow paths to both push and pull-request filters. Add the trusted workflow with `actions: read`, `checks: write`, `contents: read`, `issues: read`, and `pull-requests: read`, no path filter, and `pull_request_target` activity types `opened`, `synchronize`, `reopened`, `edited`, `labeled`, and `unlabeled`. Add a separate `Development Admission Review Signal` workflow with no permissions for `pull_request_review` activity types `submitted`, `edited`, and `dismissed`; it writes only the event PR number to a short-lived artifact, and the trusted evaluator receives those events through `workflow_run`, validates the artifact number against the associated PR set, and never executes PR code. Add a no-permission base-branch signal workflow on all `push` events. Trigger the trusted reconciliation workflow from that signal's `workflow_run` and dispatch only open PRs whose base branch matches the signaled branch. Its five-minute schedule dispatches only open PRs carrying `architecture-approved`, targeting a non-default base branch, or having a recent `architecture-approved` label removal, bounding permission-revocation fan-out while covering long-lived base branches that do not yet contain the signal workflow. Before setting `overrideAuthorized`, fetch permissions for all current-head approvals, retain only eligible maintainer/admin approvals, and validate non-placeholder PR-body evidence against that authorized subset. Resolve the PR number in a prerequisite job so direct, review, and reconciliation evaluations share the same per-PR concurrency group; direct PR and dispatch events still run the evaluator when target resolution fails so the event merge SHA can receive an error Check Run.

Keep only the proposed-policy test job in `ci.yml` and add this structure to `.github/workflows/development-admission.yml`:

~~~yaml
evaluate-admission:
  name: Development Admission Evaluator
  runs-on: ubuntu-latest
  steps:
    - name: Checkout trusted policy revision
      uses: actions/checkout@v4
      with:
        ref: ${{ github.event.repository.default_branch }}
        persist-credentials: false
    - name: Enforce pull request scope and publish head status
      uses: actions/github-script@v9
~~~

The action script imports `./.github/scripts/pr-scope-guard.cjs`, publishes `pending` to the PR `merge_commit_sha`, reads the PR metadata before and after `github.paginate(github.rest.pulls.listFiles, { owner, repo, pull_number, per_page: 100 })`, and also snapshots labels, reviews, and base-reference-change events before and after evaluation. It fails on a moving head/base/merge/update/label/review snapshot or mismatched `changed_files`, evaluates the latest labels, and when the override label is present verifies a current-head `APPROVED` review from a non-author collaborator with `role_name` `maintain` or `admin`, submitted after the latest base retarget. It writes `formatEvaluation` to logs and the step summary, publishes `success`/`failure`/`error` with the fixed `Development Admission` context to the PR `merge_commit_sha`, and calls `core.setFailed` when the decision is not allowed or evaluation cannot complete. The failure message instructs the author to split the change or obtain the label plus protected current-head approval and design evidence. The ordinary CI workflow runs only `node --test .github/scripts/pr-scope-guard.test.cjs` for proposed policy changes and does not decide admission.

Implementation update: the trusted workflow grants `checks: write` and creates
the `Development Admission` Check Run through the GitHub Actions application;
the required branch rule must bind that check to app ID 15368. The evaluator
filters override evidence against eligible current-head approvers, the
reconciler includes recent override-label removals, and direct PR/dispatch
events still publish an error Check Run when target resolution fails.

The ordinary CI notification reports `DEVELOPMENT_ADMISSION_TEST_RESULT`; the trusted admission Check Run is a separate required check and is not coupled to the WeCom aggregation job.

- [ ] **Step 3: Validate behavior and workflow structure**

~~~powershell
node --test .github/scripts/pr-scope-guard.test.cjs
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workflows/ci.yml .github/workflows/development-admission.yml
~~~

Expected: all tests PASS.

- [x] **Step 4: Manually inspect workflow authority**

Confirm the trusted workflow has no path filter, uses only the default-branch policy revision, never checks out or executes PR code, has only `contents: read`, `issues: read`, `pull-requests: read`, and `statuses: write`, publishes the fixed context to `merge_commit_sha`, re-runs on label/edited/review changes, serializes evaluations per PR, verifies current-head maintainer/admin review and base-retarget freshness before accepting an override, fails closed on incomplete/moving snapshots, has a five-minute job deadline, and never creates/applies labels or edits PRs. Confirm ordinary CI runs policy tests on push without a skipped job and the notification names them as tests.

- [ ] **Step 5: Commit Task 3**

~~~powershell
git add -- .github/workflows/ci.yml .github/workflows/development-admission.yml
git commit -m "ci: enforce development admission limits"
~~~

### Task 4: Repository Label and Final Verification

**Files:**
- Verify only; no planned repository file changes.

**Interfaces:**
- Consumes: the completed governance surfaces and CI enforcement.
- Produces: the GitHub `architecture-approved` label and final evidence.

- [ ] **Step 1: Create or update the override label**

Run:

~~~powershell
gh label create architecture-approved --color B60205 --description "Maintainer-approved exception to development admission size limits" --force
~~~

Expected: the label exists with the exact name and description. Do not apply it to any pull request.

- [ ] **Step 2: Run focused verification**

~~~powershell
node --test .github/scripts/pr-scope-guard.test.cjs
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workflows/ci.yml
git diff --check origin/main...HEAD
~~~

Expected: PASS and no whitespace errors.

- [ ] **Step 3: Run governance regression tests**

Run: `go test ./tests -count=1`

Expected: PASS.

- [ ] **Step 4: Review final state**

~~~powershell
git status --short --branch
git log --oneline --decorate -6
git diff --stat origin/main...HEAD
~~~

Expected: clean worktree on `codex/development-admission-guardrails` with design, plan, classifier, governance, and CI commits.

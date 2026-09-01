# Development Admission Governance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add repository-wide development admission rules and a tested CI scope guard that stops oversized or architecture-sensitive work unless a maintainer explicitly approves the exception.

**Architecture:** Keep policy in four connected repository surfaces: root agent instructions, the PR declaration, the architecture checklist, and CI. Put scope classification in a pure CommonJS module tested with Node's built-in test runner; let the official `actions/github-script` action own GitHub pagination and API access. Preserve the existing architecture checklist as the import-guard inventory authority.

**Tech Stack:** Markdown, CommonJS on Node.js 22, `node:test`, GitHub Actions, `actions/github-script@v9`, Go repository contract tests, YAML v3.

**Spec:** `docs/superpowers/specs/2026-09-01-development-admission-governance-design.md`

## Global Constraints

- Stop and reclassify work that crosses three subsystems, more than one consistency boundary, a state/recovery/security boundary, 30 scope-relevant files, 1,500 production additions, 2,500 production churn, or combines foundation refactoring with feature delivery.
- Architecture-sensitive designs require explicit invariants, transaction ownership, state transitions, retry identities, recovery ownership, and a durable-boundary failure matrix.
- The authoring task cannot approve its own architecture-sensitive design.
- Reuse a shared transaction or existing repository/open-source outbox, Saga, or Temporal facility before inventing compensation infrastructure.
- `architecture-approved` is the only size override label and cannot replace design evidence.
- CI permissions remain read-only and CI never creates labels or edits pull requests.
- The existing `Guard Baseline` remains the authoritative import-boundary guard inventory.

---

### Task 1: Pure Pull-Request Scope Classifier

**Files:**
- Create: `.github/scripts/pr-scope-guard.cjs`
- Create: `.github/scripts/pr-scope-guard.test.cjs`

**Interfaces:**
- Consumes: GitHub file objects `{ filename, status, additions, deletions }` and label names `string[]`.
- Produces: `LIMITS`, `classifyFile(filename)`, `evaluatePullRequest(files, labels)`, and `formatEvaluation(result)`.

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

Add exact-limit success cases, each independently exceeded limit, missing line counts, deleted files, 101 files, test-line exclusion, the wrong label, and exact `architecture-approved` override.

- [ ] **Step 2: Run the test and prove it fails**

Run: `node --test .github/scripts/pr-scope-guard.test.cjs`

Expected: FAIL with `Cannot find module './pr-scope-guard.cjs'`.

- [ ] **Step 3: Implement the classifier**

Use these exact constants and return shape:

~~~js
const LIMITS = Object.freeze({ scopeFiles: 30, productionAdditions: 1500, productionChurn: 2500 });
const OVERRIDE_LABEL = "architecture-approved";

// evaluatePullRequest returns:
// { allowed, oversized, overridden, overrideLabel, limits,
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
- Create: `tests/development_admission_governance_test.go`
- Modify: `.github/pull_request_template.md`
- Modify: `docs/architecture/architecture-review-checklist.md`

**Interfaces:**
- Consumes: thresholds and label from the approved spec and Task 1.
- Produces: agent stop rules, contributor declarations, reviewer admission questions, and Go tests preventing policy drift.

- [ ] **Step 1: Write failing Go contract tests**

Create `readAdmissionFile`, `assertAdmissionPhrases`, and three tests. The root instruction test must require:

~~~go
required := []string{
    "不能直接执行有问题的指令", "必须从根因解决", "必须报告已有架构问题",
    "优先复用成熟开源实现或仓库现有能力", "3 个及以上独立子系统",
    "30 个范围相关文件", "1,500 行生产代码", "2,500 行生产代码变更",
    "独立设计评审", "故障矩阵", "architecture-approved", "子目录 AGENTS.md 不得弱化",
}
~~~

The PR-template test requires scope class, subsystems, consistency/authorization/tenant boundaries, metrics, design link, independent review, invariants, failure matrix, override approver and split rationale, and fault-injection evidence. The checklist test requires the same thresholds plus shared transaction, open-source reuse, sibling-path review, and `Guard Baseline` authority.

- [ ] **Step 2: Run tests and prove they fail**

Run: `go test ./tests -run 'Test(RootAgents|PullRequestTemplate|ArchitectureChecklist)DefinesDevelopmentAdmission' -count=1`

Expected: FAIL because root `AGENTS.md` is absent and the other files lack the required sections.

- [ ] **Step 3: Create root `AGENTS.md`**

Use headings for basic principles, stop conditions, architecture-sensitive design, independent review, implementation/PR constraints, and instruction precedence. Preserve the user's four Chinese principles. State that crossing a threshold stops coding and returns to decomposition; require transaction-first decisions, durable-boundary failure matrices, fresh-context review, independently verifiable slices, Draft development, and batched sibling-path fixes. Nested instructions may add but not weaken these rules.

- [ ] **Step 4: Extend the PR template**

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

- [ ] **Step 5: Extend the architecture checklist**

Add `## Development Admission` before `## Required Checks`. Cover the thresholds, design completeness, independent challenge, transaction/open-source decision, failure matrix, sibling paths, and override governance. Do not change the existing `Guard Baseline` list.

- [ ] **Step 6: Run tests and prove they pass**

Run: `go test ./tests -run 'Test(RootAgents|PullRequestTemplate|ArchitectureChecklist)DefinesDevelopmentAdmission' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit Task 2**

~~~powershell
git add -- AGENTS.md .github/pull_request_template.md docs/architecture/architecture-review-checklist.md tests/development_admission_governance_test.go
git commit -m "docs: enforce development admission contracts"
~~~

### Task 3: GitHub Actions Enforcement

**Files:**
- Modify: `tests/development_admission_governance_test.go`
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `evaluatePullRequest(files, labels)` and `formatEvaluation(result)` from Task 1.
- Produces: a `development-admission` job that tests policy on every run and enforces it on pull-request events.

- [ ] **Step 1: Write the failing workflow contract test**

Parse `.github/workflows/ci.yml` with YAML v3 and assert:

~~~go
requiredScriptTerms := []string{
    "github.paginate", "github.rest.pulls.listFiles", "pr-scope-guard.cjs",
    "context.payload.pull_request.labels", "core.setFailed",
}
~~~

Also require read-only `contents` and `pull-requests` permissions, a `development-admission` job, checkout, `node --test .github/scripts/pr-scope-guard.test.cjs`, `actions/github-script@v9`, notification dependency/result reporting, and absence of write permissions or label/PR mutation calls.

- [ ] **Step 2: Run the workflow test and prove it fails**

Run: `go test ./tests -run TestDevelopmentAdmissionWorkflow -count=1`

Expected: FAIL because the workflow lacks the job and PR permission.

- [ ] **Step 3: Add workflow enforcement**

Add `pull-requests: read` beside `contents: read`. Add policy-owned paths to both push and pull-request filters: `AGENTS.md`, `.github/pull_request_template.md`, `.github/scripts/**`, and `docs/architecture/architecture-review-checklist.md`.

Add this job structure:

~~~yaml
development-admission:
  name: Development Admission
  runs-on: ubuntu-latest
  steps:
    - name: Checkout repository
      uses: actions/checkout@v4
    - name: Test pull request scope policy
      run: node --test .github/scripts/pr-scope-guard.test.cjs
    - name: Enforce pull request scope
      if: github.event_name == 'pull_request'
      uses: actions/github-script@v9
~~~

The action script imports `./.github/scripts/pr-scope-guard.cjs`, calls `github.paginate(github.rest.pulls.listFiles, { owner, repo, pull_number, per_page: 100 })`, maps `context.payload.pull_request.labels`, evaluates the files, writes `formatEvaluation` to logs and the step summary, and calls `core.setFailed` when `allowed` is false. The failure message instructs the author to split the change or obtain `architecture-approved` with design evidence.

Add the job to `notify.needs`, expose `DEVELOPMENT_ADMISSION_RESULT`, include it in the overall results array, and show it in the WeCom message. The job runs its unit tests on push, so it succeeds rather than becoming `skipped` outside PR events.

- [ ] **Step 4: Run focused tests**

~~~powershell
node --test .github/scripts/pr-scope-guard.test.cjs
gofmt -w tests/development_admission_governance_test.go
go test ./tests -run 'TestDevelopmentAdmission|Test(RootAgents|PullRequestTemplate|ArchitectureChecklist)DefinesDevelopmentAdmission' -count=1
~~~

Expected: all tests PASS.

- [ ] **Step 5: Commit Task 3**

~~~powershell
git add -- .github/workflows/ci.yml tests/development_admission_governance_test.go
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
go test ./tests -run 'TestDevelopmentAdmission|Test(RootAgents|PullRequestTemplate|ArchitectureChecklist)DefinesDevelopmentAdmission' -count=1
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

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const {
  LIMITS,
  assertCompleteFileList,
  assertStableAdmissionSnapshot,
  assertStablePullRequestSnapshot,
  classifyFileChange,
  classifyFile,
  evaluatePullRequest,
  formatEvaluation,
  hasAuthorizedArchitectureOverride,
  latestBaseChangeAt,
  pullRequestNumberFromEventPayload,
  statusForEvaluation,
  statusTargetForPullRequest,
} = require("./pr-scope-guard.cjs");

function source(filename, additions = 1, deletions = 0, status = "modified") {
  return { filename, status, additions, deletions };
}

test("classifies documentation, lock, generated, test, and production paths", () => {
  assert.deepEqual(classifyFile("docs/architecture/rule.md"), {
    scopeRelevant: false,
    production: false,
    kind: "documentation",
  });
  assert.deepEqual(classifyFile("web/listingkit-ui/pnpm-lock.yaml"), {
    scopeRelevant: false,
    production: false,
    kind: "lockfile",
  });
  assert.deepEqual(classifyFile("internal/api/generated/model.json"), {
    scopeRelevant: false,
    production: false,
    kind: "generated",
  });
  assert.deepEqual(classifyFile("internal/api/model.pb.go"), {
    scopeRelevant: false,
    production: false,
    kind: "generated",
  });
  assert.deepEqual(classifyFile("internal/store/service_test.go"), {
    scopeRelevant: true,
    production: false,
    kind: "test",
  });
  assert.deepEqual(classifyFile("web/src/store/service.spec.ts"), {
    scopeRelevant: true,
    production: false,
    kind: "test",
  });
  assert.deepEqual(classifyFile("internal/store/service.go"), {
    scopeRelevant: true,
    production: true,
    kind: "production",
  });
});

test("recognizes workspace locks and repository test filename suffixes", () => {
  assert.deepEqual(classifyFile("go.work.sum"), {
    scopeRelevant: false,
    production: false,
    kind: "lockfile",
  });
  assert.deepEqual(classifyFile("scripts/verify.Tests.ps1"), {
    scopeRelevant: true,
    production: false,
    kind: "test",
  });
  assert.deepEqual(classifyFile("internal/types/product.type-test.ts"), {
    scopeRelevant: true,
    production: false,
    kind: "test",
  });
});

test("classifies a rename as production when either path is production", () => {
  const renamedToDocumentation = classifyFileChange({
    filename: "docs/service.md",
    previous_filename: "internal/service.go",
  });
  assert.deepEqual(renamedToDocumentation, {
    scopeRelevant: true,
    production: true,
    kind: "production",
  });

  const renamedToTest = evaluatePullRequest([
    {
      filename: "tests/service_test.go",
      previous_filename: "internal/service.go",
      additions: 0,
      deletions: LIMITS.productionChurn + 1,
      status: "renamed",
    },
  ], []);
  assert.equal(renamedToTest.allowed, false);
  assert.deepEqual(renamedToTest.exceeded, ["productionChurn"]);
});

test("allows exact production limits and excludes test lines from production totals", () => {
  const result = evaluatePullRequest([
    source("internal/store/service.go", LIMITS.productionAdditions, 1000),
    source("internal/store/service_test.go", 5000, 5000),
  ], []);

  assert.equal(result.allowed, true);
  assert.equal(result.oversized, false);
  assert.deepEqual(result.metrics, {
    totalFiles: 2,
    scopeFiles: 2,
    productionAdditions: 1500,
    productionChurn: 2500,
  });
});

test("rejects a scope-relevant file count above the limit", () => {
  const files = Array.from(
    { length: LIMITS.scopeFiles + 1 },
    (_, index) => source(`internal/package${index}/file.go`, 0, 0),
  );
  const result = evaluatePullRequest(files, []);

  assert.equal(result.allowed, false);
  assert.deepEqual(result.exceeded, ["scopeFiles"]);
});

test("rejects production additions above the limit", () => {
  const result = evaluatePullRequest([
    source("internal/store/service.go", LIMITS.productionAdditions + 1, 0),
  ], []);

  assert.equal(result.allowed, false);
  assert.deepEqual(result.exceeded, ["productionAdditions"]);
});

test("rejects production churn above the limit without exceeding additions", () => {
  const result = evaluatePullRequest([
    source("internal/store/service.go", 1000, LIMITS.productionChurn - 999),
  ], []);

  assert.equal(result.allowed, false);
  assert.deepEqual(result.exceeded, ["productionChurn"]);
});

test("counts deleted production files and treats absent line counts as zero", () => {
  const result = evaluatePullRequest([
    source("internal/store/retired.go", 0, 40, "removed"),
    { filename: "internal/store/empty.go", status: "added" },
  ], []);

  assert.deepEqual(result.metrics, {
    totalFiles: 2,
    scopeFiles: 2,
    productionAdditions: 0,
    productionChurn: 40,
  });
});

test("does not count documentation, locks, or generated files toward limits", () => {
  const result = evaluatePullRequest([
    source("README.md", 10000, 10000),
    source("go.sum", 10000, 10000),
    source("internal/generated/large.gen.go", 10000, 10000),
  ], []);

  assert.equal(result.allowed, true);
  assert.deepEqual(result.metrics, {
    totalFiles: 3,
    scopeFiles: 0,
    productionAdditions: 0,
    productionChurn: 0,
  });
});

test("requires the exact architecture-approved label for an oversized change", () => {
  const files = [source("internal/store/service.go", LIMITS.productionAdditions + 1)];

  assert.equal(evaluatePullRequest(files, ["architecture-review"]).allowed, false);
  const approved = evaluatePullRequest(files, ["architecture-approved"], {
    overrideAuthorized: true,
  });
  assert.equal(approved.allowed, true);
  assert.equal(approved.overridden, true);
  assert.equal(approved.oversized, true);
});

test("evaluates more than one GitHub API page of files", () => {
  const files = Array.from(
    { length: 101 },
    (_, index) => source(`internal/package${index}/file.go`, 0, 0),
  );
  const result = evaluatePullRequest(files, ["architecture-approved"]);

  assert.equal(result.metrics.totalFiles, 101);
  assert.equal(result.metrics.scopeFiles, 101);
  assert.match(formatEvaluation(result), /Scope-relevant files: 101 \/ 30/);
  assert.match(formatEvaluation(result), /Override: architecture-approved/);
});

test("rejects malformed collection inputs", () => {
  assert.throws(() => evaluatePullRequest(null, []), /files must be an array/);
  assert.throws(() => evaluatePullRequest([], null), /labels must be an array/);
  assert.throws(() => classifyFile(""), /filename must be a non-empty string/);
});

test("fails closed when the GitHub file list is incomplete", () => {
  assert.doesNotThrow(() => assertCompleteFileList([source("internal/service.go")], 1));
  assert.throws(
    () => assertCompleteFileList([source("docs/rule.md")], 2),
    /file list is incomplete.*expected 2, received 1/,
  );
  assert.throws(
    () => assertCompleteFileList([], undefined),
    /changed_files must be a non-negative integer/,
  );
});

test("rejects a pull request snapshot that changes during evaluation", () => {
  const snapshot = {
    head: { sha: "abc" },
    base: { sha: "base-abc", ref: "main" },
    merge_commit_sha: "merge-abc",
    changed_files: 1,
    updated_at: "2026-09-01T12:00:00Z",
  };
  assert.doesNotThrow(() => assertStablePullRequestSnapshot(snapshot, snapshot));
  assert.throws(
    () => assertStablePullRequestSnapshot(snapshot, {
      head: { sha: "def" },
      base: { sha: "base-abc", ref: "main" },
      merge_commit_sha: "merge-abc",
      changed_files: 1,
      updated_at: snapshot.updated_at,
    }),
    /changed during evaluation/,
  );
  assert.throws(
    () => assertStablePullRequestSnapshot(snapshot, {
      head: { sha: "abc" },
      base: { sha: "base-def", ref: "main" },
      merge_commit_sha: "merge-abc",
      changed_files: 1,
      updated_at: snapshot.updated_at,
    }),
    /changed during evaluation/,
  );
  assert.throws(
    () => assertStablePullRequestSnapshot(snapshot, {
      head: { sha: "abc" },
      base: { sha: "base-abc", ref: "release" },
      merge_commit_sha: "merge-abc",
      changed_files: 1,
      updated_at: snapshot.updated_at,
    }),
    /changed during evaluation/,
  );
  assert.throws(
    () => assertStablePullRequestSnapshot(snapshot, {
      head: { sha: "abc" },
      base: { sha: "base-abc", ref: "main" },
      merge_commit_sha: "merge-abc",
      changed_files: 2,
      updated_at: snapshot.updated_at,
    }),
    /changed during evaluation/,
  );
  assert.throws(
    () => assertStablePullRequestSnapshot(snapshot, {
      head: { sha: "abc" },
      base: { sha: "base-abc", ref: "main" },
      merge_commit_sha: "merge-abc",
      changed_files: 1,
      updated_at: "2026-09-01T12:01:00Z",
    }),
    /changed during evaluation/,
  );
  assert.throws(
    () => assertStablePullRequestSnapshot(snapshot, {
      head: { sha: "abc" },
      base: { sha: "base-abc", ref: "main" },
      merge_commit_sha: "merge-def",
      changed_files: 1,
      updated_at: snapshot.updated_at,
    }),
    /changed during evaluation/,
  );
});

test("requires an authenticated current-head approval for an override", () => {
  const headSha = "head-abc";
  const reviews = [
    {
      id: 1,
      user: { login: "maintainer" },
      state: "APPROVED",
      commit_id: headSha,
      submitted_at: "2026-09-01T12:00:00Z",
    },
    {
      id: 2,
      user: { login: "writer" },
      state: "APPROVED",
      commit_id: headSha,
      submitted_at: "2026-09-01T12:00:00Z",
    },
    {
      id: 3,
      user: { login: "stale-maintainer" },
      state: "APPROVED",
      commit_id: "old-head",
      submitted_at: "2026-09-01T12:00:00Z",
    },
  ];

  assert.equal(
    hasAuthorizedArchitectureOverride({
      labels: ["architecture-approved"],
      headSha,
      reviews,
      permissions: {
        maintainer: { permission: "write", role_name: "maintain" },
        writer: { permission: "write", role_name: "write" },
        "stale-maintainer": { permission: "admin", role_name: "admin" },
      },
    }),
    true,
  );
  assert.equal(
    hasAuthorizedArchitectureOverride({
      labels: ["architecture-approved"],
      headSha,
      reviews: [...reviews].reverse(),
      permissions: {
        maintainer: { permission: "write", role_name: "maintain" },
        writer: { permission: "write", role_name: "write" },
        "stale-maintainer": { permission: "admin", role_name: "admin" },
      },
    }),
    true,
  );
  assert.equal(
    hasAuthorizedArchitectureOverride({
      labels: ["architecture-approved"],
      headSha,
      reviews: [reviews[1]],
      permissions: { writer: { permission: "write", role_name: "write" } },
    }),
    false,
  );
  assert.equal(
    hasAuthorizedArchitectureOverride({
      labels: [],
      headSha,
      reviews: [reviews[0]],
      permissions: { maintainer: { permission: "write", role_name: "maintain" } },
    }),
    false,
  );
  assert.equal(
    hasAuthorizedArchitectureOverride({
      labels: ["architecture-approved"],
      headSha,
      reviews: [
        reviews[0],
        {
          id: 4,
          user: { login: "maintainer" },
          state: "CHANGES_REQUESTED",
          commit_id: headSha,
          submitted_at: "2026-09-01T12:01:00Z",
        },
      ],
      permissions: { maintainer: { permission: "admin", role_name: "admin" } },
    }),
    false,
  );
  assert.equal(
    hasAuthorizedArchitectureOverride({
      labels: ["architecture-approved"],
      headSha,
      authorLogin: "maintainer",
      reviews: [reviews[0]],
      permissions: { maintainer: { permission: "write", role_name: "maintain" } },
    }),
    false,
  );
  assert.equal(
    hasAuthorizedArchitectureOverride({
      labels: ["architecture-approved"],
      headSha,
      baseChangedAt: "2026-09-01T12:01:00Z",
      reviews: [reviews[0]],
      permissions: { maintainer: { permission: "write", role_name: "maintain" } },
    }),
    false,
  );
});

test("keeps an approval effective after a later comment-only review", () => {
  const headSha = "head-abc";
  assert.equal(
    hasAuthorizedArchitectureOverride({
      labels: ["architecture-approved"],
      headSha,
      reviews: [
        {
          id: 1,
          user: { login: "maintainer" },
          state: "APPROVED",
          commit_id: headSha,
          submitted_at: "2026-09-01T12:00:00Z",
        },
        {
          id: 2,
          user: { login: "maintainer" },
          state: "COMMENTED",
          commit_id: headSha,
          submitted_at: "2026-09-01T12:01:00Z",
        },
      ],
      permissions: {
        maintainer: { permission: "write", role_name: "maintain" },
      },
    }),
    true,
  );
});

test("rejects labels, reviews, or base-change events moving during evaluation", () => {
  const snapshot = {
    head: { sha: "abc" },
    base: { sha: "base-abc", ref: "main" },
    merge_commit_sha: "merge-abc",
    changed_files: 1,
    updated_at: "2026-09-01T12:00:00Z",
    labels: [{ name: "architecture-approved" }],
  };
  const review = {
    id: 1,
    user: { login: "maintainer" },
    state: "APPROVED",
    commit_id: "abc",
    submitted_at: "2026-09-01T12:00:00Z",
  };
  const events = [{ event: "base_ref_changed", created_at: "2026-09-01T11:00:00Z" }];
  assert.doesNotThrow(() => assertStableAdmissionSnapshot(
    snapshot,
    snapshot,
    [review],
    [review],
    events,
    events,
  ));
  assert.throws(() => assertStableAdmissionSnapshot(
    snapshot,
    { ...snapshot, labels: [] },
    [review],
    [review],
    events,
    events,
  ), /admission inputs changed/);
  assert.throws(() => assertStableAdmissionSnapshot(
    snapshot,
    snapshot,
    [review],
    [{ ...review, state: "DISMISSED" }],
    events,
    events,
  ), /admission inputs changed/);
  assert.throws(() => assertStableAdmissionSnapshot(
    snapshot,
    snapshot,
    [review],
    [review],
    events,
    [{ event: "base_ref_changed", created_at: "2026-09-01T12:01:00Z" }],
  ), /admission inputs changed/);
});

test("does not treat the override label alone as authorization", () => {
  const files = [source("internal/store/service.go", LIMITS.productionAdditions + 1)];
  assert.equal(evaluatePullRequest(files, ["architecture-approved"]).allowed, false);
  const approved = evaluatePullRequest(files, ["architecture-approved"], {
    overrideAuthorized: true,
  });
  assert.equal(approved.allowed, true);
  assert.equal(approved.overridden, true);
});

test("maps an admission decision to a commit status", () => {
  assert.deepEqual(statusForEvaluation({ allowed: true }), {
    state: "success",
    description: "Within admission limits",
  });
  assert.deepEqual(statusForEvaluation({ allowed: false }), {
    state: "failure",
    description: "Exceeds admission limits",
  });
});

test("identifies an authorized architecture override in the commit status", () => {
  assert.deepEqual(statusForEvaluation({ allowed: true, overridden: true }), {
    state: "success",
    description: "Allowed by authorized architecture override",
  });
});

test("does not label ordinary or failed decisions as architecture overrides", () => {
  assert.deepEqual(statusForEvaluation({ allowed: true, overridden: false }), {
    state: "success",
    description: "Within admission limits",
  });
  assert.deepEqual(statusForEvaluation({ allowed: true, overridden: "true" }), {
    state: "success",
    description: "Within admission limits",
  });
  assert.deepEqual(statusForEvaluation({ allowed: false, overridden: true }), {
    state: "failure",
    description: "Exceeds admission limits",
  });
});

test("runs admission guard tests when policy files change", () => {
  const workflowPath = path.join(__dirname, "..", "workflows", "ci.yml");
  const workflow = fs.readFileSync(workflowPath, "utf8").replaceAll("\r\n", "\n");
  const pushSection = workflow.slice(
    workflow.indexOf("\n  push:\n"),
    workflow.indexOf("\n  pull_request:\n"),
  );
  const pullRequestSection = workflow.slice(
    workflow.indexOf("\n  pull_request:\n"),
    workflow.indexOf("\n\npermissions:"),
  );
  const admissionJob = workflow.slice(
    workflow.indexOf("\n  development-admission-tests:\n"),
    workflow.indexOf("\n  release-authority:\n"),
  );
  const notifyJob = workflow.slice(workflow.indexOf("\n  notify:\n"));

  assert.match(pushSection, /^\s+- "\.github\/scripts\/\*\*"\s*$/m);
  assert.match(pullRequestSection, /^\s+- "\.github\/scripts\/\*\*"\s*$/m);
  assert.match(admissionJob, /name: Development Admission Policy Tests/);
  assert.match(
    admissionJob,
    /run: node --test \.github\/scripts\/pr-scope-guard\.test\.cjs/,
  );
  assert.match(
    notifyJob,
    /needs:\s*[\s\S]*-\s+development-admission-tests\b/,
  );
  assert.match(
    notifyJob,
    /DEVELOPMENT_ADMISSION_TEST_RESULT:\s*\$\{\{\s*needs\.development-admission-tests\.result\s*\}\}/,
  );
  assert.match(
    notifyJob,
    /results = \[development_admission_tests, backend, frontend, code_health\]/,
  );
  assert.match(
    notifyJob,
    /> Development Admission tests：`\{development_admission_tests\}`/,
  );
});

test("keeps review and reconciliation triggers on the trusted event path", () => {
  const workflowRoot = path.join(__dirname, "..", "workflows");
  const admissionWorkflow = fs.readFileSync(
    path.join(workflowRoot, "development-admission.yml"),
    "utf8",
  ).replaceAll("\r\n", "\n");
  const signalWorkflow = fs.readFileSync(
    path.join(workflowRoot, "development-admission-review-signal.yml"),
    "utf8",
  ).replaceAll("\r\n", "\n");
  const reconcileWorkflow = fs.readFileSync(
    path.join(workflowRoot, "development-admission-reconcile.yml"),
    "utf8",
  ).replaceAll("\r\n", "\n");

  assert.match(admissionWorkflow, /workflow_dispatch:/);
  assert.match(admissionWorkflow, /needs: resolve-admission/);
  assert.match(
    admissionWorkflow,
    /group: development-admission-\$\{\{ github\.event\.repository\.full_name \}\}-\$\{\{ needs\.resolve-admission\.outputs\.pull_request_number \}\}/,
  );
  assert.match(signalWorkflow, /permissions: \{\}/);
  assert.match(signalWorkflow, /development-admission-review-\$\{\{ github\.run_id \}\}/);
  assert.match(reconcileWorkflow, /schedule:/);
  assert.match(reconcileWorkflow, /github\.rest\.actions\.createWorkflowDispatch/);
});

test("targets the test merge commit for required admission status", () => {
  assert.equal(statusTargetForPullRequest({ merge_commit_sha: "merge-abc" }), "merge-abc");
  assert.throws(
    () => statusTargetForPullRequest({ merge_commit_sha: null }),
    /missing merge_commit_sha/,
  );
});

test("exports and computes the latest base branch change timestamp", () => {
  assert.equal(
    latestBaseChangeAt([
      { event: "base_ref_changed", created_at: "2026-09-01T12:00:00Z" },
      { event: "labeled", created_at: "2026-09-01T12:02:00Z" },
      { event: "base_ref_changed", created_at: "2026-09-01T12:01:00Z" },
    ]),
    "2026-09-01T12:01:00Z",
  );
});

test("extracts a pull request number from direct, workflow-run, and dispatch events", () => {
  assert.equal(
    pullRequestNumberFromEventPayload("pull_request_target", {
      pull_request: { number: 273 },
    }),
    273,
  );
  assert.equal(
    pullRequestNumberFromEventPayload("workflow_run", {
      workflow_run: {
        pull_requests: [{ number: 272 }, { number: 273 }],
      },
    }, 273),
    273,
  );
  assert.throws(
    () => pullRequestNumberFromEventPayload("workflow_run", {
      workflow_run: { pull_requests: [{ number: 272 }, { number: 273 }] },
    }),
    /trusted signal must identify one associated pull request/,
  );
  assert.equal(
    pullRequestNumberFromEventPayload("workflow_dispatch", {
      inputs: { pull_request_number: "273" },
    }),
    273,
  );
});

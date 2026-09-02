const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const {
  LIMITS,
  authorizedArchitectureApprovers,
  assertCompleteFileList,
  assertStableAdmissionSnapshot,
  assertStablePullRequestSnapshot,
  classifyFileChange,
  classifyFile,
  evaluatePullRequest,
  EVALUATION_ERROR_SUMMARY,
  formatEvaluation,
  hasAuthorizedArchitectureOverride,
  hasRecentLabelRemoval,
  hasRequiredOverrideEvidence,
  needsAdmissionReconciliation,
  latestBaseChangeAt,
  pullRequestNumberFromEventPayload,
  statusForEvaluation,
  statusTargetForPullRequest,
} = require("./pr-scope-guard.cjs");

function source(filename, additions = 1, deletions = 0, status = "modified") {
  return { filename, status, additions, deletions };
}

test("classifies documentation, lock, source, test, and production paths", () => {
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
    scopeRelevant: true,
    production: true,
    kind: "production",
  });
  assert.deepEqual(classifyFile("internal/api/model.pb.go"), {
    scopeRelevant: true,
    production: true,
    kind: "production",
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

test("counts generated-looking source files toward limits by default", () => {
  const result = evaluatePullRequest([
    source("README.md", 10000, 10000),
    source("go.sum", 10000, 10000),
    source("internal/generated/large.gen.go", 10000, 10000),
  ], []);

  assert.equal(result.allowed, false);
  assert.deepEqual(result.metrics, {
    totalFiles: 3,
    scopeFiles: 1,
    productionAdditions: 10000,
    productionChurn: 20000,
  });
  assert.deepEqual(result.kinds, {
    documentation: 1,
    lockfile: 1,
    generated: 0,
    test: 0,
    production: 1,
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

test("requires explicit design and split evidence for an oversized override", () => {
  const completeBody = [
    "- Design: docs/superpowers/specs/2026-09-01-design.md",
    "- Independent design review: reviewer approved the failure matrix",
    "- Override approver: Henry",
    "- Split rationale (only when oversized): the consistency boundary cannot be split safely",
  ].join("\n");
  assert.equal(hasRequiredOverrideEvidence(completeBody, ["Henry"]), true);
  assert.equal(hasRequiredOverrideEvidence(completeBody, ["Other reviewer"]), false);
  assert.equal(
    hasRequiredOverrideEvidence(
      completeBody.replace("docs/superpowers/specs/2026-09-01-design.md", "N/A"),
      ["Henry"],
    ),
    false,
  );
  assert.equal(
    hasRequiredOverrideEvidence(
      completeBody.replace("the consistency boundary cannot be split safely", "N/A"),
      ["Henry"],
    ),
    false,
  );
  assert.equal(
    hasRequiredOverrideEvidence(
      completeBody.replace("the consistency boundary cannot be split safely", "TODO: explain"),
      ["Henry"],
    ),
    false,
  );
  assert.equal(
    hasRequiredOverrideEvidence([
      "- Design: docs/superpowers/specs/2026-09-01-design.md",
      "- Independent design review: reviewer approved the failure matrix",
      "- `architecture-approved` label, maintainer/admin approval review for the current head SHA, and split rationale (only when oversized): Henry approved; cannot split safely",
    ].join("\n"), ["Henry"]),
    true,
  );
  assert.equal(hasRequiredOverrideEvidence(null), false);
  assert.equal(
    hasRequiredOverrideEvidence(
      completeBody.replace(
        "the consistency boundary cannot be split safely",
        "Cannot split because none of the intermediate states is safe",
      ),
      ["Henry"],
    ),
    true,
  );
  assert.equal(
    hasRequiredOverrideEvidence(
      completeBody.replace(
        "the consistency boundary cannot be split safely",
        "None of the intermediate states is safe",
      ),
      ["Henry"],
    ),
    true,
  );
  assert.equal(
    hasRequiredOverrideEvidence(
      completeBody.replace(
        "reviewer approved the failure matrix",
        "not performed",
      ),
      ["Henry"],
    ),
    false,
  );
  assert.equal(
    hasRequiredOverrideEvidence(
      completeBody.replace(
        "the consistency boundary cannot be split safely",
        "this can be split safely",
      ),
      ["Henry"],
    ),
    false,
  );
  assert.equal(
    hasRequiredOverrideEvidence(
      completeBody.replace(
        "reviewer approved the failure matrix",
        "No blockers were identified in the independent review",
      ),
      ["Henry"],
    ),
    true,
  );
  assert.equal(
    hasRequiredOverrideEvidence(
      completeBody.replace(
        "reviewer approved the failure matrix",
        "pending",
      ),
      ["Henry"],
    ),
    false,
  );
  assert.equal(
    hasRequiredOverrideEvidence(
      completeBody.replace(
        "reviewer approved the failure matrix",
        "will be conducted before merge",
      ),
      ["Henry"],
    ),
    false,
  );
  assert.equal(
    hasRequiredOverrideEvidence(
      completeBody.replace(
        "the consistency boundary cannot be split safely",
        "This can only be split by breaking the shared transaction",
      ),
      ["Henry"],
    ),
    true,
  );
});

test("binds override evidence to an authorized approver", () => {
  const headSha = "head-abc";
  const reviews = [
    {
      id: 1,
      user: { login: "writer" },
      state: "APPROVED",
      commit_id: headSha,
      submitted_at: "2026-09-01T12:00:00Z",
    },
    {
      id: 2,
      user: { login: "maintainer" },
      state: "APPROVED",
      commit_id: headSha,
      submitted_at: "2026-09-01T12:00:00Z",
    },
  ];
  const permissions = {
    writer: { permission: "write", role_name: "write" },
    maintainer: { permission: "write", role_name: "maintain" },
  };
  assert.deepEqual(
    authorizedArchitectureApprovers({
      labels: ["architecture-approved"],
      headSha,
      authorLogin: "author",
      reviews,
      permissions,
    }),
    ["maintainer"],
  );
  const body = [
    "- Design: docs/design.md",
    "- Independent design review: approved",
    "- Override approver: writer",
    "- Split rationale: cannot split safely",
  ].join("\n");
  assert.equal(
    hasRequiredOverrideEvidence(body, ["writer"]),
    true,
  );
  assert.equal(
    hasRequiredOverrideEvidence(body, ["maintainer"]),
    false,
  );
});

test("detects a recent removal of the override label", () => {
  const now = "2026-09-01T12:10:00Z";
  assert.equal(
    hasRecentLabelRemoval([
      {
        event: "unlabeled",
        label: { name: "architecture-approved" },
        created_at: "2026-09-01T12:06:00Z",
      },
    ], "architecture-approved", now, 5 * 60 * 1000),
    true,
  );
  assert.equal(
    hasRecentLabelRemoval([
      {
        event: "unlabeled",
        label: { name: "architecture-approved" },
        created_at: "2026-09-01T12:00:00Z",
      },
    ], "architecture-approved", now, 5 * 60 * 1000),
    false,
  );
});

test("keeps reconciling after label removal until a terminal check exists", () => {
  const base = {
    labels: [],
    baseRef: "main",
    defaultBranch: "main",
    events: [
      {
        event: "unlabeled",
        label: { name: "architecture-approved" },
        created_at: "2026-09-01T12:00:00Z",
      },
    ],
    now: "2026-09-01T13:00:00Z",
  };
  assert.equal(
    needsAdmissionReconciliation({
      ...base,
      checkRuns: [{ status: "completed", conclusion: "success", completed_at: "2026-09-01T11:59:00Z" }],
    }),
    true,
  );
  assert.equal(
    needsAdmissionReconciliation({
      ...base,
      checkRuns: [{
        status: "completed",
        conclusion: "failure",
        created_at: "2026-09-01T12:00:30Z",
        started_at: "2026-09-01T11:59:30Z",
        completed_at: "2026-09-01T12:01:00Z",
      }],
    }),
    true,
  );
  assert.equal(
    needsAdmissionReconciliation({
      ...base,
      checkRuns: [{
        status: "completed",
        conclusion: "failure",
        created_at: "2026-09-01T12:00:30Z",
        started_at: "2026-09-01T12:00:01Z",
        completed_at: "2026-09-01T12:01:00Z",
        output: { summary: "Exceeds admission limits" },
      }],
    }),
    false,
  );
});

test("fails closed for an active check with no recovery timestamp", () => {
  assert.equal(
    needsAdmissionReconciliation({
      labels: [],
      baseRef: "main",
      defaultBranch: "main",
      events: [],
      checkRuns: [{ status: "queued" }],
      now: "2026-09-01T12:20:00Z",
    }),
    true,
  );
});

test("retries terminal evaluator errors instead of treating them as policy results", () => {
  assert.equal(
    needsAdmissionReconciliation({
      labels: [],
      baseRef: "main",
      defaultBranch: "main",
      events: [],
      checkRuns: [{
        status: "completed",
        conclusion: "failure",
        completed_at: "2026-09-01T12:01:00Z",
        output: { summary: EVALUATION_ERROR_SUMMARY },
      }],
      now: "2026-09-01T12:02:00Z",
    }),
    true,
  );
});

test("reconciles a check that remains in progress beyond the recovery window", () => {
  assert.equal(
    needsAdmissionReconciliation({
      labels: [],
      baseRef: "main",
      defaultBranch: "main",
      events: [],
      checkRuns: [{ status: "in_progress", created_at: "2026-09-01T12:00:00Z", started_at: "2026-09-01T12:00:00Z" }],
      now: "2026-09-01T12:11:00Z",
      inProgressTimeoutMs: 10 * 60 * 1000,
    }),
    true,
  );
  assert.equal(
    needsAdmissionReconciliation({
      labels: [],
      baseRef: "main",
      defaultBranch: "main",
      events: [],
      checkRuns: [{ status: "in_progress", created_at: "2026-09-01T12:05:00Z", started_at: "2026-09-01T12:05:00Z" }],
      now: "2026-09-01T12:11:00Z",
      inProgressTimeoutMs: 10 * 60 * 1000,
    }),
    false,
  );
});

test("ignores a superseded stale check when a newer policy result exists", () => {
  assert.equal(
    needsAdmissionReconciliation({
      labels: [],
      baseRef: "main",
      defaultBranch: "main",
      events: [],
      checkRuns: [
        {
          id: 1,
          status: "in_progress",
          created_at: "2026-09-01T11:00:00Z",
          started_at: "2026-09-01T11:00:00Z",
        },
        {
          id: 2,
          status: "completed",
          conclusion: "success",
          created_at: "2026-09-01T12:00:00Z",
          started_at: "2026-09-01T12:00:01Z",
          completed_at: "2026-09-01T12:01:00Z",
          output: { summary: "Within admission limits" },
        },
      ],
      now: "2026-09-01T12:20:00Z",
    }),
    false,
  );
});

test("fails closed for an unknown check status or timestamp", () => {
  assert.equal(
    needsAdmissionReconciliation({
      labels: [],
      baseRef: "main",
      defaultBranch: "main",
      events: [],
      checkRuns: [{ status: "future_status", created_at: "2026-09-01T12:00:00Z" }],
      now: "2026-09-01T12:01:00Z",
    }),
    true,
  );
  assert.equal(
    needsAdmissionReconciliation({
      labels: [],
      baseRef: "main",
      defaultBranch: "main",
      events: [],
      checkRuns: [{ status: "completed", conclusion: "success", completed_at: "2026-09-01T12:00:00Z" }],
      now: "2026-09-01T12:01:00Z",
    }),
    true,
  );
});

test("fails closed when an override-label removal event has an invalid timestamp", () => {
  assert.equal(
    needsAdmissionReconciliation({
      labels: [],
      baseRef: "main",
      defaultBranch: "main",
      events: [{
        event: "unlabeled",
        label: { name: "architecture-approved" },
        created_at: "not-a-timestamp",
      }],
      checkRuns: [{
        status: "completed",
        conclusion: "success",
        created_at: "2026-09-01T12:00:00Z",
        started_at: "2026-09-01T12:00:00Z",
        completed_at: "2026-09-01T12:01:00Z",
        output: { summary: "Within admission limits" },
      }],
      now: "2026-09-01T12:02:00Z",
    }),
    true,
  );
});

test("fails closed when a terminal check has incomplete timestamps", () => {
  const base = {
    labels: [],
    baseRef: "main",
    defaultBranch: "main",
    events: [],
    now: "2026-09-01T12:02:00Z",
  };
  assert.equal(
    needsAdmissionReconciliation({
      ...base,
      checkRuns: [{
        status: "completed",
        conclusion: "success",
        created_at: "2026-09-01T12:00:00Z",
        output: { summary: "Within admission limits" },
      }],
    }),
    true,
  );
  assert.equal(
    needsAdmissionReconciliation({
      ...base,
      checkRuns: [{
        status: "completed",
        conclusion: "success",
        created_at: "2026-09-01T12:00:00Z",
        started_at: "2026-09-01T12:00:00Z",
        completed_at: "not-a-timestamp",
        output: { summary: "Within admission limits" },
      }],
    }),
    true,
  );
  assert.equal(
    needsAdmissionReconciliation({
      ...base,
      checkRuns: [{
        status: "completed",
        conclusion: "success",
        created_at: 0,
        started_at: "2026-09-01T12:00:00Z",
        completed_at: "2026-09-01T12:01:00Z",
        output: { summary: "Within admission limits" },
      }],
    }),
    true,
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
  const baseSignalWorkflow = fs.readFileSync(
    path.join(workflowRoot, "development-admission-base-signal.yml"),
    "utf8",
  ).replaceAll("\r\n", "\n");

  assert.match(admissionWorkflow, /repository_dispatch:/);
  assert.doesNotMatch(admissionWorkflow, /^  workflow_dispatch:/m);
  assert.match(admissionWorkflow, /always\(\)/);
  assert.match(
    admissionWorkflow,
    /group: development-admission-\$\{\{ github\.event\.repository\.full_name \}\}-\$\{\{ needs\.resolve-admission\.outputs\.pull_request_number \|\| github\.event\.pull_request\.number/,
  );
  assert.match(admissionWorkflow, /context\.payload\.pull_request\?\.merge_commit_sha/);
  assert.match(admissionWorkflow, /merge_commit_sha:/);
  assert.match(admissionWorkflow, /checks\.create/);
  assert.match(admissionWorkflow, /checks\.update/);
  assert.match(admissionWorkflow, /admissionCheckRunId/);
  assert.match(admissionWorkflow, /external_id/);
  assert.match(admissionWorkflow, /checks\.listForRef/);
  assert.match(admissionWorkflow, /checks\.get/);
  assert.match(admissionWorkflow, /findExistingCheckRun\(\{ includeCompleted: true \}\)/);
  assert.match(admissionWorkflow, /nonterminalCheckStatuses/);
  assert.match(admissionWorkflow, /nonterminalCheckStatuses\.has\(currentCheckRun\.status\)/);
  assert.match(admissionWorkflow, /started_at/);
  assert.doesNotMatch(admissionWorkflow, /createCommitStatus/);
  assert.match(
    admissionWorkflow,
    /const initialPullRequest[\s\S]*statusSha[\s\S]*await publishStatus\(/,
  );
  assert.match(
    admissionWorkflow,
    /const directOverrideRemoval[\s\S]*await publishStatus\([\s\S]*Invalidating admission after architecture-approved removal[\s\S]*const initialPullRequest/,
  );
  assert.match(
    admissionWorkflow,
    /hasRequiredOverrideEvidence\([\s\S]*authorizedApprovalReviewers/,
  );
  assert.match(signalWorkflow, /permissions: \{\}/);
  assert.match(signalWorkflow, /MERGE_COMMIT_SHA/);
  assert.match(signalWorkflow, /merge_commit_sha/);
  assert.match(signalWorkflow, /development-admission-review-\$\{\{ github\.run_id \}\}/);
  assert.match(reconcileWorkflow, /schedule:/);
  assert.match(
    reconcileWorkflow,
    /workflow_run:[\s\S]*Development Admission Base Signal/,
  );
  assert.doesNotMatch(reconcileWorkflow, /^  push:\s*$/m);
  assert.match(baseSignalWorkflow, /^  push:\s*$/m);
  assert.match(baseSignalWorkflow, /permissions: \{\}/);
  assert.match(reconcileWorkflow, /pullRequest\.base\.ref === baseBranch/);
  assert.match(reconcileWorkflow, /architecture-approved/);
  assert.match(reconcileWorkflow, /needsAdmissionReconciliation/);
  assert.match(reconcileWorkflow, /issues: read/);
  assert.match(reconcileWorkflow, /checks: read/);
  assert.match(reconcileWorkflow, /trustedCheckAppId/);
  assert.match(reconcileWorkflow, /external_id === admissionCheckExternalId/);
  assert.match(reconcileWorkflow, /try \{[\s\S]*createDispatchEvent/);
  assert.doesNotMatch(reconcileWorkflow, /createWorkflowDispatch/);
  assert.match(reconcileWorkflow, /dispatchFailures/);
  assert.match(admissionWorkflow, /finalPermissions/);
  assert.match(
    reconcileWorkflow,
    /pullRequest\.base\.ref !== defaultBranch/,
  );
  assert.match(reconcileWorkflow, /github\.rest\.repos\.createDispatchEvent/);
});

test("uses the dedicated App installation identity for privileged admission API calls", () => {
  const workflowRoot = path.join(__dirname, "..", "workflows");
  const admissionWorkflow = fs.readFileSync(
    path.join(workflowRoot, "development-admission.yml"),
    "utf8",
  ).replaceAll("\r\n", "\n");
  const reconcileWorkflow = fs.readFileSync(
    path.join(workflowRoot, "development-admission-reconcile.yml"),
    "utf8",
  ).replaceAll("\r\n", "\n");
  const ordinaryCiWorkflow = fs.readFileSync(
    path.join(workflowRoot, "ci.yml"),
    "utf8",
  ).replaceAll("\r\n", "\n");
  const appTokenAction = /uses: actions\/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3/;
  const privateKeySecret = /private-key: \$\{\{ secrets\.DEVELOPMENT_ADMISSION_APP_PRIVATE_KEY \}\}/;
  const appTokenReference = /github-token: \$\{\{ steps\.admission-app-token\.outputs\.token \}\}/g;

  assert.match(admissionWorkflow, appTokenAction);
  assert.match(admissionWorkflow, /app-id: 4799675 # AI Commerce Governance/);
  assert.match(admissionWorkflow, privateKeySecret);
  assert.equal(
    (admissionWorkflow.match(/^    environment: development-admission-publisher$/gm) || []).length,
    2,
  );
  assert.match(admissionWorkflow, /outputs\.installation-id/);
  assert.match(admissionWorkflow, /!= ['"]158369358['"]/);
  assert.match(admissionWorkflow, /^permissions:\s*\{\}\s*$/m);
  assert.doesNotMatch(admissionWorkflow, /^  workflow_dispatch:/m);
  assert.doesNotMatch(admissionWorkflow, /secrets\.GITHUB_TOKEN|github\.token/);
  assert.match(admissionWorkflow, /actions\/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4\.2\.2/);
  assert.match(admissionWorkflow, /actions\/github-script@373c709c69115d41ff229c7e5df9f8788daa9553 # v9/);
  assert.equal(
    (admissionWorkflow.match(appTokenReference) || []).length,
    (admissionWorkflow.match(/uses: actions\/github-script@373c709c69115d41ff229c7e5df9f8788daa9553/g) || []).length +
      (admissionWorkflow.match(/uses: actions\/download-artifact@/g) || []).length,
  );
  assert.match(admissionWorkflow, /TRUSTED_CHECK_APP_ID/);
  assert.doesNotMatch(admissionWorkflow, /trustedCheckAppId = 15368/);

  assert.match(reconcileWorkflow, appTokenAction);
  assert.match(reconcileWorkflow, /app-id: 4799675 # AI Commerce Governance/);
  assert.match(reconcileWorkflow, privateKeySecret);
  assert.equal(
    (reconcileWorkflow.match(/^    environment: development-admission-publisher$/gm) || []).length,
    1,
  );
  assert.match(reconcileWorkflow, /outputs\.installation-id/);
  assert.match(reconcileWorkflow, /!= ['"]158369358['"]/);
  assert.match(reconcileWorkflow, /^permissions:\s*\{\}\s*$/m);
  assert.doesNotMatch(reconcileWorkflow, /secrets\.GITHUB_TOKEN|github\.token/);
  assert.match(reconcileWorkflow, /actions\/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4\.2\.2/);
  assert.match(reconcileWorkflow, /actions\/github-script@373c709c69115d41ff229c7e5df9f8788daa9553 # v9/);
  assert.equal((reconcileWorkflow.match(appTokenReference) || []).length, 1);
  assert.match(reconcileWorkflow, /TRUSTED_CHECK_APP_ID/);
  assert.doesNotMatch(reconcileWorkflow, /trustedCheckAppId = 15368/);
  assert.doesNotMatch(ordinaryCiWorkflow, /DEVELOPMENT_ADMISSION_APP_PRIVATE_KEY/);
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

test("extracts a pull request number from direct, workflow-run, and repository dispatch events", () => {
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
    pullRequestNumberFromEventPayload("repository_dispatch", {
      client_payload: { pull_request_number: 273 },
    }),
    273,
  );
});

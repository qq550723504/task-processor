const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const {
  LIMITS,
  assertCompleteFileList,
  assertStablePullRequestSnapshot,
  classifyFile,
  classifyFileChange,
  evaluatePullRequest,
  formatEvaluation,
  statusForEvaluation,
  statusTargetForPullRequest,
} = require("./pr-scope-guard.cjs");

const workflowPath = path.join(__dirname, "..", "workflows", "development-admission.yml");

function source(filename, additions = 1, deletions = 0, status = "modified") {
  return { filename, status, additions, deletions };
}

test("classifies repository paths deterministically", () => {
  assert.deepEqual(classifyFile("docs/architecture/rule.md"), {
    scopeRelevant: false,
    production: false,
    kind: "documentation",
  });
  assert.deepEqual(classifyFile("go.work.sum"), {
    scopeRelevant: false,
    production: false,
    kind: "lockfile",
  });
  assert.deepEqual(classifyFile("internal/store/service_test.go"), {
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

test("counts generated-looking source files as production", () => {
  const result = evaluatePullRequest([
    source("README.md", 10000, 10000),
    source("go.sum", 10000, 10000),
    source("internal/generated/large.gen.go", 10000, 10000),
  ]);

  assert.equal(result.allowed, false);
  assert.deepEqual(result.metrics, {
    totalFiles: 3,
    scopeFiles: 1,
    productionAdditions: 10000,
    productionChurn: 20000,
  });
});

test("classifies a rename as production when either path is production", () => {
  assert.deepEqual(classifyFileChange({
    filename: "docs/service.md",
    previous_filename: "internal/service.go",
  }), {
    scopeRelevant: true,
    production: true,
    kind: "production",
  });
});

test("fails closed for a classification-changing rename", () => {
  for (const [additions, deletions] of [[0, 0], [1, 0]]) {
    const result = evaluatePullRequest([{
      filename: "internal/service.go",
      previous_filename: "tests/service_test.go",
      additions,
      deletions,
      status: "renamed",
    }]);
    assert.equal(result.allowed, false);
    assert.deepEqual(result.exceeded, ["productionAdditions", "productionChurn"]);
  }
});

test("allows exact thresholds and rejects each exceeded threshold", () => {
  assert.equal(evaluatePullRequest([
    source("internal/service.go", LIMITS.productionAdditions, 1000),
    source("internal/service_test.go", 5000, 5000),
  ]).allowed, true);

  assert.deepEqual(evaluatePullRequest([
    source("internal/service.go", LIMITS.productionAdditions + 1),
  ]).exceeded, ["productionAdditions"]);
  assert.deepEqual(evaluatePullRequest([
    source("internal/service.go", 1000, LIMITS.productionChurn - 999),
  ]).exceeded, ["productionChurn"]);
  assert.deepEqual(evaluatePullRequest(
    Array.from({ length: LIMITS.scopeFiles + 1 }, (_, index) => source(`internal/pkg${index}/file.go`, 0, 0)),
  ).exceeded, ["scopeFiles"]);
});

test("validates file-list completeness and stable pull-request snapshots", () => {
  assert.throws(() => assertCompleteFileList([], 1), /file list is incomplete/);
  assert.deepEqual(assertCompleteFileList([source("internal/a.go")], 1), [source("internal/a.go")]);

  const snapshot = {
    head: { sha: "head" },
    base: { sha: "base", ref: "main" },
    merge_commit_sha: "merge",
    changed_files: 1,
    updated_at: "2026-09-02T00:00:00Z",
  };
  assert.deepEqual(assertStablePullRequestSnapshot(snapshot, structuredClone(snapshot)), snapshot);
  assert.throws(() => assertStablePullRequestSnapshot(
    snapshot,
    { ...structuredClone(snapshot), head: { sha: "changed" } },
  ), /changed during evaluation/);
});

test("publishes only the test merge commit and reports threshold decisions", () => {
  assert.equal(statusTargetForPullRequest({ merge_commit_sha: "merge-sha" }), "merge-sha");
  assert.deepEqual(statusForEvaluation({ allowed: true }), {
    state: "success",
    description: "Within admission limits",
  });
  assert.deepEqual(statusForEvaluation({ allowed: false }), {
    state: "failure",
    description: "Exceeds admission limits",
  });
  assert.match(formatEvaluation(evaluatePullRequest([source("internal/a.go")])), /Decision: allowed/);
});

test("keeps the trusted workflow small and free of override paths", () => {
  const workflow = fs.readFileSync(workflowPath, "utf8");
  assert.match(workflow, /pull_request_target:/);
  assert.match(workflow, /environment: development-admission-publisher/);
  assert.match(workflow, /ref: \$\{\{ github\.event\.repository\.default_branch \}\}/);
  assert.doesNotMatch(workflow, /architecture-approved|workflow_run|repository_dispatch|pull_request_review|readPermissions|listReviews|listEventsForIssue/);
  assert.doesNotMatch(workflow, /GITHUB_TOKEN|github\.token/);
});

test("removes obsolete review and reconciliation workflows", () => {
  for (const name of [
    "development-admission-reconcile.yml",
    "development-admission-review-signal.yml",
    "development-admission-base-signal.yml",
  ]) {
    assert.equal(fs.existsSync(path.join(__dirname, "..", "workflows", name)), false, name);
  }
});

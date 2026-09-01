const test = require("node:test");
const assert = require("node:assert/strict");

const {
  LIMITS,
  assertCompleteFileList,
  assertStablePullRequestSnapshot,
  classifyFileChange,
  classifyFile,
  evaluatePullRequest,
  formatEvaluation,
  statusForEvaluation,
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
  const approved = evaluatePullRequest(files, ["architecture-approved"]);
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
    changed_files: 1,
    updated_at: "2026-09-01T12:00:00Z",
  };
  assert.doesNotThrow(() => assertStablePullRequestSnapshot(snapshot, snapshot));
  assert.throws(
    () => assertStablePullRequestSnapshot(snapshot, {
      head: { sha: "def" },
      base: { sha: "base-abc", ref: "main" },
      changed_files: 1,
      updated_at: snapshot.updated_at,
    }),
    /changed during evaluation/,
  );
  assert.throws(
    () => assertStablePullRequestSnapshot(snapshot, {
      head: { sha: "abc" },
      base: { sha: "base-def", ref: "main" },
      changed_files: 1,
      updated_at: snapshot.updated_at,
    }),
    /changed during evaluation/,
  );
  assert.throws(
    () => assertStablePullRequestSnapshot(snapshot, {
      head: { sha: "abc" },
      base: { sha: "base-abc", ref: "release" },
      changed_files: 1,
      updated_at: snapshot.updated_at,
    }),
    /changed during evaluation/,
  );
  assert.throws(
    () => assertStablePullRequestSnapshot(snapshot, {
      head: { sha: "abc" },
      base: { sha: "base-abc", ref: "main" },
      changed_files: 2,
      updated_at: snapshot.updated_at,
    }),
    /changed during evaluation/,
  );
  assert.throws(
    () => assertStablePullRequestSnapshot(snapshot, {
      head: { sha: "abc" },
      base: { sha: "base-abc", ref: "main" },
      changed_files: 1,
      updated_at: "2026-09-01T12:01:00Z",
    }),
    /changed during evaluation/,
  );
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

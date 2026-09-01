"use strict";

const LIMITS = Object.freeze({
  scopeFiles: 30,
  productionAdditions: 1500,
  productionChurn: 2500,
});

const OVERRIDE_LABEL = "architecture-approved";
const LOCKFILES = new Set([
  "cargo.lock",
  "go.sum",
  "go.work.sum",
  "package-lock.json",
  "pnpm-lock.yaml",
  "yarn.lock",
]);
const TEST_PATH_SEGMENTS = new Set([
  "__tests__",
  "fixtures",
  "test",
  "testdata",
  "tests",
]);

function normalizeFilename(filename) {
  if (typeof filename !== "string" || filename.trim() === "") {
    throw new TypeError("filename must be a non-empty string");
  }
  return filename.replaceAll("\\", "/").toLowerCase();
}

function classifyFile(filename) {
  const normalized = normalizeFilename(filename);
  const segments = normalized.split("/");
  const basename = segments.at(-1);

  if (normalized.startsWith("docs/") || /\.mdx?$/.test(basename)) {
    return classification("documentation", false, false);
  }
  if (LOCKFILES.has(basename)) {
    return classification("lockfile", false, false);
  }
  if (
    segments.includes("generated") ||
    segments.includes("__snapshots__") ||
    basename.endsWith(".pb.go") ||
    basename.endsWith(".gen.go")
  ) {
    return classification("generated", false, false);
  }
  if (
    segments.some((segment) => TEST_PATH_SEGMENTS.has(segment)) ||
    basename.endsWith("_test.go") ||
    /\.(?:test|spec)\.[^/]+$/.test(basename) ||
    basename.endsWith(".tests.ps1") ||
    /\.type-test\.[^/]+$/.test(basename)
  ) {
    return classification("test", true, false);
  }
  return classification("production", true, true);
}

function classifyFileChange(file) {
  const current = classifyFile(file?.filename);
  const previousFilename = file?.previous_filename;
  if (typeof previousFilename !== "string" || previousFilename.trim() === "") {
    return current;
  }

  const previous = classifyFile(previousFilename);
  const production = current.production || previous.production;
  return classification(
    production ? "production" : current.kind,
    current.scopeRelevant || previous.scopeRelevant,
    production,
  );
}

function classification(kind, scopeRelevant, production) {
  return { scopeRelevant, production, kind };
}

function lineCount(value) {
  if (!Number.isFinite(value) || value <= 0) {
    return 0;
  }
  return Math.trunc(value);
}

function assertCompleteFileList(files, changedFiles) {
  if (!Array.isArray(files)) {
    throw new TypeError("files must be an array");
  }
  if (!Number.isInteger(changedFiles) || changedFiles < 0) {
    throw new TypeError("changed_files must be a non-negative integer");
  }
  if (files.length !== changedFiles) {
    throw new RangeError(
      `file list is incomplete: expected ${changedFiles}, received ${files.length}`,
    );
  }
  return files;
}

function assertStablePullRequestSnapshot(before, after) {
  const beforeSha = before?.head?.sha;
  const afterSha = after?.head?.sha;
  const beforeBaseSha = before?.base?.sha;
  const afterBaseSha = after?.base?.sha;
  const beforeBaseRef = before?.base?.ref;
  const afterBaseRef = after?.base?.ref;
  const beforeFiles = before?.changed_files;
  const afterFiles = after?.changed_files;
  const beforeUpdatedAt = before?.updated_at;
  const afterUpdatedAt = after?.updated_at;
  if (
    typeof beforeSha !== "string" ||
    typeof afterSha !== "string" ||
    typeof beforeBaseSha !== "string" ||
    typeof afterBaseSha !== "string" ||
    typeof beforeBaseRef !== "string" ||
    typeof afterBaseRef !== "string" ||
    !Number.isInteger(beforeFiles) ||
    !Number.isInteger(afterFiles) ||
    typeof beforeUpdatedAt !== "string" ||
    typeof afterUpdatedAt !== "string"
  ) {
    throw new TypeError(
      "pull request snapshot is missing head.sha, base.sha, base.ref, changed_files, or updated_at",
    );
  }
  if (
    beforeSha !== afterSha ||
    beforeBaseSha !== afterBaseSha ||
    beforeBaseRef !== afterBaseRef ||
    beforeFiles !== afterFiles ||
    beforeUpdatedAt !== afterUpdatedAt
  ) {
    throw new Error("pull request changed during evaluation; retry the check");
  }
  return after;
}

function statusForEvaluation(result) {
  if (!result || typeof result.allowed !== "boolean") {
    throw new TypeError("result.allowed must be a boolean");
  }
  return result.allowed
    ? { state: "success", description: "Within admission limits" }
    : { state: "failure", description: "Exceeds admission limits" };
}

function evaluatePullRequest(files, labels) {
  if (!Array.isArray(files)) {
    throw new TypeError("files must be an array");
  }
  if (!Array.isArray(labels)) {
    throw new TypeError("labels must be an array");
  }

  const metrics = {
    totalFiles: files.length,
    scopeFiles: 0,
    productionAdditions: 0,
    productionChurn: 0,
  };
  const kinds = {
    documentation: 0,
    lockfile: 0,
    generated: 0,
    test: 0,
    production: 0,
  };

  for (const file of files) {
    const fileClass = classifyFileChange(file);
    kinds[fileClass.kind] += 1;
    if (fileClass.scopeRelevant) {
      metrics.scopeFiles += 1;
    }
    if (fileClass.production) {
      const additions = lineCount(file.additions);
      const deletions = lineCount(file.deletions);
      metrics.productionAdditions += additions;
      metrics.productionChurn += additions + deletions;
    }
  }

  const exceeded = [];
  if (metrics.scopeFiles > LIMITS.scopeFiles) {
    exceeded.push("scopeFiles");
  }
  if (metrics.productionAdditions > LIMITS.productionAdditions) {
    exceeded.push("productionAdditions");
  }
  if (metrics.productionChurn > LIMITS.productionChurn) {
    exceeded.push("productionChurn");
  }

  const oversized = exceeded.length > 0;
  const overridden = oversized && labels.includes(OVERRIDE_LABEL);
  return {
    allowed: !oversized || overridden,
    oversized,
    overridden,
    overrideLabel: OVERRIDE_LABEL,
    limits: LIMITS,
    metrics,
    exceeded,
    kinds,
  };
}

function formatEvaluation(result) {
  const exceeded = result.exceeded.length === 0 ? "none" : result.exceeded.join(", ");
  const override = result.overridden ? result.overrideLabel : "none";
  return [
    `Scope-relevant files: ${result.metrics.scopeFiles} / ${result.limits.scopeFiles}`,
    `Production additions: ${result.metrics.productionAdditions} / ${result.limits.productionAdditions}`,
    `Production churn: ${result.metrics.productionChurn} / ${result.limits.productionChurn}`,
    `Exceeded: ${exceeded}`,
    `Override: ${override}`,
    `Decision: ${result.allowed ? "allowed" : "blocked"}`,
  ].join("\n");
}

module.exports = {
  LIMITS,
  assertCompleteFileList,
  assertStablePullRequestSnapshot,
  classifyFileChange,
  classifyFile,
  evaluatePullRequest,
  formatEvaluation,
  statusForEvaluation,
};

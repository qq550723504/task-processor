"use strict";

const LIMITS = Object.freeze({
  scopeFiles: 30,
  productionAdditions: 1500,
  productionChurn: 2500,
});
const DECISIVE_REVIEW_STATES = new Set(["APPROVED", "CHANGES_REQUESTED", "DISMISSED"]);

const OVERRIDE_LABEL = "architecture-approved";
const EVALUATION_ERROR_SUMMARY = "Admission evaluation failed; retry required";
const POLICY_RESULT_SUMMARIES = new Set([
  "Allowed by authorized architecture override",
  "Within admission limits",
  "Exceeds admission limits",
]);
const NONTERMINAL_CHECK_STATUSES = new Set([
  "queued",
  "in_progress",
  "waiting",
  "requested",
  "pending",
]);
const MAINTAINER_PERMISSIONS = new Set(["admin", "maintain"]);
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

function parseTimestamp(value) {
  return typeof value === "string" ? Date.parse(value) : NaN;
}

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
  const beforeMergeSha = before?.merge_commit_sha;
  const afterMergeSha = after?.merge_commit_sha;
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
    typeof beforeMergeSha !== "string" ||
    typeof afterMergeSha !== "string" ||
    !Number.isInteger(beforeFiles) ||
    !Number.isInteger(afterFiles) ||
    typeof beforeUpdatedAt !== "string" ||
    typeof afterUpdatedAt !== "string"
  ) {
    throw new TypeError(
      "pull request snapshot is missing head.sha, base.sha, base.ref, merge_commit_sha, changed_files, or updated_at",
    );
  }
  if (
    beforeSha !== afterSha ||
    beforeBaseSha !== afterBaseSha ||
    beforeBaseRef !== afterBaseRef ||
    beforeMergeSha !== afterMergeSha ||
    beforeFiles !== afterFiles ||
    beforeUpdatedAt !== afterUpdatedAt
  ) {
    throw new Error("pull request changed during evaluation; retry the check");
  }
  return after;
}

function statusTargetForPullRequest(snapshot) {
  const mergeSha = snapshot?.merge_commit_sha;
  if (typeof mergeSha !== "string" || mergeSha.trim() === "") {
    throw new TypeError("pull request snapshot is missing merge_commit_sha");
  }
  return mergeSha;
}

function pullRequestNumberFromEventPayload(eventName, payload, signalPullRequestNumber = null) {
  let pullRequestNumber;
  if (eventName === "workflow_run") {
    const pullRequests = payload?.workflow_run?.pull_requests;
    if (
      !Array.isArray(pullRequests) ||
      !Number.isInteger(signalPullRequestNumber) ||
      signalPullRequestNumber <= 0 ||
      !pullRequests.some(({ number }) => number === signalPullRequestNumber)
    ) {
      throw new Error("trusted signal must identify one associated pull request");
    }
    pullRequestNumber = signalPullRequestNumber;
  } else if (eventName === "pull_request_target") {
    pullRequestNumber = payload?.pull_request?.number;
  } else if (eventName === "repository_dispatch") {
    pullRequestNumber = Number(payload?.client_payload?.pull_request_number);
  } else {
    throw new Error(`unsupported admission event: ${eventName}`);
  }
  if (!Number.isInteger(pullRequestNumber) || pullRequestNumber <= 0) {
    throw new Error("admission event is missing a valid pull request number");
  }
  return pullRequestNumber;
}

function labelNames(labels) {
  if (!Array.isArray(labels)) {
    throw new TypeError("labels must be an array");
  }
  return labels
    .map((label) => label?.name)
    .filter((name) => typeof name === "string")
    .sort();
}

function reviewFingerprint(reviews) {
  if (!Array.isArray(reviews)) {
    throw new TypeError("reviews must be an array");
  }
  return reviews
    .map((review) => ({
      id: review?.id,
      login: review?.user?.login,
      state: review?.state,
      commit_id: review?.commit_id,
      submitted_at: review?.submitted_at,
    }))
    .sort((left, right) => String(left.id).localeCompare(String(right.id)));
}

function latestBaseChangeAt(events) {
  if (!Array.isArray(events)) {
    throw new TypeError("events must be an array");
  }
  const baseChanges = events
    .filter((event) => event?.event === "base_ref_changed")
    .map((event) => event.created_at)
    .filter((createdAt) => typeof createdAt === "string");
  return baseChanges.sort().at(-1) ?? null;
}

function assertStableAdmissionSnapshot(before, after, beforeReviews, afterReviews, beforeEvents = [], afterEvents = []) {
  assertStablePullRequestSnapshot(before, after);
  if (
    JSON.stringify(labelNames(before?.labels)) !== JSON.stringify(labelNames(after?.labels)) ||
    JSON.stringify(reviewFingerprint(beforeReviews)) !== JSON.stringify(reviewFingerprint(afterReviews)) ||
    latestBaseChangeAt(beforeEvents) !== latestBaseChangeAt(afterEvents)
  ) {
    throw new Error("pull request admission inputs changed during evaluation; retry the check");
  }
  return after;
}

function latestReviewsByUser(reviews) {
  if (!Array.isArray(reviews)) {
    throw new TypeError("reviews must be an array");
  }
  const latest = new Map();
  for (const review of reviews) {
    const login = review?.user?.login;
    if (typeof login === "string" && login.trim() !== "" && DECISIVE_REVIEW_STATES.has(review?.state)) {
      const previous = latest.get(login);
      if (!previous || Number(review?.id) >= Number(previous?.id)) {
        latest.set(login, review);
      }
    }
  }
  return latest;
}

function approvalReviewersForCurrentHead(reviews, headSha) {
  if (typeof headSha !== "string" || headSha.trim() === "") {
    throw new TypeError("headSha must be a non-empty string");
  }
  return [...latestReviewsByUser(reviews).entries()]
    .filter(([, review]) => review?.state === "APPROVED" && review?.commit_id === headSha)
    .map(([login]) => login);
}

function authorizedArchitectureApprovers({
  labels,
  headSha,
  authorLogin = null,
  baseChangedAt = null,
  reviews,
  permissions,
}) {
  if (!Array.isArray(labels)) {
    throw new TypeError("labels must be an array");
  }
  if (!permissions || typeof permissions !== "object" || Array.isArray(permissions)) {
    throw new TypeError("permissions must be an object");
  }
  if (authorLogin !== null && (typeof authorLogin !== "string" || authorLogin.trim() === "")) {
    throw new TypeError("authorLogin must be null or a non-empty string");
  }
  if (baseChangedAt !== null && (typeof baseChangedAt !== "string" || Number.isNaN(parseTimestamp(baseChangedAt)))) {
    throw new TypeError("baseChangedAt must be null or an ISO timestamp");
  }
  if (!labels.includes(OVERRIDE_LABEL)) {
    return [];
  }
  const latestReviews = latestReviewsByUser(reviews);
  return approvalReviewersForCurrentHead(reviews, headSha)
    .filter((login) => {
      const review = latestReviews.get(login);
      const roleName = permissions[login]?.role_name;
      if (login === authorLogin || !MAINTAINER_PERMISSIONS.has(roleName)) {
        return false;
      }
      return baseChangedAt === null || (
        typeof review?.submitted_at === "string" &&
        !Number.isNaN(parseTimestamp(review.submitted_at)) &&
        parseTimestamp(review.submitted_at) > parseTimestamp(baseChangedAt)
      );
  });
}

function hasAuthorizedArchitectureOverride(options) {
  return authorizedArchitectureApprovers(options).length > 0;
}

function hasRequiredOverrideEvidence(body, approvedLogins) {
  if (typeof body !== "string" || !Array.isArray(approvedLogins)) {
    return false;
  }
  const normalizedBody = body.replaceAll("`", "");
  const valueFor = (pattern) => {
    const match = normalizedBody.match(pattern);
    const value = match?.[1]?.trim();
    return value && !/^(?:n\/a|none|tbd|todo)(?:\s*:\s*|$)/i.test(value)
      ? value
      : null;
  };
  const design = valueFor(/^\s*-\s*Design:\s*(.+)$/im);
  const independentReview = valueFor(/^\s*-\s*Independent design review:\s*(.+)$/im);
  const approver = valueFor(
    /^\s*-\s*(?:Override approver[^:\n]*|architecture-approved[^:\n]*approval review[^:\n]*split rationale[^:\n]*):\s*(.+)$/im,
  );
  const splitRationale = valueFor(
    /^\s*-\s*(?:Split rationale[^:\n]*|architecture-approved[^:\n]*split rationale[^:\n]*):\s*(.+)$/im,
  );
  const hasDesignLink = design && /https?:\/\/\S+|(?:^|\s)(?:docs|designs?)\/\S+/i.test(design);
  const hasApprovedLogin = approvedLogins.some((login) => {
    if (typeof login !== "string" || login.trim() === "") {
      return false;
    }
    const escapedLogin = login.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    return new RegExp(`(?:^|[^A-Za-z0-9_.-])${escapedLogin}(?:$|[^A-Za-z0-9_.-])`, "i").test(approver || "");
  });
  return Boolean(hasDesignLink && independentReview && approver && splitRationale && hasApprovedLogin);
}

function hasRecentLabelRemoval(events, labelName, now = Date.now(), windowMs = 15 * 60 * 1000) {
  if (!Array.isArray(events)) {
    throw new TypeError("events must be an array");
  }
  if (typeof labelName !== "string" || labelName.trim() === "") {
    throw new TypeError("labelName must be a non-empty string");
  }
  const nowMs = typeof now === "number" ? now : parseTimestamp(now);
  if (!Number.isFinite(nowMs) || !Number.isFinite(windowMs) || windowMs <= 0) {
    throw new TypeError("now and windowMs must be valid time values");
  }
  return events.some((event) => {
    const createdAt = parseTimestamp(event?.created_at);
    return event?.event === "unlabeled" &&
      event?.label?.name === labelName &&
      Number.isFinite(createdAt) &&
      createdAt >= nowMs - windowMs &&
      createdAt <= nowMs;
  });
}

function needsAdmissionReconciliation({
  labels,
  baseRef,
  defaultBranch,
  events,
  checkRuns,
  now = Date.now(),
  inProgressTimeoutMs = 10 * 60 * 1000,
}) {
  if (!Array.isArray(labels) || !Array.isArray(events) || !Array.isArray(checkRuns)) {
    throw new TypeError("labels, events, and checkRuns must be arrays");
  }
  if (typeof baseRef !== "string" || typeof defaultBranch !== "string") {
    throw new TypeError("baseRef and defaultBranch must be strings");
  }
  const nowMs = typeof now === "number" ? now : parseTimestamp(now);
  if (!Number.isFinite(nowMs) || !Number.isFinite(inProgressTimeoutMs) || inProgressTimeoutMs <= 0) {
    throw new TypeError("now and inProgressTimeoutMs must be valid time values");
  }
  if (baseRef !== defaultBranch || labels.includes(OVERRIDE_LABEL)) {
    return true;
  }
  const overrideLabelRemovalEvents = events
    .filter((event) => event?.event === "unlabeled" && event?.label?.name === OVERRIDE_LABEL);
  if (overrideLabelRemovalEvents.some((event) => !Number.isFinite(parseTimestamp(event?.created_at)))) {
    return true;
  }
  const latestRemovalMs = overrideLabelRemovalEvents
    .map((event) => parseTimestamp(event?.created_at))
    .filter((createdAt) => Number.isFinite(createdAt))
    .reduce((latest, createdAt) => Math.max(latest, createdAt), -Infinity);
  if (checkRuns.some((run) => !Number.isFinite(parseTimestamp(run?.created_at)))) {
    return true;
  }
  const latestRun = checkRuns.reduce((latest, run) => {
    if (!latest) {
      return run;
    }
    const latestCreatedAtMs = parseTimestamp(latest?.created_at);
    const createdAtMs = parseTimestamp(run?.created_at);
    if (createdAtMs < latestCreatedAtMs) {
      return latest;
    }
    if (createdAtMs > latestCreatedAtMs) {
      return run;
    }
    return Number(run?.id) > Number(latest?.id) ? run : latest;
  }, null);
  if (!latestRun) {
    return true;
  }
  if (latestRun.status !== "completed") {
    if (!NONTERMINAL_CHECK_STATUSES.has(latestRun.status)) {
      return true;
    }
    const startedAtMs = parseTimestamp(latestRun?.started_at || latestRun?.created_at);
    return !Number.isFinite(startedAtMs) || startedAtMs <= nowMs - inProgressTimeoutMs;
  }
  const startedAtMs = parseTimestamp(latestRun?.started_at);
  const completedAtMs = parseTimestamp(latestRun?.completed_at);
  if (
    !Number.isFinite(startedAtMs) ||
    !Number.isFinite(completedAtMs) ||
    startedAtMs > completedAtMs
  ) {
    return true;
  }
  if (
    typeof latestRun.conclusion !== "string" ||
    !POLICY_RESULT_SUMMARIES.has(latestRun?.output?.summary)
  ) {
    return true;
  }
  if (Number.isFinite(latestRemovalMs)) {
    const startedAtMs = parseTimestamp(latestRun?.started_at || latestRun?.created_at);
    return !Number.isFinite(startedAtMs) || startedAtMs <= latestRemovalMs;
  }
  return false;
}

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

function evaluatePullRequest(files, labels, options = {}) {
  if (!Array.isArray(files)) {
    throw new TypeError("files must be an array");
  }
  if (!Array.isArray(labels)) {
    throw new TypeError("labels must be an array");
  }
  if (!options || typeof options !== "object" || Array.isArray(options)) {
    throw new TypeError("options must be an object");
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
  const overrideLabelPresent = labels.includes(OVERRIDE_LABEL);
  const overrideAuthorized = options.overrideAuthorized === true;
  const overridden = oversized && overrideLabelPresent && overrideAuthorized;
  return {
    allowed: !oversized || overridden,
    oversized,
    overridden,
    overrideLabelPresent,
    overrideAuthorized,
    overrideLabel: OVERRIDE_LABEL,
    limits: LIMITS,
    metrics,
    exceeded,
    kinds,
  };
}

function formatEvaluation(result) {
  const exceeded = result.exceeded.length === 0 ? "none" : result.exceeded.join(", ");
  const override = result.overridden
    ? result.overrideLabel
    : result.overrideLabelPresent
      ? `${result.overrideLabel} (unauthorized)`
      : "none";
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
  EVALUATION_ERROR_SUMMARY,
  LIMITS,
  OVERRIDE_LABEL,
  approvalReviewersForCurrentHead,
  authorizedArchitectureApprovers,
  assertCompleteFileList,
  assertStableAdmissionSnapshot,
  assertStablePullRequestSnapshot,
  classifyFileChange,
  classifyFile,
  evaluatePullRequest,
  formatEvaluation,
  hasAuthorizedArchitectureOverride,
  hasRequiredOverrideEvidence,
  hasRecentLabelRemoval,
  needsAdmissionReconciliation,
  latestBaseChangeAt,
  pullRequestNumberFromEventPayload,
  statusForEvaluation,
  statusTargetForPullRequest,
};

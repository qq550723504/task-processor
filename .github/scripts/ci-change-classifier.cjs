"use strict";

const fs = require("node:fs");

function normalizePath(value) {
  if (typeof value !== "string") {
    throw new TypeError("path must be a string");
  }
  return value.trim().replaceAll("\\", "/").replace(/^\.\//, "").toLowerCase();
}

function matchesPrefix(path, prefix) {
  return path === prefix || path.startsWith(prefix.endsWith("/") ? prefix : `${prefix}/`);
}

function classifyChangedPaths(paths, { full = false } = {}) {
  if (!Array.isArray(paths)) {
    throw new TypeError("paths must be an array");
  }
  if (full) {
    return {
      backend: true,
      frontend: true,
      code_health: true,
      release_authority: true,
    };
  }

  const result = {
    backend: false,
    frontend: false,
    code_health: false,
    release_authority: false,
  };

  for (const raw of paths) {
    const path = normalizePath(raw);
    if (!path) continue;

    const architectureAuthority =
      path === "agents.md" ||
      path === ".golangci.yml" ||
      matchesPrefix(path, "docs/architecture") ||
      matchesPrefix(path, "docs/refactoring") ||
      matchesPrefix(path, "docs/product") ||
      matchesPrefix(path, "policy");

    const backendSource =
      path === "go.mod" ||
      path === "go.sum" ||
      path === "go.work" ||
      path === "go.work.sum" ||
      path === "makefile" ||
      matchesPrefix(path, "cmd") ||
      matchesPrefix(path, "config") ||
      matchesPrefix(path, "hack/debug") ||
      matchesPrefix(path, "internal") ||
      matchesPrefix(path, "prompts") ||
      matchesPrefix(path, "scripts") ||
      matchesPrefix(path, "tests") ||
      matchesPrefix(path, "tools") ||
      path === "deployments/docker/dockerfile.product-listing-api";

    // Go-owned contract tests validate workflow and rollout-manifest semantics that
    // the release-policy shell checks intentionally do not duplicate.
    const backendContract =
      matchesPrefix(path, ".github/workflows") ||
      matchesPrefix(path, "deployments/kubernetes/listingkit-workbench");

    const backend = architectureAuthority || backendSource || backendContract;

    const frontend =
      matchesPrefix(path, "web/listingkit-ui") ||
      path === "deployments/docker/dockerfile.listingkit-ui";

    const codeHealth =
      backendSource ||
      frontend ||
      path === ".golangci.yml" ||
      path === "docs/refactoring/code-health-decisions.md";

    const releaseAuthority =
      matchesPrefix(path, ".github/workflows") ||
      path === "scripts/verify-listingkit-release-authority-policy.sh" ||
      path === "scripts/tests/listingkit-image-agent-canary-order-test.sh" ||
      matchesPrefix(path, "deployments/kubernetes/listingkit-workbench") ||
      matchesPrefix(path, "policy/listingkit-release-authority");

    result.backend ||= backend;
    result.frontend ||= frontend;
    result.code_health ||= codeHealth;
    result.release_authority ||= releaseAuthority;
  }

  return result;
}

function formatOutputs(result) {
  return Object.entries(result)
    .map(([key, value]) => `${key}=${value ? "true" : "false"}`)
    .join("\n");
}

function main(argv) {
  if (argv.includes("--all")) {
    process.stdout.write(`${formatOutputs(classifyChangedPaths([], { full: true }))}\n`);
    return;
  }
  if (argv.length !== 1) {
    throw new Error("usage: node ci-change-classifier.cjs <changed-files.txt> | --all");
  }
  const input = fs.readFileSync(argv[0], "utf8");
  const paths = input.split(/\r?\n/).filter(Boolean);
  process.stdout.write(`${formatOutputs(classifyChangedPaths(paths))}\n`);
}

if (require.main === module) {
  main(process.argv.slice(2));
}

module.exports = { classifyChangedPaths, formatOutputs, normalizePath };

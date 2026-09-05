const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { classifyChangedPaths, formatOutputs } = require("./ci-change-classifier.cjs");

test("runs all suites for main push mode", () => {
  assert.deepEqual(classifyChangedPaths([], { full: true }), {
    backend: true,
    frontend: true,
    code_health: true,
    release_authority: true,
  });
});

test("backend-only changes skip frontend", () => {
  assert.deepEqual(classifyChangedPaths(["internal/product/sourcing/service.go"]), {
    backend: true,
    frontend: false,
    code_health: true,
    release_authority: false,
  });
});

test("frontend-only changes skip backend", () => {
  assert.deepEqual(classifyChangedPaths(["web/listingkit-ui/src/app/page.tsx"]), {
    backend: false,
    frontend: true,
    code_health: true,
    release_authority: false,
  });
});

test("architecture authority docs trigger backend guards without frontend", () => {
  for (const path of [
    "AGENTS.md",
    "docs/architecture/project-target-architecture.md",
    "docs/refactoring/legacy-register.md",
    "docs/product/final-ui-ia-authority.md",
  ]) {
    const result = classifyChangedPaths([path]);
    assert.equal(result.backend, true, path);
    assert.equal(result.frontend, false, path);
    assert.equal(result.code_health, false, `${path} should not run code health`);
  }
});

test("release policy and workflow changes run their Go-owned contract tests", () => {
  for (const changedPath of [
    ".github/workflows/listingkit-deploy.yml",
    ".github/workflows/ci.yml",
    "policy/listingkit-release-authority/rules.yaml",
  ]) {
    const result = classifyChangedPaths([changedPath]);
    assert.equal(result.release_authority, true, changedPath);
    assert.equal(result.backend, true, changedPath);
    assert.equal(result.frontend, false, changedPath);
  }
});

test("workbench release documentation and manifests run release plus backend contracts", () => {
  for (const changedPath of [
    "deployments/kubernetes/listingkit-workbench/README.md",
    "deployments/kubernetes/listingkit-workbench/base/image-agent-temporal-worker-deployment.yaml",
  ]) {
    const result = classifyChangedPaths([changedPath]);
    assert.equal(result.release_authority, true, changedPath);
    assert.equal(result.backend, true, changedPath);
    assert.equal(result.frontend, false, changedPath);
  }
});

test("scripts run backend and code-health safety nets", () => {
  for (const changedPath of [
    "scripts/listingkit-shein-pod-image-index-backfill/main.go",
    "scripts/listingkit-shein-pod-image-index-backfill/main_test.go",
    "scripts/code-health-audit.config.json",
  ]) {
    const result = classifyChangedPaths([changedPath]);
    assert.equal(result.backend, true, changedPath);
    assert.equal(result.code_health, true, changedPath);
    assert.equal(result.frontend, false, changedPath);
  }
});

test("hack debug nested module runs backend and code-health safety nets", () => {
  const changedPath = "hack/debug/listingkit-phone-onboarding-preflight/main.go";
  const result = classifyChangedPaths([changedPath]);
  assert.equal(result.backend, true);
  assert.equal(result.code_health, true);
  assert.equal(result.frontend, false);
});

test("workflow-only changes run backend contracts without frontend or code-health", () => {
  const result = classifyChangedPaths([".github/workflows/ci.yml"]);
  assert.deepEqual(result, {
    backend: true,
    frontend: false,
    code_health: false,
    release_authority: true,
  });
});

test("classification diff disables rename detection so both source and destination paths are visible", () => {
  const workflowPath = path.join(__dirname, "..", "workflows", "ci.yml");
  const workflow = fs.readFileSync(workflowPath, "utf8").replaceAll("\r\n", "\n");
  assert.match(
    workflow,
    /git diff --no-renames --name-only "\$BASE_SHA\.\.\.\$HEAD_SHA"/,
  );
});

test("normalizes windows paths and emits github outputs", () => {
  const result = classifyChangedPaths(["web\\listingkit-ui\\src\\app.tsx"]);
  assert.equal(result.frontend, true);
  assert.equal(formatOutputs(result), [
    "backend=false",
    "frontend=true",
    "code_health=true",
    "release_authority=false",
  ].join("\n"));
});

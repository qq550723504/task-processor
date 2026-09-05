const test = require("node:test");
const assert = require("node:assert/strict");
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

test("release policy changes run release authority", () => {
  const result = classifyChangedPaths([
    ".github/workflows/listingkit-deploy.yml",
    "policy/listingkit-release-authority/rules.yaml",
  ]);
  assert.equal(result.release_authority, true);
  assert.equal(result.backend, true, "policy is also architecture authority");
});

test("workbench release documentation runs release authority", () => {
  const result = classifyChangedPaths([
    "deployments/kubernetes/listingkit-workbench/README.md",
  ]);
  assert.equal(result.release_authority, true);
  assert.equal(result.frontend, false);
});

test("scripts run backend and code-health safety nets", () => {
  for (const path of [
    "scripts/listingkit-shein-pod-image-index-backfill/main.go",
    "scripts/listingkit-shein-pod-image-index-backfill/main_test.go",
    "scripts/code-health-audit.config.json",
  ]) {
    const result = classifyChangedPaths([path]);
    assert.equal(result.backend, true, path);
    assert.equal(result.code_health, true, path);
    assert.equal(result.frontend, false, path);
  }
});

test("ci workflow change does not force full application suites", () => {
  const result = classifyChangedPaths([".github/workflows/ci.yml"]);
  assert.deepEqual(result, {
    backend: false,
    frontend: false,
    code_health: false,
    release_authority: true,
  });
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

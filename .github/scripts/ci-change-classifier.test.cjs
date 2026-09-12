const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { spawnSync } = require("node:child_process");
const { classifyChangedPaths, formatOutputs } = require("./ci-change-classifier.cjs");

test("runs all suites for main push mode", () => {
  assert.deepEqual(classifyChangedPaths([], { full: true }), {
    backend: true,
    frontend: true,
    code_health: true,
    release_authority: true,
    capture_extension: true,
  });
});

test("backend-only changes skip frontend", () => {
  assert.deepEqual(classifyChangedPaths(["internal/product/sourcing/service.go"]), {
    backend: true,
    frontend: false,
    code_health: true,
    release_authority: false,
    capture_extension: false,
  });
});

test("frontend-only changes skip backend", () => {
  assert.deepEqual(classifyChangedPaths(["web/listingkit-ui/src/app/page.tsx"]), {
    backend: false,
    frontend: true,
    code_health: true,
    release_authority: false,
    capture_extension: false,
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
    capture_extension: true,
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

test("pull requests always run repository-wide Go architecture contracts", () => {
  const workflowPath = path.join(__dirname, "..", "workflows", "ci.yml");
  const workflow = fs.readFileSync(workflowPath, "utf8").replaceAll("\r\n", "\n");
  const architectureJob = workflow.slice(
    workflow.indexOf("\n  architecture-contracts:\n"),
    workflow.indexOf("\n  release-authority:\n"),
  );
  const requiredGate = workflow.slice(
    workflow.indexOf("\n  required-gate:\n"),
    workflow.indexOf("\n  notify:\n"),
  );

  assert.match(architectureJob, /name: Architecture Contract Guards/);
  assert.match(architectureJob, /if: \$\{\{ github\.event_name == 'pull_request' \}\}/);
  assert.match(architectureJob, /run: go test \.\/tests\/\.\.\. -count=1/);
  assert.match(requiredGate, /needs:[\s\S]*- architecture-contracts\b/);
  assert.match(
    requiredGate,
    /ARCHITECTURE_CONTRACT_RESULT:\s*\$\{\{\s*needs\.architecture-contracts\.result\s*\}\}/,
  );
  assert.match(requiredGate, /require_success "architecture contracts" "\$ARCHITECTURE_CONTRACT_RESULT"/);
});

test("normalizes windows paths and emits github outputs", () => {
  const result = classifyChangedPaths(["web\\listingkit-ui\\src\\app.tsx"]);
  assert.equal(result.frontend, true);
  assert.equal(formatOutputs(result), [
    "backend=false",
    "frontend=true",
    "code_health=true",
    "release_authority=false",
    "capture_extension=false",
  ].join("\n"));
});

test("capture extension has a precise independent classification including its CI owners", () => {
  for (const changedPath of [
    "extensions/1688-capture/src/extractor.ts", "extensions\\1688-capture\\package-lock.json",
    ".github/workflows/ci.yml", ".github/scripts/ci-change-classifier.cjs", ".github/scripts/ci-change-classifier.test.cjs",
  ]) assert.equal(classifyChangedPaths([changedPath]).capture_extension, true, changedPath);
  for (const changedPath of ["extensions/1688-capture-other/main.ts", "extensions/other/main.ts", "README.md",
    "web/listingkit-ui/src/app.tsx", "internal/product/sourcing/service.go", ".github/workflows/listingkit-deploy.yml"]) {
    assert.equal(classifyChangedPaths([changedPath]).capture_extension, false, changedPath);
  }
  const extension=classifyChangedPaths(["extensions/1688-capture/src/extractor.ts"]);
  assert.deepEqual([extension.backend,extension.frontend,extension.code_health,extension.release_authority],[false,false,false,false]);
  assert.equal(classifyChangedPaths([],{full:true}).capture_extension,true);
});

function captureGate() {
  const workflow=fs.readFileSync(path.join(__dirname,"..","workflows","ci.yml"),"utf8").replaceAll("\r\n","\n");
  const required=workflow.slice(workflow.indexOf("\n  required-gate:\n"),workflow.indexOf("\n  notify:\n"));
  const script=required.slice(required.indexOf("        run: |\n")+"        run: |\n".length).split("\n").map(line=>line.startsWith("          ")?line.slice(10):line).join("\n");
  return {workflow,required,script};
}

test("capture classification output reaches its isolated job and required gate", () => {
  const {workflow,required}=captureGate();
  assert.match(workflow,/capture_extension: \$\{\{ steps\.classify\.outputs\.capture_extension \}\}/);
  const job=workflow.slice(workflow.indexOf("\n  capture-extension:\n"),workflow.indexOf("\n  code-health:\n"));
  assert.match(job,/needs\.changes\.outputs\.capture_extension == 'true'/);
  assert.match(job,/node-version: "24"/);
  assert.match(job,/working-directory: extensions\/1688-capture/);
  assert.match(job,/cache-dependency-path: extensions\/1688-capture\/package-lock\.json/);
  for(const command of ["npm ci","npm test","npm run typecheck","npm run lint","npm run build -- --fixture"]) assert.ok(job.includes(command),command);
  assert.match(required,/needs:[\s\S]*- capture-extension\b/);
  assert.match(required,/CAPTURE_EXTENSION_RESULT: \$\{\{ needs\.capture-extension\.result \}\}/);
  assert.match(required,/CAPTURE_EXTENSION_SELECTED: \$\{\{ needs\.changes\.outputs\.capture_extension \}\}/);
});

test("actual required-gate shell rejects missing applicable capture checks", () => {
  const {script}=captureGate();
  const bash=process.platform==='win32' ? 'C:/Program Files/Git/bin/bash.exe' : 'bash';
  for(const event of ['push','pull_request']) for(const selected of ['true','false','']) {
    for(const result of ['success','failure','cancelled','skipped','']) {
      const env={...process.env,CI_EVENT:event,CAPTURE_EXTENSION_SELECTED:selected,CAPTURE_EXTENSION_RESULT:result};
      for(const key of ['CHANGES_RESULT','DEVELOPMENT_ADMISSION_TEST_RESULT','ARCHITECTURE_CONTRACT_RESULT','ISOLATED_RUNTIME_RESULT',
        'RELEASE_RESULT','BACKEND_RESULT','FRONTEND_RESULT','CODE_HEALTH_RESULT']) env[key]='success';
      const run=spawnSync(bash,['-c',script],{env,encoding:'utf8'});
      if(run.error)throw run.error;
      const applicable=event==='push'||selected==='true';
      const shouldPass=applicable ? result==='success' : selected==='false' && ['success','skipped'].includes(result);
      assert.equal(run.status===0,shouldPass,`${event}/${selected}/${result}: ${run.stderr}`);
    }
  }
});

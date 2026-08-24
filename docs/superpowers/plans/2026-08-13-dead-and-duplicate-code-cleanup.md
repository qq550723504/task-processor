# Dead and Duplicate Code Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove every code path that can be proven dead and consolidate every same-owner duplicate implementation, with a reproducible audit proving that each remaining detector finding is intentional and classified.

**Architecture:** Run all cleanup from an isolated Git worktree because the primary checkout contains unrelated ListingKit and frontend work. Establish the true executable root set and pinned analyzer baseline before deletion, then process frontend orphans, active Go leaves, same-owner clones, dormant subgraphs, tests/scripts/tools, and dependencies as independently testable batches. Store bulk analyzer output under `.local/code-health/`; commit only code, tests, pinned configuration, and the concise decision ledger.

**Tech Stack:** Go 1.26, PowerShell 7, Node.js 22+, Next.js 16, TypeScript 6, Vitest 4, `deadcode` v0.48.0, Knip 6.32.2, jscpd 5.0.14.

## Global Constraints

- Execute from an isolated worktree created with `superpowers:using-git-worktrees`; do not modify, stage, or delete the primary checkout's unrelated files.
- Preserve the approved design in `docs/superpowers/specs/2026-08-13-dead-and-duplicate-code-cleanup-design.md` as the source of truth.
- Treat detector output as candidates, not deletion instructions.
- Generated files are changed only through their schema or generator inputs.
- Do not remove a command, route, Temporal/RabbitMQ registration, config key, migration hook, serialization contract, deferred platform capability, or test invariant without direct ownership evidence.
- Do not add a generic helper merely to reduce clone counts; shared code must have one stable owner and one reason to change.
- Keep behavior changes, package moves, and cleanup deletions in separate commits.
- Stage only the exact paths named by the current task.
- Do not push, open a PR, merge, deploy, mutate a database, or activate a provider without separate authorization.

---

## File and Artifact Map

- `tests/repository_structure_test.go`: resolves tracked paths from the repository root and enforces the complete maintained command inventory.
- `docs/development/repository-structure.md`: distinguishes four product runtimes from maintained operational commands.
- `scripts/code-health-audit.config.json`: pins analyzer versions, root modules, target configurations, input paths, and exclusions.
- `scripts/code-health-audit.ps1`: runs dead-code, frontend-graph, clone, and exact-reference evidence without changing source.
- `tests/code_health_audit_test.go`: guards analyzer versions, roots, exclusions, and `.local`-only bulk output.
- `web/listingkit-ui/knip.jsonc`: defines Next/Vitest/type-test entries and explicit generated-contract treatment.
- `docs/refactoring/code-health-decisions.md`: concise classification ledger for retained dead-code and clone findings.
- `.local/code-health/**`: untracked raw JSON/text reports for each scan configuration.

### Task 1: Repair the command inventory guard and establish executable authority

**Files:**
- Modify: `tests/repository_structure_test.go`
- Modify: `docs/development/repository-structure.md`
- Test: `tests/repository_structure_test.go`

**Interfaces:**
- Consumes: Git's tracked-file index and the deployed/build references under `.github`, `deployments`, and `scripts`.
- Produces: `trackedFiles(t, pathspec)` that works from any Go package directory, plus explicit `productRuntimeCommands` and `operationalCommands` inventories used by later reachability scans.

- [ ] **Step 1: Create the isolated worktree before any implementation edit**

Run the `superpowers:using-git-worktrees` skill, then create a branch named `codex/dead-duplicate-code-cleanup` from commit `f25e35683` or the current descendant containing the approved design.

Expected: `git status --short --branch` is clean in the new worktree; the primary checkout's child-retry files are absent from the worktree diff.

- [ ] **Step 2: Add a failing regression test for repository-root path resolution**

Add this test next to `trackedFiles`:

```go
func TestTrackedFilesResolvesFromPackageDirectory(t *testing.T) {
	t.Parallel()

	files := trackedFiles(t, "cmd")
	if len(files) == 0 {
		t.Fatal("expected tracked cmd files when the test process runs from the tests package directory")
	}
	if !slices.Contains(files, "cmd/product-listing-api/main.go") {
		t.Fatalf("tracked cmd files do not contain product-listing-api: %v", files)
	}
}
```

Add `slices` to the test imports.

- [ ] **Step 3: Run the regression test and verify the current false-green behavior**

Run:

```powershell
go test ./tests -run TestTrackedFilesResolvesFromPackageDirectory -count=1
```

Expected: FAIL with `expected tracked cmd files` because `git ls-files cmd` currently runs from `tests/` and returns an empty list.

- [ ] **Step 4: Resolve tracked paths from the Git repository root**

Replace `trackedFiles` with:

```go
func trackedFiles(t *testing.T, pathspec string) []string {
	t.Helper()

	repoRootBytes, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := strings.TrimSpace(string(repoRootBytes))
	cmd := exec.Command("git", "-C", repoRoot, "ls-files", "--", filepath.ToSlash(pathspec))
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(trimmed, "\r\n", "\n"), "\n")
}
```

- [ ] **Step 5: Make maintained command categories explicit**

Define these exact sets in `TestCmdContainsOnlyOfficialEntrypoints`:

```go
productRuntimeCommands := map[string]struct{}{
	"listing-control-plane": {},
	"product-listing-api":   {},
	"shein-listing":         {},
	"temu-listing":          {},
}
operationalCommands := map[string]struct{}{
	"fingerprint-browser-installer":          {},
	"listing-scheduler":                      {},
	"listingkit-identity-preflight":           {},
	"listingkit-owner-scope-dry-run":          {},
	"listingkit-owner-scope-exceptions":       {},
	"listingkit-schema-migrate":               {},
	"playwright-installer":                    {},
	"product-listing-api-schema-migrate":      {},
	"shein-import-platform-recovery":          {},
	"shein-login-worker":                      {},
}
```

Accept a directory only when it is present in exactly one set. Update `docs/development/repository-structure.md` to name the four product runtimes separately from the ten operational commands and to require a build/deploy/script owner for every operational entry.

- [ ] **Step 6: Run the structure tests**

Run:

```powershell
go test ./tests -run "TestTrackedFilesResolvesFromPackageDirectory|TestCmdContainsOnlyOfficialEntrypoints|TestProductionEntrypointsContainNoLocalArtifacts" -count=1
```

Expected: PASS and the command guard examines 21 tracked files under `cmd/`.

- [ ] **Step 7: Commit the repaired authority boundary**

```powershell
git add -- tests/repository_structure_test.go docs/development/repository-structure.md
git commit -m "test: enforce maintained command inventory"
```

### Task 2: Add the pinned, read-only code-health audit

**Files:**
- Create: `scripts/code-health-audit.config.json`
- Create: `scripts/code-health-audit.ps1`
- Create: `tests/code_health_audit_test.go`
- Create: `web/listingkit-ui/knip.jsonc`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: maintained command inventory from Task 1 and repository source roots.
- Produces: a timestamped `$runDir` beneath `.local/code-health`, containing `manifest.json`, `deadcode-*.json`, `knip.json`, and `jscpd.txt`; no source mutation.

- [ ] **Step 1: Write the audit-contract test**

Create `tests/code_health_audit_test.go` with a JSON model and assertions for exact pinned values:

```go
package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

type codeHealthConfig struct {
	DeadcodeVersion string   `json:"deadcode_version"`
	KnipVersion     string   `json:"knip_version"`
	JscpdVersion    string   `json:"jscpd_version"`
	RootPatterns    []string `json:"root_patterns"`
	TargetGOOS      []string `json:"target_goos"`
	OutputRoot      string   `json:"output_root"`
	ClonePaths      []string `json:"clone_paths"`
	CloneIgnore     []string `json:"clone_ignore"`
}

func TestCodeHealthAuditConfigPinsScopeAndTools(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "scripts", "code-health-audit.config.json"))
	if err != nil { t.Fatal(err) }
	var cfg codeHealthConfig
	if err := json.Unmarshal(data, &cfg); err != nil { t.Fatal(err) }
	if cfg.DeadcodeVersion != "v0.48.0" || cfg.KnipVersion != "6.32.2" || cfg.JscpdVersion != "5.0.14" {
		t.Fatalf("unexpected analyzer versions: %+v", cfg)
	}
	for _, required := range []string{"./cmd/...", "./scripts/..."} {
		if !slices.Contains(cfg.RootPatterns, required) { t.Errorf("missing root %s", required) }
	}
	for _, goos := range []string{"windows", "linux"} {
		if !slices.Contains(cfg.TargetGOOS, goos) { t.Errorf("missing GOOS %s", goos) }
	}
	if cfg.OutputRoot != ".local/code-health" { t.Errorf("unsafe output root %q", cfg.OutputRoot) }
}
```

- [ ] **Step 2: Run the contract test and verify it fails**

Run: `go test ./tests -run TestCodeHealthAuditConfigPinsScopeAndTools -count=1`

Expected: FAIL because `scripts/code-health-audit.config.json` does not exist.

- [ ] **Step 3: Create the exact analyzer configuration**

Create:

```json
{
  "deadcode_version": "v0.48.0",
  "knip_version": "6.32.2",
  "jscpd_version": "5.0.14",
  "root_patterns": ["./cmd/...", "./scripts/..."],
  "nested_modules": ["tools", "hack/debug"],
  "target_goos": ["windows", "linux"],
  "output_root": ".local/code-health",
  "clone_paths": ["internal", "cmd", "scripts", "tools", "hack/debug", "web/listingkit-ui/src"],
  "clone_ignore": [
    "**/*_test.go",
    "**/*.test.ts",
    "**/*.test.tsx",
    "**/node_modules/**",
    "**/.next/**",
    "**/.worktrees/**",
    "**/tmp/**",
    "**/.local/**",
    "**/generated/**"
  ],
  "clone_min_lines": 12,
  "clone_min_tokens": 70
}
```

- [ ] **Step 4: Configure Knip around real entrypoints and generated contracts**

Create `web/listingkit-ui/knip.jsonc`:

```jsonc
{
  "$schema": "https://unpkg.com/knip@6/schema.json",
  "entry": [
    "src/lib/api/generated-request-constraints.type-test.ts",
    "src/**/*.test.{ts,tsx}"
  ],
  "project": ["src/**/*.{ts,tsx}"],
  // The complete generated API surface is an intentional published contract.
  "ignore": ["src/lib/api/generated/**"],
  // The generator is invoked by ../../scripts/generate-api-contract.mjs via its bin path.
  "ignoreDependencies": ["@hey-api/openapi-ts"]
}
```

- [ ] **Step 5: Implement the read-only PowerShell runner**

The script must:

1. resolve the repository root from `$PSScriptRoot`;
2. create only a timestamped directory beneath `.local/code-health`;
3. run `go test ./... -run '^$'` before analysis so package errors fail fast;
4. run deadcode for Windows and Linux, with and without `-test`, over `./cmd/... ./scripts/...`;
5. repeat deadcode from `tools/` and `hack/debug/` as separate modules when they contain main packages;
6. run `npx.cmd --yes knip@6.32.2 --config knip.jsonc --reporter json` from `web/listingkit-ui`;
7. write the resolved jscpd options to `$runDir/jscpd.json`, then run `npx.cmd --yes jscpd@5.0.14 --config $runDir/jscpd.json --reporters ai` over the configured clone paths;
8. write command, exit code, GOOS, test mode, and output path into `manifest.json`;
9. include `support_candidates`, populated from tracked executable files under `scripts`, `tools`, and `hack/debug`, in `manifest.json`;
10. write the repository-relative timestamped run directory to `.local/code-health/latest-run.txt` only after the manifest is complete;
11. exit non-zero on compilation or analyzer execution failure, while treating detector findings as report data rather than execution failure.

Use `System.Diagnostics.ProcessStartInfo.ArgumentList` rather than building shell command strings, so paths and arguments are not reinterpreted.

The public parameter block is:

```powershell
param(
    [ValidateSet("All", "Go", "Frontend", "Clones", "Verify")]
    [string]$Mode = "All",
    [switch]$ListOnly,
    [switch]$Summarize
)
```

- [ ] **Step 6: Guard bulk output and exclusions**

Add `.local/code-health/` to `.gitignore` if the existing `.local/` rule does not already cover it. Extend `TestCodeHealthAuditConfigPinsScopeAndTools` to reject clone exclusions equal to `internal/**`, `cmd/**`, `scripts/**`, `tools/**`, or `web/listingkit-ui/src/**`.

- [ ] **Step 7: Run the audit contract and list-only smoke**

Run:

```powershell
go test ./tests -run TestCodeHealthAuditConfigPinsScopeAndTools -count=1
pwsh -File scripts/code-health-audit.ps1 -ListOnly
```

Expected: PASS; list-only output names four root-module deadcode scans, nested-module scans, one Knip scan, and one jscpd scan without creating a source diff.

- [ ] **Step 8: Capture the pre-cleanup test and build baseline**

Run from the isolated worktree and save console output beneath the current `.local/code-health` run directory:

```powershell
go test ./... -count=1
go build ./cmd/product-listing-api ./cmd/listing-control-plane ./cmd/shein-listing ./cmd/temu-listing
Set-Location web/listingkit-ui
npm.cmd ci
npm.cmd run lint
npm.cmd run typecheck
npm.cmd test
npm.cmd run build
```

Expected: all commands pass before deletion. If a baseline command fails, record the exact failure in `manifest.json` and repair/rebase the isolated branch before beginning Task 3; no cleanup batch may use a broken baseline as its success reference.

- [ ] **Step 9: Commit the audit foundation**

```powershell
git add -- scripts/code-health-audit.config.json scripts/code-health-audit.ps1 tests/code_health_audit_test.go web/listingkit-ui/knip.jsonc .gitignore
git commit -m "chore: add reproducible code health audit"
```

### Task 3: Remove proven frontend orphan files and dependency declarations

**Files:**
- Delete: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-batch-task-tracker.tsx`
- Delete: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-review-note.tsx`
- Delete: `web/listingkit-ui/src/lib/sds/shipment-areas.ts`
- Delete: `web/listingkit-ui/src/lib/shein-studio/render-review-mockups.ts`
- Preserve as entry: `web/listingkit-ui/src/lib/api/generated-request-constraints.type-test.ts`
- Modify: `web/listingkit-ui/package.json`
- Modify: `web/listingkit-ui/package-lock.json`
- Modify: `web/listingkit-ui/pnpm-lock.yaml`

**Interfaces:**
- Consumes: Task 2 Knip graph with Next, Vitest, type-test, and generated-client configuration.
- Produces: no unused frontend file and no genuinely unused package dependency.

- [ ] **Step 1: Record exact pre-deletion evidence**

Run from `web/listingkit-ui`:

```powershell
npx.cmd --yes knip@6.32.2 --config knip.jsonc --include files,dependencies --reporter compact
rg -n "shein-batch-task-tracker|SheinBatchTaskTracker|shein-design-review-note|SheinDesignReviewNote|sdsShipmentAreaCandidates|buildReviewMockupFiles" src
```

Expected: the four files are reported unused and `rg` finds only their declarations. The generated request constraint file is not reported unused. `@tanstack/react-table` is unused; `@hey-api/openapi-ts` is retained because the generator script resolves its executable.

- [ ] **Step 2: Delete the four orphan files**

Use `apply_patch` deletion patches for the four exact paths. Do not delete the type-test file.

- [ ] **Step 3: Remove the unused table dependency and refresh both lockfiles**

Run:

```powershell
Set-Location web/listingkit-ui
npm.cmd uninstall @tanstack/react-table
pnpm.cmd install --lockfile-only
```

Expected: `@tanstack/react-table` disappears from `package.json`, `package-lock.json`, and `pnpm-lock.yaml`; `@hey-api/openapi-ts` remains in `devDependencies`.

- [ ] **Step 4: Verify the frontend batch**

Run:

```powershell
npx.cmd --yes knip@6.32.2 --config knip.jsonc --include files,dependencies --reporter compact
npm.cmd run lint
npm.cmd run typecheck
npm.cmd test
npm.cmd run build
```

Expected: Knip reports no unused files or dependencies; all four frontend gates pass.

- [ ] **Step 5: Commit the orphan cleanup**

```powershell
git add -- web/listingkit-ui/package.json web/listingkit-ui/package-lock.json web/listingkit-ui/pnpm-lock.yaml web/listingkit-ui/src/components/listingkit/shein-studio/shein-batch-task-tracker.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-review-note.tsx web/listingkit-ui/src/lib/sds/shipment-areas.ts web/listingkit-ui/src/lib/shein-studio/render-review-mockups.ts
git commit -m "refactor(ui): remove orphaned listingkit code"
```

### Task 4: Remove or privatize every unused handwritten frontend export

**Files:**
- Modify: handwritten files reported by Knip under `web/listingkit-ui/src/app`, `src/auth.config.ts`, `src/components`, and `src/lib`, excluding `src/lib/api/generated/**`.
- Test: colocated `*.test.ts` and `*.test.tsx` files for each affected component or helper.

**Interfaces:**
- Consumes: Knip JSON issue types `exports` and `types` after Task 3.
- Produces: no unused handwritten export or exported type; declarations used only within their file become private, and declarations with no local use are deleted.

- [ ] **Step 1: Save the exact handwritten export list**

Run:

```powershell
New-Item -ItemType Directory -Force .local/code-health/manual | Out-Null
npx.cmd --yes knip@6.32.2 --config knip.jsonc --include exports,types --reporter json *> ../../.local/code-health/manual/knip-exports-before.json
```

Expected: generated files are absent; handwritten findings include the current API schemas, UI variants, studio helpers, subscription helpers, and ListingKit types.

- [ ] **Step 2: Process findings one file at a time**

For each reported symbol `S` in file `F`:

```powershell
rg -n "\bS\b" web/listingkit-ui/src -g '*.ts' -g '*.tsx'
```

Apply exactly one outcome:

- more than one occurrence in `F`, no external occurrence: remove only `export`;
- declaration occurrence only: delete the complete declaration and its now-unused imports;
- referenced by a dynamic Next/Vitest/config entry missed by Knip: add that exact entry to `knip.jsonc` and keep the export;
- generated contract: leave source untouched; generated paths are already classified by configuration.

Do not use Knip `--fix` as the final change because it can leave stripped but still-dead declarations behind.

- [ ] **Step 3: Run focused tests after each ownership group**

Use these groups and commands:

```powershell
npm.cmd test -- src/components/listingkit
npm.cmd test -- src/lib/api
npm.cmd test -- src/lib/shein-studio
npm.cmd test -- src/lib/query
```

Expected: PASS after the matching group is edited.

- [ ] **Step 4: Prove the export graph is clean**

Run:

```powershell
npx.cmd --yes knip@6.32.2 --config knip.jsonc --include exports,types --reporter compact
npm.cmd run lint
npm.cmd run typecheck
npm.cmd test
npm.cmd run build
```

Expected: no handwritten unused exports/types and all frontend gates pass.

- [ ] **Step 5: Commit the handwritten export cleanup**

Stage only the paths listed by `git diff --name-only -- web/listingkit-ui/src web/listingkit-ui/knip.jsonc`, inspect the cached diff, then commit:

```powershell
git commit -m "refactor(ui): remove unused exports"
```

### Task 5: Remove high-confidence dead Go leaves from active packages

**Files:**
- Delete: `internal/listingkit/core/string_helpers.go`
- Delete when confirmed declaration-only by the Task 2 audit: `internal/app/consumer/task_submitter.go`
- Delete when confirmed declaration-only by the Task 2 audit: `internal/app/consumer/platform_processor_registry.go`
- Delete when confirmed declaration-only by the Task 2 audit: `internal/app/httpapi/listingkit_temporal_worker.go`
- Modify: tests that assert retired symbol text rather than supported behavior.
- Create: `docs/refactoring/code-health-decisions.md`

**Interfaces:**
- Consumes: intersection of Windows/Linux and production/test deadcode reports plus exact `rg` references.
- Produces: the first active-domain dead leaf batch and the decision-ledger format used by every later batch.

- [ ] **Step 1: Run the full audit on a compiling isolated baseline**

Run:

```powershell
pwsh -File scripts/code-health-audit.ps1 -Mode All
```

Expected: all manifest commands exit zero; reports exist for Windows/Linux with and without tests. If compilation fails, fix or rebase the isolated branch before analyzing; do not interpret partial output.

- [ ] **Step 2: Create the decision ledger with explicit states**

Create `docs/refactoring/code-health-decisions.md` with this table header and allowed decisions:

```markdown
# Code Health Decisions

| Finding | Owner | Evidence | Decision | Verification |
| --- | --- | --- | --- | --- |

Allowed decisions: `removed`, `reconnected-defect`, `retained-deferred`, `retained-generated-contract`, `retained-configuration-specific`, and `detector-limitation`.
```

Each retained entry must name its command/config/build-tag/deployment owner; `keep` without evidence is invalid.

- [ ] **Step 3: Prove the initial leaf candidates**

Run exact references for:

```powershell
rg -n "\b(joinStrings|formatInt)\b" internal/listingkit/core internal -g '*.go'
rg -n "\b(NewTaskSubmitter|TaskSubmitter)\b" cmd internal scripts tests -g '*.go'
rg -n "\b(PlatformProcessorRegistry|RegisterAllProcessors)\b" cmd internal scripts tests -g '*.go'
rg -n "\bRunListingKitTemporalWorker\b" cmd internal scripts tests deployments -g '*'
```

Expected: `internal/listingkit/core/string_helpers.go` contains declarations only. The other three files may be deleted only when both OS reports mark every function unreachable and exact references contain declarations/tests only.

- [ ] **Step 4: Delete proven leaves and repair test ownership**

Delete each proven file with `apply_patch`. If a boundary test checks only that a retired symbol's signature is absent/present, replace it with a test of the supported registration path; do not delete a behavioral invariant merely to make the batch pass.

- [ ] **Step 5: Verify affected packages and maintained commands**

Run:

```powershell
go test ./internal/app/consumer ./internal/app/httpapi ./internal/listingkit/... ./tests/... -count=1
go build ./cmd/product-listing-api ./cmd/listing-control-plane ./cmd/shein-listing ./cmd/temu-listing
pwsh -File scripts/code-health-audit.ps1 -Mode Go
```

Expected: PASS; removed symbols no longer appear in deadcode output; newly exposed leaves are added to the next owner batch rather than ignored.

- [ ] **Step 6: Record and commit the batch**

Add one ledger row per deleted file with both OS report paths and verification commands. Stage only affected Go/tests plus the ledger, then commit:

```powershell
git commit -m "refactor(go): remove unreachable active code"
```

### Task 6: Consolidate the canonical 1688 handoff HTTP implementation

**Files:**
- Move implementation into: `internal/product/sourcehandoff/a1688/httpapi/handler.go`
- Keep canonical module: `internal/product/sourcehandoff/a1688/httpapi/http_module.go`
- Keep canonical routes: `internal/product/sourcehandoff/a1688/httpapi/routes.go`
- Delete: `internal/productenrich/httpapi/sourcea1688/handler.go`
- Delete: `internal/productenrich/httpapi/sourcea1688/http_module.go`
- Delete: `internal/productenrich/httpapi/sourcea1688/routes.go`
- Modify: `internal/app/httpapi/composition_builder.go`
- Modify: `internal/app/httpapi/types.go`
- Modify: `tests/a1688_source_to_task_flow_test.go`
- Test: `internal/app/httpapi/http_module_test.go`

**Interfaces:**
- Consumes: `a1688.TaskCommandService` and the existing POST contract `/api/v1/product-sourcing/1688/listingkit/tasks`.
- Produces: one HTTP handler/module/route owner under `internal/product/sourcehandoff/a1688/httpapi`; no compatibility alias back to `productenrich`.

- [ ] **Step 1: Add an import-boundary regression**

Add a test in `tests/import_boundaries_test.go` that walks production `.go` files and fails if any import equals `task-processor/internal/productenrich/httpapi/sourcea1688`.

```go
func TestProductSourcingHTTPStaysUnderSourceHandoff(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "internal")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil { return walkErr }
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil { return err }
		if strings.Contains(string(content), `"task-processor/internal/productenrich/httpapi/sourcea1688"`) {
			t.Errorf("%s imports the duplicate 1688 HTTP owner", path)
		}
		return nil
	})
	if err != nil { t.Fatal(err) }
}
```

Run: `go test ./tests -run TestProductSourcingHTTPStaysUnderSourceHandoff -count=1`

Expected: FAIL because `internal/app/httpapi/composition_builder.go` and `types.go` import the old duplicate owner.

- [ ] **Step 2: Promote the real handler into the neutral handoff package**

Replace the alias-only `internal/product/sourcehandoff/a1688/httpapi/handler.go` with the complete implementation currently in `internal/productenrich/httpapi/sourcea1688/handler.go`. Preserve request/response JSON fields, verified tenant identity, legacy `source_store_id` rejection, store-access error mapping, and `a1688.CreateTaskCommand` construction unchanged.

- [ ] **Step 3: Rewire production and tests to the canonical package**

Change app composition/types and `tests/a1688_source_to_task_flow_test.go` imports to `task-processor/internal/product/sourcehandoff/a1688/httpapi`. Delete the three old `productenrich/httpapi/sourcea1688` files once `rg` shows no imports.

- [ ] **Step 4: Verify behavior and route uniqueness**

Run:

```powershell
go test ./internal/product/sourcehandoff/a1688/... ./internal/app/httpapi ./tests -run "1688|ProductSourcing|ImportBoundaries" -count=1
go test ./tests -run TestProductSourcingHTTPStaysUnderSourceHandoff -count=1
rg -n "productenrich/httpapi/sourcea1688" . -g '*.go'
```

Expected: tests PASS and `rg` returns no Go import.

- [ ] **Step 5: Commit the owner consolidation**

```powershell
git add -- internal/product/sourcehandoff/a1688/httpapi internal/productenrich/httpapi/sourcea1688 internal/app/httpapi/composition_builder.go internal/app/httpapi/types.go internal/app/httpapi/http_module_test.go tests/a1688_source_to_task_flow_test.go tests/import_boundaries_test.go
git commit -m "refactor: consolidate 1688 handoff HTTP owner"
```

### Task 7: Consolidate duplicated string formatting through standard and existing utility owners

**Files:**
- Create: `internal/pkg/strx/join.go`
- Create: `internal/pkg/strx/join_test.go`
- Delete: `internal/listingkit/string_helpers.go`
- Modify: `internal/marketplace/shein/workspace/helpers.go`
- Modify: ListingKit files that call `joinStrings` or `formatInt`
- Modify: marketplace SHEIN workspace files that call `joinStrings` or `formatInt`

**Interfaces:**
- Produces: `strx.JoinNonBlank(values []string, sep string) string`; integer formatting uses standard-library `strconv.Itoa` directly.

- [ ] **Step 1: Write characterization tests for the one shared string rule**

```go
func TestJoinNonBlankTrimsAndDropsBlankValues(t *testing.T) {
	t.Parallel()
	got := JoinNonBlank([]string{" alpha ", "", "  ", "beta"}, " > ")
	if got != "alpha > beta" { t.Fatalf("got %q", got) }
}

func TestJoinNonBlankKeepsDuplicateNonBlankValues(t *testing.T) {
	t.Parallel()
	got := JoinNonBlank([]string{"x", "x"}, ",")
	if got != "x,x" { t.Fatalf("got %q", got) }
}
```

- [ ] **Step 2: Run tests and verify the missing API**

Run: `go test ./internal/pkg/strx -run TestJoinNonBlank -count=1`

Expected: compile FAIL because `JoinNonBlank` is undefined.

- [ ] **Step 3: Implement the minimal shared utility**

```go
func JoinNonBlank(values []string, sep string) string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			filtered = append(filtered, value)
		}
	}
	return strings.Join(filtered, sep)
}
```

- [ ] **Step 4: Replace duplicates at both owners**

Replace `joinStrings` calls with `strx.JoinNonBlank`. Replace `formatInt(v)` with `strconv.Itoa(v)`. Delete `internal/listingkit/string_helpers.go` and remove the duplicate helper definitions from `internal/marketplace/shein/workspace/helpers.go`; retain unrelated helpers in that file.

- [ ] **Step 5: Verify the behavior and clone removal**

Run:

```powershell
go test ./internal/pkg/strx ./internal/listingkit ./internal/marketplace/shein/workspace -count=1
rg -n "func (joinStrings|formatInt)" internal/listingkit internal/marketplace/shein/workspace
npx.cmd --yes jscpd@5.0.14 --min-lines 12 --min-tokens 70 --format go --reporters ai internal/listingkit internal/marketplace/shein/workspace internal/pkg/strx
```

Expected: tests PASS, exact helper declarations are absent, and the former string-helper clone is absent.

- [ ] **Step 6: Commit the consolidation**

```powershell
git add -- internal/pkg/strx internal/listingkit internal/marketplace/shein/workspace
git commit -m "refactor: consolidate string formatting helpers"
```

### Task 8: Exhaust active Go dead-code candidates owner by owner

**Files:**
- Modify/Delete: files listed by the intersection of Task 2's four root-module deadcode reports under active owners.
- Modify: `docs/refactoring/code-health-decisions.md`
- Test: owning package tests for every batch.

**Interfaces:**
- Consumes: normalized keys assembled as `$packagePath + "." + $functionName + "@" + $file + ":" + $line` from Windows/Linux, test/no-test reports.
- Produces: zero unexplained high-confidence active-domain candidates.

- [ ] **Step 1: Normalize and intersect reports**

Use the audit script's `-Summarize` mode to write `deadcode-intersection.json` in the current timestamped audit directory, grouped in this exact order:

1. `internal/app` and `internal/platform*`;
2. `internal/listingkit`, `internal/listing`, `internal/listingadmin`;
3. `internal/product*`, `internal/catalog`, `internal/asset`;
4. `internal/shein`, `internal/marketplace/shein`, `internal/publishing/shein`;
5. crawler and integration adapters;
6. technical `internal/pkg` and `internal/infra`.

- [ ] **Step 2: Process one stable owner per commit**

For every candidate, run this loop against the normalized intersection report:

```powershell
$runDir = (Get-Content -Raw .local/code-health/latest-run.txt).Trim()
$candidates = Get-Content -Raw (Join-Path $runDir 'deadcode-intersection.json') | ConvertFrom-Json
foreach ($candidate in $candidates) {
    & rg -n "\b$([regex]::Escape($candidate.name))\b" cmd internal scripts tests deployments config -g '*'
    & git log -S $candidate.name --oneline --all -- $candidate.file
}
```

Then inspect interfaces, `init` registration, route descriptors, Temporal/activity registration, RabbitMQ handlers, config names, build constraints, and serialization tags in the owning package. Delete only after the evidence meets the design definition of dead code.

- [ ] **Step 3: Use the correct test cycle for each owner batch**

For pure unreachable deletion, take the distinct package values from the intersection report and test them directly:

```powershell
$ownerPackages = $candidates | Select-Object -ExpandProperty package -Unique
& go test @ownerPackages -count=1
go test ./tests/... -count=1
go build ./cmd/product-listing-api ./cmd/listing-control-plane ./cmd/shein-listing ./cmd/temu-listing
```

For consolidation or re-registration, first add a characterization/regression test that fails for the missing canonical behavior, then implement and run the same gates.

- [ ] **Step 4: Re-scan after every owner**

Run `pwsh -File scripts/code-health-audit.ps1 -Mode Go -Summarize`.

Expected: the owned package's high-confidence count strictly decreases. Add newly exposed leaf functions to the same owner batch until the count reaches zero or every remainder has a valid ledger decision.

- [ ] **Step 5: Commit each independently reviewable owner batch**

Derive `$owner` from the package's stable business owner and commit with `git commit -m "refactor($owner): remove unreachable code"`; for consolidation, use the concrete concept named in the ledger row. Stage only that owner, its focused tests, and ledger rows.

### Task 9: Classify and clean deferred and compatibility subgraphs

**Files:**
- Modify/Delete: candidates under `internal/amazon`, `internal/amazonlisting`, `internal/temu`, historical compatibility packages, and non-product runtime commands.
- Modify: `docs/refactoring/code-health-decisions.md`
- Modify when a subsystem is retired: matching `.github`, `deployments`, `scripts`, config, docs, and tests in the same retirement commit.

**Interfaces:**
- Consumes: README product authority, deployment/build references, command inventory, and complete subgraph reachability.
- Produces: one explicit outcome for every dormant cluster: removed, reconnected defect, or retained deferred/configuration-specific.

- [ ] **Step 1: Build one evidence bundle per dormant cluster**

For each cluster, record:

```text
runtime/build owner
production/deployment reference
incoming package imports
outgoing dependencies
test-only entrypoints
product maturity statement
deadcode candidate count by OS/test mode
```

Do not use the absence of a call from the four product runtimes as sufficient evidence because operational commands and deferred runtimes are separately maintained.

- [ ] **Step 2: Retire complete unsupported subgraphs atomically**

When authority documents and repository references prove retirement, delete implementation, command, config, dependencies, tests, Docker/Kubernetes assets, scripts, and stale documentation together. Add a repository-structure regression asserting the retired entrypoint stays absent.

- [ ] **Step 3: Reconnect accidentally unreachable supported code as a defect fix**

Add a failing test for the expected command/route/worker registration, wire the missing registration through the existing module boundary, and record `reconnected-defect` rather than deleting the capability.

- [ ] **Step 4: Retain deferred capability only with exact evidence**

Ledger entries must name the product-authority paragraph and concrete build/deployment/test owner. Exceptions apply to exact packages/files, never entire `internal/amazon/**` or `internal/temu/**` trees.

- [ ] **Step 5: Verify each subgraph decision**

Run the owning tests, all repository structure/import-boundary tests, maintained command builds, and the Go audit. Expected: no unclassified candidate remains in the subgraph.

### Task 10: Exhaust genuine clone candidates without creating false abstractions

**Files:**
- Modify: same-owner clone pairs from the jscpd report.
- Modify: `docs/refactoring/code-health-decisions.md`.
- Test: focused characterization tests beside each canonical owner.

**Interfaces:**
- Consumes: jscpd pairs above 12 lines/70 tokens after dead-code removal.
- Produces: every clone pair classified as consolidated or intentionally separate by ownership/semantics.

- [ ] **Step 1: Re-run jscpd after dead-code cleanup**

Run: `pwsh -File scripts/code-health-audit.ps1 -Mode Clones -Summarize`

Expected: `jscpd-pairs.json` in the current timestamped audit directory, sorted by shared owner and then by descending duplicated tokens.

- [ ] **Step 2: Process exact same-owner clones first**

Use this order:

1. duplicate functions in the same file;
2. duplicate repository/store methods under one package;
3. duplicate HTTP module builders under one feature;
4. duplicate persistence success/failure tails under one service;
5. compatibility mirrors where one canonical domain implementation already exists.

Write characterization tests that assert return values, ordering, error wrapping, tenant scoping, retry state, and persistence side effects before extraction.

- [ ] **Step 3: Reject cross-owner abstraction when semantics differ**

Record a ledger row when clones differ by marketplace policy, error taxonomy, tenant boundary, retry/transaction semantics, or runtime ownership. The evidence column must name the difference and the separate owners; line-count similarity alone is insufficient.

- [ ] **Step 4: Verify each consolidation**

Run focused tests, jscpd on the touched paths, package/import-boundary tests, and affected command builds. Expected: the target pair disappears without introducing a new cross-boundary import.

- [ ] **Step 5: Continue until every above-threshold pair is classified**

Completion for this task is an empty unclassified-pair list, not a globally zero jscpd clone count. Intentional independent implementations remain only with ledger evidence.

### Task 11: Clean tests, scripts, tools, and executable configuration

**Files:**
- Modify/Delete: dead test helpers and superseded tests identified after production cleanup.
- Modify/Delete: `scripts/**`, `tools/**`, and `hack/debug/**` findings from their own module/entry scans.
- Modify: `docs/refactoring/code-health-decisions.md`.

**Interfaces:**
- Consumes: test-enabled deadcode deltas, exact script references, Docker/workflow invocations, and nested module scans.
- Produces: no obsolete test-only helper, script, tool command, or executable registration.

- [ ] **Step 1: Identify test-only production helpers**

Subtract no-test deadcode keys from test-enabled keys. For code reached only by tests, either move it into `_test.go` when it is test support, preserve it when it validates an exported contract, or remove it with a superseded test. Run the owning package after each move.

- [ ] **Step 2: Audit every maintained script and tool entry**

For each candidate file recorded in the audit manifest:

```powershell
$runDir = (Get-Content -Raw .local/code-health/latest-run.txt).Trim()
$manifest = Get-Content -Raw (Join-Path $runDir 'manifest.json') | ConvertFrom-Json
foreach ($candidatePath in $manifest.support_candidates) {
    $name = Split-Path $candidatePath -Leaf
    & rg -n --fixed-strings $name .github deployments docs scripts tests Makefile README.md
    & git log --follow --oneline -- $candidatePath
}
```

Delete only when no workflow, runbook, deployment, operator procedure, or maintained tool imports it.

- [ ] **Step 3: Consolidate duplicate shell logic only within matching semantics**

Before extracting shared PowerShell or shell logic, test parameter parsing, exit codes, redaction, temp-file cleanup, and platform behavior. Keep Bash and PowerShell implementations separate when their runtime environments differ.

- [ ] **Step 4: Run all support-code tests**

Run:

```powershell
pwsh -File scripts/test-all.ps1 -count=1
go test ./tests/... -count=1
Push-Location tools; go test ./... -count=1; Pop-Location
Push-Location hack/debug; go test ./... -count=1; Pop-Location
```

Run existing Pester and Bash driver tests for every changed script.

### Task 12: Tidy dependencies and ratchet prevention gates

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: frontend package/lock files when later cleanup removes dependencies.
- Modify: `.github/workflows/ci.yml`
- Modify: `Makefile`
- Modify: `docs/refactoring/code-health-decisions.md`

**Interfaces:**
- Consumes: final classified analyzer baseline.
- Produces: reproducible local targets and CI gates that reject new unexplained findings without failing on classified legacy/configuration-specific entries.

- [ ] **Step 1: Tidy every module and verify no accidental upgrade**

Run:

```powershell
go mod tidy
Push-Location tools; go mod tidy; Pop-Location
Push-Location hack/debug; go mod tidy; Pop-Location
Set-Location web/listingkit-ui
npm.cmd install --package-lock-only
pnpm.cmd install --lockfile-only
```

Inspect manifest diffs; dependency version upgrades unrelated to removal are reverted by restoring their original constraints, not accepted opportunistically.

- [ ] **Step 2: Add read-only Make targets**

Add `code-health-audit` and `code-health-verify` targets. `audit` writes `.local` reports; `verify` compares normalized findings to exact ledger classifications and fails on new/unclassified findings.

- [ ] **Step 3: Add a CI code-health job**

The job installs Node/Go using existing workflow versions, runs `go test ./... -run '^$'`, runs `pwsh scripts/code-health-audit.ps1 -Mode Verify`, and uploads `.local/code-health` only on failure. It must not use an allowlist wildcard for an entire production tree.

- [ ] **Step 4: Verify contract generation and repository boundaries**

Run:

```powershell
node scripts/generate-api-contract.mjs
git diff --exit-code -- docs/api/listingkit-asset.openapi.yaml web/listingkit-ui/src/lib/api/generated
go test ./tests/... -count=1
```

Expected: no generated drift and all boundary/structure tests pass.

- [ ] **Step 5: Commit dependency and prevention closeout**

```powershell
git add -- go.mod go.sum tools/go.mod tools/go.sum hack/debug/go.mod hack/debug/go.sum web/listingkit-ui/package.json web/listingkit-ui/package-lock.json web/listingkit-ui/pnpm-lock.yaml .github/workflows/ci.yml Makefile docs/refactoring/code-health-decisions.md
git commit -m "chore: enforce dead and duplicate code baseline"
```

### Task 13: Final completion audit

**Files:**
- Modify only if evidence requires correction: `docs/refactoring/code-health-decisions.md`.

**Interfaces:**
- Consumes: exact final commit and all approved design requirements.
- Produces: current-state evidence sufficient to mark the persistent goal complete.

- [ ] **Step 1: Prove the exact worktree and commit scope**

Run:

```powershell
git status --short --branch
git log --oneline --decorate -15
git diff --check HEAD~1..HEAD
```

Expected: clean worktree; no primary-checkout child-retry files or unrelated paths appear in cleanup commits.

- [ ] **Step 2: Run the complete Go verification**

```powershell
go test ./... -count=1
go test -race ./internal/app/runtime/listingcontrol -run TestControlPlaneService -count=1
go test -race ./internal/listingadmin -run "TestConcurrentClaimForDispatchOnlyOneWorkerWins|TestConcurrentRollbackDispatchOnlyOriginalQueuedClaimIsRestoredOnce|TestConcurrentRecoveryOnlyUpdatesStillEligibleRowsOnce" -count=1
go build ./cmd/product-listing-api ./cmd/listing-control-plane ./cmd/shein-listing ./cmd/temu-listing
```

On Windows, run race tests in Linux CI if the local toolchain cannot support `-race`; do not claim that gate passed until CI output is visible.

- [ ] **Step 3: Run the complete frontend verification**

```powershell
Set-Location web/listingkit-ui
npm.cmd ci
npm.cmd run lint
npm.cmd run typecheck
npm.cmd test
npm.cmd run build
```

Expected: every command exits zero.

- [ ] **Step 4: Run the final analyzer audit**

```powershell
Set-Location ../..
pwsh -File scripts/code-health-audit.ps1 -Mode Verify -Summarize
```

Expected:

- no unclassified high-confidence dead-code candidate;
- no unused frontend file/dependency/handwritten export/type;
- no unclassified above-threshold clone pair;
- every retained item maps to one exact ledger row and concrete owner evidence;
- all analyzer commands in the manifest exited zero.

- [ ] **Step 5: Audit every design deliverable**

Confirm current files prove: pinned reusable configuration, small owner-scoped commits, a complete retained-finding/dormant-subsystem ledger, generator-controlled output, dependency cleanup, final before/after counts, and external gate evidence.

- [ ] **Step 6: Mark the persistent goal complete only after all evidence passes**

If any candidate, clone pair, test, build, CI race gate, or ledger classification remains missing or indirect, keep the goal active and continue the owning task. Only call `update_goal(status="complete")` after this checklist is fully proven.

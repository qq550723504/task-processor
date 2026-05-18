# ListingKit Boundary Convergence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Shrink `internal/listingkit` back to a stable orchestration facade and move remaining domain-heavy workflow, workspace, and SHEIN-specific behavior behind clearer package boundaries.

**Architecture:** Keep `internal/listingkit` as the API-facing shell, service facade, and orchestration entrypoint. Keep SHEIN publishing rules in `internal/publishing/shein`, keep SHEIN client workspace rules in `internal/workspace/shein`, and make `internal/listingkit/workflow` and `internal/listingkit/workspace` the true homes for reusable subdomain orchestration instead of leaving most logic in the root facade package.

**Tech Stack:** Go, package-level boundary tests, existing service/facade layering in `internal/listingkit`, existing architecture note in `docs/architecture/listingkit-refactor-status.md`

---

## Boundary Map

### Current stable ownership

- `internal/catalog`
  Owns canonical product facts and normalized product data.
- `internal/asset`
  Owns reusable asset bundle and generated visual inventory abstractions.
- `internal/publishing/shein`
  Owns canonical-to-SHEIN package assembly, category resolution, attribute resolution, sale-attribute resolution, preview-product adaptation, and submission contract building.
- `internal/workspace/shein`
  Owns SHEIN inspection, overview, readiness, repair, editor context, revision diff, restore preview, validation payloads, and success payloads.
- `internal/listingkit`
  Should own task lifecycle, repository/service facade, preview/export aggregation, API-facing request/result shells, and orchestration glue only.

### Current fuzzy ownership

- `internal/listingkit/workflow.go`
  Owns the main orchestration flow but still lives in the root facade package instead of `internal/listingkit/workflow`.
- `internal/listingkit/service.go`
  Service wiring mixes stable facade concerns with many platform-specific dependencies and defaulting rules.
- `internal/listingkit/preview_builder.go`
  High-density aggregation hub and a likely place for accidental workspace rule leakage.
- `internal/listingkit/service_revision.go`, `service_history.go`, related `revision_*` files
  Acceptable as facade glue today, but easy to contaminate with workspace/domain decisions.
- `internal/listingkit/shein_*` helper files
  Some are intentionally thin bridges; some are at risk of becoming shadow homes for SHEIN business logic.
- `internal/listingkit/workflow_sds_sync.go`
  Already imports `internal/listingkit/workflow`, which shows the intended split exists but is only partially applied.

## Refactor Principles

- Do not move code just because a file name looks wrong; move code only when the receiving package can clearly own the rule long-term.
- Preserve the current API surface of `internal/listingkit` while shrinking its internals.
- Prefer extending existing bridge/support files over adding new root-level `shein_*` files.
- When a rule is about user-visible SHEIN editing or revision UX, it belongs in `internal/workspace/shein`.
- When a rule is about canonical data becoming a SHEIN publishable package, it belongs in `internal/publishing/shein`.
- When a rule is about cross-service sequencing, retries, child task state, or result assembly, it belongs in workflow/orchestration.

## Recommended Migration Order

1. Make `internal/listingkit/workflow` a real package for orchestration helpers and policy.
2. Freeze root-level SHEIN bridge files and prevent new domain logic from entering them.
3. Audit revision/history files and move any hidden workspace rule logic into `internal/workspace/shein`.
4. Reduce `preview_builder.go` into facade composition plus helpers owned by the right package.
5. After SHEIN is stable, decide whether TEMU should follow the same publishing/workspace split.

### Task 1: Lock the boundary contract in docs and tests

**Files:**
- Modify: `docs/architecture/listingkit-refactor-status.md`
- Modify: `tests/import_boundaries_test.go`
- Test: `tests/import_boundaries_test.go`

- [ ] **Step 1: Update the architecture note with the current target split**

Add a short section under `## Recommended Rules For Next Changes` that says:

```md
## Phase-2 Boundary Goal

- `internal/listingkit/workflow` becomes the implementation home for task orchestration helpers and policies.
- `internal/listingkit/workspace` becomes the implementation home for facade-level workspace composition that is not SHEIN-domain-specific.
- Root `internal/listingkit` stays as compatibility facade and service entrypoint.
- Root `internal/listingkit` must not gain new business rules that can live in `publishing/shein` or `workspace/shein`.
```

- [ ] **Step 2: Add boundary tests for root-level drift**

Extend `tests/import_boundaries_test.go` with a new test shaped like:

```go
func TestListingKitRootSheinHelpersStayFacadeOnly(t *testing.T) {
	assertNoBannedImports(t, filepath.Join("..", "internal", "listingkit"), []string{
		`"task-processor/internal/shein/api"`,
	}, map[string]struct{}{})
}
```

Then add a file-name allowlist or a selector-based assertion only if needed to avoid blocking legitimate facade imports.

- [ ] **Step 3: Run the boundary tests**

Run: `go test ./tests -run "TestListingKit|TestSheinPublishing|TestPublishingCommon|TestCatalog|TestCanonicalTypes" -count=1`

Expected: PASS. If a new test fails immediately, tighten its scope until it only encodes the intended architectural rule.

- [ ] **Step 4: Commit**

```bash
git add docs/architecture/listingkit-refactor-status.md tests/import_boundaries_test.go
git commit -m "docs: lock listingkit phase2 boundaries"
```

### Task 2: Promote `internal/listingkit/workflow` from placeholder to implementation package

**Files:**
- Modify: `internal/listingkit/workflow/doc.go`
- Modify: `internal/listingkit/workflow/sds_sync_policy.go`
- Modify: `internal/listingkit/workflow.go`
- Modify: `internal/listingkit/workflow_sds_sync.go`
- Test: `internal/listingkit/workflow/sds_sync_policy_test.go`
- Test: `internal/listingkit/service_generation_test.go`

- [ ] **Step 1: Identify pure orchestration helpers already suitable for extraction**

Start with functions that are rule/policy oriented and have no dependency on `service` fields. Candidate examples:

```text
SDSDesignSyncTimeoutForVariantCount
shouldGenerateAssets
shouldProcessImages
applySDSSyncMetadataToCanonical
resolveRecipesForPlatforms
```

Do not move functions that need package-private root models unless you first define a stable helper interface in `internal/listingkit/workflow`.

- [ ] **Step 2: Move one helper cluster at a time**

For the first extraction, keep it minimal. Example shape:

```go
package workflow

func ShouldGenerateAssets(req *listingkit.GenerateRequest) bool {
	// existing logic moved unchanged
}
```

If importing `listingkit` from `workflow` would violate the current direction, first extract a smaller request/options type in `workflow` and adapt in the root package.

- [ ] **Step 3: Replace root calls with package calls**

Update root package call sites in `internal/listingkit/workflow.go` and `internal/listingkit/workflow_sds_sync.go` so the root package orchestrates but defers policy logic:

```go
enableAssetGeneration := listingworkflow.ShouldGenerateAssets(task.Request)
```

- [ ] **Step 4: Keep tests at the policy package boundary**

Add focused tests in `internal/listingkit/workflow/sds_sync_policy_test.go` or new sibling test files, for example:

```go
func TestShouldGenerateAssets_DefaultsToTrueForGenerationPlatforms(t *testing.T) {}
func TestSDSDesignSyncTimeoutForVariantCount_IncreasesForLargeVariants(t *testing.T) {}
```

Keep root-level service tests only for end-to-end orchestration coverage.

- [ ] **Step 5: Run package and service tests**

Run: `go test ./internal/listingkit/workflow ./internal/listingkit -run "TestSDS|TestShould|TestServiceGeneration" -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/workflow internal/listingkit/workflow.go internal/listingkit/workflow_sds_sync.go
git commit -m "refactor: move listingkit workflow policies into subpackage"
```

### Task 3: Freeze SHEIN bridge files as adapters only

**Files:**
- Modify: `internal/listingkit/shein_workspace_types_bridge.go`
- Modify: `internal/listingkit/shein_workspace_editor_bridge.go`
- Modify: `internal/listingkit/shein_workspace_submit_bridge.go`
- Modify: `internal/listingkit/shein_workspace_repair_bridge.go`
- Modify: `internal/listingkit/shein_workspace_revision_bridge.go`
- Modify: `internal/listingkit/shein_workspace_inspection_bridge.go`
- Test: `tests/import_boundaries_test.go`

- [ ] **Step 1: Add adapter-only comments to each bridge file**

At the top of each file, add a short comment like:

```go
// Adapter-only bridge. Keep domain rules in internal/workspace/shein.
```

- [ ] **Step 2: Audit for hidden rule logic**

If a bridge file contains branching beyond trivial mapping, move that branch into `internal/workspace/shein` or `internal/publishing/shein`. Acceptable bridge code looks like:

```go
func buildSheinEditorContext(pkg *sheinpub.Package) *SheinEditorContext {
	return sheinworkspace.BuildEditorContext(pkg)
}
```

Risky bridge code looks like:

```go
func buildSomething(...) *Type {
	if x && y && z {
		// business rule tree
	}
}
```

- [ ] **Step 3: Add a naming rule to tests if drift continues**

If root-level drift keeps happening, add a test convention around bridge files, for example a file allowlist that documents the accepted root-level SHEIN adapter files.

- [ ] **Step 4: Run targeted tests**

Run: `go test ./tests ./internal/listingkit -run "TestListingKit|TestRevision|TestShein" -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/shein_* tests/import_boundaries_test.go
git commit -m "refactor: mark shein bridge files as adapter-only"
```

### Task 4: Separate facade-level revision/history composition from workspace rules

**Files:**
- Modify: `internal/listingkit/revision_workspace_bridge.go`
- Modify: `internal/listingkit/service_revision.go`
- Modify: `internal/listingkit/service_history.go`
- Modify: `internal/workspace/shein/*.go` for any moved rule logic
- Test: `internal/listingkit/service_revision_test.go`
- Test: `internal/listingkit/service_history_detail_test.go`
- Test: `internal/workspace/shein/*_test.go`

- [ ] **Step 1: Find logic that is more than response composition**

Search for files with both `sheinworkspace` imports and business branching:

```bash
rg -n "sheinworkspace|switch |if " internal/listingkit/service_revision.go internal/listingkit/service_history.go internal/listingkit/revision_workspace_bridge.go
```

Mark each branch as one of:

- facade composition
- workspace rule
- publishing rule

- [ ] **Step 2: Move workspace rules to `internal/workspace/shein`**

Typical examples to move:

- restore safety heuristics
- recommended view decisions
- revision success/relation wording rules

Preferred target shape:

```go
package shein

func BuildHistoryRestorePresentationData(...) *HistoryRestorePresentationData {
	// owns the rule here
}
```

Then keep the root bridge as a simple delegate.

- [ ] **Step 3: Keep root services thin**

After the move, root service methods should look like:

```go
detail := sheinworkspace.BuildHistoryRestoreDetailData(...)
return adaptHistoryDetail(detail), nil
```

Avoid reintroducing domain-specific `if` trees after the delegate call.

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/workspace/shein ./internal/listingkit -run "TestHistory|TestRevision|TestRestore" -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/revision_workspace_bridge.go internal/listingkit/service_revision.go internal/listingkit/service_history.go internal/workspace/shein
git commit -m "refactor: move revision workspace rules behind shein workspace package"
```

### Task 5: Split preview aggregation from workspace behavior

**Files:**
- Modify: `internal/listingkit/preview_builder.go`
- Modify: `internal/listingkit/preview_builder_shein.go`
- Modify: `internal/listingkit/preview_builder_platforms.go`
- Modify: `internal/workspace/shein/*.go` when preview behavior is really workspace logic
- Test: `internal/listingkit/preview_builder_test.go`

- [ ] **Step 1: Inventory the responsibilities in `preview_builder.go`**

Create a short checklist in the PR description or working notes:

- response assembly
- platform dispatch
- SHEIN workspace presentation rules
- asset preview rules
- revision/review wording rules

Only the first two should remain central in the root builder.

- [ ] **Step 2: Move workspace-specific sections out**

If a block answers a question like "what should the user see/edit/review in SHEIN workspace," move it behind `internal/workspace/shein`.

If a block answers "how do we aggregate already computed results into one API payload," keep it in `listingkit`.

- [ ] **Step 3: Keep builder composition explicit**

Target shape:

```go
func (b *previewBuilder) Build(...) *Preview {
	return &Preview{
		Shein: buildSheinPreview(...),
		// composition only
	}
}
```

Where `buildSheinPreview(...)` itself mostly delegates to `workspace/shein` and `publishing/shein`.

- [ ] **Step 4: Run preview tests**

Run: `go test ./internal/listingkit -run "TestPreview|TestBuildPreview" -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/preview_builder*.go internal/workspace/shein
git commit -m "refactor: reduce listingkit preview builder density"
```

### Task 6: Decide whether to replicate the pattern for TEMU or stop structural churn

**Files:**
- Modify: `docs/architecture/listingkit-refactor-status.md`
- Optional Modify: `internal/listingkit/temu_mapper.go`
- Optional Modify: `internal/listingkit/walmart_mapper.go`

- [ ] **Step 1: Make the decision explicit**

Pick one:

- freeze SHEIN structure and stop refactoring
- start `publishing/temu` and `workspace/temu`
- create an image-asset-focused layer before more platform splits

- [ ] **Step 2: Document the decision**

Add one section to the architecture note:

```md
## Post-SHEIN Direction

Decision: [chosen option]
Reason: [one paragraph]
Non-goals: [what we are intentionally not restructuring now]
```

- [ ] **Step 3: Commit**

```bash
git add docs/architecture/listingkit-refactor-status.md
git commit -m "docs: record post-shein architecture direction"
```

## Execution Order

Execute tasks in this order:

1. Task 1
2. Task 2
3. Task 3
4. Task 4
5. Task 5
6. Task 6

## Risks To Watch

- Import cycles when extracting code from root `listingkit` into `internal/listingkit/workflow`
- Accidentally moving API shell types into subpackages and breaking persisted task JSON compatibility
- Bridge files silently regrowing business rules because they look convenient
- Over-refactoring stable SHEIN code that already has a canonical home in `internal/workspace/shein` or `internal/publishing/shein`
- Spreading platform-specific defaults into generic workflow policy

## Success Criteria

- Root `internal/listingkit` becomes visibly smaller in policy density, not just file count.
- New SHEIN behavior lands in `internal/workspace/shein` or `internal/publishing/shein` by default.
- `internal/listingkit/workflow` contains reusable orchestration policy code instead of only placeholders.
- Boundary tests catch regressions before root-facade drift returns.
- `docs/architecture/listingkit-refactor-status.md` stays aligned with actual package ownership.

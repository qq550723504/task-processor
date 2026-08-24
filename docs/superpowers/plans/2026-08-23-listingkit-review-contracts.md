# ListingKit Review Contract Prevention Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent repeated 1688/marketplace review regressions by centralizing generation base-kind policy, canonical readable asset resolution, and platform-scoped dispatch.

**Architecture:** Add a low-level asset policy package for reusable base selection rules. Add a product-image readable-source resolver that ranks durable local/published/uploaded/current URLs ahead of provenance-only source URLs, and make publishers expose a common published URL contract. Route the production platform dispatch phase through the existing platform-aware dispatcher so no workflow path can bypass target isolation.

**Tech Stack:** Go, existing `asset`, `productimage`, and `listingkit` packages; Go unit and boundary tests; GitHub PR review threads.

**Spec:** The approved in-chat root-cause design: generation candidates must use one shared policy; rendered inputs must use a current readable copy while retaining original provenance separately; all production dispatch must use platform-scoped inventory.

## Global Constraints

- Preserve original 1688 provenance for audit metadata, but never let it outrank a current processed/published/uploaded readable asset.
- Keep platform inventory isolated for multi-target tasks and preserve the existing single-platform compatibility fallback.
- Do not cap image URLs in this change; unrelated listing image-count behavior must remain unchanged.
- Add regression tests before production changes and run focused plus full Go verification before claiming completion.
- Keep the PR Draft; do not merge or deploy.

---

### Task 1: Centralize generation base-kind policy

**Files:**
- Create: `internal/asset/policy/base_selection.go`
- Modify: `internal/asset/generation/deferred_executor.go`
- Modify: `internal/asset/generation/native_executor.go`
- Modify: `internal/asset/bundle/builder.go`
- Test: existing generation and bundle tests covering source candidate selection

**Interfaces:**
- Produces `policy.DeferredBaseKinds(kind asset.Kind) []asset.Kind` and `policy.CandidateSourceKinds() []asset.Kind`.
- Consumers must stop defining independent ordered candidate lists for the same selection contract.

- [ ] **Step 1: Write the failing scene-clean-base regression test**

Add a test that creates a scene task whose only `SourceAssetIDs` resolve to a `clean_image`, then asserts the deferred planner selects that clean asset instead of rejecting it or falling back to an unrelated source.

- [ ] **Step 2: Run the focused test and verify it fails for the missing policy**

Run `go test ./internal/asset/generation -run 'Scene.*Clean|Clean.*Scene' -count=1` from the feature worktree. The failure must show that scene preferences omit `KindCleanImage`.

- [ ] **Step 3: Add the shared policy and replace duplicated lists**

Implement the ordered scene preference as `Scene, Gallery, Clean, Main, Subject, Source`, preserve the existing orders for other deferred kinds, and use `CandidateSourceKinds` from both native generation and bundle candidate discovery.

- [ ] **Step 4: Run focused generation and bundle tests**

Run `go test ./internal/asset/generation ./internal/asset/bundle -count=1` and confirm all pass.

- [ ] **Step 5: Commit the policy slice**

Commit with `git add internal/asset/policy internal/asset/generation internal/asset/bundle && git commit -m "refactor: centralize asset base selection policy"`.

### Task 2: Make readable asset resolution a shared contract

**Files:**
- Create: `internal/productimage/readable_source.go`
- Modify: `internal/productimage/real_components.go`
- Modify: `internal/productimage/asset_publisher.go`
- Modify: `internal/asset/generation/pipeline_executor.go`
- Modify: `internal/asset/generation/productimage_renderer.go`
- Test: `internal/asset/generation/planner_test.go` and product-image publisher/component tests

**Interfaces:**
- Produces `productimage.ResolveReadableAssetSource(*ImageAsset) (ReadableAssetSource, error)`.
- Resolution order is durable local path, published path, published URL, uploaded URL, current asset URL, then provenance/source URL.
- `source_url` remains provenance metadata and is not used ahead of current readable outputs.

- [ ] **Step 1: Write the failing Amazon-readable-source regression test**

Add a renderer-input test with `source_url` pointing to the original 1688 image and `uploaded_url`/`URL` pointing to the Amazon-uploaded image. Assert the renderer receives the uploaded/current readable URL and not the 1688 provenance URL.

- [ ] **Step 2: Run the focused test and verify it fails**

Run `go test ./internal/asset/generation -run 'Amazon.*Readable|Uploaded.*Source|Published.*Source' -count=1`. The current implementation should expose the original `source_url` to the renderer.

- [ ] **Step 3: Implement the resolver and preserve provenance separately**

Add the resolver and make `loadAssetBytes` consume it. Update conversion from `asset.AssetRecord` so the renderer input carries the current readable source while generated record metadata still records original provenance. Ensure publication metadata is scrubbed only after the readable source has been promoted into neutral input fields.

- [ ] **Step 4: Standardize Amazon publication metadata**

When Amazon upload succeeds, set `published_url` alongside the existing `uploaded_url` and `asset.URL`, so all publishers provide the same canonical published URL field.

- [ ] **Step 5: Run focused product-image and generation tests**

Run `go test ./internal/productimage ./internal/asset/generation -count=1` and confirm both the new regression and existing publication lifecycle tests pass.

- [ ] **Step 6: Commit the readable-source slice**

Commit with `git add internal/productimage internal/asset/generation && git commit -m "fix: preserve readable published image sources"`.

### Task 3: Close the production dispatch bypass

**Files:**
- Modify: `internal/listingkit/workflow_platform_asset_dispatch_phase.go`
- Modify: relevant `internal/listingkit/*asset*test.go` and boundary tests

**Interfaces:**
- The platform dispatch phase must call `dispatchGenerationTasksByPlatform` exactly once for its pending tasks.
- No production ListingKit workflow phase may directly call `assetGenerator.Dispatch` when tasks can target multiple platforms.

- [ ] **Step 1: Add a production-phase regression/boundary assertion**

Extend the platform dispatch phase test or source boundary test to require the platform-aware helper and reject a direct generator dispatch call.

- [ ] **Step 2: Run the focused phase test and verify it fails against the current bypass**

Run the relevant `go test ./internal/listingkit -run 'PlatformAssetDispatch|AssetDispatch.*Boundary' -count=1` test. It must fail because the phase still invokes `assetGenerator.Dispatch` directly.

- [ ] **Step 3: Route the phase through the shared helper**

Replace the direct call with `dispatchGenerationTasksByPlatform`, passing the final listing result, platform-specific inventory derivation, and pending tasks. Keep existing mutation and error handling unchanged.

- [ ] **Step 4: Run focused ListingKit tests**

Run `go test ./internal/listingkit -run 'PlatformAssetDispatch|TaskGeneration|AssetGeneration' -count=1`.

- [ ] **Step 5: Commit the dispatch slice**

Commit with `git add internal/listingkit && git commit -m "fix: enforce platform-scoped asset dispatch"`.

### Task 4: Full verification and PR review closure

**Files:**
- No further production files unless verification finds a regression.

- [ ] **Step 1: Run repository verification**

Run `go test ./...`, `go vet ./...`, and the repository’s applicable build/check commands. Inspect `git diff`, `git status`, and the final commit range for unrelated changes.

- [ ] **Step 2: Push the feature branch**

Push `codex/fix-listingkit-image-url-projection` to the existing Draft PR #189 without merging or deploying.

- [ ] **Step 3: Reply to and resolve the two review threads**

Reply in the original threads with the exact regression test and invariant fixed, then resolve only after the pushed commit and focused tests verify the fix.

- [ ] **Step 4: Re-query PR comments and CI**

Confirm no unresolved review threads remain, the PR is still Draft, and required CI checks are green or explicitly pending. If a new comment identifies a genuinely uncovered contract, extend the policy/test rather than patching only its call site.

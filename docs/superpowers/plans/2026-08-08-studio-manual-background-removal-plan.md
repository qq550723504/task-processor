# Studio Manual Background Removal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow every editable Studio design to run background removal without regeneration and show its ordinary source image beside the transparent result.

**Architecture:** Reuse the existing batch background-removal endpoint and broaden only the explicitly requested-design path. A requested design uses its persisted `original_image_url`, or snapshots its current `image_url` before the first removal; the existing persisted status/result/error fields remain the source of truth. The review card renders two labeled previews from the same normalized URL helpers, while the existing lightbox keeps its enlarged original/final toggle.

**Tech Stack:** Go service/repository layer, Gin route already registered, React/Next.js client components, TypeScript, Vitest, Testing Library, Go `testing` package.

## Global Constraints

- Do not regenerate the design; manual removal only processes the current design image.
- Repeated removal always reads the persisted original image, never the previously removed result.
- On failure, preserve the original image and expose the failure status/error; never report the ordinary image as a successful transparent result.
- Reuse the existing `POST /api/v1/listing-kits/studio/batches/:batch_id/designs/background-removal/retry` route; do not add a duplicate endpoint.
- Keep read-only review, regeneration, approval, task creation, and product-image flows unchanged.
- Use the existing upload URL normalization and thumbnail proxy helpers.

---

### Task 1: Broaden manual background-removal service behavior

**Files:**
- Modify: `internal/listingkit/task_studio_batch_service.go:146-245`
- Test: `internal/listingkit/studio_background_removal_retry_test.go`

**Interfaces:**
- Consumes: `RetryStudioBatchDesignBackgroundRemovalRequest{DesignIDs []string}`, `StudioMaterializedDesignRecord`, and the injected `retryBackgroundRemoval(ctx, sourceURL, filename)` function.
- Produces: `RetryStudioBatchDesignBackgroundRemoval` accepts an explicitly requested design in any current transparency mode and returns a detail whose design contains the persisted original URL, removal status, result URL, model, and error metadata.

- [ ] **Step 1: Add a failing test for first-time manual removal.**

  Add `TestRetryStudioBatchDesignBackgroundRemovalSnapshotsCurrentImageForManualRequest` to `internal/listingkit/studio_background_removal_retry_test.go`. Seed a design with:

  ```go
  ImageURL: "https://cdn.example.test/ordinary.png",
  TransparentBackgroundMode: StudioTransparencyModeNone,
  BackgroundRemovalStatus: StudioBackgroundRemovalStatusNotRequested,
  OriginalImageURL: "",
  ```

  Inject a remover that records its source and returns `https://cdn.example.test/removed.png`. Call:

  ```go
  svc.RetryStudioBatchDesignBackgroundRemoval(
      ctx,
      "batch-1",
      &RetryStudioBatchDesignBackgroundRemovalRequest{DesignIDs: []string{"design-1"}},
  )
  ```

  Assert that the remover received `ordinary.png`, and the returned design has `OriginalImageURL == ordinary.png`, `ImageURL == removed.png`, mode `removal`, and status `succeeded`.

- [ ] **Step 2: Run the focused test and verify the expected failure.**

  Run:

  ```powershell
  go test ./internal/listingkit -run TestRetryStudioBatchDesignBackgroundRemovalSnapshotsCurrentImageForManualRequest -count=1
  ```

  Expected: `FAIL` because the current service rejects a requested design whose mode is not `removal` and whose original URL is empty.

- [ ] **Step 3: Implement the minimal requested-design source and eligibility change.**

  In `RetryStudioBatchDesignBackgroundRemoval`, distinguish explicitly requested designs from the existing no-request bulk retry behavior:

  - For an explicit `design_ids` request, accept any design with a non-empty `image_url` or `original_image_url`, including already succeeded designs so “重新抠图” is genuinely repeatable.
  - Choose `original_image_url` when present; otherwise choose `image_url` and assign it to `original_image_url` before the pending update.
  - Keep the existing mode/status eligibility filter for an empty `design_ids` request so background batch recovery does not start reprocessing every succeeded design.
  - Persist `transparent_background_mode = removal`, `original_image_url`, and pending status before invoking the remover.
  - Preserve the existing success and failure update behavior, including failure fallback to the chosen source URL.
  - Return the same validation error style for missing source URLs or unknown requested IDs.

- [ ] **Step 4: Run the focused test and verify it passes.**

  Run the same `go test` command from Step 2. Expected: `PASS`.

- [ ] **Step 5: Add the regression test for reprocessing from the persisted original.**

  Keep the existing `TestRetryStudioBatchDesignBackgroundRemovalUsesPersistedOriginalOnly` and extend it to request a design whose current `ImageURL` is already a prior transparent result. Assert the remover still receives `OriginalImageURL`, and the result keeps that original URL.

- [ ] **Step 6: Run all background-removal service tests.**

  Run:

  ```powershell
  go test ./internal/listingkit -run 'BackgroundRemoval|background removal' -count=1
  ```

  Expected: all matching tests pass, including existing failure fallback coverage.

- [ ] **Step 7: Commit the backend change.**

  ```powershell
  git add -- internal/listingkit/task_studio_batch_service.go internal/listingkit/studio_background_removal_retry_test.go
  git diff --cached --check
  git commit -m "feat: allow manual studio background removal"
  ```

### Task 2: Add reusable original/final preview source helpers

**Files:**
- Modify: `web/listingkit-ui/src/lib/shein-studio/design-image.ts`
- Test: `web/listingkit-ui/src/lib/shein-studio/design-image.test.ts`

**Interfaces:**
- Consumes: `SheinStudioGeneratedDesign.imageUrl`, `dataUrl`, and optional `originalImageUrl`.
- Produces: `resolveGeneratedDesignOriginalSrc(design)` for the ordinary source image and `resolveGeneratedDesignFinalSrc(design)` for the processed result, both using the existing URL normalization rules.

- [ ] **Step 1: Add failing helper tests.**

  Add tests covering:

  ```ts
  expect(resolveGeneratedDesignOriginalSrc({
    id: "design-1",
    imageUrl: "/api/v1/listing-kits/uploads/files/final.png",
    originalImageUrl: "/api/v1/listing-kits/uploads/files/original.png",
  })).toBe("/api/listing-kits/uploads/files/original.png");

  expect(resolveGeneratedDesignOriginalSrc({
    id: "design-2",
    imageUrl: "https://cdn.example.test/ordinary.png",
  })).toBe("https://cdn.example.test/ordinary.png");
  ```

  Also assert that the final helper returns an empty string when `backgroundRemovalStatus` is not `succeeded`, and returns the normalized `imageUrl` after a successful removal.

- [ ] **Step 2: Run the focused helper tests and verify they fail.**

  ```powershell
  npm.cmd test -- src/lib/shein-studio/design-image.test.ts
  ```

  Expected: `FAIL` because the new helpers do not exist.

- [ ] **Step 3: Implement the two minimal helpers.**

  Keep `resolveGeneratedDesignSrc` as the current-image resolver. Implement:

  ```ts
  export function resolveGeneratedDesignOriginalSrc(
    design: SheinStudioGeneratedDesign,
  ) {
    return resolveGeneratedDesignSrc({
      ...design,
      imageUrl: design.originalImageUrl || design.imageUrl,
    });
  }

  export function resolveGeneratedDesignFinalSrc(
    design: SheinStudioGeneratedDesign,
  ) {
    return design.backgroundRemovalStatus === "succeeded"
      ? resolveGeneratedDesignSrc(design)
      : "";
  }
  ```

- [ ] **Step 4: Run the focused helper tests and verify they pass.**

  Run the Step 2 command. Expected: `PASS`.

- [ ] **Step 5: Commit the source-helper change.**

  ```powershell
  git add -- web/listingkit-ui/src/lib/shein-studio/design-image.ts web/listingkit-ui/src/lib/shein-studio/design-image.test.ts
  git diff --cached --check
  git commit -m "feat: expose studio original and final image sources"
  ```

### Task 3: Render the two-image review card and manual action

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-preview-grid.tsx:1-230`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx`

**Interfaces:**
- Consumes: the existing `onRetryBackgroundRemoval(designId)` callback and the new source helpers from Task 2.
- Produces: every non-read-only card renders a `重新抠图` button; a card with a completed removal renders labeled original and final image elements; pending/failed/no-result states remain visible.

- [ ] **Step 1: Add failing component tests.**

  Add tests that render `SheinDesignPreviewGrid` with:

  ```tsx
  designs={[{
    id: "design-1",
    imageUrl: "https://cdn.example.test/final.png",
    originalImageUrl: "https://cdn.example.test/original.png",
    backgroundRemovalStatus: "succeeded",
  }]}
  ```

  Assert:

  - buttons named `重新抠图`, `原图`, and `抠图后` are present;
  - the original and final images use the corresponding URLs (with imgproxy expectations adjusted through the existing thumbnail helper);
  - clicking `重新抠图` calls the callback with `design-1`.

  Add a second test with no `originalImageUrl` and status `not_requested`; assert the card still shows `重新抠图` and `尚未抠图`. Add a read-only test asserting the two images remain visible but the action button is absent.

- [ ] **Step 2: Run the focused component tests and verify the expected failure.**

  ```powershell
  npm.cmd test -- src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx
  ```

  Expected: `FAIL` because the current card renders one preview and only renders `重试抠图` after a failed removal.

- [ ] **Step 3: Implement the card UI with minimal behavior.**

  In `shein-design-preview-grid.tsx`:

  - import `resolveGeneratedDesignOriginalSrc` and `resolveGeneratedDesignFinalSrc`;
  - render `重新抠图` beside the existing regeneration action whenever `!readOnly`, using `retryingBackgroundRemovalId === design.id` to disable the current design and show `抠图中...`;
  - render two responsive preview panes with explicit labels and `Image` elements;
  - use the original helper for the left pane;
  - use the final helper for the right pane; when empty, render the exact status copy `尚未抠图`, `抠图处理中`, or `抠图失败，当前显示原图`;
  - keep the existing approval, regeneration, lightbox opening, and create-task controls unchanged;
  - show `backgroundRemovalError` in the failed state when present, without hiding the original image.

- [ ] **Step 4: Update the lightbox labels without changing its data flow.**

  In `shein-design-lightbox.tsx`, keep the current `activeImageView` toggle but change its visible labels to `抠图后` for the final view and `查看原图` / `查看抠图后` for the toggle. Update `shein-design-lightbox.test.tsx` to assert the normalized original URL through the new copy.

- [ ] **Step 5: Run the focused preview and lightbox tests.**

  ```powershell
  npm.cmd test -- src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx src/components/listingkit/shein-studio/shein-design-lightbox.test.tsx
  ```

  Expected: all focused tests pass.

- [ ] **Step 6: Commit the review UI change.**

  ```powershell
  git add -- web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-preview-grid.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-lightbox.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-lightbox.test.tsx
  git diff --cached --check
  git commit -m "feat: show studio original and removed artwork"
  ```

### Task 4: Verify the existing workbench callback covers first-time removal

**Files:**
- Inspect: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx:1342-1371`
- Inspect: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-task-creation-controller.ts:513-560`
- Test: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-task-creation-controller.test.ts`

**Interfaces:**
- Consumes: `handleRetryBackgroundRemoval`, `runItemizedBackgroundRemovalRetry`, and `retrySheinStudioBatchBackgroundRemoval`.
- Produces: the existing callback sends the clicked design ID and refreshes the complete hydrated batch after the broadened backend accepts a normal design.

- [ ] **Step 1: Add a regression assertion for an ordinary design ID.**

  Add a runner test with a hydrated detail whose design has `transparentBackgroundMode: "none"`, `backgroundRemovalStatus: "not_requested"`, and no original URL. Assert `runItemizedBackgroundRemovalRetry` still calls the injected retry function with `batch-1` and `design-1`, rather than filtering the design on the client.

- [ ] **Step 2: Run the focused controller test and verify it fails if client filtering exists.**

  ```powershell
  npm.cmd test -- src/components/listingkit/shein-studio/shein-studio-task-creation-controller.test.ts
  ```

  Expected: the new regression test fails only if the current client helper adds eligibility filtering; if it passes immediately, record that the callback already forwards the ID and keep production code unchanged.

- [ ] **Step 3: Make only the necessary controller change.**

  If the test demonstrates client-side filtering, remove that filtering so the helper always forwards the selected design ID. Do not rename the callback or change batch hydration behavior; the backend remains responsible for source eligibility and validation.

- [ ] **Step 4: Run the focused controller suite.**

  Run the Step 2 command. Expected: all controller tests pass.

### Task 5: Full verification and handoff

**Files:**
- Inspect: all files changed in Tasks 1–4

- [ ] **Step 1: Run backend package tests.**

  ```powershell
  go test ./internal/listingkit -count=1
  ```

  Expected: exit code 0 with no test failures.

- [ ] **Step 2: Run the complete frontend test suite.**

  ```powershell
  npm.cmd test
  ```

  Working directory: `C:\Users\Henry\code\task-processor\web\listingkit-ui`.

  Expected: exit code 0 with no failed tests.

- [ ] **Step 3: Run frontend typecheck, lint, and build.**

  ```powershell
  npm.cmd run typecheck
  npm.cmd run lint
  npm.cmd run build
  ```

  Working directory: `C:\Users\Henry\code\task-processor\web\listingkit-ui`.

  Expected: all commands exit 0. Existing non-blocking lint warnings must be reported separately from errors.

- [ ] **Step 4: Run repository diff validation and inspect status.**

  ```powershell
  git diff --check HEAD~3..HEAD
  git status --short
  ```

  Confirm only the intended design, service, UI, and test files are changed, with no generated artifacts or unrelated edits.

- [ ] **Step 5: Report the evidence-backed handoff.**

  Summarize the commits, the exact test/typecheck/lint/build results, the final UI behavior, and any environment-only limitation. Do not claim runtime/customer acceptance unless the Studio page was actually exercised against a configured background-removal provider.

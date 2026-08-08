# Studio Generated Image Download and Manual Upload Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let Studio users download each generated original image and persist a user-provided transparent PNG as that design's final background-removed image.

**Architecture:** Add a Studio-specific multipart mutation that validates and stores one PNG, snapshots the design's original image when needed, and applies only image/removal fields through the existing partial-update boundary. Add a typed frontend API wrapper and a same-origin Blob download helper, then wire both actions through the existing review grid and workbench hydration flow.

**Tech Stack:** Go/Gin service and route descriptors, existing ListingKit image upload/storage services, GORM/memory Studio batch repositories, React/Next.js, TypeScript, Vitest, Testing Library, Go `testing`.

## Global Constraints

- Download only the generated original image; never add a download action for the manual/automatic removed result.
- Accept only real PNG files for manual upload, even when the browser MIME type is misleading.
- Preserve `original_image_url`, review status, review note, and sort order when applying a manual result.
- Reject manual upload while the same design has a persisted background-removal status of `pending`.
- Reuse existing upload storage, tenant ownership, image proxy, batch hydration, and partial background-removal update boundaries.
- Do not change automatic background-removal retry, approval, task creation, or product-image behavior.

---

### Task 1: Add the server-side manual-result mutation

**Files:**
- Modify: `internal/listingkit/studio_batch_service.go`
- Modify: `internal/listingkit/task_studio_batch_service.go`
- Modify: `internal/listingkit/service_studio_batch_entrypoints.go`
- Modify: `internal/listingkit/service_upload_logic.go`
- Modify: `internal/listingkit/studio_session_handler.go`
- Modify: `internal/listingkit/api/studio_sessions_handler.go`
- Modify: `internal/listingkit/httpapi/routes_descriptor_entrypoints.go`
- Modify: `internal/listingkit/httpapi/bootstrap_contracts.go`
- Modify: `internal/listingkit/api/studio_batches_handler_test.go`
- Create: `internal/listingkit/studio_manual_background_removal_test.go`

**Interfaces:**
- Consumes: a multipart `file`, `batch_id`, and `design_id`; the existing `StudioMediaService` upload path; `studioBackgroundRemovalRepository.UpdateStudioMaterializedDesignBackgroundRemoval`.
- Produces: `StudioSessionHandlerService.ApplyManualStudioBatchDesignBackgroundRemoval(ctx, batchID, designID string, input *ImageUploadInput) (*StudioBatchDetail, error)` and `StudioBatchService.ApplyManualStudioBatchDesignBackgroundRemoval(ctx, batchID, designID, imageURL string) (*StudioBatchDetail, error)`.

- [ ] **Step 1: Add failing domain tests for a first manual PNG result.**

  In `internal/listingkit/studio_manual_background_removal_test.go`, seed the memory batch repository with an approved design whose `ImageURL` is `https://cdn.example.test/generated.png`, whose `OriginalImageURL` is empty, and whose removal status is `not_requested`. Call the batch service method with `https://cdn.example.test/manual.png` and assert that the returned design has:

  ```go
  OriginalImageURL == "https://cdn.example.test/generated.png"
  ImageURL == "https://cdn.example.test/manual.png"
  TransparentBackgroundMode == StudioTransparencyModeRemoval
  BackgroundRemovalStatus == StudioBackgroundRemovalStatusSucceeded
  ReviewStatus == StudioMaterializedDesignReviewStatusApproved
  ```

- [ ] **Step 2: Run the new domain test and verify it fails for the missing method.**

  Run:

  ```powershell
  go test ./internal/listingkit -run TestApplyManualStudioBatchDesignBackgroundRemoval -count=1
  ```

  Expected: compile failure because the manual mutation method is not defined.

- [ ] **Step 3: Implement the batch-service mutation with the existing partial update boundary.**

  Add the method to `StudioBatchService` and `taskStudioBatchService`. Load the design by batch and ID, reject missing image URLs and persisted `pending` status, preserve an existing `OriginalImageURL`, set the new image URL and removal fields, and call `updateStudioBackgroundRemoval`. Return `GetStudioBatchDetail` after the write. Do not call `retryBackgroundRemoval` and do not use `UpdateStudioMaterializedDesign`, so review fields cannot be overwritten.

- [ ] **Step 4: Run the domain test and verify it passes.**

  Run the Step 2 command. Expected: PASS.

- [ ] **Step 5: Add failing tests for PNG validation and cleanup orchestration.**

  Add tests around the root `service` entrypoint using a fake media upload/delete collaborator. Assert that a valid PNG is passed to the existing upload service, its returned upload URL is applied to the batch service, and a batch update error invokes `DeleteUploadedImage` for the newly created upload. Add a second case with JPEG bytes and assert the batch service and upload store are not called.

- [ ] **Step 6: Implement the root service upload orchestration.**

  In `service_studio_batch_entrypoints.go`, validate the bytes with the existing `validateUploadedImage`, reject any decoded format other than PNG, call `UploadImages` with one `ImageUploadInput`, apply the returned URL through the batch service, and delete the new upload when applying the design fails. Keep the cleanup scoped to the upload created by this request. Return the updated batch detail.

- [ ] **Step 7: Add the multipart handler and route contract.**

  Add `ApplyManualStudioBatchDesignBackgroundRemoval` to the session service/handler interfaces. In `api/studio_sessions_handler.go`, read `c.FormFile("file")`, open and read it with the existing upload limits, pass filename/content type/data to the service, and route errors through `writeStudioBatchActionError` with the action key `studio_manual_background_removal_failed`. Register:

  ```text
  POST /api/v1/listing-kits/studio/batches/:batch_id/designs/:design_id/manual-background-removal
  ```

  Update bootstrap stubs and add a handler test that posts multipart PNG data, checks the extracted batch/design IDs and file bytes, and verifies the returned detail is JSON. Add a test that omits `file` and expects HTTP 400.

- [ ] **Step 8: Run backend focused tests and commit the server change.**

  Run:

  ```powershell
  go test ./internal/listingkit -run 'TestApplyManualStudioBatchDesignBackgroundRemoval|TestStudioBatchManualBackgroundRemoval|Test.*Manual.*Background' -count=1
  git diff --check
  git add -- internal/listingkit/studio_batch_service.go internal/listingkit/task_studio_batch_service.go internal/listingkit/service_studio_batch_entrypoints.go internal/listingkit/service_upload_logic.go internal/listingkit/studio_session_handler.go internal/listingkit/api/studio_sessions_handler.go internal/listingkit/httpapi/routes_descriptor_entrypoints.go internal/listingkit/httpapi/bootstrap_contracts.go internal/listingkit/api/studio_batches_handler_test.go internal/listingkit/studio_manual_background_removal_test.go
  git commit -m "feat: persist manually removed studio images"
  ```

  Expected: all matching tests pass and the commit contains only the server-side mutation, route, interface, and tests.

### Task 2: Add typed upload and reliable original-image download helpers

**Files:**
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batches.ts`
- Modify: `web/listingkit-ui/src/lib/api/shein-studio-batches.test.ts`
- Modify: `web/listingkit-ui/src/lib/utils/image-proxy-url.ts`
- Modify: `web/listingkit-ui/src/lib/utils/image-proxy-url.test.ts`
- Create: `web/listingkit-ui/src/lib/shein-studio/download-image.ts`
- Create: `web/listingkit-ui/src/lib/shein-studio/download-image.test.ts`

**Interfaces:**
- Consumes: `batchId`, `designId`, a browser `File`, and the existing image-source resolver.
- Produces: `uploadManualSheinStudioBackgroundRemoval(batchId, designId, file): Promise<SheinStudioBatchDetail>` and `downloadStudioImage(src, filename): Promise<void>`.

- [ ] **Step 1: Add failing API-wrapper tests.**

  Extend `shein-studio-batches.test.ts` with a test that calls `uploadManualSheinStudioBackgroundRemoval("batch-1", "design-1", pngFile)`. Assert that fetch receives:

  ```text
  /api/listing-kits/studio/batches/batch-1/designs/design-1/manual-background-removal
  ```

  and a `FormData` body containing the file under `file`, then assert the response is normalized with `parseSheinStudioBatchDetailResponse`.

- [ ] **Step 2: Run the API-wrapper test and verify it fails.**

  Run:

  ```powershell
  npm.cmd test -- src/lib/api/shein-studio-batches.test.ts
  ```

  Expected: FAIL because the manual upload wrapper is not defined.

- [ ] **Step 3: Implement the multipart API wrapper.**

  Import `apiFormRequest`, build one `FormData` with `formData.append("file", file)`, post to the design-specific path, and normalize the JSON response through the existing batch-detail parser. Do not add a second generic upload client.

- [ ] **Step 4: Add failing download-helper tests.**

  In `download-image.test.ts`, mock `fetch`, `URL.createObjectURL`, `URL.revokeObjectURL`, and `document.createElement`. Assert that a remote image is fetched through a same-origin `/api/image-proxy?url=...` URL, a Blob URL is assigned to an anchor with `download="studio-design-1-original.png"`, the anchor is clicked, and the object URL is revoked. Add a non-2xx response test that rejects with an error.

- [ ] **Step 5: Implement forced-proxy download behavior.**

  Add an optional `{ forceProxy?: boolean }` argument to `toImageProxyUrl`; preserve current direct-host behavior unless `forceProxy` is true. Implement `downloadStudioImage` with `toImageProxyUrl(src, { forceProxy: true })`, `fetch`, Blob conversion, temporary anchor creation, and cleanup in `finally`. Keep data URLs and existing same-origin upload URLs usable.

- [ ] **Step 6: Run helper tests and commit the client utility change.**

  Run:

  ```powershell
  npm.cmd test -- src/lib/api/shein-studio-batches.test.ts src/lib/shein-studio/download-image.test.ts src/lib/utils/image-proxy-url.test.ts
  git diff --check
  git add -- web/listingkit-ui/src/lib/api/shein-studio-batches.ts web/listingkit-ui/src/lib/api/shein-studio-batches.test.ts web/listingkit-ui/src/lib/utils/image-proxy-url.ts web/listingkit-ui/src/lib/utils/image-proxy-url.test.ts web/listingkit-ui/src/lib/shein-studio/download-image.ts web/listingkit-ui/src/lib/shein-studio/download-image.test.ts
  git commit -m "feat: add studio image upload and download helpers"
  ```

  Expected: all focused tests pass.

### Task 3: Wire the actions into the Studio review UI

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-preview-grid.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-sections.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx`
- Modify: `web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-test-harness.tsx`

**Interfaces:**
- Consumes: `uploadManualSheinStudioBackgroundRemoval`, `downloadStudioImage`, existing `applyItemizedBatchDetail`, and existing per-design retry state.
- Produces: `onUploadManualBackgroundRemoval?: (designId: string, file: File) => Promise<void>` and `uploadingManualBackgroundRemovalId?: string` props from the workbench to the preview grid.

- [ ] **Step 1: Add failing preview-grid tests.**

  Add tests for a completed design asserting that “下载原图” and “上传手动抠图” are visible, the download handler receives the original source and a stable original filename, and selecting a PNG calls the upload callback with the design ID and file. Add tests that a pending design disables/hides manual upload, a non-PNG file is ignored with an error message, and a read-only grid keeps download but hides upload.

- [ ] **Step 2: Run the preview-grid tests and verify the expected failure.**

  Run:

  ```powershell
  npm.cmd test -- src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx
  ```

  Expected: FAIL because the new props and action controls do not exist.

- [ ] **Step 3: Implement preview-grid download and upload controls.**

  Import the original source resolver, `downloadStudioImage`, and the new props. Render a labeled “原图” pane with the existing thumbnail plus a download button. Render the “抠图后” pane with the existing final/status logic plus a hidden input using `accept="image/png,.png"` and an upload button. Keep the input keyed per design and clear its value after handling so the same file can be selected again. Use local per-design error text for rejected files or failed callbacks, and use `uploadingManualBackgroundRemovalId` to disable only the active card.

- [ ] **Step 4: Thread the new props through the review step.**

  Extend `SheinStudioReviewStep` and its `SheinDesignPreviewGrid` call with the upload callback and uploading ID. Update the workbench test harness mocks and existing call sites so all test fixtures compile without changing read-only behavior.

- [ ] **Step 5: Add a failing workbench callback test.**

  Add a test that invokes the manual-upload action for `batch-1`/`design-1`, asserts the API mock receives the selected PNG, asserts `applyItemizedBatchDetail` receives the returned detail, and asserts a rejected API call sets the visible error and clears the uploading ID.

- [ ] **Step 6: Implement the workbench callback.**

  Import the multipart API wrapper, add `uploadingManualBackgroundRemovalId` state, and implement `handleUploadManualBackgroundRemoval`. Require an active itemized batch, set the ID before awaiting the API, apply the returned detail through the existing hydration path, report `上传手动抠图失败：...` on error, and clear the ID in `finally`. Pass the callback only when an itemized batch is active, matching the existing draft-only safety boundary.

- [ ] **Step 7: Run focused UI tests and commit the UI change.**

  Run:

  ```powershell
  npm.cmd test -- src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx src/components/listingkit/shein-studio/shein-design-lightbox.test.tsx
  git diff --check
  git add -- web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-preview-grid.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-design-preview-grid.test.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-sections.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench.test.tsx web/listingkit-ui/src/components/listingkit/shein-studio/shein-studio-workbench-test-harness.tsx
  git commit -m "feat: add studio original download and manual cutout upload"
  ```

  Expected: all focused UI tests pass and no unrelated files are staged.

### Task 4: Full verification and handoff

**Files:**
- Inspect: all files changed in Tasks 1–3

- [ ] **Step 1: Run the complete backend test suite.**

  From `C:\Users\Henry\code\task-processor\.worktrees\studio-manual-upload-download`, run:

  ```powershell
  go test ./... -count=1 -timeout 5m
  ```

  Expected: exit code 0 with no failed packages.

- [ ] **Step 2: Run the complete frontend test suite.**

  From `web/listingkit-ui`, run:

  ```powershell
  npm.cmd test
  ```

  Expected: exit code 0 with no failed test files.

- [ ] **Step 3: Run frontend typecheck, lint, and production build.**

  Run:

  ```powershell
  npm.cmd run typecheck
  npm.cmd run lint
  npm.cmd run build
  ```

  Expected: all commands exit 0; report any pre-existing lint warnings separately from errors.

- [ ] **Step 4: Run diff and status checks.**

  Run:

  ```powershell
  git diff --check origin/master..HEAD
  git status --short --branch
  ```

  Confirm only the design implementation, API/route, UI, helper, and test files are present; do not include `node_modules` or generated build output.

- [ ] **Step 5: Commit verification notes and prepare handoff.**

  Review the final diff for the exact endpoint, PNG-only enforcement, original-image preservation, download-only-original behavior, pending guard, and cleanup path. Report the branch, commits, test totals, typecheck/lint/build results, and the remaining limitation that real customer acceptance requires a configured Studio runtime and a real PNG upload.

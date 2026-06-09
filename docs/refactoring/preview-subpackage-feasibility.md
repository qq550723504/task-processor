# Preview Subpackage Feasibility Note

> Status: keep preview refactoring inside `package listingkit` for now. Do not move to `internal/listingkit/preview` until the dependencies below are reduced or explicitly bridged.

## 1. Context

The project-wide execution plan recommends preview refactoring as the first code-level slice because it is a bounded aggregation hotspot with existing test coverage.

The first two implementation steps are complete:

1. Preview platform selection was centralized through `shouldBuildPreviewPlatform(...)` and `isSelectedPreviewPlatform(...)`.
2. Platform preview section construction was delegated through package-private builder adapters.

Current files involved:

- `internal/listingkit/preview_builder.go`
- `internal/listingkit/preview_platform_sections.go`
- `internal/listingkit/preview_builder_platforms.go`
- `internal/listingkit/preview_model.go`
- `internal/listingkit/preview_platform_selection_test.go`

## 2. Current Decision

Do **not** move preview code into `internal/listingkit/preview` yet.

Reason: the preview builder currently depends heavily on root `listingkit` DTOs, package-local helpers, package-local errors, and platform payload builders. Moving it into a real subpackage now would likely require either:

- exporting many root `listingkit` internals,
- creating a cycle between `listingkit` and `listingkit/preview`, or
- moving many public API DTOs at once.

All three options would increase risk beyond the intended small-step refactor.

## 3. Root Dependencies Blocking Immediate Subpackage Extraction

### 3.1 Root preview DTO ownership

`ListingKitPreview` and related response models currently live in root `internal/listingkit/preview_model.go`.

Examples:

- `ListingKitPreview`
- `ListingKitPreviewHeader`
- `ListingKitPlatformCard`
- `AmazonPreviewPayload`
- `SheinPreviewPayload`
- `TemuPreviewPayload`
- `WalmartPreviewPayload`
- `PlatformAssetRenderPreviews`
- `AssetRenderPreview`
- `RevisionHistory` preview models

A new `internal/listingkit/preview` package would need to import these root models. But root `listingkit` currently needs to call the preview builder, creating a likely dependency cycle:

```text
listingkit -> listingkit/preview -> listingkit
```

### 3.2 Package-local error and status dependencies

Preview builder currently uses root package values such as:

- `ErrTaskNotFound`
- `ErrUnsupportedPreviewPlatform`
- `ErrPreviewPlatformUnavailable`
- `TaskStatusCompleted`
- `TaskStatusNeedsReview`
- `TaskStatusFailed`

These are valid API/service shell concepts today. Moving preview to a subpackage would require either exporting a smaller error/status contract or moving shared shell types to a lower-level package.

### 3.3 Package-local helper dependencies

Preview aggregation currently calls many root package helpers, including examples such as:

- `normalizePlatforms(...)`
- `ensureTaskPodExecution(...)`
- `buildAssetRenderPreviews(...)`
- `buildPlatformAssetRenderPreviews(...)`
- `filterPlatformAssetRenderPreviews(...)`
- `buildRevisionHistoryMeta(...)`
- `buildRevisionHistoryPreviewItems(...)`
- `reviewReasonsFromResult(...)`
- `buildPlatformPreviewCards(...)`
- `platformAssetRenderPreviewsByPlatform(...)`
- `buildPlatformScenePresetSummaries(...)`

A real subpackage would need these helpers moved, exported, or replaced with a smaller input model.

### 3.4 Platform package payload dependencies

Platform preview builders currently call platform-specific root functions and DTOs:

- `buildAmazonPreviewPayload(...)`
- `buildSheinPreviewPayload(...)`
- `buildTemuPreviewPayload(...)`
- `buildWalmartPreviewPayload(...)`
- `AmazonPackage`
- `SheinPackage`
- `TemuPackage`
- `WalmartPackage`

This is still acceptable inside root `listingkit` during the compatibility-facade phase, but it makes immediate subpackage extraction harder.

## 4. Recommended Near-term Shape

Keep a file-group modularization first:

```text
internal/listingkit/preview_builder.go
internal/listingkit/preview_platform_sections.go
internal/listingkit/preview_builder_platforms.go
internal/listingkit/preview_model.go
internal/listingkit/preview_*_test.go
```

This gives most of the short-term maintainability benefit without changing package boundaries.

## 5. Criteria Before Real Subpackage Extraction

Only reconsider `internal/listingkit/preview` when at least one of these conditions is met:

### Option A: Move preview DTOs down

Create a lower-level preview model package such as:

```text
internal/listingkit/previewmodel
```

Then both root `listingkit` and `listingkit/preview` can depend on it:

```text
listingkit -> listingkit/preview
listingkit -> listingkit/previewmodel
listingkit/preview -> listingkit/previewmodel
```

This avoids `listingkit/preview -> listingkit`.

### Option B: Move the entire preview API surface

Move preview DTOs and builders together into:

```text
internal/listingkit/preview
```

Then root `listingkit` only re-exports or delegates stable service methods. This is larger and should not be the next immediate step.

### Option C: Build a stable input/output adapter

Create a smaller input struct that contains only data needed by preview aggregation:

```go
type BuildInput struct {
    TaskID string
    Status TaskStatus
    Result *ListingKitResult
    Request *GenerateRequest
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

This only helps if the input and output types do not import root `listingkit`.

## 6. Recommended Next PRs

Continue within `package listingkit`:

1. Split platform section helpers into per-platform files if the file grows:
   - `preview_platform_amazon.go`
   - `preview_platform_shein.go`
   - `preview_platform_temu.go`
   - `preview_platform_walmart.go`
2. Extract base preview initialization into `buildBaseListingKitPreview(...)`.
3. Extract common result attachment into `attachPreviewResultSummary(...)` or similar.
4. Keep tests focused on selected-platform behavior and payload preservation.

Do not move to a real subpackage until the dependency direction can be shown without cycles.

## 7. Stop Condition

Stop preview extraction and switch to another phase if any next step requires:

- exporting many root helpers only for the new subpackage,
- moving `ListingKitPreview` and platform payload DTOs in the same PR,
- changing API JSON shape,
- changing front-end TypeScript response types,
- adding an import from a lower-level package back into root `listingkit`.

## 8. Current Status Summary

Preview is now adapter-ready inside the root package, but not subpackage-ready.

This is acceptable and intentional. The immediate goal is to reduce central function density and platform hardcoding while preserving behavior.

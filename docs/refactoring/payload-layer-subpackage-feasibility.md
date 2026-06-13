# Payload Layer Subpackage Feasibility Note

> Status: keep the platform payload layer inside `package listingkit` for now. Do not move it into a real subpackage until the DTO and platform-package dependencies are reduced or intentionally bridged.

## 1. Context

Recent refactoring has already created a recognizable payload-oriented file group inside root `internal/listingkit`.

Current payload-oriented files now include:

- `platform_payload_input_models.go`
- `platform_payload_result_context.go`
- `platform_payload_shein_context.go`
- `platform_payload_models_export.go`
- `platform_payload_models_preview_amazon.go`
- `platform_payload_models_preview_reviewable.go`
- `platform_payload_models_preview_shein.go`
- `preview_platform_payload_from_result.go`
- `preview_platform_payload_from_input.go`
- `export_platform_payload_from_result.go`
- `export_platform_payload_from_input.go`

The current internal flow is much clearer than before:

```text
result context
  -> platform-specific shared context
  -> from-result adapters
  -> shared input models
  -> from-input builders
  -> payload DTO bodies
```

This is enough structure to evaluate subpackage extraction realistically instead of speculating.

## 2. Current Decision

Do **not** move the payload layer into a real subpackage yet.

Reason: the files are now grouped well, but they still depend heavily on root `listingkit` DTOs, root result/package models, and preview/export response shapes that remain part of the legacy ListingKit API surface.

Moving now would likely force one of these bad tradeoffs:

- export many root-only types just to satisfy the new package,
- create cycles between `listingkit` and a new payload package,
- move response DTOs and builders in the same PR,
- widen the blast radius into API JSON compatibility.

That is too much risk for the next step.

## 3. What Is Now Good Enough

The recent work did materially improve readiness:

1. preview and export now use parallel `from result -> from input -> payload body` shapes
2. visual presentation construction is shared at the lower level
3. result-derived platform context is more explicit
4. SHEIN payload preprocessing is no longer duplicated across preview/export
5. payload DTOs are beginning to separate from preview/export shell files

This means the payload layer is now **file-group ready** and **migration-map ready**, even if it is not yet **subpackage ready**.

## 4. Main Blockers

### 4.1 Root response DTO ownership

Preview/export payload DTOs still live in root `listingkit`, even if they are now in better-named files.

Examples:

- `AmazonPreviewPayload`
- `SheinPreviewPayload`
- `TemuPreviewPayload`
- `WalmartPreviewPayload`
- `AmazonExportPayload`
- `SheinExportPayload`
- `TemuExportPayload`
- `WalmartExportPayload`
- `PlatformScenePresetSummary`
- `PlatformAssetRenderPreviews`

If a new subpackage built these DTOs directly, it would still need to import many root `listingkit` types.

### 4.2 Root result/package model dependencies

The payload adapters still depend directly on root result and package models:

- `ListingKitResult`
- `AmazonPackage`
- `TemuPackage`
- `WalmartPackage`
- SHEIN package and readiness bridge types exposed through root `listingkit`

That keeps the payload layer tied to ListingKit's current orchestration shell.

### 4.3 SHEIN-specific bridge gravity

The hardest payload path is still SHEIN because it mixes:

- semantic normalization
- submit-readiness projection
- repair center derivation
- workspace overview derivation
- preview/export DTO shaping

This is much cleaner than before, but it is still not a small dependency surface.

### 4.4 Preview/export shell coupling

The payload layer is still closely coupled to:

- preview shell DTO files
- export shell DTO files
- root compatibility JSON surface

Subpackage extraction should not happen until payload DTO ownership is more explicit.

## 5. Recommended Next Safe Step

Before any real subpackage move, do one more internal boundary pass:

1. keep payload DTO files grouped under the `platform_payload_*` naming family
2. keep moving preview/export payload DTOs out of mixed shell files
3. identify which DTOs are true payload DTOs versus shell-only response wrappers
4. introduce smaller adapter-facing input structs where root `ListingKitResult` is still too wide

This preserves the current low-risk cadence.

## 6. Criteria Before Real Extraction

Only reconsider a real payload subpackage when most of these are true:

1. preview/export payload DTOs live in clearly grouped payload-model files
2. `from result` adapters no longer need broad direct access to root `ListingKitResult`
3. SHEIN payload preparation has a narrower bridge surface
4. the new package can depend on shared payload models without importing broad root `listingkit` service/runtime helpers
5. dependency direction can be shown without `listingkit -> payload -> listingkit`

## 7. Candidate Future Shape

When ready, the likely first extraction shape is not a full domain split. It is a narrow internal layer such as:

```text
internal/listingkit/payload/
  context.go
  input_models.go
  preview_from_result.go
  preview_from_input.go
  export_from_result.go
  export_from_input.go
  models_preview_*.go
  models_export.go
```

Root `listingkit` would then call this layer as a compatibility facade.

## 8. Stop Condition

Stop payload-layer extraction and stay in root `listingkit` if the next step requires:

- exporting many root-only helper functions,
- moving `ListingKitResult` and payload DTOs together,
- changing API JSON shape,
- changing frontend response contracts,
- introducing a dependency from a lower-level payload package back into root `listingkit`.

## 9. Current Summary

The payload layer is now clearly visible inside root `listingkit`, which is a meaningful milestone.

It is ready for continued file-group tightening and DTO ownership cleanup.

It is **not** ready for a real subpackage move yet, and that is fine. The current best move is to preserve the new structure, keep the compatibility surface stable, and only extract when the dependency direction becomes genuinely simple.

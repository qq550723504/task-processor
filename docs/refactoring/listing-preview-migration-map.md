# Listing Preview Migration Map

> Status: first migration inventory for moving preview ownership from `internal/listingkit` toward `internal/listing/preview` and related target domains.

## 1. Purpose

This document narrows the project-wide migration roadmap down to the preview area.

It answers:

- which current `internal/listingkit` preview files should eventually move to `internal/listing/preview`
- which preview helpers are really marketplace-owned
- which files are compatibility-only bridges and should stay thin

## 2. Preview Ownership Rules

The preview area should follow these ownership rules:

- `internal/listing/preview`
  - preview aggregation
  - preview shell composition
  - selected-platform routing
  - platform builder registry
  - preview-facing semantic-field normalization
- `internal/marketplace/<platform>/*`
  - platform-specific preview payload rules when they are fundamentally platform-owned
  - SHEIN-specific final review, repair, and workspace preview logic when it is better understood as marketplace behavior than listing aggregation
- `internal/product/*`
  - reusable asset/image facts or transformations that happen to be rendered in preview
- `internal/compatibility/listingkit`
  - old service and facade entrypoints that expose preview behavior through the legacy ListingKit surface

## 3. Current File Groups

### 3.1 Direct `listing/preview` candidates

These files are strong candidates to become the first real `internal/listing/preview` package contents:

- `preview_builder.go`
- `preview_base.go`
- `preview_header.go`
- `preview_errors.go`
- `preview_result_attachment.go`
- `preview_builder_platforms.go`
- `preview_platform_sections.go`
- `preview_platform_selection.go`
- `preview_export_semantic_fields.go`
- `preview_model.go`
- `preview_platform_common.go`

Reason:

- they primarily own preview aggregation, shell composition, selected-platform behavior, and shared preview models

### 3.2 Platform builder files that likely start in `listing/preview`

These can move into `internal/listing/preview` first, even if some may later be further split by marketplace:

- `preview_platform_amazon.go`
- `preview_platform_shein.go`
- `preview_platform_temu.go`
- `preview_platform_walmart.go`

Reason:

- they currently act as preview assembly adapters
- they are small enough to move as a preview-facing bridge before deeper marketplace extraction

### 3.3 SHEIN preview files with likely future marketplace gravity

These files are preview-related today, but long-term some of their logic may belong in `internal/marketplace/shein/workspace` or `internal/marketplace/shein/publishing`:

- `preview_builder_shein.go`
- `preview_builder_shein_payload.go`
- `preview_builder_shein_review_summary.go`
- `preview_builder_shein_workspace_overview.go`
- `preview_builder_shein_source_product.go`
- `preview_builder_shein_image_upload.go`
- `preview_builder_shein_resolution_cache.go`
- `preview_builder_shein_final_review.go`
- `preview_builder_shein_final_review_images.go`
- `preview_builder_shein_final_review_skus.go`

Recommended migration order:

1. move them under a preview-owned package boundary first
2. keep file boundaries intact
3. only after that, decide which helpers should become marketplace-owned

### 3.4 Preview service and facade files that should stay compatibility-thin

These files are not the real preview-domain home and should remain thin wrappers during migration:

- `task_preview_service.go`
- `service_task_preview_logic.go`
- `service_shein_cookie_preview_helper.go`
- `service_shein_store_resolution_preview_support_helper.go`

Target direction:

- real preview logic moves to `internal/listing/preview`
- legacy service entrypoints remain in `internal/listingkit` until compatibility relocation is ready

### 3.5 Tests that should move with the preview domain

These tests should follow the preview domain as real package extraction happens:

- `preview_builder_test.go`
- `preview_builder_shein_test.go`
- `preview_platform_selection_test.go`
- `preview_platform_payload_test.go`
- `preview_export_semantic_fields_test.go`

These tests should stay where compatibility boundaries are being enforced:

- `service_wiring_test.go`
- `phase82_submit_readiness_projection_boundary_test.go`

## 4. First Safe Extraction Wave

The first safe real extraction wave for `internal/listing/preview` should be:

1. shared preview models and helpers
   - `preview_model.go`
   - `preview_errors.go`
   - `preview_base.go`
   - `preview_header.go`
   - `preview_result_attachment.go`
2. selected-platform and builder registry support
   - `preview_builder_platforms.go`
   - `preview_platform_sections.go`
   - `preview_platform_selection.go`
3. central preview aggregator
   - `preview_builder.go`
4. platform adapter bridge files
   - `preview_platform_amazon.go`
   - `preview_platform_shein.go`
   - `preview_platform_temu.go`
   - `preview_platform_walmart.go`

This keeps the first move centered on preview orchestration, not deeper marketplace semantics.

## 5. Deferred Decisions

These decisions should be postponed until after the first preview package extraction:

1. whether `SheinPreviewPayload` models stay preview-owned or partially move to marketplace-owned models
2. whether SHEIN final review helpers are preview-shell concerns or marketplace workspace concerns
3. whether image-upload preflight is preview-owned long-term or should live in a marketplace-owned submit-preparation module

## 6. Recommended Next Code Move

The next code move after this inventory should be one bounded slice:

- create the first real `internal/listing/preview` package with package comments and no behavior changes
- move a smallest stable group such as `preview_errors.go` and `preview_base.go`, or shared non-platform preview models, behind compatibility-preserving imports

Do not start with SHEIN-specific files for the first real package extraction.

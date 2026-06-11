# SHEIN Listing Sale Attribute Resolution Alignment Design

## Summary

`shein-listing` currently determines SHEIN primary and secondary sale attributes inside its SKC build flow using heuristic strategy selection. `listingkit` already has a more correct resolution-first model in `internal/publishing/shein`: it resolves source variant dimensions against real SHEIN sale-attribute templates, persists a `SaleAttributeResolution`, and only then applies that resolution to SKC/SKU grouping and payload materialization.

This design aligns `shein-listing` with the `listingkit` resolution model for primary and secondary sale attribute selection without migrating the full `shein-listing` pipeline to `internal/publishing/shein`.

## Problem

Today `shein-listing` selects the primary sale attribute in `internal/shein/product/skc/attribute_strategy.go` during SKC build. That logic is coupled to how SKCs and SKUs are assembled:

- it prefers required template attributes when they look usable
- otherwise it picks from a dynamic priority list
- it falls back to hard-coded defaults such as color `27`

This means `shein-listing` is answering two different questions at once:

1. What are the correct SHEIN primary and secondary sale attributes for this product?
2. How should the current build path group variants into SKCs and SKUs?

`listingkit` answers the first question separately and earlier. That separation is the desired model because:

- the primary/secondary choice is based on source-dimension-to-template mapping, not on the current assembly heuristic
- the result is explainable, cacheable, reviewable, and reusable
- later grouping behavior can change without redefining sale-attribute truth

## Goals

- Replace `shein-listing` primary/secondary sale attribute selection with the `listingkit`-style resolver model.
- Keep the existing `shein-listing` fetch, validation, attribute, SKC, SKU, and submit pipeline largely intact.
- Introduce a resolution object into `shein-listing` runtime state that becomes the preferred source of truth for SKC strategy construction.
- Preserve a legacy fallback path while the new resolver path is validated.

## Non-Goals

- Migrating the full `shein-listing` build pipeline to `internal/publishing/shein.Package`.
- Replacing the current submit path, image upload path, or final publish API path.
- Implementing the future "variant count threshold -> single SKC multi SKU" feature in this change.
- Replacing `SaleSpecResult` generation. The AI/generated sale attribute values remain in place for now.

## Current State

### `shein-listing`

Relevant flow:

1. `sale_attribute` generates `ctx.SaleSpecResult`
2. `build_skc_list` calls `DetermineAttributeStrategy(...)`
3. the chosen primary attribute drives SKC grouping
4. the chosen secondary attribute drives SKU grouping

Relevant files:

- `internal/shein/product/attribute/sale/handler.go`
- `internal/shein/product/skc/attribute_strategy.go`
- `internal/shein/product/skc/builder.go`
- `internal/shein/product/skc/variant.go`

### `listingkit`

Relevant flow:

1. assembler builds canonical product and SHEIN package
2. `SaleAttributeResolver.Resolve(...)` produces `SaleAttributeResolution`
3. grouping uses `PrimarySourceDimension`
4. `ApplySaleAttributeResolution(...)` maps resolved SKC/SKU sale attributes into the draft and preview payload

Relevant files:

- `internal/publishing/shein/sale_attribute_resolver.go`
- `internal/publishing/shein/runtime_sale_attribute_resolver.go`
- `internal/publishing/shein/model.go`
- `internal/publishing/shein/variant_grouping.go`
- `internal/publishing/shein/apply_resolution.go`

## Proposed Design

### 1. Add a sale attribute resolution phase to `shein-listing`

Insert a new runtime phase after `sale_attribute` and before `build_skc_list`.

Responsibilities:

- build a minimal `canonical.Product` view from `shein-listing` task context
- build a minimal `internal/publishing/shein.BuildRequest`
- build a minimal `internal/publishing/shein.Package`
- invoke the runtime sale attribute resolver
- store the returned `SaleAttributeResolution` on `TaskContext`

This phase does not replace the current `SaleSpecResult`. It only replaces how the primary and secondary attributes are finally chosen.

### 2. Introduce a thin adapter instead of full pipeline migration

The adapter will convert existing `shein-listing` runtime data into the input model expected by `internal/publishing/shein`.

Required mapped fields:

- canonical product:
  - title
  - brand
  - description
  - category path
  - variants
  - variant dimensions
  - images
- build request:
  - country
  - language
  - text
  - shein store id
  - context
- package:
  - category id
  - spu name or product name
  - optional review notes container

The adapter should intentionally map only fields actually needed by the resolver. It should not attempt to fully replicate `listingkit` assembly behavior.

### 3. Persist resolution in `TaskContext`

Extend `internal/shein/context.TaskContext` with a canonical `SaleAttributeResolution` field or equivalent wrapper type.

This becomes the preferred source for:

- primary attribute id
- secondary attribute id
- primary source dimension
- secondary source dimension
- resolved candidate metadata
- review notes for diagnostics

The field should survive across the remaining in-memory stages of the current pipeline run.

### 4. Build SKC strategy from resolution first

Change `internal/shein/product/skc/builder.go` so that strategy selection becomes:

1. If `TaskContext` contains a usable resolution, convert that resolution into the existing `AttributeStrategy` shape.
2. Otherwise, fall back to the legacy `DetermineAttributeStrategy(...)`.

This preserves the downstream SKC/SKU builders while swapping the source of truth for primary and secondary sale attributes.

The new conversion layer should:

- map `PrimarySourceDimension` and `SecondarySourceDimension` onto source variant dimensions
- use `PrimaryAttributeID` and `SecondaryAttributeID` from resolution
- materialize strategy attribute values from current `SaleSpecResult` and source dimensions
- fail closed when resolution exists but cannot be converted safely, so fallback behavior is explicit and logged

### 5. Preserve legacy fallback during rollout

The existing heuristic strategy path remains available when:

- resolver inputs cannot be built
- runtime resolver returns unusable or incomplete resolution
- resolution-to-strategy conversion fails

Logs should make the decision explicit:

- `strategy_source=resolution`
- `strategy_source=legacy`

This is required for side-by-side validation and rollback safety.

## Data Model Changes

### `TaskContext`

Add fields for:

- resolved sale attribute payload
- optional resolution diagnostics
- optional source marker indicating whether the current build used resolver output or legacy heuristic

### Optional helper types

Introduce small internal adapter/helper types if needed:

- `sheinListingSaleAttributeResolutionInput`
- `sheinListingCanonicalProductAdapter`
- `sheinListingStrategyFromResolutionBuilder`

These should remain narrow and focused. Avoid leaking `listingkit` service concepts into `shein-listing` runtime code beyond the shared resolver model.

## Detailed Flow

1. Existing `sale_attribute` step finishes and writes `ctx.SaleSpecResult`.
2. New `sale_attribute_resolution` step:
   - reads `ctx.AmazonProduct`, `ctx.Variants`, `ctx.AttributeTemplates`, `ctx.ProductData.CategoryID`, store/runtime context
   - builds resolver inputs
   - calls `runtimeSaleAttributeResolver.Resolve(...)`
   - stores result on `ctx`
3. `build_skc_list`:
   - prefers `BuildStrategyFromResolution(...)`
   - logs whether resolution or legacy heuristic was used
4. Existing SKC and SKU builders continue using `AttributeStrategy`
5. Existing submit path remains unchanged

## Error Handling

- If resolver input adaptation fails, log the reason and fall back to legacy strategy selection.
- If resolver returns `partial` or unresolved output, do not force it into SKC strategy unless conversion confirms the necessary fields are present.
- If resolution exists but cannot be safely converted, log an explicit conversion failure and use legacy fallback.
- Do not block the whole task solely because the new resolver path is unavailable in early rollout.

## Verification Plan

Primary verification is not publish success. Primary verification is strategy correctness.

Compare old and new behavior on the same task samples:

- primary attribute id
- secondary attribute id
- primary source dimension
- secondary source dimension
- SKC count
- SKU count per SKC
- notable grouping/image shifts

Validation set should prioritize known bad `shein-listing` cases where the old heuristic chooses the wrong primary sale attribute.

Recommended rollout stages:

1. resolver path behind logging only
2. resolver path active with legacy fallback
3. compare task samples and inspect deltas
4. reduce fallback dependence after confidence improves

## Risks

### Input adaptation mismatch

`listingkit` resolver expects canonicalized product and variant dimensions. `shein-listing` currently works with `model.Product`, filtered variants, and task-context-specific structures. Incorrect field mapping, especially variant attribute names, can mislead the resolver.

### Behavioral delta

Changing the primary sale attribute will legitimately change:

- SKC grouping
- SKU distribution
- image assignment behavior
- some previously successful but semantically incorrect payloads

This is expected but must be measured carefully.

### Hidden coupling in current builders

Existing SKC/SKU builders assume strategy values are shaped by the old heuristic path. The resolution-to-strategy conversion must preserve enough structure to keep downstream logic stable.

## Testing

Add focused tests for:

- adapter mapping from `TaskContext` to resolver input
- resolution-to-strategy conversion
- fallback activation when resolution is missing or unusable
- known cases where legacy chooses the wrong primary attribute and resolver chooses the expected one

Keep existing SKC/SKU builder tests and add coverage for:

- `strategy_source=resolution`
- `strategy_source=legacy`

## Open Decisions

- Whether the new resolver phase should be its own pipeline handler or embedded into the SKC build handler.

Recommendation:

Make it a distinct pipeline handler. This keeps "resolve sale attribute truth" separate from "assemble SKCs/SKUs" and matches the architectural direction we want.

## Recommendation

Implement this as a narrow alignment project:

- add a dedicated resolver phase
- adapt `shein-listing` runtime data into `internal/publishing/shein` resolver inputs
- store `SaleAttributeResolution` in `TaskContext`
- convert resolution back into `AttributeStrategy`
- keep legacy fallback temporarily

This fixes the root issue without forcing a full migration of `shein-listing` onto the `listingkit` SHEIN package builder.

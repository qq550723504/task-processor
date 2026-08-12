# Platform-aware asset result design

> Status: approved for plan revision; implementation requires review of this written design.
>
> Scope: Task 1 in `docs/refactoring/next-phase-plan.md` only.
>
> Calibrated against: `858fac46804895fe107d7aa20cdc13906f102f4b`.

## Goal

Remove implicit marketplace selection from the ListingKit image flow. A listing request that processes images must create one image-processing request per explicit target platform and retain each target's output without selecting one by request-array order.

## Decisions

### Target is explicit at the product-image boundary

`productimage.ImageProcessRequest` gains `TargetPlatform string` as the canonical target. `Marketplace` remains readable for persisted/history compatibility during migration only.

At every public or internal ingress:

1. trim and validate `TargetPlatform` against `listing/platform` supported values;
2. if `TargetPlatform` is empty and legacy `Marketplace` is present, copy the normalized legacy value into `TargetPlatform`;
3. if both values are supplied and differ after normalization, reject the request;
4. reject the request if no target remains.

No generic product-image service may invent a target. This preserves compatibility for callers that already send `marketplace`, while preventing new hidden defaults.

### ListingKit validates before it defaults

When `GenerateOptions.ProcessImages` is true and the request has processable images or a product URL, `GenerateRequest.Platforms` must contain at least one supported target. This check runs before the current normalization path can substitute `SupportedPlatforms()`.

Requests that do not process images retain their current platform normalization behavior. This limits the behavior change to the image/asset operation and avoids turning unrelated non-image listing flows into invalid requests.

### One image task and result per target

Replace the single `toImageProcessRequest(*Task)` helper with a helper returning one validated request per normalized target. The standard media phase creates and processes each request independently. Its child-task and workflow-stage identifiers include the target, for example `product_image:shein` and `product_image:temu`, so one failure does not conceal the other target's status.

The media phase may continue processing other targets after one target fails. The final ListingKit result records target-specific warnings and task IDs; the existing summary/overall status policy remains unchanged in this task.

### Results are keyed by target; scalar values are compatibility only

`ListingKitResult` gains explicit target-keyed maps:

```go
ImageAssetsByTarget            map[string]*productimage.ImageProcessResult `json:"image_assets_by_target,omitempty"`
AssetBundlesByTarget           map[string]*asset.Bundle                    `json:"asset_bundles_by_target,omitempty"`
AssetInventorySummariesByTarget map[string]*asset.InventorySummary          `json:"asset_inventory_summaries_by_target,omitempty"`
```

Add lookup helpers that normalize a requested target and return the matching image result, asset bundle, or inventory summary. New platform payload, preview, and export code uses these helpers rather than the scalar fields.

The existing `ImageAssets`, `AssetBundle`, and `AssetInventorySummary` fields remain JSON/history compatibility projections:

- for a one-target request, project that target's values into each scalar field;
- for a multi-target request, project only `GenerateOptions.CompatibilityTargetPlatform` when it is present and belongs to the requested target set;
- for a multi-target request without that explicit selection, leave scalar fields unset rather than choosing the first target;
- new code must not depend on those scalar fields for platform-specific behavior.

`GenerateOptions.CompatibilityTargetPlatform` is optional and exists only to give an identified legacy consumer a controlled migration path. It is not a routing or marketplace-policy input.

## Data flow

```text
GenerateRequest.Platforms
  -> validate image-processing target set
  -> []ImageProcessRequest{TargetPlatform: ...}
  -> productimage ingress canonicalizes legacy Marketplace if needed
  -> one process result and asset bundle per target
  -> ListingKitResult.*ByTarget
  -> target-aware platform payload / preview / export lookup
  -> optional scalar compatibility projection
```

## Error and compatibility behavior

- Missing or unsupported image-processing targets fail before image-task persistence; they do not create an Amazon fallback task.
- A direct legacy product-image request with only `Marketplace` remains readable during the migration window and is normalized once at ingress.
- A direct request with contradictory `Marketplace` and `TargetPlatform` fails deterministically.
- Existing persisted scalar result JSON remains readable. A result without target-keyed maps is treated as a legacy scalar result only; code must not fabricate a target map for it.
- This task does not migrate stored JSON, change database schema, or change Temporal/RabbitMQ acknowledgement semantics.

## Tests

Tests must exercise real request/result behavior, not source-text checks:

1. product-image accepts legacy `Marketplace` only by canonicalizing it to `TargetPlatform`;
2. product-image rejects missing and contradictory targets;
3. ListingKit rejects a processable image request with no supported targets before creating an image task;
4. a two-target request produces exactly two target-keyed outputs and no first-target scalar projection;
5. a selected compatibility target projects only that target into legacy scalar fields;
6. platform payload construction reads the requested target's result, even when the request order is reversed;
7. a target failure leaves the other target's result and stage status observable.

Focused verification after implementation:

```powershell
go test ./internal/listingkit ./internal/productimage -count=1
go test ./tests -count=1
```

## Non-goals

- Moving all ListingKit result/preview/export code to a new package.
- Removing the legacy scalar JSON fields in this PR.
- Replacing Temporal, RabbitMQ, or the current task status lifecycle.
- Adding marketplace-specific image rules to root `internal/listingkit`.
- Adding a new product source or target-platform workbench.

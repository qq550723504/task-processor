# SHEIN Final Resolution Cache Design

## Problem

SHEIN republish currently remembers `SaleAttributeResolution` before all publish-time reconciliation is reflected in that resolution. A successful first publish can therefore use a complete final SKU payload while persisting an incomplete cache entry. On republish, the cache reports a hit and returns `resolved`, but one or more current SDS variant values have no real SHEIN `attribute_value_id`.

The observed case contains sizes `S`, `M`, `L`, `XL`, `2XL`, `3XL`, `4XL`, and `5XL`. The first publish submitted `5XL` with the manually confirmed SHEIN value `Petite GGG` (`1430561`), but the remembered `sku_value_assignments` omitted `5XL`. The next task hit that cache and failed completeness validation.

Size-chart data has a related boundary problem. The ordinary attribute cache remembers resolution metadata, but not the complete, publish-ready `preview_payload.size_attribute_list`. Republish therefore regenerates the size chart and may lose rows or header mappings that were present in the successful publish.

## Goals

- Persist complete, publish-confirmed sale-attribute value assignments.
- Persist the final size-chart rows needed to reproduce a successful republish.
- Safely consume existing incomplete cache entries without treating them as complete.
- Keep task-specific identifiers, images, prices, and supplier codes out of reusable resolution caches.
- Preserve current behavior for tasks without a usable cache.

## Non-Goals

- Caching the entire SHEIN submission package.
- Automatically inventing a SHEIN value ID for an unmapped source value.
- Reusing caches across different stores, categories, product identities, or source dimensions.
- Mutating existing database rows through a one-off migration.

## Design

### Final sale-attribute reconciliation

Before a successful publish is remembered, derive a cache candidate from the final package rather than storing the pre-publish resolution unchanged. Merge confirmed SKC and SKU sale attributes from the final publish representation into cloned `skc_value_assignments` and `sku_value_assignments`, keyed by normalized source values.

The reconciled resolution is cacheable only when every current source value for each selected sale-attribute dimension has a positive real `attribute_value_id`. A generic representative `SKUAttributes` value does not satisfy per-value completeness.

The existing cache identity remains scoped by store, category path, product identity, and normalized source dimensions. The change affects the cached value, not cache-key scope.

### Legacy cache compatibility

On cache read, validate the cached resolution against the current package source dimensions. A cached `resolved` status is insufficient by itself.

- If all current source values have valid assignments, return the hit.
- If an assignment is missing and a deterministic confirmed assignment is available in the current package, merge it and rewrite the upgraded cache entry.
- If any value remains unmapped, reject the entry and run the normal resolver/review path.

Legacy compatibility must never guess an ID from display text or select an arbitrary template option.

### Final size-chart cache

Extend the reusable attribute-resolution cache value with a cloned publish-ready size-chart snapshot containing the final `size_attribute_list`. The snapshot contains only SHEIN attribute IDs, related sale-attribute IDs/value IDs, and measurement values. It must not contain task IDs, supplier SKUs, images, prices, or submission metadata.

On cache read, restore the snapshot only after validating that every referenced sale-attribute value ID exists in the current reconciled sale-attribute mapping. If validation fails, discard the snapshot and run the existing size-chart generation path. A newly successful publish then replaces the old entry with the final snapshot.

### Ordering and persistence

The successful-publish remember path performs these operations in order:

1. Reconcile the final sale-attribute resolution from the publish-ready package.
2. Verify complete SKC/SKU source-value coverage.
3. Persist the reconciled sale-attribute cache.
4. Capture and persist the final size-chart snapshot with the attribute cache.

This ordering ensures the size chart is never cached against an incomplete sale-attribute mapping.

## Failure Handling

- An incomplete legacy sale-attribute cache is treated as a miss, not a fatal cache error.
- An invalid size-chart snapshot is ignored independently; valid category and ordinary attribute cache data may still be reused.
- Cache persistence failure does not change the outcome of an already successful remote publish, but remains observable through existing logging.
- Cache reads and writes operate on clones so reconciliation cannot mutate the live task result unexpectedly.

## Tests

Regression tests will model the production sequence:

1. A first task with sizes `S` through `5XL` has a publish-confirmed manual mapping for `5XL -> 1430561`.
2. Remembering the successful publish stores all eight normalized size assignments.
3. A second task with the same identity restores all eight assignments and does not require sale-attribute review.
4. The second task restores the same final size-chart row set as the first task.
5. A legacy cache missing `5XL` is rejected unless a deterministic confirmed assignment can upgrade it.
6. Cache entries do not cross store, category, product-identity, or source-dimension boundaries.
7. Invalid size-chart snapshots fall back to normal generation without discarding unrelated valid cache data.

Package-level tests will be added around resolver cache reconciliation and ListingKit's successful-publish remember boundary. Existing package tests plus `go vet` will provide broader verification.


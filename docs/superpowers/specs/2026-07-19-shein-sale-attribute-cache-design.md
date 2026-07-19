# SHEIN sale-attribute cache repair

## Problem

A successfully published ListingKit task can contain valid, manually confirmed
SHEIN sale attributes but fail to create a reusable `sale_attribute` cache
entry. Later tasks generated from the same SDS source then rerun LLM mapping
and can select an incompatible template attribute.

The published draft's `SKC.SaleName` is display/title data, not the source
value for the primary sale dimension. Reconciliation currently uses it as that
source value, so applicability validation cannot find the actual source value
(for example, `Color=white`). In addition, the cache identity includes a
downstream `-CE...` variant-SKU suffix even when `source_sds_sku` is the same.

## Design

1. Reconcile a published primary sale-attribute assignment from each SKC's SKU
   source attributes. This preserves the real source value used for grouping
   (`Color=white`) rather than using `SKC.SaleName`.
2. Normalize the sale-attribute cache identity to stable SDS identifiers. When
   a variant has `source_sds_sku`, that value is authoritative; volatile
   downstream variant-SKU suffixes must not create a separate cache key.
3. Retain the existing safety gate: a cache is saved or reused only when every
   source value has a concrete SHEIN attribute-value ID.

## Tests

- A resolved publication whose SKC sale name is a product title records the
  primary mapping under its SKU's source `Color` value and can be reused.
- Two equivalent products with the same `source_sds_sku` but one
  `variant_sku` carrying a `-CE...` suffix produce the same sale-attribute
  cache key.

## Non-goals

- Reusing incomplete LLM selections.
- Changing SDS baseline-cache semantics or database schema.

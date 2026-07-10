# SHEIN Time-Limited Multi-SKU Discount Validation

## Goal

Require every SKU in a SHEIN time-limited activity candidate to have an activity price strictly below 95% of that SKU's original price.

## Behavior

- Validate each SKU while building the time-limited `CreateActivity` request.
- A SKU is valid only when `activity_price < original_price * 0.95`.
- Equality at 95% is invalid.
- If any SKU is invalid, exclude the entire SKC from the activity request. Do not submit a partial SKU set.
- Keep the existing SKC-level discount validation as a defensive check.
- Do not modify or clamp configured activity prices automatically.
- Record a candidate-specific rejection reason containing the SKC, SKU, activity price, original price, and the strict 95% requirement.

## Data Flow

For each promotion product, resolve the SKU original and activity prices using the existing requested-price and calculated-price fallback chain. Validate the resolved pair before appending the SKC to `add_cost_and_stock_info_list`. When one SKU fails, append one filter reason for the SKC and continue with the next product.

## Error Handling

Missing or non-positive prices continue to follow the existing price-data handling. This change only adds the strict per-SKU discount boundary after both prices have been resolved.

## Tests

- Reject an SKC when one of multiple SKUs is exactly 95% of its original price.
- Reject an SKC when one SKU is above 95%, even if another SKU is below 95%.
- Accept a multi-SKU SKC when every SKU is strictly below 95%.
- Verify the rejection reason identifies the failing SKU and relevant prices.

## Scope

This change affects only SHEIN `TIME_LIMITED` request construction. It does not change the create-activity endpoint, request schema, regular promotion enrollment, or price calculation rules.

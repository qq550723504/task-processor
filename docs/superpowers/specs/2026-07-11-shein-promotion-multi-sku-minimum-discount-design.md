# SHEIN Promotion Multi-SKU Minimum Discount Design

## Goal

Support SHEIN promotion enrollment for products whose SKUs have different original and activity prices. The promotion API accepts one SKC-level `drop_rate`, so the submitted value must be the smallest discount supported by every valid SKU.

## Confirmed Business Rules

- `TIME_LIMITED` keeps SKU-specific original prices and SKU-specific activity prices.
- `PROMOTION` calculates a discount for every SKU and submits the smallest discount as the SKC-level `drop_rate`.
- A SKU discount is `(original_price - activity_price) / original_price`.
- API `drop_rate` remains the integer percentage produced by the existing validation and flooring behavior.
- If promotion participation is `BOTH`, the limited activity uses the minimum SKU discount and the regular activity uses one percentage point less.
- If any required SKU original price or activity price is missing or invalid, exclude the entire SKC instead of calculating from an incomplete SKU set.
- Do not average SKU original prices or activity prices.

## Root Cause

The enrollment refresh path currently overwrites every entry in `price_snapshot.sku_prices` with the product-level `supply_price`. This destroys real SKU price differences before enrollment. The promotion profit builder also averages SKU original and activity prices before calculating `drop_rate`, which can produce a discount deeper than the least-discountable SKU supports. A separate guard rejects promotion products with different SKU prices, preventing the correct calculation path from running.

## Data Flow

1. Preserve existing SKU-level prices when refreshing an enrollment candidate. The product-level `sale_price` may continue to use the current `supply_price` compatibility value, but `sku_prices[*].sale_price` must retain the synchronized SKU values.
2. Convert candidate SKU price snapshots and SKU cost overrides into `marketing.SkcInfo.SkuPriceInfoList` and `SkuCostPriceInfoList`.
3. Allow promotion products with different SKU prices to enter registration.
4. For `PROFIT` and `BREAKEVEN`, pair SKU original prices with SKU activity/cost values by SKU code, calculate each SKU's supported discount, and choose the minimum.
5. Keep the existing scalar SKC calculation only as a compatibility fallback when no SKU-level data exists. Do not use it when a partial SKU-level data set exists.
6. Keep `TIME_LIMITED` request construction SKU-specific and retain its strict per-SKU 95-percent validation.

## Components

### Candidate Refresh

`internal/listingkit/sheinsync/enrollment_service_candidates.go` will stop replacing every SKU snapshot price with the product-level supply price.

### Promotion Adapter

`internal/listingkit/sheinsync/activity_adapter.go` will no longer filter promotion candidates solely because their SKU prices differ. It will continue passing SKU prices and SKU costs to the marketing model.

### Promotion Pricing

`internal/shein/activity/registration_config.go` will calculate per-SKU discounts and select the minimum for both the direct candidate path and the attributes-backed profit path. The existing `BOTH` adjustment in `promotionConfigListForActivityType` remains responsible for regular minus one and limited unchanged.

## Error Handling

- Missing SKU code matches, non-positive original prices, or non-positive activity prices make the entire SKC ineligible for the promotion request.
- Log the SKC and failing SKU with the missing or invalid price type.
- Preserve existing fallback behavior for genuinely single-price products that have no SKU-level price/cost lists.

## Tests

- Candidate refresh preserves distinct SKU prices while updating the top-level compatibility price.
- Promotion adapter accepts a candidate with distinct SKU prices and passes SKU prices and costs through.
- Three SKUs with different supported discounts produce the smallest integer `drop_rate`.
- `BOTH` produces regular `minimum - 1` and limited `minimum`.
- A partial SKU price/cost match excludes the entire SKC.
- Existing single-price promotion and SKU-specific time-limited tests remain green.
- Store `177`, SKC `sh260625180761728097751` is represented by a regression fixture whose supported minimum is `39`; `BOTH` therefore produces regular `38` and limited `39`.

## Scope

This change does not alter SHEIN API endpoints or payload schemas. It does not rewrite existing enrollment history or automatically re-enroll the affected product. Operational reset or re-enrollment remains a separate explicit action after deployment.

# SHEIN Pricing and Promotion Semantics Design

## Goal

Make SHEIN promotion pricing distinguish true supply/cost data from retail sale
prices, so profit and breakeven decisions never silently treat a retail price as
cost while discount enrollment remains operable with an explicit fallback.

## Scope

This is SHEIN stabilization B1 only: synchronized supply price, promotion
pricing, multi-SKU cost/retail completeness, and fallback semantics. Resolution
cache and readiness/idempotency belong to later independent changes.

## Price sources

For a SKU, true supply price is selected in this order:

1. `USSupplyPrice`;
2. `SupplyPrice`;
3. `SupplyPriceInfo.SupplyPrice`;
4. the synchronized SKC supply price.

SKU cost remains preferred over SKC cost for profit and breakeven calculations.
Values with a non-positive amount are absent. Price values are never silently
substituted across currencies.

## Fallback policy

`PROFIT` and `BREAKEVEN` require true synchronized supply/cost data. If it is
missing for an SKC or SKU, that candidate is excluded before the SHEIN pricing
request and the result records a stable missing-cost reason. A retail sale price
must never become the cost used in these modes.

`DISCOUNT` uses the retail sale price to calculate the discounted customer price.
When the preferred-currency retail price is unavailable, it may use an available
retail price, then any positive retail price, with an explicit
`retail_price_fallback` reason. It does not claim that this value is supply cost.

## Acceptance criteria

- Multiple SKUs retain independent customer prices and independent true costs.
- Profit and breakeven exclude candidates with no true cost; no sale-price cost
  substitution occurs.
- Discount mode retains the documented retail fallback order and emits the
  fallback reason.
- A zero, missing, disabled, or currency-incompatible value cannot become a
  positive pricing input by accident.
- Focused deterministic tests cover all source priorities, multi-SKU pricing,
  breakeven, profit, discount fallback, and exclusion reasons.

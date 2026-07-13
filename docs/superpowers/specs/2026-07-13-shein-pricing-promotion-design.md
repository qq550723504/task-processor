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

## Missing-price policy

There is no fallback policy. `PROFIT` and `BREAKEVEN` require true synchronized
supply/cost data; `DISCOUNT` requires an available retail price in the requested
currency. A missing, disabled, zero, cross-currency, product-level, or other-SKU
value excludes that candidate before the SHEIN pricing request and records a
stable missing-price reason. No retail price may become cost, and no SKU may
borrow a price from an SKC or another SKU.

## Acceptance criteria

- Multiple SKUs retain independent customer prices and independent true costs.
- Profit and breakeven exclude candidates with no true cost; no sale-price cost
  substitution occurs.
- Discount mode excludes missing target-currency retail prices without fallback.
- A zero, missing, disabled, or currency-incompatible value cannot become a
  positive pricing input by accident.
- Focused deterministic tests cover all source priorities, multi-SKU pricing,
  breakeven, profit, discount fallback, and exclusion reasons.

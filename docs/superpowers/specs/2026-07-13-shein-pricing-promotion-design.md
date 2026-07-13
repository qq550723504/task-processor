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

Promotion request preparation uses an internal SKU pricing input with `SKU`,
`RetailPrice`, `CostPrice`, and `Currency`. It is not a marketplace DTO and
must not be serialized to SHEIN unchanged.

`RetailPrice` comes only from the matching SKU's available site price in the
requested currency. `CostPrice` comes only from the matching SKU's synchronized
`SkuCostPriceInfoList` entry in the requested currency. Values with a
non-positive amount are absent. SKC prices, other SKU prices, and other
currencies are never substituted.

## Missing-price policy

There is no fallback policy. `PROFIT` and `BREAKEVEN` require `CostPrice`;
`DISCOUNT` requires `RetailPrice`. A missing, disabled, zero, cross-currency,
product-level, or other-SKU value excludes that SKU before the SHEIN pricing
request and records a stable missing-price reason.

## Acceptance criteria

- Multiple SKUs retain independent customer prices and independent true costs.
- Profit and breakeven exclude candidates with no true cost; no sale-price cost
  substitution occurs.
- Discount mode excludes missing target-currency retail prices without fallback.
- A zero, missing, disabled, or currency-incompatible value cannot become a
  positive pricing input by accident.
- Focused deterministic tests cover all source priorities, multi-SKU pricing,
  breakeven, profit, discount fallback, and exclusion reasons.

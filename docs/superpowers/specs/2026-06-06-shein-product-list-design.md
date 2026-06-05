# SHEIN Product List Redesign

## Goal

Upgrade the SHEIN enrollment product tab from a sparse data table into a dense operations-style list that is closer to the SHEIN seller-center product list. The page should support fast scanning for product identity, pricing, stock, listing state, and key timestamps without requiring backend API changes.

## Scope

This redesign only applies to the `products` tab in the SHEIN enrollment store workbench.

In scope:

- Restructure the current synced-products table into a dense multi-field table row
- Show richer product identity fields
- Show pricing, stock summary, listing status, and time metadata together
- Add safe fallbacks for missing data
- Add component-level tests for the new rendering behavior

Out of scope:

- Backend schema or API changes
- Real sales metrics such as 7-day sales
- New filtering or sorting behavior
- Changes to the `costs`, `candidates`, or `runs` tabs beyond shared display helpers if needed

## Current State

The current product tab renders a minimal table with only a few columns:

- Product name
- SKC
- Shelf status
- Publish time
- Last sync time
- Sale price snapshot
- Effective cost price

This is functional, but it is much less useful than the reference SHEIN seller-center list because it does not present the row as an operational unit.

## Proposed UX

Keep the product tab as a table for desktop scanning, but redesign each row to contain grouped information blocks similar to an operations console.

Columns:

1. Product Info
   - Selection checkbox
   - Main image thumbnail
   - Product title
   - SPU
   - Supplier code

2. Primary Variant
   - Variant / sale name
   - SKC
   - Optional small inline status tag if helpful

3. 7-Day Sales
   - Placeholder `-` for now because no trusted backend field exists

4. Price
   - Sale price snapshot with currency symbol formatting
   - Effective cost price
   - Cost source (`manual`, `auto`, or `none`)

5. Inventory
   - Prefer parsed values from `inventory_snapshot`
   - Show total inventory and available inventory when parseable
   - Otherwise show a summarized fallback or `-`

6. Listing Status
   - Visual badge for `shelf_status`

7. Time
   - Created time
   - Publish time
   - First shelf time
   - Last sync time

## Data Mapping

The redesign must use existing frontend data only.

Product identity:

- `main_image_url`
- `product_name_multi`
- fallback `spu_name`
- `spu_code`
- `supplier_code`

Primary variant:

- `sale_name`
- `skc_name`
- fallback `skc_code`

Price:

- `price_snapshot`
- `effective_cost_price`
- `cost_price_source`

Inventory:

- `inventory_snapshot`

Status:

- `shelf_status`

Time:

- `created_at`
- `publish_time`
- `first_shelf_time`
- `last_sync_at`

## Missing Data Strategy

The page must not invent data.

- If a field is absent, render `-`
- If `inventory_snapshot` is parseable, show structured values
- If `inventory_snapshot` is not parseable, show a readable fallback text instead of failing
- 7-day sales remains `-`
- Missing image falls back to a neutral thumbnail placeholder

## Rendering Rules

- Preserve table layout for scan efficiency on desktop
- Use compact multi-line cells instead of exploding into many narrow columns
- Clamp long product titles so rows stay readable
- Keep typography hierarchy clear: title first, metadata second, diagnostics third
- Use subtle badges for status and cost source
- Avoid excessive color; status should stand out without overwhelming the row

## Testing

Add or update component tests to cover:

- Product info block renders thumbnail, title, SPU, and supplier code
- Primary variant block renders sale name and SKC
- Price block renders sale price, effective cost price, and cost source
- Inventory block handles parseable and non-parseable snapshots
- Time block renders multiple timestamps together
- Missing fields safely fall back to `-`

## Risks

1. `inventory_snapshot` may not have a stable structure across rows.
   Mitigation: implement best-effort parsing with graceful fallback.

2. More content may make rows visually heavy.
   Mitigation: group metadata into clear stacked blocks instead of adding many separate columns.

3. Existing text encoding issues are present in some files.
   Mitigation: normalize touched UI strings to plain readable text while editing only the relevant files.

## Recommended Implementation Plan

1. Add or update tests that describe the denser row layout
2. Introduce small presentation helpers for:
   - price snapshot formatting
   - cost source labeling
   - inventory snapshot parsing / formatting
   - timestamp labeling
3. Refactor the synced-products table into grouped multi-line cells
4. Verify typecheck and targeted tests


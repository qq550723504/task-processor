# Historical Platform Migration Inventory

## Goal

This inventory turns the platform convergence strategy into a migration-cost
view. It is intentionally not a migration plan. Its job is to make the next
platform-boundary slice easier to choose without re-scanning the same historical
packages every time.

Snapshot date: 2026-06-17.

## Current Scale

| Package | Go files | Test files | Cost signal |
| --- | ---: | ---: | --- |
| `internal/shein` | 329 | 73 | Highest cost: broad API, pipeline, submit, content, category, pricing, and product responsibilities. |
| `internal/temu` | 245 | 7 | High cost: broad platform runtime with relatively thin direct test coverage. |
| `internal/amazon` | 70 | 8 | Medium cost: more concentrated crawler/listing/model/pipeline responsibilities. |

The file counts come from `git ls-files` and are meant as a coarse ownership
signal, not a quality metric.

## Cost Tiers

### Tier 1: Low-risk facade or adapter slices

These are good first candidates because they usually involve narrow delegation
or naming cleanup:

- package-level facades that only forward to a clearer owner
- platform registration descriptors under `internal/platforms/*`
- status or reason mapping helpers with existing tests
- compatibility aliases that already have a documented replacement

### Tier 2: Medium-risk policy and payload slices

These should move only when the owning module is clear and tests can describe
the behavior:

- publishing payload shaping
- submission preparation and validation
- category, attribute, pricing, and product-field rules
- workspace/editor projections that are platform-specific

Preferred convergence homes:

- `internal/publishing/*`
- `internal/marketplace/*/publishing`
- `internal/marketplace/*/workspace`
- product modules such as `internal/catalog` or `internal/asset` when the rule
  is not marketplace-specific

### Tier 3: High-risk runtime slices

These should not be moved as opportunistic cleanup:

- pipeline runners and task processors
- scheduler and worker lifecycle behavior
- external API client construction and auth/session behavior
- code paths coupled to retry, persistence, or remote state reconciliation

These require a specific design note, a narrow migration slice, and focused
tests before movement.

## Package Notes

### `internal/shein`

`internal/shein` is the broadest historical platform package. It mixes API
client code, pipeline runtime, product and attribute rules, content handling,
submit preparation, pricing, validation, and scheduler behavior.

Near-term direction:

- keep existing runtime and client-heavy code in place
- prefer `internal/publishing/shein` for new publishing semantics
- prefer marketplace workspace packages for editor/workspace behavior
- avoid adding new SHEIN rules to root `internal/listingkit`

### `internal/temu`

`internal/temu` has a broad platform surface with low direct test count relative
to size. It includes API packages, pipeline/processor entrypoints, scheduler,
rules, pricing, product, template, and store areas.

Near-term direction:

- do not start with broad pipeline movement
- first identify stable policy/payload helpers that can be tested in place
- keep new TEMU publishing semantics out of app assembly and ListingKit facade
- add tests around a slice before extracting it

### `internal/amazon`

`internal/amazon` is smaller and more concentrated around crawler/listing/model
and pipeline concerns. It is a better candidate for inventory-driven follow-up
than for immediate restructuring in this branch.

Near-term direction:

- keep crawler/runtime behavior stable
- move only clearly reusable product or listing facts toward product/catalog
  modules
- avoid using Amazon as the template for all platform convergence until SHEIN
  and TEMU ownership is clearer

## Next Slice Candidates

Good candidates for the next framework slice:

1. Keep extending import or structure guards that keep `internal/platforms/*`
   thin. Current guard:
   `TestPlatformModulesDoNotImportBusinessOrHTTPAssemblyPackages`.
2. Identify one SHEIN publishing helper still living in a root or runtime-heavy
   package and move it only if tests can pin the behavior.
3. Add a focused TEMU inventory before moving code, because test coverage is
   thin relative to package size.
4. Catalog legacy facade-only files that can be retired without touching runtime
   behavior.

## Non-goals

This inventory does not authorize:

- large moves out of `internal/shein`, `internal/temu`, or `internal/amazon`
- renaming package families for cosmetic consistency
- moving pipeline, scheduler, or external client runtime code without a design
  note and focused tests
- treating `internal/platforms/*` as the home for marketplace business rules

## Review Questions

Before starting a historical platform migration slice, ask:

1. Which cost tier is this change in?
2. What behavior proves the current owner is wrong?
3. What test will fail if the moved behavior regresses?
4. Is the target package already named in
   `docs/architecture/platform-boundary-strategy.md`?
5. Does this reduce future migration cost, or just move code to a nicer-looking
   directory?

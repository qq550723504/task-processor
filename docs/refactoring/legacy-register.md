# Legacy Register

> Status: Active migration register  
> Policy: `docs/refactoring/legacy-hard-cut-policy.md`  
> Last reviewed: 2026-09-05  
> Repository scan baseline: `main @ 6e3b87208e2c4e51039b95145a2377edfbb9b1cb`

This register records known obsolete or partially migrated internal abstractions so implementation agents do not independently invent compatibility work.

Current repository policy has only two legacy decisions:

- `EXTRACT` — reusable behavior is moved/reimplemented under the current owner; the new architecture must stop depending on the legacy owner.
- `RETIRE` — obsolete design is not extended or internally supported; remove it after cutover.

There is currently **no Compatibility category**. A directory or type named `compatibility`, `legacy`, `bridge`, `adapter`, or `facade` does not create an exception to this rule.

## 1. Repository scan summary

### Physically retired — keep absent

The following old production package roots were not present on the scan baseline and must remain absent:

- `internal/productenrich`
- `internal/productimage`
- `internal/catalog`
- `internal/asset`
- `internal/imageasset`

Their current owners exist under:

```text
internal/product/catalog
internal/product/asset
internal/product/enrichment
internal/product/image
internal/product/sourcing
```

Historical plans, test names, baselines, and architecture guards may still mention the retired names. Such references are evidence or anti-regression guards, not permission to recreate the packages.

### Active legacy hotspots

The scan found four high-value areas that still affect production paths:

1. `internal/compatibility/listingkit`
2. root `internal/listingkit`
3. `internal/tenantbridge`
4. `web/listingkit-ui` old Task-first / ListingKit product projection

These areas are drain targets. New Product, Marketplace, Agent, Tool, BusinessTask, Store, and Console work must not add new dependency on them merely because they already exist.

## 2. Active register

| Legacy area / abstraction | Decision | Observed state | Reusable behavior | Current owner / destination | Exit condition / related work |
| --- | --- | --- | --- | --- | --- |
| `internal/productenrich` task / queue / worker / API architecture | RETIRE | Production package root is absent on scan baseline | enrichment analysis, normalization, scoring, proposal logic that remains independently valid | `internal/product/enrichment` + AI Capability where applicable | Keep old package absent. Do not recreate workflow/task ownership. |
| `internal/productimage` task / queue / worker / API architecture | RETIRE | Production package root is absent | provider-neutral image analysis/transformation logic | `internal/product/image` + ImageAgent | Keep old package absent. ImageAgent remains workflow/budget/retry/recovery/approval owner. |
| old `internal/catalog` | RETIRE | Production package root is absent | canonical product behavior already moved | `internal/product/catalog` | Keep absent; new callers use current Product contract. |
| old `internal/asset` | RETIRE | Production package root is absent | approved asset behavior already moved | `internal/product/asset` | Keep absent; no second asset authority. |
| old `internal/imageasset` | RETIRE | Production package root is absent | independently valid image/asset helpers only | `internal/product/image` or `internal/product/asset` by ownership | Keep absent; do not recreate as convenience package. |
| `internal/compatibility/listingkit` as a long-term facade / landing zone | RETIRE | Directory still exists and its README describes backward-compatible facades | only behavior needed by current owners; no compatibility ownership itself | `internal/listing/*`, `internal/marketplace/*`, `internal/product/*`, `internal/integration/*`, `internal/app/*` | #29/#30/#300 drain callers and delete shells. No new consumers. |
| `internal/compatibility/listingkit/preview_adapter.go` | RETIRE | Code search found only the implementation and its own test; no production caller found | none required after current `internal/listing/preview` projection exists | `internal/listing/preview` | #300: confirm with tests, then delete adapter and its dedicated test. |
| `internal/compatibility/listingkit/sourcehandoff/a1688` | EXTRACT | Active production wiring in `internal/app/httpapi`; exposes `/api/v1/product-sourcing/1688/listingkit/tasks` and creates legacy ListingKit task/request objects | SourceEnvelope construction, publication/idempotency identity, bounded product key, verified identity checks, source/store access checks, useful error semantics | `internal/product/sourcing` + Store/Organization/Application boundaries + downstream Listing seam | #30 performs `EXTRACT -> RETIRE`; after controlled 1688 → ProductSnapshot/ApprovedAsset → readiness cutover, remove route/wiring/tests and legacy handoff. No fallback. |
| root `internal/listingkit` mixed ownership | EXTRACT | Still a large active production orchestration/API/runtime owner | deterministic marketplace rules, listing orchestration primitives, readiness/submission semantics, valid transport/runtime behavior | `internal/listing/*`, `internal/marketplace/*`, `internal/product/*`, `internal/integration/*`, `internal/app/*` | #29 drains ownership. After each caller cutover, retire old path; root ListingKit is not a permanent facade. |
| marketplace rules still owned by root ListingKit | EXTRACT | Remaining rule/policy ownership is part of #29 hard-cut | deterministic platform rules/policies/readiness logic | `internal/marketplace/*` / stable Listing seams | Move rule to current owner and switch callers. Do not wrap the old rule owner. |
| `internal/tenantbridge` Organization ↔ legacy Yudao numeric tenant mapping | EXTRACT | Active GORM-backed production bridge used by ListingKit, SHEIN login, 1688 crawler/admin and other legacy paths | only the ownership/mapping facts required to migrate existing rows/callers safely | current Organization/identity ownership + each domain's current persistence boundary | Freeze consumer count; no new caller. Create dedicated drain/cutover work, migrate callers/data ownership, then RETIRE the bridge. |
| `web/listingkit-ui` Task-first / CreateListingKitTask product projection | RETIRE | Still the only current web app and serves existing commercial paths | reusable UI primitives or domain projections only when aligned with final Figma IA | #298 / final Figma Product Projection; Store Center / AI Workbench / current product pages | Existing released paths may run while replacement is incomplete, but new product design targets Figma directly. Retire old pages after page-level cutover; no BusinessTask ↔ legacy Task dual product model. |
| Generic independent Listing Workspace (#27) | RETIRE | Backlog abstraction already closed | independently valid Product/Listing UI behavior only if required by current IA | final Product Projection pages defined by Figma authority | Do not revive a generic top-level Listing Workspace. |
| Old Product Task Center / unified internal Task Dashboard (#35) | RETIRE | Old issue closed; final UI now has a different BusinessTask Center | diagnostics may be projected if useful | #298 BusinessTask Product Projection; internal execution remains with Workflow/Task owners | BusinessTask is a new user-facing object, not a renamed legacy Task. No bidirectional sync. |
| Task-first Product architecture | RETIRE | Superseded by Product/Listing facts + BusinessTask | none as an ownership model | Product/Listing domain facts + BusinessTask projection | Internal Task/Workflow stays execution infrastructure. |
| Platform-specific TEMU/Amazon Workbench expansion model | RETIRE | Superseded product projection | marketplace clients, rules, submission adapters and other valid capability code | shared Marketplace/Listing capability; Store Center / AI Workbench projection | #31/#32 expand capabilities without copying Workbenches. |
| Independent top-level Product Center / Listing Center assumption | RETIRE | Conflicts with final Figma IA | Product/Listing facts and useful screens only | Figma `31:463`; `docs/product/final-ui-ia-authority.md` | Domain facts remain; obsolete navigation assumptions do not. |
| Old tenant quota / task-limit resource model (#42) | RETIRE | Superseded | migration facts needed to preserve valid current value only | Resource Ledger / Store Service / entitlement owners | Do not restore task-count quota as resource authority. |
| “Build ZITADEL tenant/role model from scratch” backlog abstraction (#39) | RETIRE | Superseded by current identity/Organization implementation | existing verified identity/organization behavior | current ZITADEL + Organization/RBAC boundaries | Remaining work is release/staging/production acceptance. |
| Separate Product/Asset/Listing fact source introduced for Agent convenience | RETIRE | Forbidden architectural duplication | none | existing canonical domain owners | Agent output is Proposal/trace, never a second business fact source. |
| Parallel Agent RBAC / Tool Registry / durable retry/state-machine ownership | RETIRE | Forbidden architectural duplication | none | current authorization, #133 Tool Registry, Temporal/queue/submission owners | Do not isolate Agent code by cloning platform contracts. |

## 3. Current drain rules

### `internal/compatibility/*`

- Existing production callers are migration debt, not precedent.
- New code must not add another consumer.
- A reusable behavior is moved to its current owner first; callers switch there directly.
- Do not create a new `internal/compatibility/<domain>` package to solve a new feature.
- Consumer count should monotonically decrease.

### `internal/tenantbridge`

- Do not add new consumers of legacy numeric tenant resolution.
- New schemas and new domain contracts use current Organization identity directly.
- Existing callers are drained by their owning domain, not hidden behind a new facade.
- A one-time migration/backfill or explicit cutover is preferred over permanent dual-read/dual-write.

### `web/listingkit-ui`

- Existing commercial pages may continue operating until their replacement is actually ready; this is current runtime reality, not an architecture compatibility requirement.
- New product IA follows Figma and `docs/product/final-ui-ia-authority.md`.
- Do not reproduce old Task-first semantics inside BusinessTask just to make both models align.

## 4. How to use this register

Before reusing a legacy package/service/type:

1. Find the matching row.
2. Follow its `EXTRACT` or `RETIRE` decision.
3. For `EXTRACT`, name the exact reusable behavior and current owner before coding.
4. For `RETIRE`, do not add a new caller, wrapper, fallback, or synchronization path.
5. If useful behavior is not covered, update this register before propagating the dependency.
6. Do not create a third `compatibility` state locally.

If code turns out to be reusable, tests should be rewritten against the current owner/contract. If a test only proves an obsolete implementation detail, retire the test with the code.

## 5. New discoveries

When a new legacy area is discovered, record:

```text
Legacy area:
Decision: EXTRACT | RETIRE
Observed state:
Reusable behavior:
Current owner:
Cutover/deletion condition:
Related issue/PR:
```

A future externally required compatibility exception, if one is ever proven necessary, must be documented separately and must not silently become a new register category.

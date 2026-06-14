# Module Target Mapping

> Status: working mapping from current package areas to target domains. This is a migration aid, not a guarantee that every listed package is ready for an immediate move.

## 1. Purpose

This document maps the current major project areas to the approved target domains:

- `listing`
- `marketplace`
- `product`
- `integration`
- `platform`
- `app`
- `compatibility`
- `shared`

Use it before broad refactoring so package moves follow a stable direction.

## 2. Mapping Table

| Current area | Current role | Target home | Notes |
| --- | --- | --- | --- |
| `internal/listingkit` | legacy listing orchestration and API shell | `internal/listing/*` plus `internal/compatibility/listingkit` | shrink into compatibility facade over time |
| `internal/listingadmin` | listing-admin behavior | `internal/listing/settings` or `internal/app/httpapi` depending on content | split runtime-facing handlers from business logic |
| `internal/listingsubscription` | listing-related subscription behavior | `internal/listing/task` or `internal/listing/workflow` | keep orchestration in listing domain |
| `internal/platformtask` | task execution helpers | `internal/listing/task` or `internal/platform/temporal` depending on ownership | split business task semantics from runtime task execution |
| `internal/taskstatus` | task status models and helpers | `internal/listing/task` or `internal/shared` | keep only generic status primitives in shared |
| `internal/catalog` | canonical product facts | `internal/product/catalog` | direct migration candidate |
| `internal/asset` | reusable asset facts | `internal/product/asset` | direct migration candidate |
| `internal/imageasset` | image-asset helpers | `internal/product/image` or `internal/product/asset` | decide by ownership of behavior |
| `internal/productimage` | product image behavior | `internal/product/image` | direct migration candidate |
| `internal/product` | product domain logic | `internal/product/*` | split by clearer product subdomain |
| `internal/productenrich` | product enrichment | `internal/product/ai` or `internal/product/sourcing` | decide based on AI versus sourcing normalization role |
| `internal/pricing` | pricing helpers | `internal/product`, `internal/marketplace/*`, or `internal/listing/submission` | classify by whether pricing is product-level, platform-specific, or listing-flow-specific |
| `internal/sds` | product or asset generation support | `internal/product/asset`, `internal/product/image`, or `internal/listing/studio` | split generation runtime from product facts |
| `internal/prompt` | prompt definitions | `internal/product/ai` or `internal/integration/openai` | prompt ownership should track business use |
| `internal/promptmgmt` | prompt management | `internal/product/ai` or `internal/app/runtime` | split management UI and domain behavior |
| `internal/crawler` | legacy crawler implementations | `internal/integration/crawler/*` | split by source such as Amazon and 1688 |
| `internal/amazon` | mixed Amazon logic | `internal/marketplace/amazon/*` and `internal/integration/crawler/amazon` | separate listing-target behavior from source-crawler behavior |
| `internal/amazonlisting` | Amazon listing behavior | `internal/marketplace/amazon/publishing` or `internal/listing/export` | classify by platform rule versus listing orchestration |
| `internal/shein` | mixed SHEIN logic | `internal/marketplace/shein/*` and `internal/integration/shein` | split API client, model, publishing, and workspace |
| `internal/publishing/shein` | legacy SHEIN publishing compatibility shell | `internal/marketplace/shein/publishing` | keep thin; new rules should land in marketplace |
| `internal/workspace/shein` | legacy SHEIN workspace compatibility shell | `internal/marketplace/shein/workspace` | keep thin; new rules should land in marketplace |
| `internal/sheinlogin` | SHEIN login adapters | `internal/integration/shein` | authentication adapter, not listing owner |
| `internal/sheinloginmanaged` | managed SHEIN login integration | `internal/integration/shein` or `internal/platform/authz` | decide by whether it is external adapter or runtime auth support |
| `internal/temu` | mixed TEMU logic | `internal/marketplace/temu/*` and `internal/integration/temu` | follow the SHEIN split pattern |
| `internal/platforms` | mixed platform abstractions | `internal/marketplace/*` or `internal/listing/*` | only keep generic cross-platform abstractions in listing |
| `internal/workspace` | generic workspace behavior | `internal/listing/studio` or `internal/marketplace/*/workspace` | split generic orchestration from platform-specific rules |
| `internal/publishing` | generic and platform publishing support | `internal/marketplace/*/publishing` and `internal/listing/submission` | separate platform rules from listing orchestration |
| `internal/app` | runtime assembly | `internal/app/*` | keep as runtime assembly only |
| `internal/httpbootstrap` | HTTP bootstrap helpers | `internal/app/httpapi` | merge toward runtime assembly |
| `internal/httproute` | route registration | `internal/app/httpapi` | route-only ownership |
| `internal/taskrpcapi` | task RPC API surface | `internal/app/httpapi` or `internal/app/runtime` | decide by transport ownership |
| `internal/infra` | runtime infra and external clients | `internal/platform/*` and `internal/integration/*` | split infra by runtime versus external-adapter role |
| `internal/platformbase` | shared platform base helpers | `internal/platform/*` or `internal/shared` | keep only truly generic primitives in shared |
| `internal/authz` | authorization support | `internal/platform/authz` | runtime support domain |
| `internal/kernel` | cross-cutting bootstrap/core helpers | `internal/platform/*` or `internal/shared` | classify carefully to avoid becoming a dumping ground |
| `internal/core` | cross-cutting core helpers | `internal/shared` or a clearer owning domain | only keep stable primitives |
| `internal/shared` | shared utilities | `internal/shared` | keep small and disciplined |
| `internal/validation` | validation primitives | `internal/shared/validation` | direct migration candidate |
| `internal/state` | generic state handling | `internal/shared` or `internal/platform` | depends on whether it is runtime state or business state |
| `internal/processor` | process orchestration | `internal/app/worker`, `internal/listing/workflow`, or `internal/platform/queue` | split by worker bootstrap versus business orchestration |
| `internal/pipeline` | cross-domain pipeline logic | `internal/product/sourcing`, `internal/listing/workflow`, or `internal/platform` | classify by business ownership before moving |
| `internal/ports` | interfaces and ports | local owning domains | avoid one global ports package over time |
| `internal/domain` | legacy domain aggregation | local owning domains | decompose into product, listing, marketplace, shared |
| `internal/model` | shared or mixed DTO/model package | local owning domains | move models close to the business owner |
| `internal/scheduler` | scheduling runtime | `internal/app/worker` or `internal/platform/queue` | keep runtime concerns out of business domains |
| `internal/tenantbridge` | tenant bridge logic | `internal/platform/authz`, `internal/app/runtime`, or `internal/shared` | decide by whether it is runtime tenancy or business tenancy |
| `internal/pkg` | generic helpers | `internal/shared` or more explicit owners | reduce generic catch-all usage |
| `internal/zitadelprovision` | Zitadel integration | `internal/integration` or `internal/platform/authz` | depends on runtime versus external-adapter ownership |

## 3. Explicit Crawler Guidance

The project should treat Amazon and 1688 crawling as sourcing inputs.

Target placement:

```text
internal/integration/crawler/amazon
internal/integration/crawler/a1688
internal/product/sourcing
```

Rules:

- crawlers own extraction adapters
- `product/sourcing` owns normalization and handoff
- marketplace publishing packages do not own crawler extraction logic

Immediate write-path rule:

- if the work is about fetching, browser control, anti-bot adaptation, or raw page extraction, prefer `internal/integration/crawler/{amazon,a1688}`
- if the work is about converting raw source output into reusable product facts, assets, or enrichment-ready models, prefer `internal/product/sourcing`
- if the work is about using sourced facts inside listing generation or publishing, prefer the owning `listing` or `marketplace` package rather than pushing that logic back into crawler adapters

Current checkpoint:

- the next preferred refactor direction is to inspect crawler/source boundaries and extract only one small normalization or source-identity seam into `internal/product/sourcing`,
- source identity and source request normalization now live in `internal/product/sourcing`,
- source result alignment should also live in `internal/product/sourcing`, while crawler adapters keep returning raw execution results,
- do not move crawler execution/runtime behavior into product packages,
- do not route sourced product normalization through root `internal/listingkit`.

## 4. Explicit `listingkit` Guidance

The current `internal/listingkit` area should be split conceptually into:

- `internal/listing/*` for real listing orchestration ownership
- `internal/compatibility/listingkit` for retained compatibility shells

Do not keep adding new business ownership to legacy mixed files just because the old path is already imported.

Immediate write-path rule:

- use `internal/listingkit` only for bounded compatibility, orchestration, or extraction-prep work that does not yet have a fully extracted owner
- if the real owner already exists under `internal/listing`, `internal/marketplace`, `internal/product`, `internal/integration`, or `internal/compatibility/listingkit`, new code should land there first
- treat broad root files under `internal/listingkit` as shrink targets, not as preferred extension points

## 5. Immediate Landing Zones

Use this table when the target architecture exists but the legacy package is still present.

| New work type | Preferred landing zone now | Avoid defaulting to |
| --- | --- | --- |
| Amazon source crawler adapter work | `internal/integration/crawler/amazon` | `internal/crawler/amazon`, mixed `internal/amazon` packages |
| 1688 source crawler adapter work | `internal/integration/crawler/a1688` | `internal/crawler/alibaba1688` |
| Source normalization and handoff | `internal/product/sourcing` | `internal/crawler/*`, `internal/listingkit` |
| Listing compatibility bridge | `internal/compatibility/listingkit` | new root-level `internal/listingkit` mixed files |
| Marketplace publishing rules | `internal/marketplace/<platform>/publishing` | `internal/listingkit`, generic `internal/publishing` if rule is platform-specific |
| Marketplace workspace/editor rules | `internal/marketplace/<platform>/workspace` | `internal/listingkit`, generic `internal/workspace` if rule is platform-specific |
| Runtime assembly and route/worker wiring | `internal/app/httpapi`, `internal/app/runtime`, `internal/app/worker` | business packages with embedded bootstrap logic |
| External API adapter work | `internal/integration/<system>` | `internal/listingkit`, `internal/product`, or generic infra packages |

## 6. Migration Usage Notes

Use this mapping as follows:

1. before moving a file, identify its primary business owner
2. if the file mixes multiple owners, split it first
3. if ownership is still unclear, document the ambiguity instead of forcing a move
4. update this table when a major area has been normalized

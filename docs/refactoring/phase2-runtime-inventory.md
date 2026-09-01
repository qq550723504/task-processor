# Phase 2 runtime convergence inventory

This inventory records the pre-migration dependency debt for Phase 2. Counts are
ceilings for one-way convergence, not target package sizes and not approval for
the listed dependencies to remain. A package that follows a relocated runtime
temporarily still owes a domain-local port in its assigned later phase.

## Final closure inventory

The closure guard uses fresh `go list` results from the final Phase 2 tree.
These are enforced ceilings: later changes may keep or lower them, but must
never increase them.

| Legacy root | Production Go files | Internal importer packages |
| --- | ---: | ---: |
| `core` | 46 | 134 |
| `infra` | 16 | 4 |
| `crawler` | 134 | frozen by file count |

| Relocated concrete package | Importer ceiling |
| --- | ---: |
| `core/logger` | 82 |
| `platform/logging` | 9 |
| `platform/database` | 21 |
| `platform/redis` | 8 |
| `platform/queue/rabbitmq` | 18 |
| `platform/workerpool` | 23 |
| `integration/openai` | 28 |
| `integration/geminiimage` | 1 |
| `integration/grsai` | 2 |
| `integration/s3` | 4 |
| `integration/httpimage` | 8 |

The reviewed plan's proposed 8/20 values were stale arithmetic, not valid
targets. `platform/logging` has one intentional same-platform workerpool
consumer in addition to app/config/facade wiring, so its final value is 9.
`platform/database` has 21 because `internal/app/schema/productlisting` is the
Goose migration owner introduced after the earlier database count. Removing
either dependency merely to hit 8/20 would invert approved ownership. The
lower natural values, `core/logger` 82 and `integration/s3` 4, replace the
plan's higher draft ceilings.

The retirement set contains the 19 package paths enumerated by the executable
guard. The plan's prose called this set “20”, but its path list contains 19;
`internal/pkg/timeout` remains a live package and `internal/infra/clients` is
not banned as a whole. No package was invented or retired to satisfy that
counting error.

## Initial baseline

| Root | Production Go files | Test files | Go packages | Internal importers |
| --- | ---: | ---: | ---: | ---: |
| `internal/core` | 58 | 22 | 5 | 145 |
| `internal/infra` | 68 | 35 | 14 | 75 |
| `internal/platform` | 0 | 0 | 0 | 0 |
| `internal/integration` | 10 | 13 | 4 | measured per slice |
| `internal/crawler` | 134 | 51 | 4 | frozen pending product/marketplace ports |

The architecture tests freeze `core`, `infra`, and `crawler` production-file
counts and the direct internal importer counts for `core`, `infra`, and
`core/logger`. A reduction is expected; an increase is a regression.

## Source directory classification

### `internal/core`

| Current directory | Approved disposition |
| --- | --- |
| `core/config` | Split, do not move wholesale. Generic loading, merging, paths, defaults application, and source selection move to `platform/config`; Amazon, AI, listing, processor, product-image, and other business option types stay with their consuming domains until their phases. |
| `core/errors` | Review each error for ownership. Truly neutral error helpers can move to `shared/errors`; `productjson_errors.go` belongs to the product review in Phase 3. |
| `core/lifecycle` | Runtime component management belongs to app-owned lifecycle assembly, with only domain-neutral mechanics under `platform` if needed. |
| `core/logger` | Move to `platform/logging`; app selects file/stdout policy and owns shutdown. |
| `core/metrics` | **Retain for now.** These are business task and SHEIN metrics, not generic observability. Move them with the listing/marketplace owners in Phases 4-5. |

### `internal/infra`

| Current directory | Approved disposition |
| --- | --- |
| `infra/auth` | External identity/session adapter; move only after organization ports exist, to `integration/organization` in Phase 7. |
| `infra/clients` | Container only; retire after its provider subpackages move. |
| `infra/clients/openai` | OpenAI adapter under `integration/ai/openai`; business callers first receive agent/product/listing-local ports. |
| `infra/clients/geminiimage` | Image-provider adapter under `integration/image/gemini`; callers receive product-image/listing-local ports. |
| `infra/clients/grsai` | Image-provider adapter under `integration/image/grsai`; callers receive product-image/listing-local ports. |
| `infra/database` | Database connection/runtime and migrations move to `platform/database`; domain repositories do not move with it. |
| `infra/httpx` | Mixed transport package. Generic response/logging/metrics mechanics belong to app HTTP assembly; crawler handlers remain frozen until product/marketplace ports exist. |
| `infra/lock` | Domain-neutral memory/Redis locking runtime moves to `platform/lock`, constructed by app. |
| `infra/metrics` | RabbitMQ consumer-to-business-metric bridge; app wiring plus the owning listing/marketplace metric contract, not generic platform observability. |
| `infra/monitoring` | Process health and technical telemetry move to `platform/observability`; app owns registration and lifecycle. |
| `infra/rabbitmq` | Queue runtime moves to `platform/queue/rabbitmq`; app owns connection, consumer registration, and shutdown. |
| `infra/redisclient` | Redis runtime moves to `platform/redis`; app owns construction and shutdown. |
| `infra/resilience` | Provider-neutral runtime retry/rate-limit/breaker mechanics move to `platform/resilience`; domains expose policy inputs rather than importing the implementation. |
| `infra/storage` | S3/object-storage adapter moves to `integration/objectstore/s3`; consuming domains define object-store ports and app wires them. |
| `infra/worker` | Worker-pool runtime moves to `platform/workerpool`; domains expose submit/queue-local contracts and app wires them. |

### Technical `internal/pkg`

| Current directory | Approved disposition |
| --- | --- |
| `pkg/appenv` | App runtime metadata belongs under `app`; logging dependency is rewired to `platform/logging`. |
| `pkg/cache` | Review as a domain-neutral in-memory primitive, then move to `shared/cache` or the single owning domain; do not preserve a generic dumping ground. |
| `pkg/downloader` | **Retain pending owning-domain review** (product/product-image in Phase 3 and marketplace in Phase 4). |
| `pkg/fileio` | Small file primitive can move to `shared/fileio` only after removing runtime logging policy. |
| `pkg/goroutine` | Runtime concurrency mechanics move with `platform/workerpool` or app lifecycle. |
| `pkg/hashx` | Stable semantic-free hashing primitive moves to `shared/hashx`. |
| `pkg/httpapicmd` | Thin app HTTP command shim moves into `app` and is deleted after callers use the app entry point. |
| `pkg/httpclient` | Outbound transport support belongs inside `integration` adapter support; it must not become a domain-visible client. |
| `pkg/imagex` | **Retain pending owning-domain review** in product/product-image and marketplace phases. |
| `pkg/jsonx` | **Retain pending owning-domain review**; LLM JSON behavior is not presumed universally shared. |
| `pkg/mathx` | Stable semantic-free arithmetic primitive moves to `shared/mathx`. |
| `pkg/perf` | Technical timing/measurement moves to `platform/observability`. |
| `pkg/ptr` | Stable semantic-free pointer helpers move to `shared/ptr` if still justified by callers. |
| `pkg/recovery` | Runtime panic recovery moves to `platform/lifecycle`; app selects process policy. |
| `pkg/safeimagehttp` | Image HTTP adapter support moves under `integration/image`; direct-importer debt is frozen below. |
| `pkg/skugen` | **Retain pending owning-domain review** (marketplace/product SKU ownership). |
| `pkg/strx` | Stable semantic-free string primitives move to `shared/strx`; business cleaners stay with their owner. |
| `pkg/timeout` | Stable context timeout primitive moves to `shared/time`; app/platform retain lifecycle policy. |
| `pkg/timex` | Stable semantic-free formatting primitive moves to `shared/time`. |
| `pkg/types` | **Retain pending owning-domain review**; flexible values must be assigned to the contract that consumes them. |
| `pkg/watermark` | **Retain pending owning-domain review** in the product-image/marketplace phases. |

The explicit retention set is
`pkg/{downloader,imagex,jsonx,skugen,types,watermark}`. Retention is temporary
review status, not final shared ownership.

## Crawler freeze

| Slice | Production files | Test files | Disposition |
| --- | ---: | ---: | --- |
| `crawler/alibaba1688` | 36 | 16 | Freeze; extract product sourcing and marketplace-specific ports before Phase 3/4 movement. |
| `crawler/amazon` | 77 | 27 | Freeze; extract product sourcing and Amazon marketplace ports before Phase 3/4 movement. |
| `crawler/fetcher` | 3 | 2 | Freeze; queue/worker orchestration moves to app after a product sourcing contract exists. |
| `crawler/shared` | 18 | 6 | Freeze; split browser adapter support from business request/model behavior only after consumers have local ports. |

These packages currently import business config/models, app ports,
queue/Redis/worker runtime, and product/sourcing behavior. Moving them now would
preserve the same dependency defect under a new path. Their file counts are
therefore frozen, and they move only with the Phase 3 product and Phase 4
marketplace vertical slices.

## Mixed persistence adapters

GORM use remains mixed with domain models, services, HTTP assembly, and schema
startup. These are recorded and frozen rather than renamed into `integration`:

| Owning phase | Current adapter-bearing packages |
| --- | --- |
| Phase 3 — product | `internal/asset/repository`, `internal/productenrich`, `internal/productenrich/store`, `internal/productimage/store` |
| Phase 4 — marketplace | `internal/amazonlisting/store`, `internal/publishing/shein`, `internal/shein/aicache`, `internal/shein/productsync`, `internal/temu/sync` |
| Phase 5 — listing | `internal/listingadmin`, `internal/listingkit`, `internal/listingkit/api`, `internal/listingkit/httpapi`, `internal/listingkit/imageagentacceptance`, `internal/listingkit/memberinvite`, `internal/listingkit/reviewstore`, `internal/listingkit/schema`, `internal/listingkit/sheinsync`, `internal/listingkit/store`, `internal/listingkit/studiostore`, `internal/listingruntime/local` |
| Phase 6 — agent/knowledge/resource catalog | `internal/aicapability/store`, `internal/imageagent/store`, `internal/imageagent/temporal`, `internal/prompt`, `internal/promptmgmt/api` |
| Phase 7 — commercial/organization | `internal/listingsubscription`, `internal/sourceaccount`, `internal/sourceaccount/bootstrap`, `internal/tenantbridge`, `internal/tenantbridge/bootstrap` |

App schema/runtime packages may continue to execute the migration baseline while
Phase 2 centralizes database lifecycle. Each listed domain package must first
define a focused repository port; only its GORM implementation then moves to an
owning `integration/<domain>` adapter.

## Legacy consumer register

The initial counts below preserve the migration baseline. The named package
lists are also the exact legacy consumer register used at closure: app,
platform-to-platform, and integration-to-integration wiring is intentional;
every other named package is frozen and moves only with its later owner. They
must gain no new consumers.

The table below is the authoritative current register. Each row names one
current legacy consumer and its later owner. App, platform, and integration
internal wiring is intentionally excluded; no other package class is excluded.

| Target import path | Exact consumer package | Class | Explicit later owner / phase |
| --- | --- | --- | --- |
| `task-processor/internal/core/logger` | `task-processor/internal/amazon/api` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/amazon/image` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/amazon/listing` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/amazon/llm` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/amazon/pipeline` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/amazon/schema` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/core/metrics` | business metrics | marketplace + listing / Phases 4-5 |
| `task-processor/internal/core/logger` | `task-processor/internal/crawler/alibaba1688` | crawler business consumer | product + marketplace / Phases 3-4 |
| `task-processor/internal/core/logger` | `task-processor/internal/crawler/alibaba1688/extractor` | crawler business consumer | product + marketplace / Phases 3-4 |
| `task-processor/internal/core/logger` | `task-processor/internal/crawler/amazon` | crawler business consumer | product + marketplace / Phases 3-4 |
| `task-processor/internal/core/logger` | `task-processor/internal/crawler/amazon/browser` | crawler business consumer | product + marketplace / Phases 3-4 |
| `task-processor/internal/core/logger` | `task-processor/internal/crawler/amazon/extractor` | crawler business consumer | product + marketplace / Phases 3-4 |
| `task-processor/internal/core/logger` | `task-processor/internal/crawler/amazon/variations` | crawler business consumer | product + marketplace / Phases 3-4 |
| `task-processor/internal/core/logger` | `task-processor/internal/crawler/fetcher` | crawler business consumer | product + marketplace / Phases 3-4 |
| `task-processor/internal/core/logger` | `task-processor/internal/crawler/shared/browser` | crawler business consumer | product + marketplace / Phases 3-4 |
| `task-processor/internal/core/logger` | `task-processor/internal/listingkit` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/core/logger` | `task-processor/internal/listingkit/api` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/core/logger` | `task-processor/internal/listingkit/httpapi` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/core/logger` | `task-processor/internal/localagent` | agent business consumer | agent owner / Phase 6 |
| `task-processor/internal/core/logger` | `task-processor/internal/pipeline` | mixed product pipeline | product owner / Phase 3 |
| `task-processor/internal/core/logger` | `task-processor/internal/pkg/appenv` | runtime-support debt | app retirement / Phase 8 |
| `task-processor/internal/core/logger` | `task-processor/internal/pkg/downloader` | mixed business helper | product + marketplace / Phases 3-4 |
| `task-processor/internal/core/logger` | `task-processor/internal/pkg/fileio` | runtime-support debt | shared primitive + app policy / Phase 8 |
| `task-processor/internal/core/logger` | `task-processor/internal/platformbase` | runtime-support debt | app retirement / Phase 8 |
| `task-processor/internal/core/logger` | `task-processor/internal/platformtask` | runtime-support debt | app retirement / Phase 8 |
| `task-processor/internal/core/logger` | `task-processor/internal/processor` | runtime-support debt | app retirement / Phase 8 |
| `task-processor/internal/core/logger` | `task-processor/internal/productenrich` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/core/logger` | `task-processor/internal/productenrich/api` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/core/logger` | `task-processor/internal/productenrich/enrich` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/core/logger` | `task-processor/internal/productimage` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/core/logger` | `task-processor/internal/prompt` | agent business consumer | agent owner / Phase 6 |
| `task-processor/internal/core/logger` | `task-processor/internal/publishing/shein` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/sds/client` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/sds/design` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/sdslogin` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/activity` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/aicache` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/category` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/client` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/content` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/inventory` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/managedclient` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/mapping` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/pipeline` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/pricing` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/product` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/product/attribute` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/product/attribute/sale` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/product/build` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/product/image` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/product/skc` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/product/sku` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/product/variant` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/productdata` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/productsync` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/publish` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/scheduler` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/store` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/translate` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/shein/validation` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/sheinbridge/saleattribute` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/sheinlogin` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/state` | runtime-support debt | app retirement / Phase 8 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/ai` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/api/client` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/bulkrelist` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/category` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/filter` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/handlerbase` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/image` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/pricing` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/product` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/property` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/rules` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/scheduler` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/sku` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/spec` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/store` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/sync` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/core/logger` | `task-processor/internal/temu/template` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/platform/logging` | `task-processor/internal/core/config` | application schema | owning domains / Phases 3-7 |
| `task-processor/internal/platform/logging` | `task-processor/internal/core/logger` | compatibility facade | owning domains / Phases 3-8 |
| `task-processor/internal/platform/database` | `task-processor/internal/amazonlisting/httpapi` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/platform/database` | `task-processor/internal/listingkit/httpapi` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/platform/database` | `task-processor/internal/listingruntime/local` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/platform/database` | `task-processor/internal/productenrich/httpapi` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/platform/database` | `task-processor/internal/productimage/httpapi` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/platform/database` | `task-processor/internal/shein/pipeline` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/platform/database` | `task-processor/internal/sourceaccount/bootstrap` | organization persistence consumer | organization owner / Phase 7 |
| `task-processor/internal/platform/database` | `task-processor/internal/tenantbridge/bootstrap` | organization persistence consumer | organization owner / Phase 7 |
| `task-processor/internal/platform/redis` | `task-processor/internal/crawler/amazon` | crawler business consumer | product + marketplace / Phases 3-4 |
| `task-processor/internal/platform/redis` | `task-processor/internal/crawler/shared` | crawler business consumer | product + marketplace / Phases 3-4 |
| `task-processor/internal/platform/redis` | `task-processor/internal/productenrich/httpapi` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/platform/queue/rabbitmq` | `task-processor/internal/crawler/fetcher` | crawler business consumer | product + marketplace / Phases 3-4 |
| `task-processor/internal/platform/queue/rabbitmq` | `task-processor/internal/listingcontrol` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/platform/queue/rabbitmq` | `task-processor/internal/platformbase` | runtime-support debt | app retirement / Phase 8 |
| `task-processor/internal/platform/queue/rabbitmq` | `task-processor/internal/shein/pipeline` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/platform/queue/rabbitmq` | `task-processor/internal/shein/scheduler` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/platform/queue/rabbitmq` | `task-processor/internal/temu` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/platform/queue/rabbitmq` | `task-processor/internal/temu/scheduler` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/platform/queue/rabbitmq` | `task-processor/internal/temu/sync` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/amazon` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/amazonlisting` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/amazonlisting/httpapi` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/crawler/alibaba1688` | crawler business consumer | product + marketplace / Phases 3-4 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/crawler/amazon` | crawler business consumer | product + marketplace / Phases 3-4 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/crawler/shared` | crawler business consumer | product + marketplace / Phases 3-4 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/httpbootstrap` | runtime-support debt | app retirement / Phase 8 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/kernel/module` | runtime-support debt | app retirement / Phase 8 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/listingkit` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/listingkit/httpapi` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/processor` | runtime-support debt | app retirement / Phase 8 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/productenrich/httpapi` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/productenrich/pipeline` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/productimage/httpapi` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/productimage/pipeline` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/shein/pipeline` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/platform/workerpool` | `task-processor/internal/temu` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/amazon` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/amazon/llm` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/core/config` | application schema | owning domains / Phases 3-7 |
| `task-processor/internal/integration/openai` | `task-processor/internal/listingadmin` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/integration/openai` | `task-processor/internal/listingkit/httpapi` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/integration/openai` | `task-processor/internal/productenrich` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/integration/openai` | `task-processor/internal/productenrich/httpapi` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/integration/openai` | `task-processor/internal/productimage` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/integration/openai` | `task-processor/internal/productimage/httpapi` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/integration/openai` | `task-processor/internal/publishing/sheinmanaged` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/shein/category` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/shein/content` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/shein/pipeline` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/shein/product/attribute` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/shein/product/attribute/sale` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/shein/product/build` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/shein/product/skc` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/shein/submitprep` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/shein/translate` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/sheinbridge/saleattribute` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/temu` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/temu/ai` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/temu/image` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/temu/product` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/openai` | `task-processor/internal/temu/sku` | marketplace business consumer | marketplace owner / Phase 4 |
| `task-processor/internal/integration/geminiimage` | `task-processor/internal/listingkit/httpapi` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/integration/grsai` | `task-processor/internal/listingkit/httpapi` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/integration/grsai` | `task-processor/internal/productimage/httpapi` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/integration/s3` | `task-processor/internal/listingkit/httpapi` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/integration/s3` | `task-processor/internal/productimage/httpapi` | product business consumer | product owner / Phase 3 |
| `task-processor/internal/integration/httpimage` | `task-processor/internal/imageagent` | agent business consumer | agent owner / Phase 6 |
| `task-processor/internal/integration/httpimage` | `task-processor/internal/listingkit` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/integration/httpimage` | `task-processor/internal/listingruntime/local` | listing business consumer | listing owner / Phase 5 |
| `task-processor/internal/integration/httpimage` | `task-processor/internal/productimage` | product business consumer | product owner / Phase 3 |

## Historical pre-migration snapshot

The material below preserves the Phase 2 starting point for audit history. It
is not the current register; only the exact table above defines current debt.

Technical compatibility consumers are explicit too:
`internal/core/config` consumes platform logging and OpenAI configuration
types, while the forwarding facade `internal/core/logger` consumes platform
logging. They are schema/facade debt, not business-domain precedents.

| Slice | Initial direct importers |
| --- | ---: |
| `core/logger` | 92 packages |
| `infra/database` | 19 packages |
| `infra/redisclient` | 6 packages |
| `infra/rabbitmq` | 17 packages |
| `infra/worker` | 23 packages |
| `infra/clients/openai` | 28 packages |
| `infra/clients/geminiimage` | 1 package |
| `infra/clients/grsai` | 2 packages |
| `infra/storage` | 6 packages |
| `pkg/safeimagehttp` | 8 packages |

The importer disposition separates composition callers from legacy business
callers. Supporting runtime packages remain app-retirement debt and do not
grant a domain dependency.

### `core/logger` — closure ceiling 82

- App importers rewired in Phase 2: `internal/app/runner`, `internal/app/scheduler`, `internal/app/task`, `internal/app/taskstatus`, `internal/app/updater`, `internal/app/worker`.
- Historical runtime-support importers included configuration, storage, worker,
  environment, file, and platform-task support; the authoritative table above
  records only the compatibility consumers that remain.
- Phase 3/4 owning-domain debt: `internal/pkg/downloader` remains with the product/product-image and marketplace reviews; remove its logger dependency only after the owning-domain review and local port are in place.
- Phase 3 product debt: `internal/crawler/alibaba1688`, `internal/crawler/alibaba1688/extractor`, `internal/crawler/amazon`, `internal/crawler/amazon/browser`, `internal/crawler/amazon/extractor`, `internal/crawler/amazon/variations`, `internal/crawler/fetcher`, `internal/crawler/shared/browser`, `internal/pipeline`, `internal/product`, `internal/productenrich`, `internal/productenrich/api`, `internal/productenrich/enrich`, `internal/productimage`.
- Phase 4 marketplace debt: `internal/amazon/api`, `internal/amazon/image`, `internal/amazon/listing`, `internal/amazon/llm`, `internal/amazon/pipeline`, `internal/amazon/schema`, `internal/publishing/shein`, `internal/sds/client`, `internal/sds/design`, `internal/sdslogin`, `internal/shein`, `internal/shein/activity`, `internal/shein/aicache`, `internal/shein/category`, `internal/shein/client`, `internal/shein/content`, `internal/shein/inventory`, `internal/shein/managedclient`, `internal/shein/mapping`, `internal/shein/pipeline`, `internal/shein/pricing`, `internal/shein/product`, `internal/shein/product/attribute`, `internal/shein/product/attribute/sale`, `internal/shein/product/build`, `internal/shein/product/image`, `internal/shein/product/skc`, `internal/shein/product/sku`, `internal/shein/product/variant`, `internal/shein/productdata`, `internal/shein/productsync`, `internal/shein/publish`, `internal/shein/scheduler`, `internal/shein/store`, `internal/shein/translate`, `internal/shein/validation`, `internal/sheinbridge/saleattribute`, `internal/sheinlogin`, `internal/temu`, `internal/temu/ai`, `internal/temu/api/client`, `internal/temu/bulkrelist`, `internal/temu/category`, `internal/temu/filter`, `internal/temu/handlerbase`, `internal/temu/image`, `internal/temu/pricing`, `internal/temu/product`, `internal/temu/property`, `internal/temu/rules`, `internal/temu/scheduler`, `internal/temu/sku`, `internal/temu/spec`, `internal/temu/store`, `internal/temu/sync`, `internal/temu/template`.
- Phase 5 listing debt: `internal/core/metrics`, `internal/listingkit`, `internal/listingkit/api`, `internal/listingkit/httpapi`.
- Phase 6 agent debt: `internal/localagent`, `internal/prompt`.
- Phase 8 app-retirement debt: `internal/processor`, `internal/state`.

### `platform/database` — closure ceiling 21

- App importers rewired in Phase 2: `internal/app/bootstrap/resources`, `internal/app/httpapi`, `internal/app/runtime/listing`, `internal/app/runtime/listingcontrol`, `internal/app/runtime/listingkitidentitypreflight`, `internal/app/runtime/listingkitownerexceptions`, `internal/app/runtime/listingkitownerreconcile`, `internal/app/runtime/listingkitschemamigrate`, `internal/app/runtime/productlistingschemamigrate`, `internal/app/runtime/sheinplatformrecovery`, `internal/app/worker/imageagent`.
- Phase 3 product debt: `internal/productenrich/httpapi`, `internal/productimage/httpapi`.
- Phase 4 marketplace debt: `internal/amazonlisting/httpapi`, `internal/shein/pipeline`.
- Phase 5 listing debt: `internal/listingkit/httpapi`, `internal/listingruntime/local`.
- Phase 7 organization debt: `internal/sourceaccount/bootstrap`, `internal/tenantbridge/bootstrap`.

### `platform/redis` — closure ceiling 8

- App importers rewired in Phase 2: `internal/app/consumer`, `internal/app/httpapi`, `internal/app/runtime/listing`.
- Phase 3/4 crawler debt: `internal/crawler/amazon`, `internal/crawler/shared`.
- Phase 3 product debt: `internal/productenrich/httpapi`.

### `platform/queue/rabbitmq` — closure ceiling 18

- App importers rewired in Phase 2: `internal/app/bootstrap`, `internal/app/bootstrap/fetchers`, `internal/app/bootstrap/resources`, `internal/app/bootstrap/schedulers`, `internal/app/consumer`, `internal/app/crawler/distributed`, `internal/app/runner`, `internal/app/runtime/listingcontrol`.
- Runtime support that remains app-retirement debt: `internal/platformbase`.
- Phase 3 crawler debt: `internal/crawler/fetcher`.
- Phase 5 listing debt: `internal/listingcontrol`.
- Phase 4 marketplace debt: `internal/shein/pipeline`, `internal/shein/scheduler`, `internal/temu`, `internal/temu/scheduler`, `internal/temu/sync`.

### `platform/workerpool` — closure ceiling 23

- App importers rewired in Phase 2: `internal/app/consumer`, `internal/app/httpapi`, `internal/app/runtime/listing`, `internal/app/task`, `internal/app/worker`.
- Runtime support that co-relocates or is retired: `internal/httpbootstrap`, `internal/kernel/module`, `internal/processor`.
- Phase 3/4 crawler debt: `internal/crawler/alibaba1688`, `internal/crawler/amazon`, `internal/crawler/shared`.
- Phase 3 product debt: `internal/productenrich/httpapi`, `internal/productenrich/pipeline`, `internal/productimage/httpapi`, `internal/productimage/pipeline`.
- Phase 4 marketplace debt: `internal/amazon`, `internal/amazonlisting`, `internal/amazonlisting/httpapi`, `internal/shein/pipeline`, `internal/temu`.
- Phase 5 listing debt: `internal/listingkit`, `internal/listingkit/httpapi`.
- Phase 2 boundary correction: `internal/listing/submission` is removed from this debt by consuming a local `QueueFull()` classification rather than the worker sentinel.

### `integration/openai` — closure ceiling 28

- App importers rewired in Phase 2: `internal/app/httpapi`, `internal/app/schema/productlisting`, `internal/app/worker/imageagent`.
- Runtime support that co-relocates: `internal/core/config`.
- Phase 3 product debt: `internal/productenrich`, `internal/productenrich/httpapi`, `internal/productimage`, `internal/productimage/httpapi`.
- Phase 4 marketplace debt: `internal/amazon`, `internal/amazon/llm`, `internal/publishing/sheinmanaged`, `internal/shein/category`, `internal/shein/content`, `internal/shein/pipeline`, `internal/shein/product/attribute`, `internal/shein/product/attribute/sale`, `internal/shein/product/build`, `internal/shein/product/skc`, `internal/shein/submitprep`, `internal/shein/translate`, `internal/sheinbridge/saleattribute`, `internal/temu`, `internal/temu/ai`, `internal/temu/image`, `internal/temu/product`, `internal/temu/sku`.
- Phase 5 listing debt: `internal/listingadmin`, `internal/listingkit/httpapi`.

### Image-provider clients

- `integration/geminiimage` (1): legacy business importer `internal/listingkit/httpapi`, removed through a listing-local image port in Phase 5; there is no app importer today.
- `integration/grsai` (2): legacy business importers `internal/listingkit/httpapi` (Phase 5) and `internal/productimage/httpapi` (Phase 3); there is no app importer today.

### `integration/s3` — closure ceiling 4

- App importers rewired in Phase 2: `internal/app/httpapi`, `internal/app/worker/imageagent`.
- Phase 3 product debt: `internal/productimage/httpapi`.
- Phase 5 listing debt: `internal/listingkit/httpapi`.

### `integration/httpimage` — closure ceiling 8

- App importer rewired in Phase 2: `internal/app/worker/imageagent`.
- Phase 3 product debt: `internal/productimage`.
- Phase 5 listing debt: `internal/listingkit`, `internal/listingruntime/local`.
- Phase 6 agent debt: `internal/imageagent`.

All entries in this section are migration debt, not approved final dependency
directions. The target remains: domains own narrow ports, app wires them,
platform stays domain-neutral, and provider/persistence implementations live in
integration.

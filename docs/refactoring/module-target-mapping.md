# Module Target Mapping

> Status: active migration mapping from current package areas to current target owners.  
> Legacy policy: `docs/refactoring/legacy-hard-cut-policy.md`  
> Scan baseline: `main @ 6e3b87208e2c4e51039b95145a2377edfbb9b1cb`

## 1. Purpose

This document maps mixed or historical package areas toward the current modular-monolith owners. It is a migration aid, not a requirement to rename everything at once.

Current target ownership domains include:

- `listing`
- `marketplace`
- `product`
- `agent`
- `commercetool`
- `knowledge`
- `resourcecatalog`
- `commercial`
- `ledger`
- `organization`
- `integration`
- `platform`
- `app`
- `shared`

**`compatibility` is not a target domain.** Existing `internal/compatibility/*` code is a drain/retirement area governed by `EXTRACT | RETIRE`.

## 2. Mapping table

| Current area | Current role | Target owner / destination | Migration rule |
| --- | --- | --- | --- |
| `internal/listingkit` | mixed legacy listing orchestration/API/runtime | `internal/listing/*`, `internal/marketplace/*`, `internal/product/*`, `internal/integration/*`, `internal/app/*` according to ownership | #29: extract valid behavior, switch caller, retire old path. Do not preserve root ListingKit as permanent facade. |
| `internal/compatibility/listingkit` | remaining historical migration bridges | current real owner above | Drain only. No new consumer or feature landing. #30 owns active 1688 handoff cutover; #300 owns guard/obvious cleanup. |
| `internal/listingadmin` | listing-admin behavior | `internal/listing/settings` or `internal/app/httpapi` | Split business policy from transport/assembly. |
| `internal/listingsubscription` | listing-era plans/entitlements/usage behavior | `internal/commercial/*`, current resource/entitlement owners, narrow listing policy where truly listing-specific | Do not restore old task-quota authority. |
| `internal/platformtask` | task execution helpers | `internal/listing/task` or runtime owner such as Temporal/queue | Business task semantics and runtime execution stay separate. |
| `internal/taskstatus` | task status helpers | owning listing/runtime domain or `internal/shared` for truly generic primitives | Do not make internal Task the user product model. |
| old `internal/catalog` | old canonical product package | `internal/product/catalog` | **RETIRE: package is already absent; keep it absent.** |
| old `internal/asset` | old asset package | `internal/product/asset` | **RETIRE: package is already absent; keep it absent.** |
| old `internal/imageasset` | old image/asset helpers | `internal/product/image` or `internal/product/asset` | **RETIRE: package is already absent; extract only if a still-valid behavior is rediscovered.** |
| old `internal/productimage` | old ProductImage runtime | `internal/product/image` + ImageAgent | **RETIRE: package is already absent; do not recreate task/queue/worker/API ownership.** |
| old `internal/productenrich` | old ProductEnrich runtime | `internal/product/enrichment` + AI Capability where needed | **RETIRE: package is already absent; do not recreate workflow/task ownership.** |
| `internal/product` | current Product Domain | `internal/product/*` | Current owner; extend by Product subdomain rather than creating another product fact source. |
| `internal/pricing` | mixed pricing helpers | Product, marketplace-specific policy, or listing submission according to semantics | Classify by business owner before moving. |
| `internal/sds` | specialized POD/design capability | Product asset/image or listing studio according to behavior | Preserve POD specialization; do not make it a generic source owner. |
| `internal/ai` | mixed provider-neutral/provider runtime AI concerns | `internal/agent/*`, current AI Capability owner, or `internal/integration/<provider>` | Provider SDK types do not leak into domain contracts. |
| `internal/aicapability` | current model capability/routing/policy/cost foundation | current AI Capability/Agent control-plane owner + persistence adapters | #126/#130 stabilize before Product Agent main implementation. |
| `internal/prompt` / `internal/promptmgmt` | prompt contracts, persistence, management transport | Agent/prompt owner + integration persistence + app transport | Split domain contract, persistence, and HTTP assembly. |
| `internal/crawler` | historical crawler implementations | `internal/integration/crawler/*` | Crawler owns extraction/runtime, not canonical Product facts. |
| `internal/amazon` | mixed Amazon source/target logic | `internal/marketplace/amazon/*` + `internal/integration/crawler/amazon` | Keep source and target listing concepts separate. |
| `internal/amazonlisting` | Amazon listing behavior | `internal/marketplace/amazon/*` / listing seams | Extract platform rules; do not copy a platform Workbench. |
| `internal/shein` | mixed SHEIN logic | `internal/marketplace/shein/*` + `internal/integration/shein` | Separate API/client integration from platform business rules. |
| `internal/publishing/shein` | historical SHEIN publishing shell | `internal/marketplace/shein/*` / existing submission owner | Extract valid rule/transport behavior; retire shell after callers switch. |
| `internal/workspace/shein` | historical SHEIN workspace shell | current SHEIN marketplace/listing projection where still needed | Final UI does not require a platform Workbench. Extract useful behavior, then retire shell. |
| `internal/sheinlogin` / `internal/sheinloginmanaged` | SHEIN authentication integration | `internal/integration/shein` + app lifecycle wiring | External auth adapter, not listing/product owner. |
| `internal/temu` | mixed TEMU capability | `internal/marketplace/temu/*` + `internal/integration/temu` | #31 expands shared capability without independent Workbench. |
| `internal/platforms` | mixed platform abstractions | marketplace or listing owner | Keep only genuinely cross-platform contracts outside platform-specific owners. |
| `internal/workspace` | historical generic workspace behavior | listing/marketplace owner only if behavior remains valid | Generic Listing Workspace is retired; do not recreate it as target architecture. |
| `internal/publishing` | generic/platform publishing support | marketplace-specific rules + single listing/submission owner | No second submission state machine. |
| `internal/app` | runtime composition/lifecycle | `internal/app/*` | Wiring only; no new business-rule ownership. |
| `internal/httpbootstrap` / `internal/httproute` | HTTP bootstrap/routes | `internal/app/httpapi` | Transport/assembly ownership. |
| `internal/taskrpcapi` | internal task transport | app transport/runtime owner | Do not expose as BusinessTask product semantics. |
| `internal/infra` | mixed runtime infra/external clients | `internal/platform/*` or `internal/integration/*` | Split runtime infrastructure from external adapters. |
| `internal/platformbase` | shared runtime helpers | platform/shared only if truly generic | Drain catch-all behavior toward named owners. |
| `internal/authidentity` | verified tenant/user/organization identity envelope | current Organization/identity authority or minimal shared identity envelope | Organization/membership policy remains with Organization owner. |
| `internal/authruntime/zitadel` | ZITADEL verification/middleware | integration identity + app HTTP wiring | ZITADEL remains identity provider; no parallel IAM. |
| `internal/authz` | permissions + policy-engine concerns | Organization/business permission owner + Casbin integration | Permission semantics are business-owned; engine is adapter. |
| `internal/kernel` / `internal/core` / `internal/pkg` | generic/cross-cutting helpers | `internal/platform/*`, `internal/shared`, or a named domain | Avoid new dumping grounds. |
| `internal/validation` | generic validation primitives | `internal/shared/validation` or deterministic validator owner when business-specific | #34 owns readiness/validator business contract. |
| `internal/state` | generic state handling | runtime or business state owner | State machines must have one named owner. |
| `internal/processor` / `internal/pipeline` | mixed orchestration | app worker, Product sourcing, Listing workflow, or runtime owner | Split by business/side-effect ownership. |
| `internal/ports` / `internal/domain` / `internal/model` | broad aggregation packages | local owning domains | Prefer local contracts/models over global catch-alls. |
| `internal/scheduler` | scheduling runtime | app worker / platform queue/Temporal | Runtime only. |
| `internal/sourceaccount` | source-account metadata/access/persistence | Product sourcing account + persistence adapter | Source access is not Organization membership. |
| `internal/tenantbridge` | active Organization ↔ legacy numeric tenant mapping | current Organization identity + each consuming domain's current persistence ownership | #301: freeze new consumers, migrate/backfill/cut over, then delete package. Never move it to another compatibility facade. |
| `internal/zitadelprovision` | ZITADEL management/provisioning | integration identity provisioning + app operational entrypoint | External client stays in integration; app owns lifecycle. |
| `web/listingkit-ui` | current legacy ListingKit/Task-first product shell | final Figma Product Projection, especially #298 and Store Center surfaces | Existing release paths may run until replacement; new product work does not target old IA; page-level cutover then retire. |

## 3. Crawler / sourcing direction

Treat Amazon/1688 crawling as source adapters:

```text
internal/integration/crawler/amazon
internal/integration/crawler/a1688
        ↓
internal/product/sourcing
        ↓
ProductSnapshot / ApprovedAsset / downstream Listing
```

Rules:

- crawler packages own fetching, browser/runtime adaptation, parsing, and raw extraction;
- Product Sourcing owns `SourceIdentity`, `SourceEnvelope`, normalization, lineage, warnings, and handoff to canonical Product facts;
- marketplace publishing does not own source crawling;
- Product packages do not import crawler/integration/legacy ListingKit packages directly.

For the active 1688 legacy path, #30 extracts required behavior out of `internal/compatibility/listingkit/sourcehandoff/a1688` and retires the old ListingKit task handoff after controlled acceptance.

## 4. ListingKit direction

`internal/listingkit` and `internal/compatibility/listingkit` are **drain targets**, not target layers.

When touching them:

1. identify the actual reusable behavior;
2. identify the current owner (`listing`, `marketplace`, `product`, `integration`, `app`, etc.);
3. put new/rewritten behavior behind that current owner's contract;
4. switch callers;
5. remove the legacy dependency/path.

Do not add a new internal compatibility wrapper just because a broad cutover is inconvenient.

## 5. Immediate landing zones

| New work type | Preferred landing zone | Do not default to |
| --- | --- | --- |
| Amazon source crawler | `internal/integration/crawler/amazon` | mixed `internal/amazon`, old crawler roots |
| 1688 source crawler | `internal/integration/crawler/a1688` | `internal/crawler/alibaba1688`, compatibility handoff |
| Source normalization / lineage | `internal/product/sourcing` | crawler DTOs, root ListingKit |
| Product facts/assets/enrichment/image | `internal/product/{catalog,asset,enrichment,image}` | retired ProductEnrich/ProductImage/catalog/asset roots |
| Marketplace rules | `internal/marketplace/<platform>/*` | root ListingKit or compatibility tree |
| Listing orchestration/submission | `internal/listing/*` / existing single submission owner | another platform-specific state machine |
| Runtime assembly | `internal/app/*` | business packages |
| External client/integration | `internal/integration/<system>` | Product/Listing business owners |
| Commerce Tools | `internal/commercetool` contract + narrow domain adapters | direct DB/provider/marketplace-client access |
| Agent runtime/capability | current Agent/AI Capability owners | legacy Task/Workflow ownership |
| Organization/membership/identity policy | current Organization/identity owner | `internal/tenantbridge` or new compatibility package |
| New AI Workbench UI | #298/Figma product projection | old Task-first ListingKit IA |

## 6. Legacy drain rules

The active rules from `legacy-register.md` are mandatory:

- no new `internal/compatibility/*` consumers;
- no new `internal/tenantbridge` consumers;
- retired package roots stay absent;
- existing legacy consumer counts should monotonically decrease;
- reusable behavior is extracted, not wrapped;
- cutover does not leave permanent fallback/dual-read/dual-write paths.

## 7. Migration usage

Before moving or reusing code:

1. identify the primary business owner;
2. classify the legacy behavior as `EXTRACT` or `RETIRE`;
3. if a file mixes owners, split behavior by ownership rather than moving the whole legacy abstraction;
4. use `docs/refactoring/legacy-register.md` for active drain decisions;
5. update this map when a major ownership boundary is completed.

## 6. Phase 2 closure landing rules

Runtime ownership remains with Platform/Integration and application assembly. Legacy business consumers drain to their current owners: the product owner uses `internal/product/*` and keeps the retired Product roots absent; the marketplace owner extracts platform rules under `internal/marketplace/*`; the organization owner handles current identity contracts while #301 coordinates tenantbridge consumer cutover. #29 owns the remaining ListingKit extraction. These are EXTRACT destinations followed by RETIRE, never new compatibility landing zones.

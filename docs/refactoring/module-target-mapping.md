# Module Target Mapping

> Status: active current-owner and retirement mapping.<br>
> Legacy policy: `docs/refactoring/legacy-hard-cut-policy.md`  
> Product baseline: `PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08`
> Scan baseline: `main @ 6e3b87208e2c4e51039b95145a2377edfbb9b1cb`
> Bounded delivery calibration: Issue #386 against `main @ 08a6195c7aa8528acdec0bf0f4e742a8c4998e6d`

## 1. Purpose

This document maps current package areas to current target owners. It is an ownership/retirement aid, not a requirement to rename everything at once or migrate old business data, IDs, profiles or runtime state.

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

| Current area | Current role | Target owner / destination | Current owner / retirement rule |
| --- | --- | --- | --- |
| `internal/listingkit` | mixed legacy listing orchestration/API/runtime | `internal/listing/*`, `internal/marketplace/*`, `internal/product/*`, `internal/integration/*`, `internal/app/*` according to ownership | #29: extract valid behavior, switch caller, retire old path. Do not preserve root ListingKit as permanent facade. |
| `internal/compatibility/listingkit` | remaining historical drain paths | current real owner above | Drain only. No new consumer or feature landing. A future #30 owner may EXTRACT valid behavior or RETIRE the handoff; it must not migrate old tasks/data or add a compatibility path. #300 owns guard/obvious cleanup. |
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
| `internal/sourceaccount` | legacy source-account metadata/access/persistence still used by default ListingKit/crawler composition | `internal/sourceaccountregistry` + `internal/integration/persistence/sourceaccountregistry` for current Source Account registration; provider connection remains a separate current capability | **EXTRACT/RETIRE:** do not wrap this package or route a current registry caller through it. Source access is not Organization membership. See the bounded calibration and legacy register. |
| `internal/tenantbridge` | active Organization ↔ legacy numeric tenant mapping | current Organization identity + each consuming domain's current persistence ownership | #301: freeze new consumers; new domains use native Organization identity; RETIRE legacy callers and package by owner; it allows no legacy numeric mapping, old-data migration/backfill or cutover. Never move it to another compatibility facade. |
| `internal/zitadelprovision` | ZITADEL management/provisioning | integration identity provisioning + app operational entrypoint | External client stays in integration; app owns lifecycle. |
| `web/listingkit-ui` | current web app with legacy ListingKit/Task-first responsibilities and valid current surfaces | final Figma Product Projection, especially #298 and Store Center surfaces | Retire only the specific Task-first/legacy dependencies. Keep current Console, account, plans and review surfaces as current product work; do not rebuild them merely because of this path name. |

### 2.1 Bounded current-delivery calibration (#386)

This is a **bounded symbol/caller calibration**, not a replacement full-repository scan. The historical scan baseline above remains `6e3b87208e2c4e51039b95145a2377edfbb9b1cb`; the rows below were checked against `main @ 08a6195c7aa8528acdec0bf0f4e742a8c4998e6d`. `CURRENT` identifies the allowed owner, not deployment or default runtime presence. `MERGED`, isolated acceptance, default mounting, deployment health, and business acceptance are deliberately separate states.

| Object / exact entry | Owner and decision | Code / assembly state | Runtime evidence | Allowed new calls | Forbidden edge / exit |
| --- | --- | --- | --- | --- | --- |
| SA1 registry + SA2 Console/BFF: `internal/sourceaccountregistry`; `internal/integration/persistence/sourceaccountregistry`; `internal/app/schema/sourceaccountregistry.Migrate`; `internal/app/httpapi.NewSourceAccountApplication`; `web/listingkit-ui/src/lib/api/source-accounts.ts`; `web/listingkit-ui/src/app/api/workbench/[...path]/route.ts`; `web/listingkit-ui/src/lib/server/workbench-proxy.ts` | `sourceaccountregistry` owns Organization-scoped registration metadata and lifecycle; persistence is an adapter; app owns schema/HTTP lifecycle; Console/BFF owns same-origin transport. **CURRENT.** Provider authentication/connection is a separate capability and registration creates `pending_connection`, not a connected account. | PR #369 merged as `ef6c89ec9388f16d35a9ff964511ab7b2d3d3f0f`; PR #373 merged as `63c1806606a2a98faf510116d0119506ba1f454c`. `NewSourceAccountApplication` has no non-test default-composition caller, so its Go server and explicit schema remain opt-in even though the BFF/client code exists. | #386 controlled chain PASS: browser client -> Next/Auth.js encrypted session -> same-origin BFF -> Go verified identity/effective Organization/permission/UoW -> task-owned empty PostgreSQL. Real ZITADEL issuance, real 1688, shared/default service mount, deployment, runtime health, and business acceptance: **NOT_RUN**. | Call the current `/api/v1/workbench/source-accounts` contract through verified identity/effective Organization; initialize its explicit schema and own DB/listener/server lifecycle at the application boundary. | No call through old `internal/sourceaccount`, `internal/tenantbridge`, or compatibility handoff; no connection-state fiction, fallback, migration, dual read/write, or second registry. Exit is a separately admitted default mount/provider connection, not reuse of the old repository. |
| SRC-1 source evidence -> Catalog: `internal/app/productsourcing.NewInternalProducer`; `internal/product/sourcing.InternalProducer.{Publish,Verify,Read}`; `internal/integration/persistence/product/sourcing.NewRepository`; `internal/product/catalog.ProductSnapshot` | Product Sourcing owns immutable source evidence, normalization and publication identity; Catalog owns canonical `ProductSnapshot`, version/head and publication facts. **CURRENT.** | PR #381 merged as `675302829a4ad40ffedab04c7ba93f19ab7141e0`. The persistence adapter publishes sourcing evidence and Catalog facts under its admitted transaction boundary. The only production construction found is inside the isolated REV-1 application; there is no default producer mount. | Controlled PostgreSQL publication/read/replay paths PASS in #386 REV-1 verification. Real source provider acquisition and default/deployed runtime: **NOT_RUN**. | Use the narrow admitted producer/reader contracts with verified authorization and immutable publication identity. | No direct Product import of crawler/provider DTOs; no old A1688 handoff, root ListingKit Task, tenantbridge, caller-supplied Organization, fallback or second Catalog fact source. Default provider ingress requires separate admission. |
| PA-1 approved inventory: `internal/product/asset.ApprovedAsset`; `internal/product/asset.ApprovedAssetInventory`; `internal/product/asset.Repository.GetApprovedInventory`; `internal/integration/persistence/product/asset.NewRepository` | Product Asset owns explicit human-approved asset facts; persistence reconstructs that inventory. **CURRENT.** | The current contract and PostgreSQL adapter are present. PR #375's exact-version repair merged as `dc2fd156f6872e075963cd6b939b89b5dc8713f6`: a requested source snapshot version does not fall back to another version. This does not classify every still-present unversioned call. | Unit/contract paths are included in #386 full Go PASS; a PA-1 provider/approval/deployed business chain was not rerun: **NOT_RUN**. | Read the exact approved inventory required by the consumer's Product/version/platform scope; keep approval authority in Product Asset. | No “latest” fallback for an exact-version request, no generated/selected asset promoted as approved without the approval owner, and no second asset store. Remaining unversioned callers need their own evidence and admission. |
| DRAFT-S1: `internal/app/httpapi.NewSheinRecordApplication`; `internal/listing/record`; `internal/marketplace/shein/draft.Builder` | Listing Record owns the immutable local record/UoW; SHEIN draft owns the deterministic marketplace projection/validation; app owns isolated composition. **CURRENT.** A local record is not a remote SHEIN draft or submit authority. | PR #377 merged as `06beadc4c4a818fe4b01220a9f5c9b625ce6cb71`. The constructor has no non-test default-composition caller, so schema/server construction remains opt-in. | Unit/browser fixture paths are included in #386 full Go PASS. Real SHEIN API, default mount, deployment, runtime health and business acceptance: **NOT_RUN**. | Compose exact Catalog snapshot + PA-1 approved inventory + current Store context through the DRAFT-S1 application boundary. | No provider write, AI proposal, root ListingKit Task/Workspace, compatibility bridge, fallback or claim that local persistence equals remote publication. Exit requires separately admitted provider/submission wiring. |
| REV-1: `internal/app/httpapi.InstallProductReviewSchema`; `internal/app/httpapi.NewProductReviewApplication`; current Catalog/Sourcing readers; Product Review repository/UoW | Catalog remains product-fact owner, Sourcing remains evidence owner, and Product Review owns proposal/decision/apply state and UoW. **CURRENT.** | PR #383 merged as `c3c098da00ef4f27e7c291434924e314eb855f29`. Explicit schema and constructor exist, but `NewProductReviewApplication` has no non-test default-composition caller. | #386 task-owned PostgreSQL lifecycle, concurrency, replay, response-loss, authorization-revocation, exact-schema and race checks PASS. Default mount, deployment, runtime health, real model/provider and business acceptance: **NOT_RUN**. | Use the current Catalog/Sourcing readers and Review repository behind verified identity, effective Organization and existing authorization; caller owns schema/DB/listener/server lifecycle. | No in-memory binding fallback, caller-supplied Organization/publication authority, provider publication from Review, legacy Task coupling, fallback or second product fact source. Default mounting needs separate admission. |
| SUB-K1: `internal/listing/submission.ExecutionKernel`; `internal/listing/submission.NewExecutionKernel`; `internal/integration/persistence/listing/submission.NewRepository` | Listing Submission owns the durable provider-neutral execution kernel and its idempotent state transitions; persistence is an adapter. **CURRENT kernel, not whole submission stack.** | PR #385 merged as `08a6195c7aa8528acdec0bf0f4e742a8c4998e6d`. `NewExecutionKernel` has no non-test application/default-composition caller. | Kernel PostgreSQL/race evidence belongs to the merged slice; #386 full Go/race selection PASS. Live app authorization, provider call/readback, default mount, deployment and business acceptance: **NOT_RUN**. | Add only provider-neutral ports/adapters after a separate application/provider admission preserves kernel idempotency and recovery invariants. | Do not infer that old root ListingKit submission, `submit_lock`, platform callers or provider side effects are cut over. No wrapper, duplicate state machine or compatibility path; exit is caller/provider cutover under their own owner. |
| ImageAgent split: `internal/imageagent`; `internal/app/worker/imageagent`; `cmd/image-agent-temporal-worker`; `internal/app/httpapi.buildImageAgentModuleResult`; `internal/app/httpapi.newImageAgentAuthorizedAssetCatalog`; `internal/app/httpapi.NewImageAgentOrganizationApplication` | ImageAgent domain/workflow/store, Temporal worker and deployment path are **CURRENT**. The root ListingKit Task input catalog bridge is **LEGACY-RETIRE**. Organization-scoped review governance is **CURRENT but isolated**. | Default composition builds the ImageAgent module and worker manifests/workflow exist, but `newImageAgentAuthorizedAssetCatalog(listingkitstore.NewTaskRepository(...))` still adapts `listingkit.Task`. `NewImageAgentOrganizationApplication` explicitly has no default registration. | Existing deployment definitions are code/config evidence only; #386 did not inspect a live cluster or business run: deployment/runtime/business **NOT_RUN**. Organization-scoped application is **NOT_PRESENT** in default production composition. | Extend ImageAgent's current domain/workflow/worker contracts; an admitted current Product/Asset input may replace the legacy task catalog. Keep Organization review governance isolated until separately mounted. | No new caller of the ListingKit Task bridge and no claim that all ImageAgent code is legacy. Retire only the task-input bridge after a current authoritative input and caller cutover; do not add fallback/dual catalogs. |
| Console/BFF versus old Task-first UI: current `/workbench` Console, Source Account and Product Review client/BFF paths; old `app/listing-kits/[taskId]/*` and `components/listingkit/tasks/*` projections | Current Console/BFF/review surfaces are **CURRENT** by capability; old Task-first ListingKit product projection is **RETIRE**. Directory/name alone is not authority. Shared leaf UI primitives are classified by behavior when touched. | Current and retired responsibilities coexist in `web/listingkit-ui`. PR #373 proves the Source Account BFF/client merge; REV-1 client/proxy code exists separately from the old task projection. | #386 frontend lint/typecheck/tests PASS; SA2 controlled BFF chain PASS. A successful build/test is not proof of default Go mount, deployment health or user acceptance: those remain **NOT_RUN** unless separately evidenced. | New Console/BFF calls target current Organization-scoped application contracts and truthful unavailable states. Reuse neutral leaf primitives only after ownership review. | No new Task-first Product flow, BusinessTask/legacy Task synchronization, legacy route fallback, or blanket retirement of the whole web tree. Retire page/caller slices after their current replacement is mounted and accepted. |
| Default mixed edge: `internal/app/httpapi/composition_builder.go` -> old `internal/sourceaccount` + `internal/compatibility/listingkit/sourcehandoff/a1688`; legacy handoff -> `internal/tenantbridge`; root `internal/listingkit` | This default chain remains **EXTRACT/RETIRE debt**, not the owner of SA1/SRC-1/Catalog. `ListingKitAuthorizer` and leaf `internal/listing/*` / `internal/marketplace/*` contracts remain classified by their behavior, not by the ListingKit name. | Default composition still builds the old source repository/crawler and calls `a1688handoff.NewTaskCommandService(...)`; therefore merged current applications have not cut over this default path. | Presence in default composition is not acceptance. #386 did not call the legacy path or a real provider: **NOT_RUN**. | Extract only independently valid identity/access/error/idempotency behavior into the named current owner, then switch the caller directly. | No new consumer, wrapper, tenant-ID mapping, old-task migration, fallback, dual write/read or second facts. #29/#30/#301 (or a newly admitted bounded caller cutover) own exit; #386 only records the edge. |

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

For the active 1688 legacy path, a future #30 owner may extract independently valid behavior out of `internal/compatibility/listingkit/sourcehandoff/a1688` and RETIRE the old ListingKit task handoff. It must not require old-task/data migration, profile reuse or a compatibility transition.

## 4. ListingKit direction

`internal/listingkit` and `internal/compatibility/listingkit` are **drain targets**, not target layers.

When touching them:

1. identify the actual reusable behavior;
2. identify the current owner (`listing`, `marketplace`, `product`, `integration`, `app`, etc.);
3. put new/rewritten behavior behind that current owner's contract;
4. switch callers;
5. remove the legacy dependency/path.

Do not add a new internal compatibility wrapper just because retirement is inconvenient.

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
- retirement does not leave permanent fallback/dual-read/dual-write paths.

## 7. Existing code usage

Before moving or reusing code:

1. identify the primary business owner;
2. classify the legacy behavior as `EXTRACT` or `RETIRE`;
3. if a file mixes owners, split behavior by ownership rather than moving the whole legacy abstraction;
4. use `docs/refactoring/legacy-register.md` for active drain decisions;
5. update this map when a major ownership boundary is completed.

## 6. Phase 2 closure landing rules

Runtime ownership remains with Platform/Integration and application assembly. Legacy business consumers drain to their current owners: the product owner uses `internal/product/*` and keeps the retired Product roots absent; the marketplace owner extracts platform rules under `internal/marketplace/*`; the organization owner handles current identity contracts while #301 coordinates tenantbridge consumer retirement. #29 owns the remaining ListingKit extraction. These are EXTRACT destinations followed by RETIRE, never new compatibility landing zones.

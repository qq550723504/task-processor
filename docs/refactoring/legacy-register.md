# Legacy Register

> Status: Active migration register  
> Policy: `docs/refactoring/legacy-hard-cut-policy.md`  
> Last reviewed: 2026-09-05

This register records known obsolete or partially migrated internal abstractions so implementation agents do not independently invent compatibility work.

Current repository policy has only two legacy decisions:

- `EXTRACT` — reusable behavior is moved/reimplemented under the current owner; the new architecture must stop depending on the legacy owner.
- `RETIRE` — obsolete design is not extended or internally supported; remove it after cutover.

There is currently no compatibility category.

| Legacy area / abstraction | Decision | Reusable behavior | Current owner / destination | Current rule / exit condition |
| --- | --- | --- | --- | --- |
| ProductEnrich task / queue / worker / API architecture | RETIRE | enrichment analysis, normalization, scoring or proposal logic that is independently valid | `internal/product/enrichment` + current AI Capability boundary where applicable | Do not recreate workflow/task ownership. Extract valid logic; delete/ignore obsolete runtime paths. |
| ProductImage task / queue / worker / API architecture | RETIRE | provider-neutral image analysis/transformation logic | `internal/product/image` + ImageAgent | ImageAgent remains image workflow/budget/retry/recovery/approval owner. No second ProductImage runtime. |
| Marketplace rules still owned by root ListingKit | EXTRACT | deterministic platform rules/policies/readiness logic | `internal/marketplace/*` / stable Listing seams | #29 owns remaining hard-cut. Move rules to current owner; root ListingKit must not remain the permanent rule owner. |
| Generic independent Listing Workspace (#27) | RETIRE | only independently valid Product/Listing UI behavior, if needed by current IA | current Product Projection pages defined by Figma authority | Do not revive a generic top-level Listing Workspace or use it as a compatibility target. |
| Old Product Task Center / unified internal Task Dashboard (#35) | RETIRE | diagnostic facts may be projected if still useful | #298 BusinessTask Product Projection; internal execution remains with existing Workflow/Task owners | BusinessTask is a new user-facing object, not a renamed legacy Task. No bidirectional Task ↔ BusinessTask sync. |
| Task-first Product architecture | RETIRE | none as an ownership model | Product/Listing domain facts + BusinessTask projection | Internal Task/Workflow stays execution infrastructure. Do not make it the primary product identity again. |
| Platform-specific TEMU/Amazon Workbench expansion model | RETIRE | marketplace clients, rules, submission adapters and other platform capability code that is independently valid | shared Marketplace / Listing Domain; Store Center / AI Workbench projection | #31/#32 extend platform capability without copying a platform Workbench. |
| Independent top-level Product Center / Listing Center assumption | RETIRE | Product/Listing facts and useful screens under the final IA | Figma `31:463`; `docs/product/final-ui-ia-authority.md` | Domain facts remain; obsolete navigation/center assumptions do not. |
| Old tenant quota / task-limit resource model (#42) | RETIRE | only migration facts needed to preserve valid current resource values | Resource Ledger / Store Service / current entitlement owners | Do not restore task-count quota as the new billing/resource authority. |
| “Build ZITADEL tenant/role model from scratch” backlog abstraction (#39) | RETIRE | existing verified identity/organization behavior | current ZITADEL + Organization/RBAC boundaries | Remaining work is release/staging/production acceptance, not rebuilding a parallel identity model. |
| Separate Product/Asset/Listing fact source introduced for Agent convenience | RETIRE / FORBIDDEN | none | existing canonical domain owners | Agent output is Proposal/trace, not a second business fact source. |
| Parallel Agent RBAC / Tool Registry / durable retry/state-machine ownership | RETIRE / FORBIDDEN | none | current authorization, #133 Tool Registry, Temporal/queue/submission owners | Do not introduce compatibility or duplication to isolate Agent code from current platform contracts. |

## How to use this register

Before reusing a legacy package/service/type:

1. Find the matching row.
2. Follow its `EXTRACT` or `RETIRE` decision.
3. If useful behavior is not covered, add/adjust the row based on current architecture before propagating the legacy dependency.
4. Do not create a third `compatibility` state locally.

If code turns out to be reusable, tests should be rewritten against the current owner/contract. If it only proves an obsolete implementation detail, retire the test with the code.

## New discoveries

When a new legacy area is discovered, record:

```text
Legacy area:
Decision: EXTRACT | RETIRE
Reusable behavior:
Current owner:
Cutover/deletion condition:
Related issue/PR:
```

A future externally required compatibility exception, if one is ever proven necessary, must be documented separately and must not silently become a new register category.

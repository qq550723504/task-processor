# Product Sourcing MVP Status and Closeout Plan

> ACTIVE: acceptance guide under current Product / #30 / #307 contracts.
> CURRENT STATE: inspection at `main @ cae67730c5c0e645d708cb2f6814f14781962bb1`.
> This file does not maintain a second execution queue. Use the current
> [#30](https://github.com/qq550723504/task-processor/issues/30),
> [#307](https://github.com/qq550723504/task-processor/issues/307) and
> [#137](https://github.com/qq550723504/task-processor/issues/137) bodies for scope,
> dependencies, owners and permission.

## 1. Goal and approved boundary

The closeout target is **new controlled import → new Catalog snapshot → explicit
asset approval → readiness**, preserving source lineage and warnings. Creating
a legacy ListingKit task is not the target acceptance criterion.

Read [Product Sourcing Handoff](product-sourcing-handoff.md) for owner/code links,
the [Product Domain contract](../superpowers/specs/2026-09-01-internal-target-architecture-phase3-product-design.md)
for canonical facts and approval, and the
[clean-slate decision](issue30-clean-slate-cutover.md) for data protection.
Use [current status](../refactoring/current-refactoring-status.md) for maturity
and [final UI / IA](final-ui-ia-authority.md) for projection, not permission to
enable an unvalidated capability.

## 2. Implementation versus target

| Area | Observed state on the baseline | Remaining acceptance |
| --- | --- | --- |
| Neutral sourcing | Envelope, normalization, deterministic identity, lineage/warnings and `ToSnapshot` exist | Exact-candidate tests and controlled new import evidence |
| Source adapters | Amazon/1688 mappings are adapter-owned under `internal/integration/crawler/*`; SDS remains specialized | Does not open Amazon target listing or new source expansion |
| Canonical facts | `sourcing.Publisher` → `catalog.Publisher` and durable Catalog composition exist | New-system replay/conflict, immutable history and authorized publication |
| Old 1688 route | App HTTP wires the compatibility task handoff consuming the current Publisher | EXTRACT valid behavior, then RETIRE in a separately approved cutover PR |
| New import HTTP | `internal/app/productsourcing/httpapi` is merged, prepared and unregistered | Current-owner application access/orchestration and approved wiring; no production acceptance claimed |
| Source-account ownership | #303 preflight is merged | Independent account/Organization/profile/access acceptance; merge is not cutover |
| Asset/readiness | Product Asset and ImageAgent approval contracts exist | Source images remain candidates; demonstrate explicit approval and readiness |
| Overall #30 closeout | Not complete | #307 environment/scope, old-execution isolation, new import and explicit approvals remain gates |

Retired `internal/catalog`, `internal/asset`, `internal/productenrich` and
`internal/productimage` are not build/test targets. Current owners are
`internal/product/{catalog,asset,enrichment,image}`. Legacy code is observed
debt, not permission to extend it.

## 3. Data decision and cutover limits

#307 cancels migration of old ProductSnapshot/publication/version, old approved
assets and task/results, historical mapping/stream merging and cross-cutover
replay returning old versions. Do not build alias/resolver/migration machinery
for cancelled requirements or describe them as a repaired defect.

The decision protects Organization/IAM, permissions, accounts, Store, existing
browser profiles/login state/credentials, plans/entitlements, financial ledgers
and unresolved external effects, orders, remote platform state and data outside
the approved business scope. “Do not retain” is not permission to delete a
database, bucket, queue, Redis or profile directory.

The environment owner must confirm exact scope and protected/shared resources.
Application owners must isolate old writers/retries, reject late results and
prove the new scope cannot read or revive old business state. Wiring, cleanup
and real runtime execution require separate approval. Existing prepared-path
gates cannot simply be removed because historical retention was cancelled.
Full steps and unresolved conditions live in the clean-slate decision and #307.

## 4. Verification for an authorized implementation slice

Run existing tests appropriate to changed owners; record the exact SHA, commands,
outcomes and skipped dependencies in its PR. Representative isolated checks:

```powershell
go test ./internal/product/sourcing/... ./internal/product/catalog/... ./internal/product/asset/... -count=1
go test ./internal/integration/crawler/a1688/... ./internal/app/productsourcing/httpapi/... -count=1
go test ./tests/... -count=1
```

These commands target current packages; this document does not claim they ran
or prove production readiness. A future cutover also needs affected application/
persistence tests and final-HEAD CI. Do not execute deployment/deletion/live
browser scripts merely to validate docs.

For a controlled new import, record:

1. Fixture/source identity, permitted raw evidence reference and exact code SHA.
2. Verified effective Organization and actor, source-account/store access result.
3. Envelope, warnings and durable lineage; no manufactured title, price or assets.
4. Catalog receipt, product key and immutable version; new-system same-key replay,
   changed-payload conflict and changed-content key behavior.
5. Missing source/title/assets/price/identity, duplicate URLs, partial variants,
   adapter failures and cancellation/error propagation as applicable.
6. Explicit asset approval identity/result; not-ready response before approval.
7. Deterministic readiness and the existing marketplace/submission owner.
8. Old-execution rejection and protected-scope evidence required by #307.

Fixtures do not require uncontrolled browser automation. Real source access,
local integration and production verification remain separate evidence classes.
Do not expose credentials or sensitive raw data in public reports.

## 5. Closeout and next work

#30 remains incomplete until its current acceptance has evidence: authorized
new import, durable facts/lineage, new-system idempotency and tenant isolation,
explicit asset approval/readiness, old-execution isolation and approved
route/handoff retirement. Record blockers by owner and affected gate. A merge
does not authorize Issue closure, deployment or data operations.

Select any next source through #137 and a bounded execution Issue after the
current gate is satisfied. Its identity, supplier/cost/availability fields,
authentication/scope, pagination/incremental behavior, rate limits, evidence and
errors reuse SourceEnvelope and current Product owners. Do not introduce a
parallel source model, submission owner or independent platform Workbench.

## 6. Historical records

[product-sourcing-validation-2026-07-13.md](product-sourcing-validation-2026-07-13.md)
and the [former MVP plan](https://github.com/qq550723504/task-processor/blob/73aa79cb98def16536123dc1ea3f55c578ed253b/docs/product/product-sourcing-mvp-plan.md)
are HISTORICAL checkpoint evidence for the earlier task-to-preview flow. Their
commands/statuses/results do not define current acceptance.
[next-phase-plan.md](../refactoring/next-phase-plan.md) is historical implementation
evidence, not the execution queue.

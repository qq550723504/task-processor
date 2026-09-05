# ListingKit Code Guide

> Status: current lightweight guide.
>
> This file replaces the old manually maintained code wiki. It should stay short and point to source-of-truth documents instead of duplicating package maps, generated baselines, or large code summaries.

## 1. Project summary

The product direction is AI Commerce Agent Platform. ListingKit remains an
existing execution surface, not final UI authority or a permanent package facade.
Use [final UI / IA](product/final-ui-ia-authority.md) for BusinessTask / AI工作台 /
店铺中心 projection, and current domain contracts for business facts.

Use these documents as the source of truth:

- [README.md](../README.md) for product positioning and runtime navigation.
- [Architecture index](architecture/README.md) for document reading order.
- [Project boundaries](architecture/project-boundaries.md) for ownership and dependencies.
- [Repository structure](development/repository-structure.md) for layout and local artifacts.
- [Current status](refactoring/current-refactoring-status.md) for maturity and gates.

## 2. Current official runtime entrypoints

The complete maintained product/operational command list and build/script owners
live only in [Repository Structure](development/repository-structure.md#顶层目录约定).
`TestCmdContainsOnlyOfficialEntrypoints` checks the actual tree and documentation.
Maintained does not mean production accepted.

## 3. Package ownership snapshot

ACTIVE target ownership:

```text
cmd/* -> internal/app/* runtime assembly
      -> internal/listing/*, internal/product/*, internal/marketplace/*
      -> narrow ports implemented by platform/integration adapters
```

- Product facts use `internal/product/catalog.ProductSnapshot`; approved assets
  use `internal/product/asset.ApprovedAsset`. Source images are candidates until
  explicit approval; ImageAgent owns image execution and approval.
- Sourcing owns neutral envelopes/lineage; concrete source DTOs belong to adapters.
  Follow [Sourcing Handoff](product/product-sourcing-handoff.md) for implemented,
  prepared-only and target flows.
- App owns assembly/lifecycle/route aggregation, not domain rules. Marketplace
  rules go to `internal/marketplace/*`; submission has one existing owner.
- CURRENT STATE at `cae67730c5c0e645d708cb2f6814f14781962bb1`: root ListingKit,
  compatibility shells and some historical marketplace/infra paths remain.
  They are drain debt under the [Legacy Register](refactoring/legacy-register.md),
  not new feature homes. [Module mapping](refactoring/module-target-mapping.md)
  specifies EXTRACT destinations followed by RETIRE; retired Product roots stay absent.
- Identity uses verified effective Organization; see [Auth and Tenancy](architecture/auth-and-tenancy.md).
  Do not add tenantbridge consumers or parallel Agent IAM.
- Execution order belongs to [#137](https://github.com/qq550723504/task-processor/issues/137)
  and the current execution Issue. [next-phase-plan.md](refactoring/next-phase-plan.md)
  is historical evidence only.

## 4. Generated baselines

Dependency baselines and package maps are generated evidence, not hand-maintained documentation.

Run the generator when fresh evidence is needed:

```powershell
./scripts/dependency-baseline.ps1
```

Do not treat old generated snapshots as architecture authority. Stable boundary rules belong in `docs/architecture/*`; current execution posture belongs in `docs/refactoring/current-refactoring-status.md` and the active decision documents.

## 5. Maintenance rule

Keep this guide concise. If a section starts duplicating file lists, package maps, or old implementation details, either move the rule into a stable architecture document or delete the duplication.

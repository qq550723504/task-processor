# ListingKit Code Guide

> Status: current lightweight guide.
>
> This file replaces the old manually maintained code wiki. It should stay short and point to source-of-truth documents instead of duplicating package maps, generated baselines, or large code summaries.

## 1. Project summary

ListingKit is the product-facing shape of this repository. The project has moved beyond a generic `task-processor` and now focuses on AI-assisted product normalization, ListingKit workbench flows, multi-tenant task data, and marketplace listing workflows.

Use these documents as the source of truth:

- `README.md` for product positioning and maintained runtime entrypoints.
- `docs/architecture/README.md` for architecture document reading order.
- `docs/architecture/project-boundaries.md` for package ownership and dependency direction.
- `docs/development/repository-structure.md` for repository layout and local artifact rules.
- `docs/refactoring/current-refactoring-status.md` for the current Now / Next / Later posture.

## 2. Current official runtime entrypoints

Only these production `cmd/` entrypoints are currently maintained:

- `cmd/product-listing-api`
- `cmd/listing-control-plane`
- `cmd/shein-listing`
- `cmd/temu-listing`

The repository structure tests are the executable guard for this list. Historical crawler, subscription, compatibility, and one-off debug commands should not be reintroduced under `cmd/` without updating both the structure document and the guard tests.

## 3. Package ownership snapshot

The current architectural posture is:

```text
cmd/*
  -> internal/app/* runtime assembly
  -> internal/listingkit facade / orchestration during migration
  -> internal/listing, internal/product, internal/asset, internal/catalog
  -> internal/marketplace/* and legacy marketplace packages
  -> internal/infra and integration adapters through narrow interfaces
```

Key rules:

- `internal/app/*` owns runtime assembly, dependency construction, lifecycle, and route aggregation. It should not own marketplace or product rules.
- `internal/listingkit` remains a product orchestration and compatibility facade. New platform-specific rules should not be added to the root package.
- Generic listing behavior should move toward `internal/listing/*` packages when the seam is stable and platform-neutral.
- SHEIN-specific publishing and workspace rules should live under `internal/marketplace/shein/*` or the current compatibility publishing packages.
- Product facts and reusable asset facts should stay outside root ListingKit.
- Concrete external clients should be hidden behind narrow interfaces when business packages need them.

## 4. Generated baselines

Dependency baselines and package maps are generated evidence, not hand-maintained documentation.

Run the generator when fresh evidence is needed:

```powershell
./scripts/dependency-baseline.ps1
```

Do not treat old generated snapshots as architecture authority. Stable boundary rules belong in `docs/architecture/*`; current execution posture belongs in `docs/refactoring/current-refactoring-status.md` and the active decision documents.

## 5. Maintenance rule

Keep this guide concise. If a section starts duplicating file lists, package maps, or old implementation details, either move the rule into a stable architecture document or delete the duplication.

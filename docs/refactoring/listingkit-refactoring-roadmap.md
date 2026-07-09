# ListingKit Refactoring Roadmap

> Status: directional / historical roadmap.
>
> Original active baseline: 2026-06-24, `master` commit `4829df08677a8b21960bfef59c702c3dc5027a2e`.
>
> Current authority: use `docs/refactoring/current-refactoring-status.md` for Now / Next / Later, `docs/refactoring/listingkit-boundary-checkpoint.md` for current ListingKit stop lines, and `docs/refactoring/next-phase-plan.md` for the next execution queue.

## 1. Why this document still exists

This roadmap captured the major ListingKit refactoring direction when the project was moving from broad file splitting into capability-driven boundary work.

It remains useful as context for why the project chose these principles:

- use capability-driven, small-step migration instead of a large rewrite;
- keep ListingKit as orchestration and compatibility facade, not a dumping ground for every platform rule;
- move generic listing behavior toward `internal/listing/*` only when the seam is stable;
- move SHEIN publishing/workspace rules toward marketplace-owned packages;
- keep product facts and reusable asset facts outside root ListingKit;
- keep app/runtime packages focused on assembly rather than business policy.

It is **not** the current active execution plan. Do not use this document to justify a new extraction if the newer status/checkpoint documents say to stop.

## 2. Current direction summary

The current direction is:

```text
1. Close out runtime and boundary ownership first.
2. Expand product sources second.
3. Defer full new sales-platform workbenches until the SHEIN template is stable.
```

The current stop line is:

```text
Do not keep splitting internal/listingkit files only because a file can be made smaller.
Only move code when ownership, dependency direction, or guard coverage improves.
```

## 3. Still-valid long-term goals

These goals remain valid, but the timing and exact package names must follow the current authority documents:

- SDS-to-SHEIN batch production should have durable task ownership and recoverable execution.
- Multi-platform Listing packages should reuse product facts and assets instead of duplicating source logic inside ListingKit.
- Submission should remain observable, idempotent, recoverable, and guarded by one generic submission domain plus marketplace-specific publishing packages.
- Product, asset, platform rule, and external integration boundaries should stay clear enough for review and tests.
- New marketplace work should not copy old ListingKit root coupling into a new package tree.

## 4. Historical target module map

The historical target map was directional, not a migration checklist:

```text
internal/app
  runtime/                 process startup, dependency assembly, lifecycle
  httpapi/                 route/runtime assembly and auth context

internal/catalog           canonical product facts
internal/asset             reusable asset and image facts

internal/listing
  studio/                  platform-neutral batch/item/attempt/design seams
  preview/                 platform-neutral preview rules
  submission/              generic submit attempt, retry, lock, event, recovery rules

internal/marketplace/shein
  workspace/               SHEIN operator-facing review and repair rules
  publishing/              SHEIN draft/publish and remote-result rules

internal/product/sourcing  product source identity and normalization
internal/integration/*     external protocol and client adapters

internal/listingkit        product API facade, orchestration, legacy compatibility glue
```

Some current package names still differ from this ideal target. Do not rename packages for consistency alone.

## 5. Current replacement documents

Use these instead of this roadmap for current decisions:

- `docs/refactoring/current-refactoring-status.md`
  - current Now / Next / Later posture;
- `docs/refactoring/next-phase-plan.md`
  - immediate validation and PR queue;
- `docs/refactoring/listingkit-boundary-checkpoint.md`
  - current ListingKit stop lines and safe next seams;
- `docs/product/product-sourcing-handoff.md`
  - product-source expansion ownership and handoff rules;
- `docs/architecture/project-boundaries.md`
  - repository-wide ownership and dependency direction rules.

## 6. Review warning

If this roadmap conflicts with a newer checkpoint, ADR, architecture document, or guard test, follow the newer document or test.

When in doubt, prefer the rule that keeps:

```text
app/runtime assembly thin,
ListingKit root narrow,
product/source facts outside ListingKit,
marketplace policy in marketplace packages,
external clients behind local interfaces,
generated baselines as local evidence only.
```

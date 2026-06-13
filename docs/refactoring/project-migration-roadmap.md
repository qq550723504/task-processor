# Project Migration Roadmap

> Status: active project-level migration roadmap aligned with [`../architecture/project-target-architecture.md`](../architecture/project-target-architecture.md).

## 1. Purpose

This roadmap defines how the project should move from the current package layout toward the approved business-domain-first target architecture.

It is intentionally staged to avoid a high-risk rewrite.

## 2. Core Migration Principles

1. Keep each PR behavior-preserving unless the PR explicitly introduces a product change.
2. Prefer file-group modularization before real package extraction.
3. Create target directories and documentation before broad code moves.
4. Keep old public entrypoints stable through compatibility facades.
5. Move one bounded capability at a time.
6. Record ownership ambiguity instead of guessing through a broad move.
7. Treat crawlers as sourcing adapters and keep them out of listing publication domains.

## 3. Migration Stages

### Stage 0: Freeze the target shape

Goal:

- make the approved architecture explicit enough that future PRs can align to it

Deliverables:

- `docs/architecture/project-target-architecture.md`
- `docs/architecture/project-boundaries.md`
- `docs/refactoring/project-migration-roadmap.md`
- `docs/refactoring/module-target-mapping.md`

Acceptance criteria:

- target domains are named
- dependency direction is explicit
- compatibility strategy is explicit
- crawler placement is explicit

### Stage 1: Create target skeletons and rules

Goal:

- establish the new directory vocabulary without forcing immediate package moves

Recommended actions:

1. Create empty or lightly documented target directories under:
   - `internal/listing/`
   - `internal/marketplace/`
   - `internal/product/`
   - `internal/integration/crawler/`
   - `internal/compatibility/listingkit/`
2. Add README or package notes where a new directory would otherwise be ambiguous.
3. Update contributor guidance so new code prefers the target domains.

Acceptance criteria:

- new work has an approved landing zone
- the project stops drifting further into legacy generic packages

Current read:

- `internal/marketplace/{shein,amazon,temu,walmart}` already exists and is the approved landing zone for new marketplace-specific rules
- `internal/integration/crawler/{amazon,a1688}` already exists and is the approved landing zone for new crawler-adapter work
- `internal/product/sourcing` already exists and is the approved landing zone for normalized source-to-product handoff logic
- `internal/compatibility/listingkit` already exists and is the approved landing zone for explicit ListingKit compatibility shims

This means Stage 1 is no longer about inventing the directory vocabulary. It is now about enforcing that new code prefers these target homes instead of re-expanding legacy roots.

### Stage 2: Reduce `internal/listingkit` into domain-ready file groups

Goal:

- turn `internal/listingkit` from a complexity center into a migration-friendly compatibility shell

Priority order:

1. preview
2. submission
3. revision
4. studio
5. task and workflow
6. settings and admin bridges

Recommended method:

1. keep `package listingkit` first
2. group files by future target ownership
3. add package-private facades and file-boundary tests
4. only extract real subdirectories after ownership is stable

Acceptance criteria:

- root `internal/listingkit` logic becomes thinner
- new code stops landing in generic mixed files
- future `internal/listing/*` extraction gets simpler

### Stage 3: Extract the `internal/listing/*` domain

Goal:

- move stable listing-task orchestration capabilities out of the legacy facade

Target order:

1. `listing/preview`
2. `listing/submission`
3. `listing/revision`
4. `listing/studio`
5. `listing/task`
6. `listing/workflow`
7. `listing/export`
8. `listing/settings`

Compatibility strategy:

- keep old service entrypoints delegating to new listing services
- migrate imports inward before deleting legacy shells

Acceptance criteria:

- `listing` becomes the real owner of listing business orchestration
- `listingkit` is mostly delegation and compatibility

### Stage 4: Normalize marketplace ownership

Goal:

- move platform-specific rules to `internal/marketplace/*`

Recommended order:

1. SHEIN
2. Amazon
3. TEMU
4. Walmart

Per-marketplace target shape:

```text
internal/marketplace/<platform>/
  publishing/
  workspace/
  model/
  api/
```

Migration rules:

- publishing rules move before compatibility shell cleanup
- workspace/editor/repair rules move before listing-domain cleanup
- old thin bridge helpers may remain temporarily in legacy packages

Acceptance criteria:

- platform rules no longer accumulate in generic ListingKit packages
- each marketplace has a clearer internal home

### Stage 5: Normalize product and sourcing ownership

Goal:

- make product facts and sourcing pipelines first-class domains

Recommended order:

1. stabilize `product/catalog`
2. stabilize `product/asset`
3. stabilize `product/image`
4. stabilize `product/ai`
5. introduce `product/sourcing`
6. move crawler consumers to `product/sourcing`

Crawler-specific rule:

- `integration/crawler/amazon` and `integration/crawler/a1688` own extraction adapters only
- `product/sourcing` owns normalization, enrichment, and handoff to product facts

Acceptance criteria:

- product facts are no longer hidden behind listing-only flows
- new source integrations do not distort marketplace or listing package ownership

Near-term sourcing split rule:

1. `internal/crawler/amazon` and `internal/crawler/alibaba1688` should shrink toward source-specific extraction and browser/runtime adapter concerns only.
2. `internal/integration/crawler/amazon` and `internal/integration/crawler/a1688` should become the steady-state home for new crawler-adapter code.
3. `internal/product/sourcing` should become the steady-state home for normalization, enrichment handoff, and source-result contracts.
4. Marketplace publishing packages must not absorb crawler extraction ownership just because a source is commonly used by one marketplace flow.

### Stage 6: Clean up runtime assembly and external adapters

Goal:

- make app, platform, and integration boundaries match the target architecture

Recommended order:

1. `app/httpapi` and runtime assembly cleanup
2. `platform/*` normalization
3. `integration/*` normalization
4. interface cleanup where business packages depend on concrete clients

Acceptance criteria:

- runtime assembly is wiring only
- infrastructure and external clients are easier to replace and test

### Stage 7: Shrink and relocate the compatibility shell

Goal:

- finish the reduction of legacy ListingKit ownership

Recommended actions:

1. inventory remaining logic in `internal/listingkit`
2. classify what is true compatibility versus accidental ownership
3. move compatibility-only pieces toward `internal/compatibility/listingkit`
4. delete dead wrappers after downstream imports are migrated

Acceptance criteria:

- `listingkit` no longer acts as the center of the architecture
- compatibility becomes explicit rather than accidental

## 4. Recommended Near-term Work Queue

The next project-level work queue should be:

1. keep the architecture and roadmap docs current
2. continue shrinking `internal/listingkit` preview, submission, revision, and studio hotspots
3. keep `internal/listingkit/httpapi` moving toward pure assembly and runtime-adapter composition
4. document and then normalize SHEIN ownership
5. define `product/sourcing` inputs and outputs
6. inventory Amazon and 1688 crawler outputs and consumers

Recommended landing-zone rules for new work right now:

- new Amazon source crawling code goes to `internal/integration/crawler/amazon`
- new 1688 source crawling code goes to `internal/integration/crawler/a1688`
- new source normalization or product handoff code goes to `internal/product/sourcing`
- new marketplace-specific publishing or workspace behavior goes to `internal/marketplace/<platform>/*`
- new listing-task orchestration goes to `internal/listing/*` when available, otherwise stays in narrowly grouped `internal/listingkit` files pending extraction
- new compatibility-only bridges go to `internal/compatibility/listingkit`
- legacy roots such as `internal/crawler`, `internal/amazon`, `internal/amazonlisting`, and broad mixed `internal/listingkit` files should be treated as shrink-only unless a behavior-preserving bridge absolutely requires touching them

## 5. What Not To Do

Do not:

- rename every package tree in one PR
- move all crawler code and sourcing code together with marketplace moves
- delete `internal/listingkit` before the replacement services exist
- create technical-layer packages that ignore the agreed business boundaries
- move runtime code and business logic in the same sweeping change

## 6. Exit Criteria

This roadmap has materially succeeded when:

- the main business work lands in `listing`, `marketplace`, `product`, `integration`, and `platform`
- `internal/listingkit` is thin or compatibility-only
- Amazon and 1688 crawlers are clearly treated as sourcing adapters
- package ownership can be explained without referencing historical accidents
- dependency enforcement can be tightened with fewer legacy exceptions

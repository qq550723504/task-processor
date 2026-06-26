# Decision: Refactoring Closeout, Product Sources, Then Sales Platforms

> Status: accepted direction for the next planning cycle.
>
> Date: 2026-06-26.
>
> Scope: Task Processor / ListingKit growth sequencing after the SHEIN path and Go Listing Control Plane hardening work.

## Decision

Prioritize the next directions in this order:

```text
1. Refactoring closeout and runtime boundary stabilization.
2. Product-source expansion through product sourcing, canonical facts, and asset normalization.
3. Full new sales-platform expansion only after the SHEIN template is stable.
```

## Context

The project now has a validated SHEIN-oriented ListingKit path and a Go Listing Control Plane backend closeout path. However, the project is still in a structural closeout phase rather than a broad expansion phase.

The current risks are:

- remaining Management Client compatibility shell in listing runtime boundaries;
- deferred or not-yet-recorded validation evidence for latest CI, race, build, and runtime smoke checks;
- `internal/listingkit` still needing strict ownership stop-lines;
- frontend operational visibility needing a clear deployment / publication path when the UI artifact is separate;
- risk that new platform work will copy old coupling before the SHEIN template is fully stabilized.

## Reasoning

### Why refactoring closeout first

Closeout work reduces the cost and risk of every later product-source or platform expansion.

The current closeout target is not broad package reshaping. It is:

- retiring runtime-facing Management Client shells;
- proving CI / race / build gates on latest master;
- keeping SHEIN runtime smoke tests stable;
- freezing ListingKit boundary stop-lines;
- ensuring new rules land in their true owner packages.

### Why product sources second

Product-source expansion increases supply while reusing the existing SHEIN listing path.

It mainly exercises:

- product source identity;
- canonical product facts;
- image and asset normalization;
- cost / source-SDS identity;
- source facts readiness;
- crawler adapter thinness.

This is a lower-risk growth direction than adding a full new marketplace because it does not require another complete publishing runtime, dispatch model, submission state machine, and operational workbench.

### Why sales platforms later

A new sales platform multiplies marketplace-specific complexity:

- category and attribute systems;
- image rules;
- SKU / variation rules;
- price and inventory rules;
- draft / publish APIs;
- remote validation;
- error mapping;
- failure recovery;
- operator repair workflows;
- platform-specific observability.

Starting this before the SHEIN template is stable risks copying the old coupling into a new package tree.

## Allowed now

The following work is allowed in the current cycle:

- closeout of Management Client runtime shells;
- current validation evidence for CI, race, build, and runtime smoke checks;
- guard-backed SHEIN publishing / workspace rule seams;
- product-source inventory;
- product source identity and normalization;
- canonical product facts;
- image / asset normalization;
- source-SDS cost identity and cost mapping when it stays product/source owned;
- capability inventory for TEMU / Amazon / Walmart without building full workbenches.

## Deferred

The following work should remain deferred until the closeout criteria are met:

- full TEMU / Amazon / Walmart workbench expansion;
- a new platform auto-publish runtime;
- a second dispatch scheduler or watchdog owner;
- a new marketplace submission state machine outside `internal/listing/submission` and marketplace-owned publishing packages;
- broad package rename / reshaping for directory consistency only;
- new marketplace-specific business rules inside root `internal/listingkit`.

## Exit criteria before full sales-platform expansion

Full new sales-platform workbench expansion may be reconsidered when:

```text
[ ] Latest master has recorded go test ./... -count=1 evidence.
[ ] Listing control-plane and listingadmin race gates are recorded.
[ ] cmd/listing-control-plane and cmd/shein-listing builds are recorded.
[ ] SHEIN listing runtime browser startup smoke test is stable.
[ ] Management Client no longer appears as the runtime semantic dependency for listing task, store, product, pricing, or health capabilities.
[ ] ListingKit boundary checkpoint is current and accepted.
[ ] Product-source normalization has at least one stable source path outside root ListingKit.
[ ] SHEIN publishing/workspace rules remain guarded outside root ListingKit.
```

## Consequences

This decision intentionally favors fewer large feature starts and more structural confidence.

Expected benefits:

- lower regression risk for the existing SHEIN path;
- easier product-source expansion;
- clearer package ownership before multi-platform copying;
- fewer compatibility shells carried into new marketplaces;
- better review discipline for future feature PRs.

Expected cost:

- full marketplace expansion is delayed;
- some short-term growth ideas must be constrained to inventories, contracts, or payload-preview exploration;
- docs and validation evidence must be kept current before broad code movement resumes.

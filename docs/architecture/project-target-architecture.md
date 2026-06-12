# Project Target Architecture

> Status: approved target architecture for project-wide modularization. This document complements [`project-boundaries.md`](./project-boundaries.md) by describing the intended end-state package shape, not just the dependency rules.

## 1. Goal

The project should converge toward a modular monolith organized by business domains first, with runtime infrastructure and external-system adapters clearly separated.

This target shape is intended to solve the current problems:

- `internal/listingkit` still acts as the main complexity sink.
- Marketplace rules are spread across generic packages and legacy facades.
- Product facts, listing orchestration, platform-specific rules, runtime assembly, and external integrations are not consistently separated.
- Crawlers such as Amazon and 1688 exist as useful capabilities but are not yet placed inside a stable long-term architecture.

## 2. Final Top-level Shape

Preferred project shape:

```text
cmd/
  product-listing-api/
  product-listing-worker/
  listingkit-temporal-worker/
  tools/

internal/
  app/
    httpapi/
    worker/
    runtime/

  platform/
    config/
    logging/
    metrics/
    authz/
    database/
    redis/
    queue/
    temporal/
    objectstore/

  integration/
    openai/
    s3/
    playwright/
    shein/
    amazon/
    temu/
    walmart/
    crawler/
      amazon/
      a1688/

  product/
    catalog/
    asset/
    image/
    ai/
    sourcing/

  listing/
    task/
    workflow/
    preview/
    export/
    revision/
    submission/
    studio/
    settings/

  marketplace/
    shein/
      publishing/
      workspace/
      model/
      api/
    amazon/
      publishing/
      workspace/
      model/
      api/
    temu/
      publishing/
      workspace/
      model/
      api/
    walmart/
      publishing/
      workspace/
      model/
      api/

  compatibility/
    listingkit/

  shared/
    errors/
    timeutil/
    pagination/
    validation/

web/
  listingkit-ui/

docs/
  architecture/
  refactoring/
  api/
  product/
```

This is the destination map, not a one-shot rename plan.

## 3. Domain Responsibilities

### 3.1 `internal/listing/*`

Owns listing-task business orchestration:

- task lifecycle
- workflow entrypoints
- preview aggregation
- export aggregation
- revision/history coordination
- submission orchestration
- studio orchestration
- settings orchestration

This domain is responsible for cross-platform listing flows, but it must not own marketplace-specific rules.

### 3.2 `internal/marketplace/*`

Owns marketplace-specific behavior and models.

Examples:

- category and attribute rules
- publishing payload rules
- editor and workspace rules
- platform-specific validation
- platform-specific DTOs and API surface models

The project should prefer marketplace ownership over generic ListingKit ownership whenever behavior is specific to SHEIN, Amazon, TEMU, or Walmart.

### 3.3 `internal/product/*`

Owns reusable product facts and content-production capabilities:

- canonical product facts
- asset bundle ownership
- image-processing behavior
- AI-assisted product enrichment
- normalized sourcing flows

This domain should stay reusable across marketplaces and listing flows.

### 3.4 `internal/product/sourcing`

Owns normalized sourcing pipelines for external product-data inputs.

Responsibilities:

- unified sourcing service surface
- source-result normalization
- enrichment and fact extraction
- image and asset candidate extraction
- handoff into `product/catalog`, `product/asset`, and downstream listing flows

This module should consume crawler outputs but should not implement crawling itself.

### 3.5 `internal/integration/crawler/*`

Owns external crawling adapters.

Initial planned sources:

- `integration/crawler/amazon`
- `integration/crawler/a1688`

Responsibilities:

- page fetch / browser automation
- anti-bot or source-specific runtime adaptation
- source-specific DOM or payload parsing
- raw extraction result output

These adapters are sourcing inputs, not listing-domain owners and not marketplace publishing owners.

### 3.6 `internal/platform/*`

Owns runtime infrastructure used by the application itself:

- config
- logging
- metrics
- database access bootstrap
- Redis bootstrap
- queue bootstrap
- Temporal runtime bootstrap
- object-storage runtime support

### 3.7 `internal/integration/*`

Owns external-system adapters:

- OpenAI
- S3
- Playwright
- marketplace API clients
- crawler adapters

### 3.8 `internal/compatibility/listingkit`

Owns backward-compatible facades during migration.

Its long-term role:

- keep old entrypoints stable while internals move
- translate old DTOs or service entrypoints into new domain services
- avoid forcing a single broad rename PR

New business logic should not be added here unless it is genuinely compatibility-only.

## 4. Strategic Architectural Decisions

### 4.1 Business-domain-first packaging

The project should not converge to a purely technical layout such as `handler/service/repository` across the whole monolith.

Reason:

- The main complexity is business workflow and platform-rule ownership, not CRUD layering.
- Business-domain-first packages make ownership, review, and migrations easier to explain.

### 4.2 `listingkit` becomes a compatibility shell

`internal/listingkit` should stop being the long-term home of new business logic.

Migration intent:

- first shrink it into a thin orchestration and compatibility facade
- later move the compatibility role under `internal/compatibility/listingkit`

### 4.3 Marketplace rules are grouped by marketplace

The project should prefer:

```text
internal/marketplace/shein/*
internal/marketplace/amazon/*
internal/marketplace/temu/*
internal/marketplace/walmart/*
```

instead of grouping all publishing logic or all workspace logic together across platforms.

Reason:

- platform rules are tightly coupled within the platform
- ownership is clearer
- incremental migration is easier

### 4.4 Crawlers are sourcing adapters, not publishing modules

Amazon and 1688 crawlers should be treated as data-source adapters for `product/sourcing`.

That avoids mixing:

- Amazon as a source of product data
- Amazon as a target marketplace for listing publication

These are related but distinct business concepts.

## 5. Dependency Direction

Preferred dependency direction:

```text
cmd
  -> app
  -> listing / product / marketplace
  -> platform / integration

app
  -> listing / product / marketplace
  -> platform / integration
  -> shared

listing
  -> product
  -> marketplace
  -> shared

marketplace
  -> product
  -> integration
  -> shared

product
  -> shared

platform
  -> shared

integration
  -> shared

compatibility
  -> listing
  -> marketplace
  -> product
  -> shared
```

Important consequences:

- `product` must not depend on `listing`.
- `marketplace/*` must not depend on `compatibility/listingkit`.
- `app/*` must remain wiring-focused.
- `compatibility/*` may depend inward on the new domains, but the new domains should not depend back on compatibility.

## 6. What This Architecture Enables

If the project converges toward this shape, it should become easier to:

- review package ownership
- move code without large multi-domain PRs
- add new marketplaces without bloating `listingkit`
- add new product-data sources without inventing new special-case package shapes
- keep runtime assembly separate from business rules
- gradually enforce dependency checks in CI

## 7. Non-goals

This target architecture does not require:

- immediate broad directory renames
- immediate package extraction for every domain
- microservice splits
- one-time migration of all legacy packages
- replacing every old package name before behavior-preserving refactoring is complete

## 8. Relationship to Existing Planning Documents

This document should be used together with:

- [`project-boundaries.md`](./project-boundaries.md)
- [`../refactoring/project-wide-refactoring-plan.md`](../refactoring/project-wide-refactoring-plan.md)
- [`../refactoring/project-migration-roadmap.md`](../refactoring/project-migration-roadmap.md)
- [`../refactoring/module-target-mapping.md`](../refactoring/module-target-mapping.md)

This document defines the desired destination shape.

# Project Target Architecture

> Status: approved target architecture for project-wide modularization.  
> Legacy handling: `docs/refactoring/legacy-hard-cut-policy.md` and `docs/refactoring/legacy-register.md`.  
> This document describes the intended end state; current runtime reality is tracked separately in `docs/refactoring/current-refactoring-status.md`.

## 1. Goal

The project converges toward a business-domain-first modular monolith with explicit Product, Listing, Marketplace, Agent/Tool, Organization, commercial/resource, runtime, and external-integration ownership.

The target architecture is designed to eliminate the historical problems where:

- root `internal/listingkit` becomes a complexity sink;
- platform rules, Product facts, Listing orchestration, Task runtime and HTTP assembly mix ownership;
- old Task-first product models leak into the final product experience;
- migration bridges become permanent extension points;
- Agent work creates duplicate facts, state machines, retry owners, RBAC, provider routing or marketplace clients.

**Compatibility is not a target architecture domain.** Existing compatibility/legacy code is drained through `EXTRACT | RETIRE` and disappears from the end state.

## 2. Preferred top-level shape

The exact directory tree may evolve, but the intended ownership shape is:

```text
cmd/

internal/
  app/
    httpapi/
    runtime/
    worker/

  product/
    catalog/
    sourcing/
    enrichment/
    asset/
    image/

  listing/
    task/
    workflow/
    preview/
    revision/
    submission/
    settings/

  marketplace/
    shein/
    amazon/
    temu/
    ...

  commercetool/

  agent/
    runtime/
    prompt/
    ...

  organization/
  commercial/
  ledger/
  knowledge/
  resourcecatalog/

  integration/
    identity/
    authz/
    crawler/
      amazon/
      a1688/
    shein/
    amazon/
    temu/
    ...

  platform/
    config/
    logging/
    metrics/
    database/
    redis/
    queue/
    temporal/
    objectstore/

  shared/

web/
  ... final Product Projection defined by Figma authority ...
```

There is intentionally no final `internal/compatibility/*` owner in this shape.

## 3. Core domain responsibilities

### 3.1 Product

`internal/product/*` owns reusable commerce product facts and capabilities:

- `ProductSnapshot` / canonical product facts;
- source identity, evidence, lineage and normalized sourcing;
- enrichment Proposal behavior;
- approved asset facts;
- provider-neutral image capability.

Product does not depend on Listing, crawler implementations, root ListingKit, compatibility bridges, Agent frameworks, or marketplace SDKs.

### 3.2 Listing

`internal/listing/*` owns marketplace-neutral Listing orchestration and stable listing facts/seams where they are genuinely cross-platform:

- Platform Draft / Listing coordination;
- preview/read models;
- revision/history coordination;
- submission orchestration around the existing single submission-state owner;
- deterministic cross-platform seams.

Listing is a domain capability, not a requirement for a top-level “Listing Center” product navigation.

### 3.3 Marketplace

`internal/marketplace/<platform>/*` owns platform-specific business behavior:

- category/attribute rules;
- platform validation adapters;
- publishing payload rules;
- platform-specific models and capability seams.

SHEIN/TEMU/Amazon capabilities are projected mainly through Store Center / Store Products and AI Workbench according to final Figma IA. They do not each get an independent Workbench architecture.

### 3.4 Product Sourcing and crawlers

Crawler implementations live under Integration and output source-specific/raw extraction results:

```text
integration/crawler/*
        ↓
product/sourcing
        ↓
ProductSnapshot / ApprovedAsset / downstream Listing
```

`product/sourcing` owns `SourceIdentity`, `SourceEnvelope`, normalization, lineage, warnings and handoff. Crawlers do not own canonical Product facts; Listing/Marketplace do not own crawling.

### 3.5 Commerce Tool

`internal/commercetool` owns the framework-neutral Tool contract and governance:

- definition/schema;
- registry;
- risk class;
- allowlist/policy;
- invocation/audit boundary;
- stable error contract.

Business behavior stays in the owning domain and is exposed through narrow adapters. Tools do not become a second implementation of Product/Marketplace/Listing logic.

Agent/Tool code does not directly import GORM repositories, provider SDKs or marketplace clients.

### 3.6 Agent / AI Capability

Agent runtime owns bounded Agent execution semantics such as AgentRun/AgentStep, budgets, tool allowlists, checkpoints/interrupts and stop reasons.

AI Capability owns provider-neutral model capability/routing/policy/ledger/cost/fallback control-plane behavior.

Agent does not own:

- canonical Product/Listing/Asset facts;
- BusinessTask product lifecycle;
- Temporal durable workflow ownership;
- marketplace submission state;
- provider API-key/routing implementation;
- an independent RBAC system.

### 3.7 BusinessTask / product projection

Final product semantics are:

```text
BusinessTask
  -> AgentRun
    -> AgentStep
      -> ToolCall / ModelCall
        -> Temporal / Queue / Internal Task
```

BusinessTask is the user-facing business goal/progress/decision/result object. Internal Task/Workflow remains execution infrastructure.

The final UI/IA and user-facing names are governed by Figma `31:463` and `docs/product/final-ui-ia-authority.md`, not by historical ListingKit navigation.

### 3.8 Organization / identity

ZITADEL remains the trusted identity provider. Organization/membership/authorization semantics stay with current Organization/identity owners and existing RBAC boundaries.

`internal/tenantbridge` is not target architecture. It is active legacy debt tracked for `EXTRACT -> RETIRE` by #301. New schemas/contracts use current Organization identity directly.

### 3.9 App / Platform / Integration

- `internal/app/*`: composition, bootstrap and lifecycle only.
- `internal/platform/*`: runtime infrastructure owned by the application.
- `internal/integration/*`: external systems and concrete adapters.
- `internal/shared`: small stable primitives only.

App must not accumulate business rules; Integration must not become the business owner of Product/Listing/Marketplace semantics.

## 4. Legacy Hard-Cut

The target architecture does not include an internal compatibility layer.

Current legacy handling:

```text
EXTRACT
  reusable behavior
    -> current owner
    -> switch callers
    -> remove old dependency

RETIRE
  obsolete design
    -> no extension
    -> cut over
    -> delete / keep absent
```

Consequences:

- `internal/compatibility/*` is a retirement zone, not a final package layer;
- root `internal/listingkit` is drained by #29, not converted into a permanent facade;
- active 1688 → legacy ListingKit handoff is drained by #30;
- `internal/tenantbridge` is drained by #301;
- already removed `internal/productenrich`, `internal/productimage`, old `internal/catalog`, old `internal/asset`, and `internal/imageasset` remain absent;
- new code does not add legacy fallback, permanent dual-read/write, bidirectional new↔old synchronization, or second fact/state owners.

If a future externally observable contract or persisted runtime state truly requires temporary compatibility, it must be approved as a specific exception with owner, scope and deletion condition. It does not create a general Compatibility domain.

## 5. Dependency direction

Preferred dependency direction is owner-oriented, not layer-oriented:

```text
cmd
  -> app

app
  -> product / listing / marketplace / agent / commercetool / organization / commercial / ...
  -> platform / integration

Agent adapters
  -> Commerce Tool contracts
  -> narrow domain ports

listing
  -> product
  -> marketplace seams where required

marketplace
  -> product/domain contracts
  -> integration through narrow adapters

product
  -> shared / local contracts

platform
  -> shared

integration
  -> shared / external SDKs
```

Hard constraints:

- Product does not depend on Listing or legacy compatibility paths.
- Marketplace does not depend on `internal/compatibility/listingkit`.
- New architecture does not depend on `internal/tenantbridge`.
- New Product/Agent/Tool/Console code does not depend on root ListingKit as a convenience service owner.
- Agent/Tool does not bypass domain authorization or deterministic validation.
- There is one owner for submission state, durable retry, IAM, Tool registry and canonical facts.

## 6. Deterministic validation and side effects

Readiness/Validator is deterministic authority shared by UI/fixed pipeline/Tool/Agent. A model can propose a repair; it cannot reinterpret a failed validator as success.

Agent evolution is read/compute/propose first. Write/publish side effects require independent approval/version/authorization/idempotency/audit/readiness gates. Agent Runtime does not create a second publication state machine.

## 7. Migration method

Do not execute another broad directory rewrite. For each legacy seam:

1. name the current owner;
2. identify reusable behavior;
3. classify `EXTRACT | RETIRE`;
4. add/verify tests on the current contract;
5. switch one bounded caller/path;
6. remove the old dependency/path;
7. add a guard so it cannot be reintroduced.

Use `docs/refactoring/module-target-mapping.md` and `docs/refactoring/legacy-register.md` for the active drain inventory.

## 8. Non-goals

This architecture does not require:

- microservice decomposition;
- immediate renaming of every historical package;
- a generic MarketplaceAdapter framework;
- a generic Listing Workspace;
- a permanent compatibility/anti-corruption layer for internal legacy design;
- duplicate Product/Listing/Asset facts for Agent convenience;
- replacing Temporal with Agent Runtime;
- Multi-Agent orchestration before one bounded Agent proves value.

## 9. Authority relationship

Use together with:

- `docs/product/final-ui-ia-authority.md` — final UI/IA/Product Projection;
- Product/architecture specs — business/safety/contracts;
- `docs/refactoring/legacy-hard-cut-policy.md` — legacy treatment;
- `docs/refactoring/legacy-register.md` — current legacy drain inventory;
- `docs/refactoring/current-refactoring-status.md` — current implementation reality;
- GitHub #137 — executable backlog order.

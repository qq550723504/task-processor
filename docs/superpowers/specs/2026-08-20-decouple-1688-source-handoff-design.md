# Decouple the 1688 Source Handoff

**Status:** Approved direction; written specification pending final user review

**Date:** 2026-08-20

**Scope:** Dependency-boundary refactor only; preserve existing 1688 HTTP and task-creation behavior

## Context

The current 1688-to-ListingKit path crosses both sides of the product boundary:

- `internal/product/sourcing` imports the legacy
  `internal/crawler/alibaba1688/model` DTOs.
- `internal/product/sourcehandoff` imports `internal/listingkit` request, task,
  identity, store-access, and option types.
- `internal/product/sourcehandoff/a1688/httpapi` also imports Gin-facing and
  ListingKit compatibility types.

This conflicts with the target dependency direction. Product sourcing should
own normalized product/source facts, while crawler adaptation and legacy
ListingKit translation should remain outside the product domain.

The current flow is working and covered by source, handoff, HTTP, and replay
tests. The refactor must therefore change ownership without changing the
external endpoint, authentication rules, store validation, source lineage,
warning semantics, task request contents, or response shape.

## Goals

1. Remove production imports of `internal/listingkit`,
   `internal/crawler/**`, and `internal/integration/**` from
   `internal/product/**`.
2. Keep `internal/product/sourcing.SourceEnvelope` as the normalized source
   facts contract.
3. Make the 1688 integration adapter the only owner of conversion from the
   legacy crawler model into a product-owned snapshot.
4. Move ListingKit-specific source handoff and HTTP compatibility code under
   `internal/compatibility/listingkit`.
5. Preserve the existing 1688 create-task HTTP contract and observable
   behavior.
6. Add a CI-enforced import boundary that prevents the removed dependencies
   from returning.

## Non-goals

- Do not add URL crawling to the create-task command.
- Do not change the `SourceEnvelope`, catalog facts, asset facts, ListingKit
  task lifecycle, store-access rules, or tenant identity semantics.
- Do not rename or rewrite the legacy 1688 crawler.
- Do not redesign `listingkit.GenerateOptions` or duplicate it in product
  packages.
- Do not introduce a new service or message broker.
- Do not migrate all repository architecture checks to a new linter in this
  slice.
- Do not change deployment, runtime configuration, or production data.

## Approaches Considered

### A. Move compatibility code and introduce a product-owned snapshot

This is the selected approach.

- Product sourcing owns a small 1688 snapshot made only of the fields required
  for normalization.
- The integration adapter converts the legacy crawler DTO into that snapshot.
- ListingKit request/task/store/identity translation moves under the existing
  compatibility boundary.

This removes both forbidden dependencies without duplicating ListingKit
contracts. It also keeps the existing domain normalization code and tests.

### B. Keep `product/sourcehandoff` and replace ListingKit types with neutral ports

Rejected because `GenerateOptions`, task results, store-access errors, and
request identity would need parallel product-owned representations. The result
would be a second ListingKit contract and a large mapping surface, not a clean
domain boundary.

### C. Put all conversion and orchestration in `internal/app/httpapi`

Rejected because the application assembly layer would inherit source-field
mapping, task validation, and compatibility response logic. That would make app
assembly another business-logic owner and conflict with its wiring-only role.

## Dependency Decision

The resulting direction is:

```text
legacy crawler model
  -> integration/crawler/a1688 adapter
  -> product/sourcing snapshot + SourceEnvelope

compatibility/listingkit/sourcehandoff
  -> integration/crawler/a1688 adapter
  -> product/sourcing
  -> listingkit compatibility facade

app/httpapi
  -> compatibility/listingkit/sourcehandoff/httpapi
```

`integration/crawler/a1688` may depend inward on the narrow
`product/sourcing` contract that it adapts. Product sourcing must not depend
back on integration or the legacy crawler model. The target architecture and
boundary documentation will be updated to state this adapter-to-domain
exception explicitly; it is not permission for arbitrary integration packages
to import business services.

## Component Changes

### 1. Product-owned 1688 snapshot

Add product-sourcing snapshot types containing only fields already consumed by
the current normalization functions: product identity, title, category, brand,
currency, unit, prices, MOQ, sales/review facts, images, details, variants,
variation dimensions, specifications, package, supplier, shipping, and video
facts.

The snapshot is a value/data contract. It has no crawler execution methods and
no ListingKit fields.

Change these product functions to accept the snapshot rather than the legacy
crawler model:

- source-result normalization;
- `Alibaba1688SourceEnvelope`;
- `Convert1688ProductToScrapedData` and its helpers.

Pure URL and source-identity functions remain in product sourcing.

### 2. Legacy crawler adapter

Extend `internal/integration/crawler/a1688` with a pure conversion function from
`*internal/crawler/alibaba1688/model.Product1688` to the new product snapshot.
It must:

- return `nil` for a nil product;
- deep-copy maps and slices so downstream normalization cannot mutate crawler
  results;
- preserve every field currently consumed by envelope and scraped-data
  normalization;
- remain independent of ListingKit.

The existing processor wrapper remains unchanged. Existing enrichment and
handoff callers will invoke this conversion before product normalization.

### 3. ListingKit compatibility handoff

Move the current source handoff packages to:

```text
internal/compatibility/listingkit/sourcehandoff/
internal/compatibility/listingkit/sourcehandoff/a1688/
internal/compatibility/listingkit/sourcehandoff/a1688/httpapi/
```

The moved packages retain their current responsibilities:

- build a ListingKit `GenerateRequest` from a `SourceEnvelope`;
- validate required 1688 envelope facts;
- enforce verified tenant/user identity;
- validate both source-account and SHEIN target-store access;
- create a ListingKit task through the existing lifecycle service;
- expose the existing HTTP route and response contract.

The command/HTTP boundary may continue accepting the legacy crawler JSON shape
for compatibility, but it must convert that value through the integration
adapter before invoking product sourcing. No legacy crawler type may escape
into the product package.

Update app assembly imports to the new compatibility path. Do not leave type
aliases or forwarding packages under `internal/product/sourcehandoff`; the old
path must be retired so new code cannot continue depending on the wrong owner.

### 4. Architecture enforcement

Extend the existing Go architecture tests, which already run under the backend
CI job, with a production-code rule:

```text
internal/product/** must not import:
  task-processor/internal/listingkit/**
  task-processor/internal/compatibility/**
  task-processor/internal/crawler/**
  task-processor/internal/integration/**
```

Remove the current `internal/crawler/alibaba1688/model` exception from the
product-sourcing package guard.

Do not add more dependency rules to `scripts/analyze-project-deps.ps1`; it
remains advisory. A repository-wide move to the open-source `depguard` linter
is valuable, but it requires pinning golangci-lint in CI and validating the
existing configuration, so it is a separate follow-up rather than hidden scope
inside this refactor.

## Data Flow

1. The existing HTTP handler receives the authenticated request and legacy
   1688 product JSON.
2. The compatibility command validates request identity and store access using
   the unchanged ListingKit services.
3. The integration adapter deep-copies the legacy crawler product into the
   product-owned snapshot.
4. Product sourcing converts the snapshot into `SourceEnvelope`, product facts,
   asset facts, warnings, and enrichment scraped data.
5. The compatibility handoff converts the envelope into the existing
   ListingKit `GenerateRequest`.
6. The existing task lifecycle service creates the task.
7. The handler returns the same response fields and status codes as before.

## Error Handling and Compatibility

- Missing product, source error, missing source ID, missing title/assets, and
  invalid URL behavior remain unchanged.
- Store-access errors retain their existing codes and HTTP mapping.
- `source_store_id` remains rejected; `source_account_id` remains canonical.
- Tenant and user identity must still come from the verified request context,
  never from caller-controlled headers or body fields.
- The snapshot conversion does not swallow crawler errors; errors continue as
  source warnings and task-creation validation failures where applicable.
- No compatibility package aliases are left at the old product path. Compile
  failures are used to force all internal callers to the correct owner.

## Testing Strategy

Implementation follows TDD.

1. Add failing architecture tests proving product code cannot import ListingKit,
   legacy crawler, compatibility, or integration packages.
2. Add failing adapter contract tests for nil handling, deep copies, and every
   consumed 1688 field family.
3. Move handoff tests with the packages and update imports without weakening
   assertions.
4. Preserve and run the HTTP handler tests covering identity, store access,
   `source_account_id`, error mapping, and response shape.
5. Run the controlled source-to-task flow tests to prove source lineage and task
   request contents are unchanged.
6. Run the focused packages, architecture tests, root Go suite, and build
   commands before completion.

Minimum verification commands:

```powershell
$env:GOWORK='off'
go test ./internal/product/sourcing/... ./internal/integration/crawler/a1688/... -count=1
go test ./internal/compatibility/listingkit/sourcehandoff/... ./internal/app/httpapi/... -count=1
go test ./tests/... -count=1
go test ./... -count=1
go build ./cmd/product-listing-api ./cmd/listing-control-plane ./cmd/shein-listing ./cmd/temu-listing
git diff --check
```

## Acceptance Criteria

- No production Go file under `internal/product` imports ListingKit,
  compatibility, crawler, or integration packages.
- `internal/product/sourcehandoff` no longer exists.
- The legacy crawler model is referenced only by crawler/integration or outer
  compatibility adapters, never product sourcing.
- The existing 1688 create-task route, request/response JSON, identity checks,
  store checks, warnings, and ListingKit request contents remain unchanged.
- Focused tests, architecture tests, the root Go suite, command builds, and
  `git diff --check` pass.
- No unrelated files, including the main worktree's `.dockerignore`, are
  modified or staged.

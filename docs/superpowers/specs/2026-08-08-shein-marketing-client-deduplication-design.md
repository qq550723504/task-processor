# SHEIN Marketing Client Deduplication Design

## Goal

Remove the unused duplicate SHEIN marketing client from `internal/shein/productsync` and make `internal/shein/api/marketing.Client` the only concrete marketing API implementation.

## Current State and Evidence

Two packages implement the same seven marketing operations over the same `client.BaseAPIClient`:

- `internal/shein/productsync/marketing_repo.go` defines `productsync.MarketingAPI` and `NewMarketingAPI`.
- `internal/shein/api/marketing/client.go` defines the canonical `marketing.Client` and `NewClient`.

Repository-wide searches show that production code constructs only `marketing.NewClient`. The legacy `NewMarketingAPI` constructor is referenced only by its own test. A commented workflow example also still names the obsolete `repo.NewMarketingAPI` constructor.

The duplicate appeared across historical package splits and remained after marketing API ownership moved to `internal/shein/api/marketing`. It is now dead code rather than a compatibility boundary.

## Options Considered

### 1. Delete the dead implementation — selected

Delete the legacy implementation and its redundant test, then correct the stale workflow example. This fully removes the duplicate behavior and leaves one clear protocol-client owner.

### 2. Keep a deprecated wrapper

Replace the implementation with a type alias or constructor that delegates to `marketing.NewClient`. This would avoid duplicated behavior but retain an unused API surface and continue to imply that product synchronization owns marketing transport.

### 3. Move the canonical client into `productsync`

This would invert the established dependency direction and force unrelated activity, scheduler, ListingKit, pricing, and SHEIN-context consumers through a product-synchronization package. It is rejected because marketing transport is not a product-sync responsibility.

## Architecture

`internal/shein/api/marketing` remains the sole owner of:

- marketing request and response protocol types;
- endpoint selection and request payload construction;
- business-response validation and `api.APIError` creation;
- the concrete client implementing `marketing.MarketingAPI`.

`internal/shein/productsync` remains responsible only for product snapshot persistence, fetching, enrichment, and synchronization. It will no longer expose a marketing transport client.

No new abstraction, registry, adapter, or dependency is introduced.

## Changes

1. Delete `internal/shein/productsync/marketing_repo.go`.
2. Delete `internal/shein/productsync/marketing_repo_test.go` because its `SaveConfig` assertion is already covered by `internal/shein/api/marketing/client_test.go`.
3. Update the commented workflow example in `internal/shein/api/marketing/workflow_example.go` to construct the canonical client with `NewClient(baseClient)`.
4. Add no compatibility alias for `productsync.NewMarketingAPI`; repository evidence shows it has no production consumer.

## Behavior and Error Handling

This change must not alter any production request path. Existing consumers already call the canonical client, so endpoint selection, payload serialization, response decoding, error wrapping, and API error messages remain those of `internal/shein/api/marketing.Client`.

The stale example change is documentation-only and must not introduce executable workflow code.

## Testing and Verification

Verification will prove both behavior preservation and ownership:

1. Search the repository for `NewMarketingAPI`, `productsync.MarketingAPI`, and `marketing_repo`; no production definition or call may remain.
2. Run canonical client tests, including the existing `SaveConfig` promotion-type coverage.
3. Run the `productsync` package tests to confirm removal does not affect product synchronization.
4. Run related marketing consumers' tests, at minimum `internal/shein/activity` and `internal/listingkit/sheinsync`.
5. Run targeted `go vet`, `git diff --check`, and the full `go test ./... -count=1` suite.
6. Re-run the duplicate-code scan on the affected packages and confirm the removed clone is gone.

## Compatibility and Risk

The removed symbol is under Go's `internal` tree and cannot be imported by external modules. A repository-wide search covers in-repository callers, including tests and documentation. The only identified references are the redundant test and stale example, both handled in this change.

The primary risk is an unsearched build-tagged or generated in-repository consumer. Verification therefore searches all tracked text, not only default-build `.go` files, before deletion and again before completion.

## Non-Goals

- Refactoring the canonical marketing client's request-building code.
- Changing marketing protocol types or endpoint constants.
- Changing activity enrollment, scheduler, pricing, or ListingKit behavior.
- Addressing other duplicate-code findings in the same branch.

## Acceptance Criteria

- `internal/shein/api/marketing.Client` is the only concrete SHEIN marketing API client.
- No tracked production code references `productsync.NewMarketingAPI` or `productsync.MarketingAPI`.
- The stale workflow example uses the canonical constructor.
- Canonical, product-sync, related-consumer, static, and full-suite checks pass.
- The branch contains only this design, the focused deletion, the example correction, and verification-oriented tests or documentation required by this scope.

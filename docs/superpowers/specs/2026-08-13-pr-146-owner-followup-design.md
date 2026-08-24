# PR #146 Owner Follow-up Design

## Context

PR #146 made `owner_user_id` mandatory for product-import mapping writes and propagated store ownership through the primary ListingKit paths. Three unresolved P2 review threads identify follow-up gaps: SHEIN and TEMU can dereference a missing store while building mapping requests, and several SHEIN mapping-builder entry points can still construct ownerless requests.

Repository tracing also found an active gap not named explicitly in the comments: `SmartRepairStrategy.buildEnhancedMappingOptions` constructs `MappingCreateOptions` without copying the store owner. The five convenience methods named by the third review comment have no repository callers and cannot be used outside this module because the package is under `internal/`.

## Goals

- Prevent missing SHEIN or TEMU store ownership from panicking a worker.
- Reject ownerless mapping work before iterating SKU mappings or calling a persistence gateway.
- Preserve the repository-level invariant that every product-import mapping has a canonical owner.
- Ensure every live SHEIN repair path propagates the store owner.
- Remove unused mapping-builder entry points that cannot satisfy the owner contract.

## Non-goals

- Reopen or rewrite PR #146.
- Backfill existing database rows.
- Change production deployment state.
- Reply to or resolve GitHub review threads without separate authorization.
- Introduce a new dependency; the change is repository-local validation and data propagation.

## Design

### Input boundaries

`buildMappingRequestInput` will require a non-nil runtime store and a non-blank `OwnerUserID` before returning a `MappingRequestInput`. This keeps `buildMappingReq` simple and ensures both SHEIN result paths fail through their existing error channel rather than panic during request construction.

`buildSavePublishResultInput` will apply the same rule to the TEMU store DTO. `HandleTemu` will retain its current optional post-processing behavior, but its warning will include the actual validation error so an owner propagation regression is observable instead of being mislabeled as an empty submit response.

Whitespace-only owner values are invalid at both boundaries because the repository trims and rejects them.

### Mapping builder invariant

`MappingBuilder.validateOptions` will reject a blank `OwnerUserID` before calling the mapping gateway. `CreateMappingFromContext` already copies the owner from `StoreInfo`; `SmartRepairStrategy.buildEnhancedMappingOptions` will be brought into the same contract.

The unused `CreateBasicMapping`, `CreateMappingWithSPU`, `CreateMappingWithPrice`, `CreateMappingWithRules`, and `CreateMappingFromTaskContext` methods will be removed. Adding owner parameters would preserve dead API surface without a real caller or ownership source. The live `CreateMappingRelation`, `CreateMappingFromContext`, and batch path remain, with validation enforcing their contract.

### Error behavior

Missing store and missing owner are handled as validation errors, not panics and not synthetic fallback owners. No mapping write is attempted. This preserves the fail-closed ownership policy introduced by PR #146.

## Testing

- Add SHEIN input-builder tests for a missing store and a blank owner; each must return an error without panic.
- Add TEMU input-builder tests for a missing store and a blank owner; each must return an error without panic.
- Add MappingBuilder tests proving ownerless options are rejected before the gateway and valid owners reach the gateway.
- Add a SmartRepair strategy test proving the store owner is included in the created mapping request.
- Run targeted tests for `internal/shein/publish`, `internal/temu/product`, and `internal/shein/mapping`, then run `go test ./... -count=1` and `git diff --check`.

## Acceptance criteria

- All three P2 findings have a test that fails on the PR #146 merge state and passes after the fix.
- No live product-import mapping creation path found by repository search constructs an ownerless request.
- Missing owner data cannot cause a nil dereference.
- No persistence gateway is called for an ownerless mapping request.
- Full repository tests pass from the isolated follow-up branch.

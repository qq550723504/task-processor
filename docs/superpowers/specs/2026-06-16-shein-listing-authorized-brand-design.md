# SHEIN Listing Authorized Brand Design

## Background

Some SHEIN stores have brand authorization and must publish with an approved SHEIN brand. For those stores, `shein-listing` must:

- use a store-level switch to enable authorized-brand behavior
- query the SHEIN brand list through the platform API
- bind the listing payload to a configured authorized brand
- preserve the authorized brand in title, description, and SKC title cleanup
- continue removing other brand words as before

The current implementation removes Amazon and context brand words during sensitive-word cleanup, and it does not yet connect the newly added `query_brand_list` API to store-level behavior.

## Goal

Add a store-scoped authorized-brand mode for `shein-listing` so that an authorized store can publish with a specific SHEIN brand and keep that approved brand in user-facing copy.

## Non-Goals

- No UI work in ListingKit or management admin
- No automatic fuzzy brand selection without store configuration
- No change to non-authorized stores
- No rewrite of title generation or general sensitive-word policy

## Store Configuration

Add three store-level fields to the management store model:

- `EnableBrandAuthorization *bool`
- `AuthorizedBrandCode string`
- `AuthorizedBrandName string`

Rationale:

- `AuthorizedBrandCode` is the publish-time source of truth
- `AuthorizedBrandName` is used for copy-preservation and diagnostics
- `EnableBrandAuthorization` allows the feature to be opt-in and isolated per store

If the management side has only the code initially, the implementation may still work with code-only mode, but the target design keeps both code and name because name preservation is part of the requirement.

## Runtime Flow

### 1. Load store configuration

When `StoreInfoHandler` loads `StoreRespDTO`, the task context should receive the authorized-brand settings together with the rest of the store information.

### 2. Resolve authorized brand

When authorized-brand mode is enabled:

- call `ProductAPI.QueryBrandList()`
- resolve the configured brand against the returned list
- prefer exact `AuthorizedBrandCode` match
- if needed, allow exact trimmed match on `brand_name` or `brand_name_en` against `AuthorizedBrandName`

If the configured brand cannot be found in the SHEIN list, fail the task early with a clear non-retryable error because the store configuration is invalid for publishing.

### 3. Apply brand to publish payload

When authorized-brand mode is enabled:

- set the final SHEIN `Product.BrandCode` from the resolved authorized brand
- keep the resolved brand name available for text-sanitization allowlisting

When the mode is disabled, retain current behavior.

### 4. Preserve only the authorized brand during cleanup

Current cleanup removes:

- platform sensitive words
- hardcoded Amazon brand words
- the contextual brand from `ctx.AmazonProduct.Brand`

In authorized-brand mode, cleanup must change from "remove all known brands" to "remove all brands except the resolved authorized brand". Specifically:

- title should preserve the authorized brand
- description should preserve the authorized brand
- SKC titles should preserve the authorized brand
- other brand words should still be removed

This should be implemented as a small allowlist-aware extension to the existing cleanup path rather than a separate content pipeline.

## Component Changes

### Management DTO and local provider

Files expected to change:

- `internal/infra/clients/management/api/store.go`
- `internal/infra/clients/management/local_data_provider.go`

Add the new store fields to both the API DTO and local DB mapping so local debug mode behaves the same as remote management mode.

### Task context and store loading

Files expected to change:

- `internal/shein/context/context.go`
- `internal/shein/store/store_info.go`

Either read the configuration directly from `ctx.StoreInfo` where needed or add a small derived helper on the task context for authorized-brand settings.

### SHEIN product API usage

Files expected to change:

- brand list query call site in publishing / submit preparation logic

The new `QueryBrandList()` method should be reused rather than duplicating raw HTTP calls.

### Payload assembly

Files expected to change:

- `internal/publishing/shein/assembler.go`
- potentially product submit preparation files if final submit payload is normalized later in the chain

The resolved authorized brand must be applied at the final payload-writing point, not only at preview/package metadata level.

### Sensitive-word cleanup

Files expected to change:

- `internal/shein/content/processor.go`
- possibly helper methods in `internal/shein/content/text_cleaner.go` or adjacent utilities

Add a targeted allowlist mechanism:

- preserve one approved brand token or phrase
- continue existing cleanup for everything else

## Error Handling

If authorized-brand mode is enabled:

- missing configured brand code and missing brand name: fail non-retryable
- brand list API failure: treat as retryable infrastructure/API failure
- configured brand not found in current SHEIN brand list: fail non-retryable
- resolved brand exists but payload cannot be updated: fail non-retryable

If the mode is disabled, the existing chain remains unchanged.

## Testing

Add tests for:

1. store DTO mapping
2. authorized brand resolution by code
3. optional resolution by exact name
4. publish payload uses authorized `brand_code`
5. title/description/SKC title preserve the authorized brand
6. non-authorized brands are still removed
7. disabled mode keeps current cleanup behavior
8. missing or invalid authorized brand configuration fails clearly

## Risks

### Brand phrase normalization

Brand names may differ in case or bilingual representation, for example `Logitech` vs `Logitech罗技`. Matching should stay conservative:

- exact code match first
- exact trimmed name match second
- avoid aggressive fuzzy matching in the first implementation

### Cleanup over-preservation

If preservation logic is too broad, other brand words might slip through. The allowlist must preserve only the resolved authorized brand, not a whole class of brand-like terms.

### Multiple payload mutation points

If `BrandCode` can be overwritten later in the chain, setting it too early may not be enough. The implementation should verify the final submit payload mutation point and place the override there.

## Recommended Implementation Order

1. extend store DTO and local provider fields
2. add authorized-brand resolution helper around `QueryBrandList()`
3. inject resolved brand into final SHEIN payload
4. add cleanup allowlist for the authorized brand
5. add tests for payload and cleanup behavior

## Acceptance Criteria

- authorized store can publish with configured SHEIN `brand_code`
- authorized brand remains in title, description, and SKC title
- other brands are still removed
- non-authorized stores behave exactly as before
- invalid authorized-brand store config fails early with clear diagnostics

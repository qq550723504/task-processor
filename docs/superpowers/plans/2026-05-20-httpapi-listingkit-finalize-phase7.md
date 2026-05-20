## Objective

Finish the `ListingKit` httpapi decoupling by moving the last `ListingKit`-specific helpers out of `internal/app/httpapi`.

## Scope

- Move ZITADEL auth runtime and route authorization helpers into `internal/listingkit/httpapi`.
- Move ListingKit AI client routing/config helpers into `internal/listingkit/httpapi`.
- Keep `internal/app/httpapi` as the generic HTTP server/bootstrap layer.

## Constraints

- Preserve current route protection behavior and AI client resolution semantics.
- Do not change external route registration or config contracts.

## Verification

- `go test ./internal/app/httpapi ./internal/listingkit/httpapi ./internal/listingkit/... -count=1`
- `go test ./cmd/product-listing-api ./cmd/productenrich-api ./cmd/listingkit-temporal-worker -count=1`

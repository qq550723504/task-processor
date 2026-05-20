## Objective

Finish the `httpapi` module-builder split by extracting `ListingKit` assembly out of `internal/app/httpapi/modules.go`, while preserving current route and runtime behavior.

## Scope

- Add a dedicated `internal/listingkit/httpapi` bootstrap entry for service/module assembly.
- Keep app-level DB factories and transitional helpers reusable through explicit builder callbacks instead of hidden package coupling.
- Update standalone Temporal worker bootstrap to use the same `ListingKit` service builder path.
- Shrink `internal/app/httpapi/modules.go` so it coordinates modules instead of owning `ListingKit` runtime assembly.

## Constraints

- No behavioral changes to ListingKit handlers, worker startup, or route registration.
- Do not touch unrelated frontend changes or `.local/chrome/`.
- Preserve existing test coverage and add no new runtime dependencies.

## Verification

- `go test ./internal/app/httpapi ./internal/listingkit/... -count=1`
- If shared helper wiring changes force it, expand to adjacent packages only.

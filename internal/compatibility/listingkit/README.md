# ListingKit Compatibility

Owns legacy ListingKit-compatible entrypoints, DTO bridges, and thin delegation layers while the real business logic moves into `internal/listing`, `internal/marketplace`, and `internal/product`.

The 1688 compatibility path lives at
`internal/compatibility/listingkit/sourcehandoff`. It owns the existing
command and HTTP handoff to ListingKit, including tenant/user identity checks,
source and target-store validation, and request/response compatibility. It
converts the legacy 1688 product shape through
`internal/integration/crawler/a1688` before invoking product sourcing.

Allowed here:

- backward-compatible service entrypoints
- DTO or response-shape translation
- temporary adapters that delegate inward

Avoid adding here:

- new long-lived business rules
- new marketplace-specific behavior
- new crawler or sourcing ownership

When in doubt, put new logic in the real owner package first and keep this layer thin.

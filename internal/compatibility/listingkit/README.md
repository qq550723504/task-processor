# ListingKit Compatibility

Owns legacy ListingKit-compatible entrypoints, DTO bridges, and thin delegation layers while the real business logic moves into `internal/listing`, `internal/marketplace`, and `internal/product`.

Allowed here:

- backward-compatible service entrypoints
- DTO or response-shape translation
- temporary adapters that delegate inward

Avoid adding here:

- new long-lived business rules
- new marketplace-specific behavior
- new crawler or sourcing ownership

When in doubt, put new logic in the real owner package first and keep this layer thin.

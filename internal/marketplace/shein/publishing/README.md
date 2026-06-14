# SHEIN Publishing

Owns SHEIN publishing rules, payload shaping, category and attribute publishing behavior, and submit-time validations.

Boundary rule:

- this package must not depend on `internal/listingkit` or root runtime wiring packages.
- legacy `internal/publishing/shein` may remain as a compatibility/model package until submission flows are migrated deliberately.

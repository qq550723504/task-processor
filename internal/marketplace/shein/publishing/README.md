# SHEIN Publishing

Owns SHEIN publishing rules, payload shaping, category and attribute publishing behavior, and submit-time validations.

Current stable ownership includes:

- pricing policy and rounding rules,
- remote record classification rules for publish/save-draft confirmation state,
- remote confirmation fallback/default-confirmed policy for publish refresh flows,
- remote confirmation decision rules for on-way documents, remote record outcomes, inventory confirmation, and fallback status/detail selection,
- remote record selection rules such as preferred SPU match and latest-create-time fallback,
- remote response parsing rules for on-way documents, record-query success handling, and inventory confirmation.

Boundary rule:

- this package must not depend on `internal/listingkit` or root runtime wiring packages.
- legacy `internal/publishing/shein` may remain as a compatibility/model package until submission flows are migrated deliberately.

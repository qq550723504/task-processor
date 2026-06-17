# SHEIN Publishing

Owns SHEIN publishing rules, payload shaping, category and attribute publishing behavior, and submit-time validations.

Current stable ownership includes:

- pricing policy and rounding rules,
- remote record classification rules for publish/save-draft confirmation state,
- remote confirmation fallback/default-confirmed policy for publish refresh flows,
- remote confirmation decision rules for on-way documents, remote record outcomes, inventory confirmation, and fallback status/detail selection,
- remote confirmation update-message selection for record-query errors and not-yet-visible records,
- remote refresh and missing-supplier-code fallback wording/status rules for confirmation flows,
- submission projection workflow-status mapping for publish/save-draft readiness and terminal states,
- remote record selection rules such as preferred SPU match and latest-create-time fallback,
- remote lookup identity rules such as accepted-with-SPU detection, preferred SPU fallback, remote-resolution SPU precedence, and normalized supplier-code collection,
- action-aware remote response acceptance rules for publish/save-draft flows,
- confirmed remote-check response wording for publish and save-draft flows,
- submit phase default detail wording for publish/save-draft event assembly,
- remote response parsing rules for on-way documents, record-query success handling, and inventory confirmation.

Boundary rule:

- this package must not depend on `internal/listingkit` or root runtime wiring packages.
- legacy `internal/publishing/shein` may remain as a compatibility/model package until submission flows are migrated deliberately.

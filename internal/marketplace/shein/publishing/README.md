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
- sensitive-word retry eligibility for publish failures with validation notes,
- preferred warehouse-code selection for submit payload defaults,
- submit weight unit conversion, rounding, and bound-clamping,
- submit supplier-code derivation from product and SKU identifiers,
- submit image URL classification plus upload-cache normalization for uploaded SHEIN hosts and SDS source hosts,
- submit payload gallery normalization, site-detail image selection, image URL de-duplication, and SKU image detail normalization rules,
- submit payload validation rules for required SKC images and normalized SKU fields,
- submit SKU style-token classifiers, suffix derivation, and task/request discriminator shaping,
- remote response parsing rules for on-way documents, record-query success handling, and inventory confirmation.
- recovered-submit local-recovery acceptance rules for publish/save-draft responses.
- recovery remote-lookup confirmation policy selection for publish/save-draft responses.
- confirm-remote decision, SPU precedence, and update-message policy for root remote-status orchestration.

## Cost-conversion calculation

`CalculateCostPrice` / `CostPriceRule` own the numeric calculation used by the
ordinary price review and the missing-price branch of draft-backed review.
The application composition maps the numeric input contract into this rule;
the policy does not import the consumers' DTOs. The former root ListingKit
`calculateSheinPrice` function is removed, not retained as a forwarding wrapper.

Operation order stays cost conversion/multiplier, minimum, price ending,
increment ceiling, then two-decimal rounding. Configuration defaults, draft-price
precedence, manual overrides and review/persistence remain with their existing
callers. The existing ListingKit builders receive the calculation through their
service dependency; they do not import this package. This is distinct from
`PricingPolicy.Apply` (shipping, fixed markup and commission); the formulas must
not be substituted for each other.
No external calls, new persistence, readiness decision or publish permission
are added. This extraction does not retire the remaining ListingKit workflow.

Boundary rule:

- this package must not depend on `internal/listingkit` or root runtime wiring packages.
- remaining legacy `internal/publishing/shein` models/callers are a drain area under `EXTRACT | RETIRE`, not a new compatibility destination. Current policy consumers must not depend on that legacy owner.

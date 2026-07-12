# SHEIN Submit Readiness Action Policy Plan

1. Add a marketplace publishing policy that classifies the actions allowed to bypass readiness blockers, with table-driven coverage for draft, publish, and unknown actions.
2. Replace ListingKit's local `save_draft` string comparison with the marketplace policy while retaining the generic readiness-gate delegation and callback adaptation.
3. Extend the ListingKit boundary guard to assert the delegation and reject a local action-string comparison.
4. Run focused Go tests and vet for ListingKit and SHEIN marketplace publishing.

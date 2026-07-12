# SHEIN POD Submit Readiness Policy Plan

1. Add an action-aware POD submit-readiness decision to SHEIN publishing. Reuse `sdspod` normalization, blocking, and message APIs; test disabled, required-blocked, optional-degraded, and save-draft outcomes.
2. Make ListingKit adapt `PodExecutionSummary` into the new decision and construct the existing workspace check without locally interpreting action or POD status.
3. Extend the ListingKit readiness boundary guard to require publishing delegation and reject local POD action/status branches.
4. Run focused publishing, SDS POD, and ListingKit tests plus vet.

# SHEIN Submit Readiness Action Policy

## Goal

Move the stable SHEIN action rule that permits `save_draft` to bypass submit-readiness blockers out of root ListingKit and into `internal/marketplace/shein/publishing`.

## Boundary

`internal/marketplace/shein/publishing` owns the action classification:

- `save_draft` permits readiness blockers;
- `publish`, empty, and unknown actions do not.

`internal/listingkit` retains readiness snapshot adaptation, task/package inputs, freshness callback wiring, and error translation.  It delegates only the action policy.

## Compatibility

This is a behavior-preserving extraction.  The existing early return for `save_draft` remains before base and freshness validation, so save-draft requests continue to skip the freshness callback.

## Verification

- publishing package tests cover the new action-policy contract;
- ListingKit gate tests preserve draft bypass behavior;
- a ListingKit boundary guard verifies the gate delegates classification to the marketplace publishing package and no longer performs its own string comparison.

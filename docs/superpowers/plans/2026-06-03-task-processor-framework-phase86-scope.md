# Phase 86 scope

## Candidate hotspot

`ListingKit asset-generation projection closure audit`

## Why this is the next step

- `buildAssetGenerationOverview(...)` was already a shared owner
- phase 85 closed the larger bundle contract around result / preview / export outward projections
- before opening another refactor line, we should verify whether the remaining generation-queue/review wiring is already at a practical closure point

## Decision rule

- only continue if we find another real multi-consumer outward contract nearby
- if remaining logic is review-local or queue-local composition, stop slicing this line

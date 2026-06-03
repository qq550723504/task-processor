# Phase 87 architecture hotspot discovery

## Goal

Find the next real framework-level hotspot in ListingKit after closing:

- shared readiness contracts
- readiness-derived outward projections
- submission status projections
- asset-generation outward projections

## Candidate categories

- task result and task list shared adapters outside submission state
- revision / preview / review flows rebuilding the same policy or projection in parallel
- generation action routing or review routing policies with multiple production consumers

## Stop condition

If no hotspot clearly satisfies multi-consumer or cross-flow criteria, stop and report that the current framework line is at a practical closure point.

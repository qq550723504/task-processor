# Phase 85 architecture hotspot discovery

## Goal

Find the next real framework-level hotspot in ListingKit after closing:

- shared readiness contracts
- readiness-derived outward projections
- submission status projections

## Candidate categories

- shared outward adapters used by task result and task list together
- revision / preview / review flows rebuilding the same contract in parallel
- task-state and action-routing policies with multiple production consumers

## Stop condition

If no hotspot clearly satisfies multi-consumer or cross-flow criteria, stop and report that the current framework line is at a practical closure point.

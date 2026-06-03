# Phase 84 scope

## Candidate hotspot

`ListingKit submission projection closure audit`

## Why this is the next step

- we now have two adjacent architecture-level seams:
  - readiness-derived outward projection
  - submission status / remote summary projection
- before opening another refactor line, we should verify whether submission projection is already at a practical closure point

## Decision rule

- if the remaining logic in `submission_state_support.go` is mostly package-local derivation and fallback policy, stop slicing
- only continue if we find another real multi-consumer outward contract nearby

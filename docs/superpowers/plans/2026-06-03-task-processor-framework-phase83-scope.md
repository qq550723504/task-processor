# Phase 83 scope

## Candidate hotspot

`ListingKit readiness-derived workspace projection contract`

## Why this is the next candidate

- `buildSheinPreviewPayload(...)` still owns the final assembly from:
  - readiness projection
  - repair center
  - repair state
  - workspace overview
- that assembly is currently only consumed by preview, so it is not automatically a framework seam
- however, it is adjacent to the new shared projection layer and is the next place where cross-flow reuse may emerge

## Decision rule

- only continue if we find a real multi-consumer or cross-flow contract
- if the remaining assembly is preview-local, stop slicing and keep it local

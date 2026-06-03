# Phase 83 readiness-derived workspace projection discovery

## Question

Now that `readiness -> checklist / submit-state / status-overview` has a shared owner, is there another true shared seam around:

- repair center
- repair state
- workspace overview
- final review

## What to look for

- multiple production consumers rebuilding the same projection
- duplicated contract wiring between preview and another flow
- outward policy drift risk that cannot be contained inside a single local owner

## Stop condition

If the remaining logic is preview-local composition, do not keep abstracting it into more helpers.

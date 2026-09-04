# Summary

- What changed?
- Why is this needed?

# Product Boundary

Fill this section for architecture-sensitive, security-sensitive, or product-contract changes. For routine bounded changes, write `N/A` where appropriate instead of inventing policy.

## Product Decisions

- Must:
- Should:
- Out of Scope:
- Accepted Risks:

## Threat Model

- Risks this change must protect against:
- Risks explicitly not being solved in this phase:

## Review State

- Architecture review round: `0 / 1 / 2 / frozen`
- Implementation status: `DESIGNING / IMPLEMENTATION_READY / IMPLEMENTING`

- [ ] Reviewer findings were classified before changing architecture.
- [ ] `BLOCKER` findings identify the concrete blocker category from `AGENTS.md`.
- [ ] `IMPLEMENTATION_TEST`, `BACKLOG`, `ACCEPTED_RISK`, and `NOT_APPLICABLE` findings do not block implementation.
- [ ] If the design is `IMPLEMENTATION_READY`, non-blocking findings did not create another versioned design document.

# Validation

- [ ] `go test ...`
- [ ] Other validation steps documented below when needed

Validation notes:

- 

# Architecture Checklist

Use this checklist for changes touching `internal/app`, `internal/listingkit`,
`internal/publishing`, `internal/productenrich`, `internal/productimage`,
`internal/amazonlisting`, or runtime assembly paths.

- [ ] The change keeps assembly logic and domain logic separate.
- [ ] `internal/app/*` remains orchestration / assembly focused and does not absorb new business rules.
- [ ] No new code imports deprecated compatibility paths:
  - `task-processor/internal/app/processor`
  - `task-processor/internal/app/state`
- [ ] No domain `httpapi` package imports `task-processor/internal/app/httpapi`.
- [ ] No new centralized `build*Module` wiring was added to `internal/app/httpapi/modules.go`.
- [ ] `internal/app/httpapi/listingkit_support.go` only changed for assembly input adaptation, repo wiring, or explicit runtime bridging.
- [ ] For ListingKit / SHEIN semantic fields, new code reads and writes canonical names first:
  - Backend: `SDSDesignResult`, `DraftPayload`, `PreviewPayload`, `SubmissionState`, `FinalSubmissionDraft`
  - Frontend/API: `sds_design_result`, `draft_payload`, `preview_payload`, `submission_state`, `final_submission_draft`
  - Any legacy field usage is limited to explicit compatibility helpers, protocol types, or documented fallback boundaries.
- [ ] If package boundaries changed, the corresponding architecture docs were updated.

# Notes For Reviewers

- Risk areas:
- Follow-up work:
- Any intentional boundary exceptions:
- Finding classification notes (`BLOCKER / IMPLEMENTATION_TEST / BACKLOG / ACCEPTED_RISK / NOT_APPLICABLE`):

Relevant docs when needed:

- `AGENTS.md`
- `docs/architecture/architecture-review-checklist.md`
- `docs/architecture/httpapi-assembly-boundaries.md`
- `docs/architecture/app-assembly-boundaries.md`
- `docs/architecture/next-steps.md`

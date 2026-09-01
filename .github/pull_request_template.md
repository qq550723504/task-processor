# Summary

- What changed?
- Why is this needed?

# Development Admission

- Scope class: bounded / architecture-sensitive
- Affected subsystems:
- Consistency, authorization, or tenant boundaries:
- Scope metrics: scope-relevant files / production additions / production churn
- Design:
- Independent design review:
- Invariants and failure matrix:
- `architecture-approved` label, maintainer/admin approval review for the current head SHA, and split rationale (only when oversized):

Admission checklist:

- [ ] The change is at or below 30 scope-relevant files, 1,500 production additions, and 2,500 production churn, or has the documented `architecture-approved` exception with a maintainer/admin approval review for the current head SHA.
- [ ] Architecture-sensitive work has an approved design and a fresh independent design review.
- [ ] Stateful or multi-boundary work documents durable states, retry identity, recovery ownership, and a failure matrix.
- [ ] A shared transaction and existing repository or mature open-source facilities were evaluated before adding compensation infrastructure.
- [ ] Cross-cutting behavior was checked across sibling routes, consumers, and recovery entry points.

# Validation

- [ ] `go test ...`
- [ ] Lost-response and partial-persistence behavior tested, or `N/A` explained below.
- [ ] Retry, restart, and concurrency behavior tested, or `N/A` explained below.
- [ ] Tenant, authorization, Cookie, cache, and multi-tab context drift tested, or `N/A` explained below.
- [ ] Request size, deadline, and resource bounds tested, or `N/A` explained below.
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
- Residual consistency or recovery risks:
- Follow-up work:
- Any intentional boundary exceptions:

Relevant docs when needed:

- `docs/architecture/httpapi-assembly-boundaries.md`
- `docs/architecture/app-assembly-boundaries.md`
- `docs/architecture/next-steps.md`

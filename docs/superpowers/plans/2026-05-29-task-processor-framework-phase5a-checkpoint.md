# Task Processor Framework Phase 5A Checkpoint

## Status

`Phase 5A` is functionally complete for the intended slice.

This phase was not about redesigning ListingKit workflow internals or building a repo-wide process engine. The goal was narrower:

1. stop ListingKit task claim, workflow execution, terminal-state persistence, and worker retry gating from remaining split across `service_process.go` and `processor.go`
2. make service-side process phases explicit
3. align worker-side skip/retry decisions on a small ListingKit-owned state helper
4. lock the new ownership split so process orchestration and retry gating do not silently drift back into public entry files

That goal is now met on the active `codex/framework-phase1` branch.

## What Landed

### 1. Deterministic process outcome rules are now separated from the process entry point

The deterministic terminalization rules now live in:

- [internal/listingkit/service_process_outcome.go](/D:/code/task-processor/internal/listingkit/service_process_outcome.go:1)

This file now owns the process-outcome helpers for:

- `deriveProcessTerminalStatus(...)`
- `applyProcessTerminalResult(...)`
- `persistProcessFailure(...)`
- `persistProcessSuccess(...)`

The public service entry point still lives in:

- [internal/listingkit/service_process.go](/D:/code/task-processor/internal/listingkit/service_process.go:1)

but it no longer needs to keep terminal-state shaping and persistence rules inline.

That matters because the process-layer bug risk here was not “too many lines in one file.” The real risk was that workflow success/failure handling and terminal-state derivation were still hidden inside the same method that also acted as the public service entry.

### 2. Service-side process orchestration now flows through an explicit ListingKit-owned seam

The new service-side flow seam lives in:

- [internal/listingkit/service_process_flow.go](/D:/code/task-processor/internal/listingkit/service_process_flow.go:1)

This file now owns coordinated service-side phases across:

- task claim
- workflow execution
- failure persistence
- success terminalization

The seam is constructed through:

- `buildListingKitProcessFlow(s)`

and is now the main home of service-side process orchestration.

This is the main architectural result of `Phase 5A`: ListingKit process flow is still feature-owned, but it is no longer primarily expressed as implicit orchestration embedded in the public `ProcessListingKit(...)` method.

### 3. Worker retry and skip gating now flows through a ListingKit-owned state helper

The new worker-side state helper lives in:

- [internal/listingkit/processor_state_machine.go](/D:/code/task-processor/internal/listingkit/processor_state_machine.go:1)

This file now owns the bounded gating rules for:

- `CanProcess(...)`
- `ShouldRetry(...)`

The worker entry point in:

- [internal/listingkit/processor.go](/D:/code/task-processor/internal/listingkit/processor.go:1)

now consumes this state helper instead of keeping pending/retry decisions inline beside identity injection and service invocation.

This matters because `processor.go` and `service_process.go` previously had room to evolve their process rules independently. After this phase, the worker-side gating seam is visible and locally testable.

### 4. This slice explicitly reused a mature local pattern instead of inventing a new framework

The new ListingKit state helper follows the same bounded idea already proven in:

- [internal/productenrich/pipeline/state_machine.go](/D:/code/task-processor/internal/productenrich/pipeline/state_machine.go:1)
- [internal/productimage/pipeline/state_machine.go](/D:/code/task-processor/internal/productimage/pipeline/state_machine.go:1)

That reuse decision matters. The root problem here was crossed ownership, not the absence of a generic engine. This phase deliberately borrowed the mature local pattern and kept the implementation feature-owned inside ListingKit.

### 5. Guardrails now lock the process phase ownership split

The new boundary protections live in:

- [internal/listingkit/phase5a_process_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase5a_process_boundary_test.go:1)
- [internal/listingkit/phase4a_collaborator_boundary_test.go](/D:/code/task-processor/internal/listingkit/phase4a_collaborator_boundary_test.go:1)
- [internal/listingkit/processor_process_test.go](/D:/code/task-processor/internal/listingkit/processor_process_test.go:1)
- [internal/listingkit/processor_state_machine_test.go](/D:/code/task-processor/internal/listingkit/processor_state_machine_test.go:1)
- [internal/listingkit/service_process_status_test.go](/D:/code/task-processor/internal/listingkit/service_process_status_test.go:1)

These checks now explicitly protect both sides of the seam:

1. `service_process.go` does not regrow inline claim/finalize/persist branches
2. `service_process_flow.go` remains the home of service-side process orchestration
3. `processor.go` does not regrow inline pending/retry decision logic
4. `processor_state_machine.go` remains the home of worker-side skip/retry gating
5. process result persistence and retry scheduling still behave the same as before

## Acceptance Check

`Phase 5A` was meant to prove four things:

1. deterministic process-outcome rules can be separated from the service entry point
2. service-side process orchestration can flow through one explicit ListingKit-owned seam
3. worker-side skip/retry gating can flow through a small local state helper
4. the new ownership split can be protected with narrow source and behavior guardrails

All four are now true.

More concretely:

- terminal-state derivation is isolated
- service-side process phases are centralized in one flow seam
- worker-side retry gating is centralized in one state helper
- ListingKit package tests still pass without behavior regressions

## What This Phase Did Not Try To Solve

### 1. It did not redesign workflow internals

Files such as:

- [internal/listingkit/workflow_standard.go](/D:/code/task-processor/internal/listingkit/workflow_standard.go:1)
- [internal/listingkit/workflow_platform_adaptation.go](/D:/code/task-processor/internal/listingkit/workflow_platform_adaptation.go:1)

still represent the existing ListingKit workflow model.

That is correct for this phase. The goal here was process ownership clarity, not workflow redesign.

### 2. It did not introduce richer failure-disposition modeling

Unlike `productenrich/productimage`, ListingKit’s new worker-side helper intentionally stays small:

- `CanProcess(...)`
- `ShouldRetry(...)`

That is intentional. There is not yet enough evidence that ListingKit needs a richer local failure classification model for this slice.

### 3. It did not change retry semantics

The worker still:

- skips non-pending tasks
- treats `ErrTaskNotPending` as a safe skip
- increments retry count and prepares retry only while still under `maxRetries`

That is correct. This phase made the rules explicit without changing their behavior.

## Residual Responsibilities Still Present

### `service_process_flow.go` is now a real hotspot

The new flow seam is a better ownership boundary, but it is also now the main concentration point for service-side process orchestration.

That is acceptable for this phase because the service process logic needed one visible home first. If future change pressure keeps landing there, the next slice should be driven by the kinds of orchestration changes inside that file, not by symmetry concerns elsewhere.

### `processor.go` still owns identity injection and service invocation sequencing

The worker entry file still coordinates:

- task loading
- tenant/OpenAI identity injection
- service invocation
- retry submission handoff

That is acceptable for now. `Phase 5A` was about making skip/retry decisions explicit, not about moving every line out of the worker entry point.

### Service flow and worker flow are still separate seams

There is now:

- a service-side process flow seam
- a worker-side state helper

That is still the right tradeoff today because they solve different ownership problems. Combining them into one abstraction would be premature unless change pressure starts crossing both seams again.

## What Should Move To The Next Phase

If we continue, the next highest-value work should not be “extract more helpers because process files are smaller now.” Better next steps are:

### 1. Watch whether the service-side flow seam becomes behaviorally overloaded

If future changes keep landing in:

- [internal/listingkit/service_process_flow.go](/D:/code/task-processor/internal/listingkit/service_process_flow.go:1)

then the next slice should be driven by real orchestration hotspots inside that seam, for example whether claim/finalize behavior and workflow invocation need separate collaborators.

### 2. Reassess whether workflow/process execution modeling is now the better hotspot

If the next wave of changes is more about:

- readiness checks
- workflow recovery
- execution branching
- review/failure disposition

then work should move outward to workflow/process modeling rather than continuing to polish the process seams themselves.

### 3. Leave this seam alone unless another concrete ownership problem appears

This layer is now in a good enough state:

- explicit service flow seam exists
- explicit worker state helper exists
- guardrails exist
- behavior remained stable

Do not keep editing it for symmetry alone.

## Verification Summary

The final `Phase 5A` verification that passed on this branch was:

```powershell
go test ./internal/listingkit -run "TestProcessListingKit(MarksNeedsReviewWhenSummaryRequiresReview|MarksSheinCookieUnavailableAsBlockingIssue|MarksCompletedWhenSummaryDoesNotRequireReview|PersistsPartialResultBeforeMarkingFailed|InitializesDefaultSheinPricing|ReusesPublishedSheinPricingCache)" -count=1
go test ./internal/listingkit -run "Test(ProcessListingKit.*|ServiceProcessFileUsesExplicitFlowSeam)" -count=1
go test ./internal/listingkit -run "TestProcessor(ProcessTask.*|StateMachine.*)" -count=1
go test ./internal/listingkit -count=1
go test ./internal/listingkit/... -count=1
go test ./internal/listingkit/httpapi ./internal/listingkit/temporal -count=1
```

Notes:

- `./internal/listingkit/...` already covers `httpapi` and `temporal`, but the explicit final run was kept as a focused confirmation for the integration seams this phase could have affected.

## Current Branch Notes

The main `Phase 5A` commits are:

- `84480c19` `docs: add framework phase5a plan`
- `3a1fbc70` `refactor: extract listingkit process outcome rules`
- `86b02b32` `refactor: introduce listingkit process flow seam`
- `ec59f1d5` `refactor: align listingkit processor retry gating`
- `368d381b` `test: lock listingkit process phase model boundary`

## Recommendation

Mark `Phase 5A` complete.

Do not keep working this seam just because the new flow and state-helper files are now easier to edit. The main ownership bug this phase addressed is already fixed:

- ListingKit task claim, terminal persistence, and worker retry gating no longer primarily live as implicit logic spread across `service_process.go` and `processor.go`

If we continue, the better next step is to choose a new hotspot based on where behavior-level changes are actually landing, most likely around workflow/process execution modeling rather than more process-seam symmetry cleanup.

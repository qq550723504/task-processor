# ListingKit SHEIN Submit State Machine Plan

## Scope

Second-phase refactor for ListingKit SHEIN submit only. Keep `/generate`, workflow generation, SHEIN domain rules, and submit remote API behavior unchanged.

Goals:

- Make submit attempts observable as structured state instead of only final status.
- Add request-level idempotency for duplicate submit calls.
- Prevent concurrent same-task submit execution from causing duplicate remote API calls.
- Keep compatibility with existing JSON task results and existing UI fields.

Non-goals:

- No database migration.
- No new async queue.
- No redesign of SHEIN publish payload rules.
- No cross-process distributed lock in this phase.

## Current Problems

- `SubmitTask` is a single large imperative workflow with no durable phase state.
- `SubmissionReport` only records last status and per-action final records.
- `SubmissionEvent.RequestID` exists but is not populated.
- Concurrent submit calls can both reach SHEIN remote APIs.
- There is no request idempotency key in the submit request or handler.
- UI can show final submit result, but cannot explain the active or failed phase.

## Data Model

Update `internal/publishing/shein/submission.go`:

- Add stable phase/status constants for submit:
  - phases: `validate`, `prepare_product`, `upload_images`, `pre_validate`, `submit_remote`, `persist_result`
  - statuses reuse existing semantic values where possible: `running`, `succeeded`, `failed`, `blocked`
- Extend `SubmissionReport` with current attempt fields:
  - `current_action`
  - `current_phase`
  - `current_request_id`
  - `in_flight_started_at`
  - `attempt_count`
- Extend `SubmissionRecord` with:
  - `request_id`
  - `phase`
  - `started_at`
  - `finished_at`
  - `attempt`
- Keep existing `last_*`, `save_draft`, `publish`, and `last_result` fields.

Update frontend SHEIN types to match the additive fields.

## Backend State Helper

Add `internal/listingkit/shein_submit_state.go` with small helpers that operate on `*listingkit.ResultPackage`:

- Normalize request ID/idempotency key.
- Begin an attempt and set report current state.
- Advance phase.
- Complete attempt and write final record/event.
- Fail or block attempt and write final record/event.
- Resolve a completed record by request ID for idempotent replay.

Tests first in `internal/listingkit/shein_submit_state_test.go`:

- Beginning an attempt sets current action, phase, request ID, started time, and attempt count.
- Advancing phases preserves request ID and attempt.
- Completing clears in-flight fields and writes request ID/started/finished on the action record.
- Failing records the failed phase and error.
- Existing completed request ID can be replayed without creating a new attempt.

## Submit Lock And Idempotency

Add an in-process keyed lock owned by `service`:

- Key by `taskID + ":" + action`.
- Serialize same-task same-action submit calls.
- After acquiring the lock, reload the task/result and check whether the idempotency key already has a completed record.
- If a completed record exists for the same idempotency key, return current preview without remote calls.
- If there is no idempotency key, preserve current behavior except for serialization.

Add optional request fields:

- `idempotency_key` in `SubmitTaskRequest`
- API handler also accepts `Idempotency-Key` header when body omits the field.

Tests first:

- API handler maps `Idempotency-Key` header into the service request.
- Two calls with the same idempotency key invoke the SHEIN remote publish/save draft API once.
- A replay after success returns the saved preview and does not call remote API.

## Submit Workflow Refactor

Refactor `internal/listingkit/service_submit.go` without changing business order:

1. Load task/result and normalize action/platform.
2. Acquire submit lock.
3. Reload result and check idempotency replay.
4. Begin attempt phase `validate`.
5. Run readiness checks. Blocked readiness records blocked/failed state and returns `ErrSubmitBlocked`.
6. Phase `prepare_product`: clone product, rebuild attributes, optimize/translate, prepare product.
7. Phase `upload_images`: upload/cache images.
8. Phase `pre_validate`: run SHEIN validator.
9. Phase `submit_remote`: call save draft or publish.
10. Phase `persist_result`: write final record/event and save task result.

Failure rules:

- Readiness blockers remain `ErrSubmitBlocked`.
- Remote/payload/validation failures record request ID, phase, and error before returning.
- Existing final success/failure event behavior stays visible through old fields.

## Frontend Updates

Update `web/listingkit-ui`:

- Type additions in `src/lib/types/listingkit/shein.ts`.
- Submit API helper sends a generated idempotency key per user submit action.
- Existing submission panel displays current phase when present.
- Existing final result display keeps fallback behavior for old tasks.

Tests first:

- Submit request includes an idempotency key.
- Submission display renders current phase when report has one.
- Old report without new fields still renders final status.

## Verification

Run:

- `go test ./internal/listingkit ./internal/app/httpapi`
- In `web/listingkit-ui`, use scripts from `package.json`:
  - `npm test`
  - `npm run lint`

Known baseline note:

- `go mod download` timed out in this worktree, but the target Go tests compile and pass.

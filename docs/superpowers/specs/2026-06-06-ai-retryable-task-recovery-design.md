# AI Retryable Task Recovery Design

## Background

ListingKit tasks currently treat several upstream dependency failures as terminal failures. A representative example is OpenAI image/category assistance returning `insufficient credits`. When that happens today:

- The task may end in `failed` or `needs_review` without a clean recovery path.
- Operators must inspect logs, restore the upstream dependency, then manually discover which tasks are safe to continue.
- Some stages can only continue by recreating or manually patching work, even though the original task context is still valid.

This is operationally expensive and will recur whenever AI credits, rate limits, transient upstream outages, or short network failures happen.

## Goal

Turn retryable upstream dependency failures into recoverable task states with:

- Automatic resume when the dependency becomes healthy again.
- Clear operator-visible status instead of ambiguous terminal failure.
- Explicit manual recovery controls for single-task and batch recovery.
- No duplicate task creation and no loss of already-completed intermediate work.

## Non-Goals

- Redesign every ListingKit workflow state.
- Replace existing business review flows such as real SHEIN category/attribute confirmation.
- Introduce a generic distributed job orchestration system for all services.
- Auto-recover truly invalid input, deterministic code bugs, or unrecoverable data corruption.

## Problem Statement

Today the system mixes at least three different situations:

1. Permanent task failure
   Example: invalid request payload, missing mandatory business data, unsupported product structure.

2. Human review required
   Example: SHEIN category/attribute mapping needs confirmation.

3. Upstream dependency temporarily unavailable
   Example: OpenAI credits exhausted, 429 rate limit, transient upstream timeout, temporary auth or network outage.

The third case should not behave like the first two. It should pause and recover.

## Proposed Outcome

Introduce a retryable-blocked lifecycle for ListingKit tasks:

- Retryable dependency failures transition tasks into a dedicated recoverable state.
- The state stores enough structured reason metadata to decide whether auto-recovery is allowed.
- A recovery runner periodically rechecks and resumes eligible tasks.
- Operators can also trigger immediate single-task or bulk recovery.
- UI surfaces these tasks as “waiting for dependency recovery” instead of generic failure.

## Approach Options

### Option A: Keep `failed`, add only a batch retry button

Pros:

- Minimal schema and code changes.

Cons:

- Still conflates terminal failure and retryable blockage.
- UI remains misleading.
- Automatic recovery remains impossible or fragile.
- Operators still need to know which failures are safe to retry.

### Option B: Reuse `pending` for blocked tasks

Pros:

- Avoids a new visible task status.

Cons:

- Hides why the task is not progressing.
- Makes queue semantics ambiguous.
- Hard to distinguish never-started work from blocked work.
- Poor operator observability.

### Option C: Add an explicit recoverable blocked state plus recovery runner

Pros:

- Clear semantics.
- Supports both automatic and manual recovery.
- Makes UI, metrics, and ops actions straightforward.
- Preserves current task identity and completed work.

Cons:

- Requires schema, state machine, API, and UI changes.

### Recommendation

Use Option C.

This is the smallest design that correctly models the operational reality and prevents repeated manual cleanup.

## Domain Model Changes

### New task status

Add a new task status:

- `blocked_retryable`

Meaning:

- The task cannot currently make progress because a dependency is temporarily unavailable.
- The task is expected to continue later without recreating it.

Keep existing statuses:

- `pending`
- `processing`
- `needs_review`
- `completed`
- `failed`

Interpretation:

- `failed` becomes terminal and non-auto-recoverable.
- `needs_review` remains a human business-decision state, not a dependency outage state.

### New retryable failure metadata

Persist structured recovery metadata on the task result or task record:

- `retryable_block.reason_code`
  Example: `openai_insufficient_credits`, `openai_rate_limited`, `upstream_timeout`, `upstream_network_error`
- `retryable_block.reason_message`
- `retryable_block.blocked_at`
- `retryable_block.last_retry_at`
- `retryable_block.next_retry_at`
- `retryable_block.retry_attempts`
- `retryable_block.max_auto_retry_attempts`
- `retryable_block.recovery_scope`
  Example: `full_task`, `stage:category_resolution`, `stage:image_generation`
- `retryable_block.auto_resume_enabled`

This metadata must be durable so restarts do not lose recovery context.

## Failure Classification

Introduce a centralized classifier for upstream errors.

### Retryable dependency failures

These should map to `blocked_retryable`:

- OpenAI `insufficient credits`
- OpenAI 429 or explicit rate limiting
- transient 5xx upstream errors
- temporary network timeouts
- temporary cookie/auth refresh outage when known to be recoverable
- queue/worker backpressure that should continue later

### Non-retryable failures

These remain `failed`:

- invalid request payload
- missing required product fields
- invalid revision payload
- unsupported business rule combinations
- deterministic code-level validation errors

### Business review states

These remain `needs_review`:

- category/attribute/sale-attribute confirmation required
- policy or content review required
- human confirmation steps before final publish

## Execution and Resume Strategy

### Core rule

Resume from the last unfinished stage whenever possible. Do not recreate the task.

### Resume granularity

Recovery scope should support:

- full task resume when the workflow can safely restart idempotently
- stage-level resume when a specific stage was blocked

Examples:

- OpenAI category-assist blocked before a real SHEIN category was resolved:
  resume category resolution and then downstream attribute stages.
- AI image generation blocked before assets were produced:
  resume image generation stage only.

### Idempotency

All resumed stages must be safe to rerun:

- no duplicate task creation
- no duplicate downstream publish side effects
- no duplicate revision history spam where avoidable

If a stage has an external side effect, its recovery path must check for an already-produced result before rerunning.

## Automatic Recovery Runner

Add a background recovery runner that periodically scans for tasks in `blocked_retryable`.

### Runner responsibilities

- Load eligible tasks where `next_retry_at <= now`
- Revalidate whether the failure is still retryable
- Requeue or directly resume the task from its saved recovery scope
- Update `last_retry_at`, `retry_attempts`, and `next_retry_at`
- Clear retryable-block metadata when recovery succeeds
- Escalate to manual-recovery-required when automatic retry attempts are exhausted

### Backoff policy

Use bounded exponential backoff, for example:

- 1 min
- 5 min
- 15 min
- 30 min
- 60 min
- then fixed interval

The exact values can be tuned, but the system should avoid thundering-herd retries immediately after dependency recovery.

### Exhaustion behavior

When max auto-retry attempts are reached:

- Keep the task in a recoverable state, not terminal `failed`
- Mark it as auto-retry paused
- Surface “manual recovery required” in UI and metrics

This preserves recoverability without infinite churn.

## Manual Recovery

Support two operator actions:

### Single-task recover now

- Trigger immediate retry for one task.
- Reset `next_retry_at` to now.
- Attempt resume using the stored recovery scope.

### Bulk recover retryable-blocked tasks

Allow filtering and bulk retry by:

- all retryable-blocked tasks
- reason code
- tenant
- store
- time window

This is the fast path after credits are restored or a shared dependency is fixed.

## Frontend and UX

### Task status presentation

Show retryable blocked tasks as:

- `等待依赖恢复`

Not as generic `失败`.

### Task detail messaging

Show:

- dependency name
- specific reason
- whether auto-retry is active
- next scheduled retry time
- retry attempt count

### Task detail actions

Add:

- `立即重试`
- `继续等待自动恢复`

Only show `立即重试` when the user has permission.

### List and queue filtering

Add filtering for:

- `blocked_retryable`
- reason code

### Bulk recovery entry

Expose a bulk action in the relevant task list or admin surface:

- `恢复所有可重试阻塞任务`

## API Changes

Add or extend APIs for:

- querying retryable-block metadata in task detail/list responses
- retrying a single recoverable task
- bulk retrying recoverable tasks

Response payloads should clearly separate:

- `status`
- `retryable_block`
- `needs_review`

so the frontend does not infer this from free-form error strings.

## Metrics and Observability

Add metrics for:

- count of tasks in `blocked_retryable`
- blocked tasks by reason code
- automatic recovery attempts
- successful recoveries
- exhausted auto-retries
- mean recovery time by reason code

Add structured logs for:

- classification into retryable-blocked
- scheduled retry creation
- recovery start
- recovery success
- recovery exhaustion

## Data Migration

Existing tasks already failed due to retryable dependency reasons should be recoverable where possible.

Migration strategy:

- Add schema support for new status and metadata
- Backfill recent `failed` tasks whose error matches known retryable patterns into `blocked_retryable`
- Only backfill tasks whose workflow state is safe to resume

Unsafe legacy failures may remain terminal.

## Rollout Plan

### Phase 1: Classification and status support

- Add `blocked_retryable`
- Add structured retryable-block metadata
- Route new retryable upstream failures into this state

### Phase 2: Manual recovery

- Add single-task recover endpoint
- Add bulk recover endpoint
- Add frontend display and actions

### Phase 3: Automatic recovery

- Add background recovery runner
- Add backoff policy
- Add metrics and dashboards

### Phase 4: Legacy backfill

- Backfill safe historical failures
- Add admin tools for selective recovery

## Risks

### Misclassification

If a permanent business error is incorrectly treated as retryable, the system may retry uselessly.

Mitigation:

- central classifier
- explicit allowlist of retryable reason codes
- bounded auto-retries

### Duplicate side effects

If resume logic is not idempotent, recovery can duplicate downstream work.

Mitigation:

- resume-from-stage design
- side-effect existence checks
- integration tests around repeat recovery

### Hidden stuck tasks

If blocked tasks are neither auto-retried nor surfaced clearly, they become silent backlog.

Mitigation:

- dedicated status
- next retry timestamp
- dashboards and filters

## Testing Strategy

### Unit tests

- classify OpenAI insufficient credits as retryable-blocked
- classify validation errors as terminal failed
- compute next retry time correctly
- preserve resume scope metadata

### Service tests

- task enters `blocked_retryable` on retryable upstream failure
- task can recover after dependency resumes
- task remains resumable after process restart
- exhausted auto-retries pause without converting to terminal failure

### Integration tests

- recover a task blocked during category resolution
- recover a task blocked during image generation
- bulk recover multiple blocked tasks
- ensure no duplicate downstream side effects after repeated retry

### Frontend tests

- render retryable-blocked status distinctly
- show retry metadata
- allow single-task immediate retry
- allow bulk recover action

## Success Criteria

- Retryable upstream outages no longer require task recreation.
- Restoring AI credits allows blocked tasks to continue automatically.
- Operators can manually recover all blocked tasks in one action.
- UI clearly distinguishes human review from dependency blockage.
- Recovered tasks do not duplicate already-completed work.

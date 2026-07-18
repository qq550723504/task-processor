# SHEIN Cost Save Concurrency Design

## Goal

Make rapid, consecutive SHEIN cost-price edits complete without tying each HTTP response to a full candidate-cost rebuild.

## Decision

The cost-group API persists the manual cost and returns immediately. It then schedules one best-effort, in-process refresh per `(tenant_id, store_id, group_key)`. Calls for a key already running are coalesced, so the latest persisted cost is rebuilt by the running refresh or one follow-up run. A failed background refresh is logged but does not turn a successful cost write into a failed request.

The UI retains per-row save state, serializes saves for the current workbench, and updates/refetches only the cost-group query after a successful save. It must not await invalidation of the complete enrollment-store scope for every edit.

## Constraints

- Manual cost remains durable before the API responds.
- Candidate costs become eventually consistent; a later candidate refresh remains authoritative.
- No external queue or schema migration is introduced in this slice.
- Concurrent writes for different groups must not cause duplicate rebuilds for the same group.

## Verification

- A service test proves that a cost save returns before a deliberately blocked rebuild finishes and that duplicate requests for one group coalesce.
- A UI/query test proves a successful cost mutation invalidates only cost-related keys.
- Existing SHEIN sync and focused UI tests remain green.

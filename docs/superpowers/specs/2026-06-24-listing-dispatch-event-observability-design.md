# Listing Dispatch Event Observability Design

Date: 2026-06-24

## Goal

Add a lightweight operator-facing observability surface for `listing_dispatch_event`.

The control plane now writes dispatch audit facts, but operators still need a productized way to answer:

- Are tasks being dispatched or skipped?
- Which reason codes are blocking dispatch most often?
- Which stores are repeatedly blocked?
- What did the latest dispatch decisions look like?

This feature should expose that data through a backend JSON API and a simple ListingKit admin page. It should not become a full BI system.

## Scope

In scope:

- Backend summary endpoint for dispatch event aggregates.
- Backend paged list endpoint for recent dispatch events.
- ListingKit admin page showing filters, summary cards, reason distribution, store blockers, and recent events.
- Tests for repository, handler, API schema, and page rendering.

Out of scope:

- CSV export.
- Long-range trend charts.
- Alerting rules.
- Automatic remediation.
- Publishing the frontend artifact during this task.

## Recommended approach

Use two backend endpoints:

- `GET /api/v1/listing-kits/admin/dispatch-events/summary`
- `GET /api/v1/listing-kits/admin/dispatch-events`

This keeps aggregate queries and row-list queries independently evolvable. It also mirrors the existing admin pattern used by store statistics and import tasks.

## Backend design

### Data model

Reuse the existing `listing_dispatch_event` table and model fields:

- `id`
- `task_id`
- `tenant_id`
- `store_id`
- `platform`
- `action`
- `reason_code`
- `stage`
- `capacity`
- `queued`
- `processing`
- `completed_today`
- `daily_limit`
- `owner_node`
- `created_at`

No schema change is required.

### Query parameters

Both endpoints should accept:

- `platform`
- `tenantId`
- `storeId`
- `action`
- `reasonCode`
- `from`
- `to`

The list endpoint also accepts:

- `page`
- `page_size`

Default time window:

- If neither `from` nor `to` is provided, use the last 60 minutes.
- If only `from` is provided, use `from` through now.
- If only `to` is provided, use one hour before `to` through `to`.

Time parsing:

- Accept RFC3339 timestamps.
- Return `400` for invalid timestamps.

Tenant scope:

- Apply the existing request tenant scope by default.
- Allow explicit `tenantId` only when it matches the scoped tenant or when existing admin scope rules allow broader access.
- Preserve existing owner/user scoping conventions where applicable.

### Summary response

Return:

```json
{
  "window": {
    "from": "2026-06-24T14:00:00+08:00",
    "to": "2026-06-24T15:00:00+08:00"
  },
  "total": 1450,
  "dispatched": 37,
  "skipped": 1413,
  "failed": 0,
  "reasonCounts": [
    { "reasonCode": "store_paused", "action": "skipped", "count": 725 },
    { "reasonCode": "no_capacity", "action": "skipped", "count": 398 }
  ],
  "storeBlockers": [
    {
      "tenantId": 246,
      "storeId": 1041,
      "reasonCode": "no_capacity",
      "count": 145,
      "dailyLimit": 500,
      "maxQueued": 8,
      "maxProcessing": 0,
      "maxCompletedToday": 0,
      "ownerNode": "shein-listing-shard-1"
    }
  ]
}
```

Rules:

- Empty `reason_code` should be represented as `"<dispatched>"` in distribution rows.
- `failed` should be counted if future rows use `action=failed`; for now it will normally be `0`.
- `storeBlockers` should include skipped events only.
- Limit `storeBlockers` to the top 20 by count.

### List response

Return a paged response:

```json
{
  "items": [
    {
      "id": 123,
      "createdAt": "2026-06-24T14:54:09+08:00",
      "taskId": 8417710,
      "tenantId": 246,
      "storeId": 1041,
      "platform": "shein",
      "action": "skipped",
      "reasonCode": "no_capacity",
      "stage": "dispatch",
      "capacity": 8,
      "queued": 8,
      "processing": 0,
      "completedToday": 0,
      "dailyLimit": 500,
      "ownerNode": "shein-listing-shard-1"
    }
  ],
  "total": 1450,
  "page": 1,
  "page_size": 50
}
```

Ordering:

- `created_at desc`
- `id desc`

Page size:

- Default `50`.
- Maximum `200`.

## Frontend design

Add an admin page at:

- `/listing-kits/admin/dispatch-events`

The page should use existing ListingKit admin UI patterns rather than introducing a new visual system.

### Page layout

Top section:

- Title: `调度事件`
- Description: current time window and total event count.
- Filters:
  - platform
  - store ID
  - action
  - reason code
  - from
  - to
  - refresh button

Summary section:

- Total events
- Dispatched
- Skipped
- Failed

Distribution section:

- Reason code rows with count badges.
- Store blocker rows showing tenant/store/reason/count/daily limit/queue depth.

Recent events table:

- Created time
- Task ID
- Tenant/store
- Action
- Reason
- Capacity
- Queue depth
- Daily limit
- Owner node

### Frontend API module

Add `web/listingkit-ui/src/lib/api/admin-dispatch-events.ts`.

Use `zod` schemas with `.passthrough()` to tolerate additive backend fields.

Expose:

- `getListingDispatchEventSummary(query)`
- `getListingDispatchEvents(query)`

### Navigation

If the admin shell has an obvious ListingKit admin nav registry, add a link labelled `调度事件`.

If navigation is hard-coded and risky to touch, leave the page routable and document that nav wiring can be done with the frontend publication task.

## Error handling

Backend:

- Invalid timestamps return `400` with a stable error code such as `invalid_dispatch_event_time_range`.
- Repository/query errors return `500` using existing handler error helpers.
- Empty result sets return valid empty arrays and zero counts.

Frontend:

- Show the existing destructive alert pattern for API errors.
- Show loading states in summary cards and table.
- Show empty states for no distribution rows or no recent events.

## Testing

Backend repository tests:

- Summary counts dispatched/skipped by action and reason.
- Summary top store blockers.
- List endpoint filtering by store/action/reason/time.
- Default one-hour window behavior.

Backend handler tests:

- Valid summary response shape.
- Valid list response shape.
- Invalid timestamp returns `400`.
- Tenant scoping is applied.

Frontend tests:

- API schema parses summary and paged list.
- Page renders summary cards, reason counts, store blockers, and recent events from mocked API calls.
- Empty state renders without crashing.

## Rollout notes

Backend can be merged and deployed independently.

Frontend page publication can wait for a ListingKit UI deployment window. Until then, the backend API still gives operators and scripts a stable productized query surface.

## Risks and mitigations

- Large event volume could make summary queries expensive.
  - Mitigation: default to last 60 minutes, cap page size, and group over indexed time-window filters.
- Event fields may evolve.
  - Mitigation: frontend schemas use `.passthrough()` and only depend on stable fields.
- Operators may confuse `no_capacity` from queue depth with daily limit exhaustion.
  - Mitigation: show `capacity`, `queued`, `processing`, `completedToday`, and `dailyLimit` side by side.

## Success criteria

- Operators can see dispatch health without direct database access.
- The last hour of dispatch decisions can be summarized by action, reason, and store.
- Recent events can be inspected from the admin page.
- Backend and frontend tests cover the API contract and page rendering.

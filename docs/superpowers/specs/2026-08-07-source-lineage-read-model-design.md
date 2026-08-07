# Source Lineage Read Model

## Status

Design approved for implementation planning.

## Goal

Make the normalized source identity already persisted on ListingKit tasks visible through the existing task-list API and ListingKit workbench, so an operator can trace a task to its 1688 source without reading the internal request payload.

## Current Boundary

The existing flow preserves source identity through:

```text
1688 source result
  -> ProductFacts
  -> listingkit.GenerateRequest.Source
  -> persisted Task.Request JSON
```

The gap is the read side: `TaskListItem` exposes a coarse `source_type`, but not the normalized source key, platform, ID, or URL. Pending tasks can also lack `source_type` because the current list builder prefers the completed result summary.

## Design

### 1. Add a read-only source reference field

Add `SourceReference *SourceReference` to `TaskListDisplayFields` with the JSON name `source_reference`. The existing neutral `listingkit.SourceReference` type is reused because it already contains identity-only fields and is already persisted in `GenerateRequest`.

The API must expose only the normalized identity fields:

```json
{
  "source_reference": {
    "key": "crawler:1688:888",
    "type": "crawler",
    "platform": "1688",
    "id": "888",
    "url": "https://detail.1688.com/offer/888.html"
  }
}
```

No raw crawler payload, warning collection, credentials, or marketplace publishing state is exposed.

### 2. Populate the task-list projection

`applyTaskListRequestFields` copies `task.Request.Source` into a new value so the API response cannot mutate the persisted request through shared pointers. When the source reference is absent, the field remains omitted for legacy tasks.

When a source reference has a non-empty `Type`, it also supplies the existing `source_type` display fallback if the result summary has not populated one yet. Completed result behavior remains unchanged.

### 3. Update the existing UI contract only

Extend the existing ListingKit task TypeScript type and Zod response schema with the optional `source_reference` object. Display the source platform and a link to the normalized source URL in the existing task card/list metadata area when present. Do not add a new page, portal, workspace, database table, or task filter in this slice.

### 4. Preserve security and compatibility

The legacy public generate endpoint continues to clear caller-supplied `Source` before task creation. Only the internal normalized source-facts bridge can create trusted source lineage. Existing clients that do not receive `source_reference` continue to parse successfully because the field is optional.

## Data Flow

```text
Task.Request.Source
        |
        v
TaskListItem.SourceReference
        |
        v
GET /api/v1/listing-kits/tasks
        |
        v
ListingKit task list/card
```

## Testing Strategy

1. Add a Go task-list projection test for a pending task with a source reference, including pointer-copy behavior and `source_type` fallback.
2. Add a Go API serialization test proving `source_reference` is present for sourced tasks and omitted for legacy tasks.
3. Extend the existing UI task schema test with a complete source reference and verify the task card renders the platform/link metadata.
4. Run focused Go ListingKit tests, focused UI tests, and the existing full backend/frontend checks.

## Non-goals

- No new source table, migration, or source-specific query model.
- No change to task lifecycle, authorization, retry, preview, readiness, or publishing behavior.
- No exposure of `Task.Request` itself through a new endpoint.
- No consumer portal or subscription work.

## Acceptance Criteria

- A pending 1688 task list item exposes its normalized source key, platform, ID, and URL.
- Legacy tasks omit `source_reference` and retain existing response behavior.
- The UI displays the source identity without exposing raw crawler data.
- The public generate endpoint cannot forge the displayed source reference.
- Existing backend and frontend test suites remain green.

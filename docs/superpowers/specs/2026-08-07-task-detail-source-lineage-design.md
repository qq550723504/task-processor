# Task Detail Source Lineage Design

## Goal

Expose the persisted source identity on the ListingKit task-detail response and status page so a user can reopen or refresh a task and still see its original 1688 source without relying on browser-local creation data.

## Problem

`TaskListItem` now exposes `source_reference`, but `TaskResult`—the response returned by `GET /listing-kits/tasks/:task_id`—does not. The status page therefore renders `TaskSourceSummary` from a local creation draft. That summary disappears after a browser refresh, on another device, or for a historical task whose creation draft is no longer present.

The persisted source is already available on `Task.Request.Source`. The detail projection should read it server-side and expose it as read-only data.

## Scope

### In scope

- Add a top-level optional `source_reference` field to the task-detail `TaskResult` response.
- Populate it from the persisted task request during `buildTaskResult`.
- Preserve the existing defensive-copy behavior used by task-list projections.
- Extend the TypeScript type and Zod response schema.
- Render a reusable source-lineage card on the task status page when persisted source data exists.
- Keep legacy tasks without source data backward compatible by omitting the field and retaining existing local-draft behavior when available.
- Add backend projection, API/schema, and status-page rendering tests.

### Out of scope

- Retry or recreate actions based on the source reference.
- Accepting `source_reference` from public client task-creation requests.
- New database tables, migrations, endpoints, filters, or source-provider integrations.
- Replacing the existing creation-draft summary for tasks that have no persisted source.

## Design

### Backend response model

Add this optional field to `listingkit.TaskResult`:

```go
SourceReference *SourceReference `json:"source_reference,omitempty"`
```

`buildTaskResult` copies the identity from `task.Request.Source` into the response. The copy must be independent of the persisted request object so callers cannot mutate task state through the response model. If the task, request, or source reference is absent, the field remains nil and is omitted from JSON.

The public creation boundary remains unchanged: clients still cannot forge persisted source identity by submitting `source` in a public create request. This feature only projects source data that the server already persisted.

### Frontend contract and rendering

Extend `ListingKitTaskResult` and `taskResultSchema` with the same optional `ListingKitSourceReference` shape already used by task-list items.

Create `web/listingkit-ui/src/components/listingkit/tasks/task-persisted-source-reference.tsx` as a small presentational component for persisted lineage, without coupling it to local draft storage. Its test belongs in `task-persisted-source-reference.test.tsx`. It should render:

- a source label using `platform` and `id` when available;
- a safe external `查看来源` link only when `url` is present;
- no source card when all source identity fields are absent;
- no nested link inside the task page's existing navigation links.

The task status page should prefer persisted `task.source_reference` for the source card. When it is absent, it may continue rendering the existing local-draft `TaskSourceSummary` so legacy and newly-created tasks do not regress.

### Data flow

```text
listing_kit_tasks.request.source
        |
        v
taskLifecycleService.GetTaskResult
        |
        v
TaskResult.source_reference
        |
        v
GET /listing-kits/tasks/:task_id
        |
        v
TaskStatusScreen -> persisted source card
```

## Error and compatibility behavior

- Missing source data is normal and produces no `source_reference` field.
- Malformed or absent local draft data must not prevent the persisted task response from rendering.
- Existing source-summary wording and local draft behavior remain available for tasks created before source persistence.
- The external link uses the existing URL value and opens in a new tab with `rel="noreferrer"`, matching the task-list source link behavior.

## Testing strategy

- Backend unit test: a pending task with persisted `Request.Source` produces a populated, independent `TaskResult.SourceReference`; a legacy task omits it.
- Backend JSON/API test: the task-detail payload contains `source_reference` for persisted source data and does not expose the request object as a client-controlled input.
- Frontend schema test: the task-result parser accepts the normalized source-reference shape.
- Frontend component test: the status page renders the persisted source summary after local draft data is absent, and does not create invalid nested links.
- Regression tests: existing task-status and task-creation source-summary tests remain green.

## Acceptance criteria

1. Reloading a task-detail URL displays its persisted 1688 source without local storage.
2. A source URL is clickable from the detail page and opens safely in a new tab.
3. Legacy tasks remain renderable and do not gain an empty source block.
4. Public task creation still ignores client-supplied source identity.
5. All focused and full backend/frontend checks pass.

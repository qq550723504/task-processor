# Image Agent Workspace Entry and Style Authorization Design

## Goal

Make the manual image-agent workflow reachable from a ListingKit workspace and make style references an explicit, task-owned authorization choice captured immutably when a run is created.

## Problem

The workspace currently renders `WorkspaceAgentSurface` only when the URL already contains `image_agent_run_id`; no product UI creates a run or navigates to it. The existing generic image-agent create API requires caller-supplied run ID, idempotency key, plan, and budget, so it is not a safe browser-facing orchestration boundary.

The ListingKit catalog adapter currently maps only `asset.KindSourceImage` to `AuthorizedAssetSource`. It deliberately excludes all other bundle assets, and `asset.Asset` does not encode an authorization intent of “use this as a style reference.” Reclassifying scene or generated assets by kind would turn a presentation/derivation type into a provider-authorization decision without user intent.

## Decision

Add a task-scoped ListingKit image-agent creation boundary. The browser obtains an owned, sanitized preflight catalog, explicitly chooses zero or more style candidates, and requests a run. The server validates ownership and selection, generates run/action identities, creates the initial plan and conservative budget, and starts the existing image-agent application service. The returned run ID becomes `image_agent_run_id` in the workspace URL.

The authorization snapshot contains:

- Every safe `KindSourceImage` bundle asset as `AuthorizedAssetSource`.
- Only caller-selected, safe, non-source bundle assets as `AuthorizedAssetStyle`.

Style authorization is therefore an explicit create-command intent, not a new durable asset kind and not an inferred mapping from `scene`, `gallery`, `main`, or `Role`. Source assets cannot be selected as style in this slice because the current catalog identity model permits one type per asset ID; keeping source IDs source-only avoids duplicate or type-changing provider identities.

## API Contract

### Preflight catalog

`GET /api/v1/listing-kits/tasks/:task_id/image-agent-assets`

Requires the existing image-agent read permission and authenticated identity. It loads the task through ListingKit ownership rules and returns only safe, canonical task assets:

```json
{
  "source_assets": [{"id":"source-1","label":"Front view","display_url":"https://..."}],
  "style_candidates": [{"id":"scene-1","label":"Lifestyle scene","display_url":"https://..."}]
}
```

`source_assets` contains safe `KindSourceImage` assets. `style_candidates` contains safe, non-source bundle assets. URLs are validated with `imageagent.ValidateSafeImageURL`; task metadata is not exposed as provider authorization data.

### Create run

`POST /api/v1/listing-kits/tasks/:task_id/image-agent-runs`

Requires the existing image-agent write permission and authenticated identity.

```json
{"style_asset_ids":["scene-1"]}
```

The server deduplicates IDs, rejects an unknown, unsafe, source, cross-task, or cross-owner style ID with a validation error, and never accepts client-supplied run IDs, idempotency keys, plans, budgets, tenant IDs, or user IDs. It responds:

```json
{"run_id":"...","status":"accepted"}
```

The server generates a run ID and idempotency key using the existing project ID-generation convention. It builds plan revision 1 with all authorized source IDs, selected style IDs, and exactly one `main` slot. The slot’s source and style references mirror the plan-level selections. It starts the existing `imageagent.Application.Start` with `RunModeManual` and the conservative immutable budget:

```go
imageagent.Budget{
    MaxImages:     1,
    EnabledLimits: imageagent.BudgetLimitImages,
}
```

The run’s existing catalog snapshot is authoritative after creation; later task asset changes or browser requests cannot alter it.

## Backend Boundaries

- `internal/listingkit/httpapi` owns task lookup, authenticated ownership verification, canonical asset translation, ID generation, initial plan construction, and the task-scoped HTTP endpoints.
- `internal/imageagent` remains the owner of run lifecycle, validation, repository writes, and Temporal workflow start. Its generic HTTP API remains available for trusted callers and is not repurposed as a browser orchestration API.
- The catalog translation accepts an explicit style-ID set. It never reads arbitrary metadata or uses `asset.Kind` as a style heuristic.
- New route descriptors and ZITADEL authorization tests register the two task-scoped endpoints with the existing image-agent read/write permissions.

## Frontend Flow

When no valid `image_agent_run_id` exists, the workspace shows a “创建图片方案” action alongside the existing product workspace surface. Selecting it opens a small dialog:

1. Fetch the preflight catalog through the ListingKit BFF proxy.
2. Show source assets as read-only context and style candidates as optional checkboxes.
3. Submit only the selected `style_asset_ids`.
4. On success, preserve current workspace query parameters and set `image_agent_run_id` to the returned run ID using the existing route/navigation pattern.
5. The existing `WorkspaceAgentSurface` mounts and loads the run.

The UI does not manufacture IDs, plans, budgets, ownership fields, or direct provider asset URLs. It renders request failures and disables duplicate submits while creation is pending.

The Next BFF allow-list is extended only for the two explicit task-scoped image-agent routes. No catch-all route expansion is introduced.

## Compatibility and Safety

- Existing run URLs and generic image-agent APIs remain unchanged.
- Tasks with only source assets can create a run with an empty style selection; the style panel remains empty by design.
- Existing task bundles have no implicit styles, preserving current behavior until a user explicitly selects an eligible style candidate at run creation.
- The server checks identity, tenant, task ownership, canonical asset membership, ID uniqueness, and safe URLs before starting a run.
- A failed duplicate click is safe through server-generated idempotency handling; the UI also disables the creation control while the request is pending.

## Tests and Acceptance

Backend tests prove:

- owned task preflight returns safe source assets and safe non-source style candidates;
- create rejects unauthorized ownership, unsafe/unknown/source style IDs, and client attempts to supply server-owned fields;
- create snapshots only selected styles, starts the existing application with revision 1, one main slot, and `max_images=1` enabled;
- changing the task bundle after creation does not alter the started run input;
- route descriptors enforce existing read/write permissions.

Frontend tests prove:

- a workspace without `image_agent_run_id` renders the creation action;
- the dialog fetches candidates, submits only selected style IDs, prevents duplicate submit, and navigates to the returned run ID;
- a workspace with an existing run ID retains current workbench behavior;
- the BFF rejects unlisted task-scoped paths while permitting exactly the new GET and POST paths.

The final verification runs focused Go and UI tests, `go test ./internal/imageagent/...`, the relevant UI test suites, complete repository verification appropriate to changed packages, remote PR CI, then replies to and resolves the two remaining review threads only after those checks pass.

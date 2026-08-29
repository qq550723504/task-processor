# Image Agent Workspace Entry and Style Authorization Design

## Goal

Make the manual image-agent workflow reachable from a ListingKit workspace with immutable target-platform and primary-source authorization, explicit optional style references, and a recoverable lifecycle before the Temporal execution deadline.

## Problem

The workspace currently renders `WorkspaceAgentSurface` only when the URL already contains `image_agent_run_id`; no product UI creates a run or navigates to it. The existing generic image-agent create API requires caller-supplied run ID, idempotency key, plan, and budget, so it is not a safe browser-facing orchestration boundary.

The ListingKit catalog adapter currently maps only `asset.KindSourceImage` to `AuthorizedAssetSource`. It deliberately excludes all other bundle assets, and `asset.Asset` does not encode an authorization intent of “use this as a style reference.” Reclassifying scene or generated assets by kind would turn a presentation/derivation type into a provider-authorization decision without user intent.

Multi-platform tasks keep their assets in `AssetBundlesByTarget`. A run without a persisted target cannot safely choose which target bundle to read or publish, so target-keyed tasks are fail-closed today. Slot execution also accepts multiple source IDs but consumes only the first. Finally, a Temporal server timeout can terminate the workflow without a durable blocked projection, and a `recovery_blocked` effect cannot yet be explicitly redriven.

## Decision

Add a task-scoped ListingKit image-agent creation boundary. The browser obtains an owned, sanitized preflight catalog for one explicit target, chooses exactly one primary source and zero or more style candidates, and requests a run. The server validates ownership and selection, generates run/action identities, creates the initial plan and conservative budget, and starts the existing image-agent application service. The returned run ID becomes `image_agent_run_id` in the workspace URL.

The authorization snapshot contains:

- The selected, safe `KindSourceImage` bundle asset as `AuthorizedAssetSource`.
- Only caller-selected, safe, non-source bundle assets as `AuthorizedAssetStyle`.

Style authorization is therefore an explicit create-command intent, not a new durable asset kind and not an inferred mapping from `scene`, `gallery`, `main`, or `Role`. Source assets cannot be selected as style in this slice because the current catalog identity model permits one type per asset ID; keeping source IDs source-only avoids duplicate or type-changing provider identities.

## API Contract

### Preflight catalog

`GET /api/v1/listing-kits/tasks/:task_id/image-agent-assets?target_platform=shein`

Requires the existing image-agent read permission and authenticated identity. It loads the task through ListingKit ownership rules and returns only safe, canonical task assets:

```json
{
  "source_assets": [{"id":"source-1","label":"Front view","display_url":"https://..."}],
  "style_candidates": [{"id":"scene-1","label":"Lifestyle scene","display_url":"https://..."}]
}
```

`source_assets` contains safe `KindSourceImage` assets. `style_candidates` contains safe, non-source bundle assets. URLs are validated with `imageagent.ValidateSafeImageURL`; task metadata is not exposed as provider authorization data. Target-keyed tasks require an owned `target_platform`; scalar tasks omit it. The preflight always reads one selected bundle.

### Create run

`POST /api/v1/listing-kits/tasks/:task_id/image-agent-runs`

Requires the existing image-agent write permission and authenticated identity.

```json
{"target_platform":"shein","source_asset_id":"source-1","style_asset_ids":["scene-1"]}
```

The server deduplicates style IDs; requires one safe owned source ID; rejects unknown, unsafe, source-as-style, cross-task, cross-owner, and target-mismatched assets; and never accepts client-supplied run IDs, idempotency keys, plans, budgets, tenant IDs, or user IDs. It responds:

```json
{"run_id":"...","status":"accepted"}
```

The server generates a run ID and idempotency key using the existing project ID-generation convention. It persists an optional `target_platform` on the run and builds plan revision 1 with exactly one authorized source ID, selected style IDs, and exactly one `main` slot. The slot’s source and style references mirror the plan-level selections. It starts the existing `imageagent.Application.Start` with `RunModeManual` and the conservative immutable budget:

```go
imageagent.Budget{
    MaxImages:     1,
    EnabledLimits: imageagent.BudgetLimitImages,
}
```

The run’s existing catalog snapshot is authoritative after creation; later task asset changes or browser requests cannot alter it.

## Backend Boundaries

- `internal/listingkit/httpapi` owns task lookup, authenticated ownership verification, target-bundle selection, canonical asset translation, ID generation, initial plan construction, and the task-scoped HTTP endpoints.
- `internal/imageagent` remains the owner of run lifecycle, validation, repository writes, and Temporal workflow start. Its generic HTTP API remains available for trusted callers and is not repurposed as a browser orchestration API.
- The catalog translation accepts one source ID and an explicit style-ID set. It never reads arbitrary metadata or uses `asset.Kind` as a style heuristic.
- New route descriptors and ZITADEL authorization tests register the two task-scoped endpoints with the existing image-agent read/write permissions.
- `imageagent.Service.Start` receives an injected provider-eligibility port and rejects an ineligible tenant before catalog resolution, projection initialization, or Temporal start. The ListingKit adapter implements the port from the same configured product-image-scene tenant allowlist used by the provider runtime; route permissions remain necessary but are not treated as capability entitlement.
- Catalog display labels are normalized at the adapter boundary to at most 256 Unicode code points before the existing `varchar(256)` catalog persistence contract. The bound applies to source and selected style labels; it does not change source URL or provider identity.

### Target publication

`Run.TargetPlatform` is immutable after creation. A scalar run has an empty target and keeps the existing standard-snapshot/scalar publication path. A target-keyed run publishes only into `AssetBundlesByTarget[TargetPlatform]` and its matching inventory summary; no other target bundle or selection may change. Existing runs with an empty target remain fail-closed for target-keyed tasks.

### Deadline and explicit redrive

New v3 workflows retain the 30-day Temporal execution timeout but receive an application-owned lifecycle deadline one day earlier. At that deadline the parent stops new dispatch, performs only bounded safe settlement, and durably persists a blocked projection before the server can terminate the execution. The block records the deadline cause and exposes only recovery/cancel actions.

When bounded effect recovery reaches `recovery_blocked`, persistence retains the exact previous safe effect phase. An authenticated explicit redrive atomically restores that phase for the matching tenant, owner, run, plan revision, slot, and attempt before starting the recovery workflow. It never regenerates provider output and remains idempotent for repeated action IDs.

## Frontend Flow

When no valid `image_agent_run_id` exists, the workspace shows a “创建图片方案” action alongside the existing product workspace surface. Selecting it opens a small dialog:

1. Derive the selected target platform from workspace context and fetch its preflight catalog through the ListingKit BFF proxy.
2. Show source assets as one required radio selection and style candidates as optional checkboxes.
3. Submit only `target_platform`, `source_asset_id`, and selected `style_asset_ids`.
4. On success, preserve current workspace query parameters and set `image_agent_run_id` to the returned run ID using the existing route/navigation pattern.
5. The existing `WorkspaceAgentSurface` mounts and loads the run.

The UI does not manufacture IDs, plans, budgets, ownership fields, or direct provider asset URLs. It renders request failures and disables duplicate submits while creation is pending.

The Next BFF allow-list is extended only for the two explicit task-scoped image-agent routes. No catch-all route expansion is introduced.

## Compatibility and Safety

- Existing run URLs and generic image-agent APIs remain unchanged.
- Tasks with only source assets can create a run with an empty style selection; the style panel remains empty by design.
- A task with multiple sources cannot silently default to the first source; the UI requires one explicit primary source.
- Existing task bundles have no implicit styles, preserving current behavior until a user explicitly selects an eligible style candidate at run creation.
- The server checks identity, tenant, task ownership, canonical asset membership, ID uniqueness, and safe URLs before starting a run.
- The server also checks provider tenant eligibility before allocating a run. An eligible permission alone cannot create a run that the configured provider will deterministically block.
- Labels longer than 256 Unicode code points are safely truncated for display before persistence; they remain non-authoritative metadata.
- A failed duplicate click is safe through server-generated idempotency handling; the UI also disables the creation control while the request is pending.

## Tests and Acceptance

Backend tests prove:

- owned task preflight returns safe source assets and safe non-source style candidates;
- create rejects unauthorized ownership, unsafe/unknown/source style IDs, target-mismatched assets, missing primary source, and client attempts to supply server-owned fields;
- create rejects a verified but provider-ineligible tenant before any run/projection write;
- create snapshots only the selected primary source and styles, persists the selected target, starts the existing application with revision 1, one main slot, and `max_images=1` enabled;
- changing the task bundle after creation does not alter the started run input;
- source and style labels longer than 256 Unicode code points persist as bounded display labels without rejecting an otherwise valid task;
- route descriptors enforce existing read/write permissions.

Frontend tests prove:

- a workspace without `image_agent_run_id` renders the creation action;
- the dialog fetches candidates for the selected target, requires a primary source, submits only target/source/selected style IDs, prevents duplicate submit, and navigates to the returned run ID;
- a workspace with an existing run ID retains current workbench behavior;
- the BFF rejects unlisted task-scoped paths while permitting exactly the new GET and POST paths.

Lifecycle tests prove deadline expiry durably blocks before the server execution deadline and explicit redrive restores a `recovery_blocked` effect's previous recoverable phase. Slot tests reject multiple source IDs at ingress and prove the executor receives only the selected source. The final verification runs focused Go and UI tests, `go test ./internal/imageagent/...`, the relevant UI test suites, complete repository verification appropriate to changed packages, remote PR CI, then replies to and resolves the remaining review threads only after those checks pass.

# ListingKit Service Slimming Checkpoint

> Status: Phase 4 first checkpoint. This note records behavior-preserving file-group splits around the root `internal/listingkit` service object.

## 1. Purpose

Phase 4 reduces the root ListingKit `service` object from an all-in-one dependency and constructor sink into clearer file groups.

This checkpoint records the first service-object slimming wave. It does not introduce a new package, change the public `Service` interface, change `NewService(...)` behavior, or move business rules.

## 2. Current File Groups

The root service construction surface is now split into these files:

```text
internal/listingkit/service.go                            // runtime setters / workflow client config / request normalization
internal/listingkit/service_types.go                      // service struct / dependency config types
internal/listingkit/service_config.go                     // NewService / factory wiring
internal/listingkit/service_defaults.go                   // config defaults / default builders
internal/listingkit/service_collaborators.go              // collaborator initialization groups
internal/listingkit/service_admin_collaborators.go       // admin collaborator accessors
internal/listingkit/service_ai_client_settings.go        // AI client settings facade delegates
internal/listingkit/service_child_task_retry_facade.go   // child task retry facade delegate
internal/listingkit/service_shein_category_search.go     // SHEIN category search facade delegates
internal/listingkit/service_shein_category_client.go     // SHEIN category/attribute API facade helpers
internal/listingkit/service_shein_cookie_preview_facade.go // SHEIN cookie preview facade helper
internal/listingkit/service_shein_cookie_note.go         // SHEIN cookie availability note helper
internal/listingkit/service_shein_store_selection.go     // SHEIN store selection facade helpers
internal/listingkit/service_shein_settings.go            // SHEIN settings facade delegates
internal/listingkit/service_store_profile.go             // store profile / routing facade delegates
internal/listingkit/service_process_facade.go            // process facade delegate
internal/listingkit/service_task_collaborators.go        // task collaborator accessors
internal/listingkit/service_task_generation.go           // task generation facade delegates
internal/listingkit/service_task_revision.go             // task revision facade delegates
internal/listingkit/service_task_lifecycle.go            // task lifecycle facade delegates
internal/listingkit/service_task_sds_baseline.go         // task SDS baseline facade delegates
internal/listingkit/service_task_preview.go              // task preview facade logic
internal/listingkit/service_task_preview_builder.go      // task preview builder helper
internal/listingkit/service_task_export.go               // task export facade logic
internal/listingkit/service_studio_collaborators.go      // studio collaborator accessors
internal/listingkit/service_studio_session.go            // studio session facade delegates
internal/listingkit/service_studio_media.go              // studio media facade delegates
internal/listingkit/service_studio_batch.go              // studio batch facade delegates
internal/listingkit/service_studio_batch_run.go          // studio batch run facade delegates
internal/listingkit/service_submission_collaborators.go   // submission collaborator container
internal/listingkit/service_task_wiring.go               // task/generation/revision collaborator config builders
internal/listingkit/service_studio_wiring.go             // studio collaborator config builders
internal/listingkit/service_submit.go                     // submit facade entrypoint
internal/listingkit/service_submit_primitives.go          // shared submit TTL / sentinel errors
internal/listingkit/service_submit_contracts.go           // shared submit option structs / normalization helpers
internal/listingkit/service_submit_collaborators.go       // submit collaborator accessors
internal/listingkit/service_submit_routing.go             // thin submit/recovery/refresh routing delegates
internal/listingkit/service_submit_workflow.go            // workflow-specific submit gating / launch helpers
internal/listingkit/service_submit_workflow_facade.go     // workflow submit facade helpers
internal/listingkit/service_submit_temporal.go            // Temporal submit facade delegates
internal/listingkit/service_submit_default_action_facade.go // default SHEIN submit action resolver facade
internal/listingkit/service_submit_context_facade.go      // submit runtime context facade helpers
internal/listingkit/service_submit_store_context_facade.go // submit store-context facade helpers
internal/listingkit/service_submit_wiring.go              // submit collaborator config builders
```

## 3. Responsibility Map

### `service.go`

Owns runtime-facing service helpers that are not constructor definitions:

- `SetTaskSubmitter(...)`
- workflow client configuration methods,
- package-level workflow client configuration helpers,
- `currentSheinSubmitSettings(...)`,
- `normalizeGenerateRequest(...)`,
- `normalizePlatforms(...)`.

### `service_types.go`

Owns root service and config type definitions:

- `service`,
- `ServiceCoreDependencies`,
- `ServiceAssetDependencies`,
- `ServiceSheinDependencies`,
- `ServiceWorkflowDependencies`,
- `ServiceConfig`.

### `service_config.go`

Owns factory behavior only:

- `NewService(...)`,
- `newServiceWithConfig(...)`.

This keeps constructor wiring visible without mixing it with default resolver/builder construction.

### `service_defaults.go`

Owns default configuration and default dependency builders:

- `ServiceConfig.applyDefaults(...)`,
- `ensureSheinResolvers(...)`,
- `ensureAssembler(...)`,
- `ensureAssetDependencies(...)`,
- `ensureCoreRepositories(...)`,
- `ensureSheinDefaults(...)`,
- `defaultSheinSettings(...)`,
- `amazonDraftBuilder`,
- default asset recipe/bundle/generation builders.

### `service_collaborators.go`

Owns root collaborator initialization ordering:

- task collaborators,
- admin collaborators,
- submission collaborators,
- Temporal collaborators.

Submission initialization is now grouped as:

1. task-level retry/recovery collaborators,
2. submission state and execution collaborators,
3. SHEIN submission orchestrators, including refresh/status handling,
4. Temporal adapter initialization in its own step.

### `service_admin_collaborators.go`

Owns admin collaborator accessors:

- settings admin,
- SHEIN admin.

### `service_child_task_retry_facade.go`

Owns root child-task retry facade delegate:

- child-task retry entrypoint.

### `service_store_profile.go`

Owns root store profile facade delegates:

- store profile list / upsert / delete,
- store routing settings fetch / update.

### `service_ai_client_settings.go`

Owns root AI client settings facade delegates:

- AI client settings fetch / update.

### `service_shein_settings.go`

Owns root SHEIN settings facade delegates:

- SHEIN settings fetch / update.

### `service_shein_category_search.go`

Owns root SHEIN category search facade delegates:

- manual SHEIN category search entrypoint.

### `service_shein_category_client.go`

Owns root SHEIN category/attribute API facade helpers:

- category API builder,
- attribute API builder.

### `service_shein_cookie_preview_facade.go`

Owns root SHEIN cookie preview facade helper:

- preview rebuild / blocker decoration entrypoint.

### `service_shein_cookie_note.go`

Owns SHEIN cookie availability note helper:

- cookie availability note resolution for preview/recompute surfaces.

### `service_shein_store_selection.go`

Owns root SHEIN store selection facade helpers:

- store id/profile/selection resolver entrypoints.

### `service_process_facade.go`

Owns root process facade delegate:

- listing kit process entrypoint.

### `service_task_collaborators.go`

Owns task collaborator accessors:

- task generation,
- task revision,
- task lifecycle,
- SDS baseline helper access.

### `service_task_generation.go`

Owns root task generation facade delegates:

- generation task listing and actions,
- review queue / session delegates,
- generation retry and navigation dispatch delegates.

### `service_task_revision.go`

Owns root task revision facade delegates:

- revision history listing and detail,
- revision apply and validation delegates.

### `service_task_lifecycle.go`

Owns root task lifecycle facade delegates:

- task creation and result lookup,
- task list and SDS baseline readiness delegates,
- internal inline/enqueue lifecycle helpers used by studio/task flows.

### `service_task_sds_baseline.go`

Owns root task SDS baseline facade delegates:

- SDS baseline warmup delegate.

### `service_task_preview.go`

Owns task preview facade logic:

- task preview fetch and projection assembly.

### `service_task_preview_builder.go`

Owns task preview builder helper:

- preview payload construction and cookie decoration bridge.

### `service_task_export.go`

Owns task export logic:

- export assembly,
- asset generation projection attachment,
- asset render preview fallback population.

### `service_studio_collaborators.go`

Owns studio collaborator accessors:

- studio session,
- studio batch draft,
- studio media,
- studio batch,
- studio batch run.

### `service_studio_session.go`

Owns root studio session facade delegates:

- session gallery and batch listing,
- batch draft fetch / upsert / delete,
- async job status synchronization.

### `service_studio_media.go`

Owns root studio media facade delegates:

- studio design generation entrypoints,
- studio product image generation entrypoints,
- shared media input/output helper delegates.

### `service_studio_batch.go`

Owns root studio batch facade delegates:

- batch detail,
- batch generation lifecycle,
- batch retry / approve actions,
- batch task creation preparation and execution.

### `service_studio_batch_run.go`

Owns root studio batch run facade delegates:

- create batch run,
- fetch batch run,
- list batch run items,
- cancel batch run.

### `service_task_wiring.go`

Owns explicit config builders for non-submit task collaborators:

- task generation,
- task revision,
- task lifecycle,
- SDS baseline.

This keeps accessor files thin while leaving task-specific wiring visible in one place.

### `service_studio_wiring.go`

Owns explicit config builders for studio collaborators:

- studio session,
- studio batch draft,
- studio media,
- studio batch,
- studio batch run.

Studio batch generation wiring is also routed through this file so nested studio
collaborator construction stays visible without re-expanding accessor files.
Studio batch run coordinator/executor config also flows through this seam.
Coordinator-owned batch run start/recovery helpers live alongside the
coordinator in `studio_batch_run_coordinator.go`.

### `service_submission_collaborators.go`

Owns the submission collaborator container.

Fields are grouped by responsibility:

- task-level retry/recovery,
- submission state and execution,
- SHEIN submission orchestrators, including the dedicated refresh collaborator,
- workflow-facing adapter,
- shared submit coordination primitives.

### Submit facade files

The root submit surface is now split so the facade file stays intentionally thin:

- `service_submit.go`: public `SubmitTask(...)` facade entrypoint only,
- `service_submit_primitives.go`: shared in-flight TTL and sentinel errors,
- `service_submit_contracts.go`: shared submit option structs, target normalization, and Temporal replay detection helpers,
- `service_submit_collaborators.go`: submit/recovery/direct/Temporal/state/execution/refresh collaborator accessors,
- `service_submit_routing.go`: thin submit/recovery/requeue/refresh routing delegates that bridge facade calls to collaborators,
- `service_submit_workflow.go`: SHEIN publish workflow gating and launch helpers,
- `service_submit_workflow_facade.go`: root-facing SHEIN workflow submit/gating delegates,
- `service_submit_temporal.go`: root-facing Temporal SHEIN submit and preview delegates,
- `service_submit_default_action_facade.go`: root-facing default SHEIN submit-action resolver,
- `service_submit_context_facade.go`: root-facing store info, API client, and other-api submit helpers,
- `service_submit_store_context_facade.go`: root-facing submit settings and warehouse context helpers,
- `service_submit_wiring.go`: collaborator config builders only.

`service_submit_wiring.go` stays focused on config builder seams, while Temporal
task-loading helpers live alongside the Temporal facade methods in
`service_submit_temporal_adapter.go`.

## 4. Boundary Decision

This checkpoint keeps everything in root `package listingkit`.

Reasons:

- `service` still coordinates many root models and collaborators,
- `NewService(...)` is a public construction boundary,
- default builders still bridge root ListingKit interfaces with marketplace and asset dependencies,
- collaborator accessors still depend on root service fields and package-private service types,
- moving these to a subpackage now would create more import pressure than value.

## 5. Behavior Compatibility

The split is intended to be behavior-preserving:

- no public API changes,
- no `NewService(...)` signature changes,
- no dependency default changes,
- no workflow client configuration changes,
- no request normalization changes,
- no collaborator construction semantics changes.

## 6. Suggested Local Validation

From the repository root:

```powershell
go test ./internal/listingkit/... -run "Service|Config|Generate|Default" -count=1
go test ./internal/listingkit/... -run "Submission|Submit|Recovery|Requeue|Temporal|Publish" -count=1
```

Run broader tests before merging a larger Phase 4 branch:

```powershell
go test ./internal/listingkit/... -count=1
go test ./... -count=1
```

## 7. Next Phase 4 Candidates

Recommended next low-risk slices:

1. Review `service_types.go` for possible internal grouping comments or dependency buckets without changing fields.
2. Review service accessor files to ensure each accessor remains thin and delegates to concept-specific config builders.
3. Keep `service_config.go` focused on factory wiring; avoid adding new default-building logic there.
4. Continue moving default construction helpers to concept-specific files only when the move is behavior-preserving.

Avoid for now:

- moving `service` into a subpackage,
- changing `ServiceConfig`,
- collapsing constructor dependencies into new public DTOs,
- changing workflow client configuration semantics.

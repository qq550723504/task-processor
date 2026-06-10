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
internal/listingkit/service_ai_client_settings_facade.go // AI client settings facade delegates
internal/listingkit/service_child_task_retry_logic.go    // child task retry logic
internal/listingkit/service_shein_category_search_facade.go // SHEIN category search facade delegates
internal/listingkit/service_shein_category_client_helpers.go // SHEIN category/attribute API helpers
internal/listingkit/service_shein_cookie_preview_helper.go  // SHEIN cookie preview helper
internal/listingkit/service_shein_cookie_note_helper.go  // SHEIN cookie availability note helper
internal/listingkit/service_shein_final_draft_facade.go  // SHEIN final draft facade delegate
internal/listingkit/service_shein_image_regeneration.go // SHEIN data image regeneration logic
internal/listingkit/service_shein_pricing_facade.go      // SHEIN pricing preview facade delegate
internal/listingkit/service_shein_resolution_cache_facade.go // SHEIN resolution cache facade delegate
internal/listingkit/service_shein_submission_events_facade.go // SHEIN submission event facade delegate
internal/listingkit/service_shein_store_selection_helpers.go // SHEIN store selection helpers
internal/listingkit/service_shein_settings_facade.go     // SHEIN settings facade delegates
internal/listingkit/service_store_profile_facade.go      // store profile / routing facade delegates
internal/listingkit/service_process_entry.go             // process entry logic
internal/listingkit/service_process_review_helper.go     // process review helper
internal/listingkit/service_task_layers_facade.go        // task layer facade delegates
internal/listingkit/service_task_collaborators.go        // task collaborator accessors
internal/listingkit/service_task_generation_facade.go    // task generation facade delegates
internal/listingkit/service_task_revision_facade.go      // task revision facade delegates
internal/listingkit/service_task_lifecycle_facade.go     // task lifecycle facade delegates
internal/listingkit/service_task_sds_baseline_facade.go  // task SDS baseline facade delegates
internal/listingkit/service_task_preview.go              // task preview logic
internal/listingkit/service_task_preview_helper.go       // task preview helper
internal/listingkit/service_task_export.go               // task export logic
internal/listingkit/service_studio_collaborators.go      // studio collaborator accessors
internal/listingkit/service_studio_session_facade.go     // studio session facade delegates
internal/listingkit/service_studio_media_facade.go       // studio media root facade delegates
internal/listingkit/service_studio_media_helpers.go      // studio media helper delegates
internal/listingkit/service_studio_batch_facade.go       // studio batch facade delegates
internal/listingkit/service_studio_batch_run_facade.go   // studio batch run facade delegates
internal/listingkit/service_submission_collaborators.go   // submission collaborator container
internal/listingkit/service_task_wiring.go               // task/generation/revision collaborator config builders
internal/listingkit/service_studio_wiring.go             // studio collaborator config builders
internal/listingkit/service_submit_facade.go              // submit facade entrypoint
internal/listingkit/service_submit_lease_helper.go        // shared submit lease helper
internal/listingkit/service_submit_contracts.go           // shared submit option structs / normalization helpers
internal/listingkit/service_submit_collaborators.go       // submit collaborator accessors
internal/listingkit/service_submit_routing.go             // thin submit/recovery/refresh routing delegates
internal/listingkit/service_submit_workflow_helpers.go    // workflow submit helpers
internal/listingkit/service_submit_temporal_facade.go     // Temporal submit facade delegates
internal/listingkit/service_submit_default_action_helper.go  // default SHEIN submit action resolver helper
internal/listingkit/service_submit_identity_helper.go     // submit identity helper
internal/listingkit/service_submit_context_helpers.go     // submit runtime context helpers
internal/listingkit/service_submit_settings_context_helpers.go // submit settings context helpers
internal/listingkit/service_submit_warehouse_helper.go    // submit warehouse helper
internal/listingkit/service_submit_wiring.go              // submit collaborator config builders
internal/listingkit/service_upload.go                     // uploaded image logic
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

### `service_child_task_retry_logic.go`

Owns root child-task retry logic:

- child-task retry entrypoint.

### `service_child_task_retry_helpers.go`

Owns shared child-task retry helpers:

- retry execution helpers,
- retry persistence/state helpers.

### `service_store_profile_facade.go`

Owns root store profile facade delegates:

- store profile list / upsert / delete,
- store routing settings fetch / update.

### `service_ai_client_settings_facade.go`

Owns root AI client settings facade delegates:

- AI client settings fetch / update.

### `service_shein_settings_facade.go`

Owns root SHEIN settings facade delegates:

- SHEIN settings fetch / update.

### `service_shein_category_search_facade.go`

Owns root SHEIN category search facade delegates:

- manual SHEIN category search entrypoint.

### `service_shein_category_client_helpers.go`

Owns root SHEIN category/attribute API helpers:

- category API builder,
- attribute API builder.

### `service_shein_cookie_preview_helper.go`

Owns root SHEIN cookie preview helper:

- preview rebuild / blocker decoration entrypoint.

### `service_shein_cookie_note_helper.go`

Owns SHEIN cookie availability note helper:

- cookie availability note resolution for preview/recompute surfaces.

### `service_shein_final_draft_facade.go`

Owns root SHEIN final draft facade delegate:

- SHEIN final draft update entrypoint.

### `service_shein_image_regeneration.go`

Owns root SHEIN data image regeneration logic:

- SHEIN data image regeneration entrypoint.

### `service_shein_pricing_facade.go`

Owns root SHEIN pricing preview facade delegate:

- SHEIN pricing preview entrypoint.

### `service_shein_resolution_cache_facade.go`

Owns root SHEIN resolution cache facade delegate:

- SHEIN resolution cache clear entrypoint.

### `service_shein_submission_events_facade.go`

Owns root SHEIN submission event facade delegate:

- SHEIN submission event listing entrypoint.

### `service_shein_store_selection_helpers.go`

Owns root SHEIN store selection helpers:

- store id/profile/selection resolver entrypoints.

### `service_process_entry.go`

Owns root process entry logic:

- listing kit process entrypoint.

### `service_process_review_helper.go`

Owns process review helper:

- process review reason derivation.

### `service_process_persistence_helper.go`

Owns process persistence helper:

- process terminal status derivation,
- process success/failure persistence.

### `service_task_layers_facade.go`

Owns root task layer facade delegates:

- standard product layer entrypoint,
- platform adaptation layer entrypoint.

### `service_task_collaborators.go`

Owns task collaborator accessors:

- task generation,
- task revision,
- task lifecycle,
- SDS baseline helper access.

### `service_task_generation_facade.go`

Owns root task generation facade delegates:

- generation task listing and actions,
- review queue / session delegates,
- generation retry and navigation dispatch delegates.

### `service_generation_helpers.go`

Owns shared generation helpers:

- temporal platform selection bridge,
- generation retry task selection/planning helpers,
- generation task/review listing helpers.

### `service_task_revision_facade.go`

Owns root task revision facade delegates:

- revision history listing and detail,
- revision apply and validation delegates.

### `service_task_lifecycle_facade.go`

Owns root task lifecycle facade delegates:

- task creation and result lookup,
- task list and SDS baseline readiness delegates,
- internal inline/enqueue lifecycle helpers used by studio/task flows.

### `service_task_sds_baseline_facade.go`

Owns root task SDS baseline facade delegates:

- SDS baseline warmup delegate.

### `service_task_preview.go`

Owns task preview logic:

- task preview fetch and projection assembly.

### `service_task_preview_helper.go`

Owns task preview helper:

- preview payload construction and cookie decoration bridge.

### `service_shein_store_resolution_preview_helper.go`

Owns SHEIN store resolution preview helper:

- store-resolution preview decoration,
- store-resolution summary/snapshot helper values.

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

### `service_studio_session_facade.go`

Owns root studio session facade delegates:

- session gallery and batch listing,
- batch draft fetch / upsert / delete,
- async job status synchronization.

### `service_studio_media_facade.go`

Owns root studio media facade delegates:

- studio design generation entrypoints,
- studio product image generation entrypoints.

### `service_studio_media_helpers.go`

Owns studio media helper delegates:

- shared media input/output helper delegates.

### `service_studio_batch_facade.go`

Owns root studio batch facade delegates:

- batch detail,
- batch generation lifecycle,
- batch retry / approve actions,
- batch task creation preparation and execution.

### `service_studio_batch_run_facade.go`

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

- `service_submit_facade.go`: public `SubmitTask(...)` facade entrypoint only,
- `service_submit_lease_helper.go`: shared in-flight TTL and submit-lease sentinel errors,
- `service_submit_contracts.go`: shared submit option structs, target normalization, and Temporal replay detection helpers,
- `service_submit_collaborators.go`: submit/recovery/direct/Temporal/state/execution/refresh collaborator accessors,
- `service_submit_routing.go`: thin submit/recovery/requeue/refresh routing delegates that bridge facade calls to collaborators,
- `service_submit_workflow_helpers.go`: root-facing SHEIN workflow submit/gating helpers,
- `service_submit_temporal_facade.go`: root-facing Temporal SHEIN submit and preview delegates,
- `service_submit_default_action_helper.go`: root-facing default SHEIN submit-action resolver,
- `service_submit_action_preference_helper.go`: shared preferred submit-action normalization helper,
- `service_submit_identity_helper.go`: shared submit task identity context helper,
- `service_submit_context_helpers.go`: root-facing store info, API client, and other-api submit helpers,
- `service_submit_settings_context_helpers.go`: root-facing submit settings and warehouse context helpers,
- `service_submit_warehouse_helper.go`: shared warehouse-code selection helper,
- `service_submit_wiring.go`: collaborator config builders only.

`service_submit_wiring.go` stays focused on config builder seams, while workflow
launch/gating entry helpers live in `service_submit_workflow_helpers.go` and
Temporal task-loading helpers live alongside the Temporal adapter in
`service_submit_temporal_loader_helper.go`.

### `service_upload.go`

Owns root uploaded image logic:

- uploaded image save,
- uploaded image read,
- uploaded image delete.

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

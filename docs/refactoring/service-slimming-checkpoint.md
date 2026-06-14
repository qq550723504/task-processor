# ListingKit Service Slimming Checkpoint

> Status: Phase 4 first checkpoint. This note records behavior-preserving file-group splits around the root `internal/listingkit` service object.

## 1. Purpose

Phase 4 reduces the root ListingKit `service` object from an all-in-one dependency and constructor sink into clearer file groups.

This checkpoint records the first service-object slimming wave. It does not introduce a new package, change the public `Service` interface, change `NewService(...)` behavior, or move business rules.

The current internal representation direction is also clearer now:

- legacy dependency mirrors should live under a dedicated `mirrors` bucket instead of remaining as flat root `service` fields;
- task/studio/admin grouped collaborator containers should remain the single ownership boundary for collaborator instances.

Recent progress in this area:

- studio dependency resolution now reads directly from `studioDeps`; legacy dependency mirrors no longer own studio session/batch repositories or studio OpenAI hooks.
- admin dependency resolution now reads directly from `adminDeps`; AI client settings and store-profile admin wiring no longer depend on legacy dependency mirrors.
- submission store-profile and submit API builder resolution now reads directly from `submissionDeps`; legacy dependency mirrors no longer own submission-only store-profile or submit builder dependencies.
- shein runtime-only resolver and pricing-cache dependencies now read directly from `sheinRuntimeDeps`; legacy dependency mirrors no longer own runtime-only resolution cache, category/attribute resolvers, or pricing policy.
- workflow asset dispatch persistence phases now resolve their asset repository dependency up front instead of reaching through `service.mirrors` at runtime.

## 2. Current File Groups

The root service construction surface is now split into these files:

```text
internal/listingkit/service.go                            // runtime setters / workflow client config / request normalization
internal/listingkit/service_types.go                      // service struct / dependency config types
internal/listingkit/service_config.go                     // NewService / factory wiring
internal/listingkit/service_defaults.go                   // config defaults / default builders
internal/listingkit/service_collaborators.go              // collaborator initialization groups
internal/listingkit/service_admin_collaborators.go       // admin collaborator accessors
internal/listingkit/service_admin_wiring_support.go      // admin collaborator wiring plus config assembly helpers
internal/listingkit/service_ai_client_settings_entrypoints.go // AI client settings entrypoints
internal/listingkit/service_child_task_retry_logic.go    // child task retry logic
internal/listingkit/service_shein_category_search_entrypoint.go // SHEIN category search entrypoint
internal/listingkit/service_shein_category_api_helpers.go   // SHEIN category/attribute API helpers
internal/listingkit/service_shein_category_search_support.go // SHEIN category search support
internal/listingkit/service_shein_store_resolution_support.go // SHEIN store-resolution support
internal/listingkit/service_shein_cookie_preview_helper.go  // SHEIN cookie preview helper
internal/listingkit/service_shein_cookie_availability_note_helper.go // SHEIN cookie availability note helper
internal/listingkit/service_shein_final_draft_update_entrypoint.go // SHEIN final draft update entrypoint
internal/listingkit/service_shein_data_image_regeneration_logic.go // SHEIN data image regeneration logic
internal/listingkit/service_shein_pricing_preview_entrypoint.go // SHEIN pricing preview entrypoint
internal/listingkit/service_shein_resolution_cache_clear_entrypoint.go // SHEIN resolution cache clear entrypoint
internal/listingkit/service_shein_submission_event_listing_entrypoint.go // SHEIN submission event listing entrypoint
internal/listingkit/service_shein_store_selection_resolvers.go // SHEIN store selection helpers
internal/listingkit/service_shein_settings_entrypoints.go // SHEIN settings entrypoints
internal/listingkit/service_shein_store_settings_entrypoints.go // SHEIN store settings entrypoints
internal/listingkit/service_process_entry.go             // process entry logic
internal/listingkit/service_process_review_helper.go     // process review helper
internal/listingkit/service_task_layers_logic.go         // task layer processing logic
internal/listingkit/service_task_collaborators.go        // task collaborator accessors
internal/listingkit/service_task_generation_logic.go     // task generation logic delegates
internal/listingkit/service_task_revision_entrypoints.go // task revision entrypoints
internal/listingkit/service_task_lifecycle_logic.go      // task lifecycle logic delegates
internal/listingkit/service_sds_baseline_warmup_entrypoint.go // SDS baseline warmup entrypoint
internal/listingkit/service_task_preview_logic.go        // task preview logic
internal/listingkit/service_task_preview_payload_helper.go // task preview helper
internal/listingkit/service_task_export_logic.go         // task export logic
internal/listingkit/service_studio_collaborators.go      // studio collaborator accessors
internal/listingkit/service_studio_batch_draft_session_entrypoints.go // studio batch draft/session entrypoints
internal/listingkit/service_studio_media_generation_entrypoints.go // studio media generation entrypoints
internal/listingkit/service_studio_media_generation_helpers.go // studio media helper delegates
internal/listingkit/service_studio_batch_entrypoints.go  // studio batch entrypoints
internal/listingkit/service_studio_batch_run_entrypoints.go // studio batch run entrypoints
internal/listingkit/service_submission_collaborators.go   // submission collaborator container
internal/listingkit/service_task_wiring.go               // task/generation/revision collaborator config builders
internal/listingkit/service_studio_wiring_support.go     // studio collaborator wiring plus config assembly helpers
internal/listingkit/service_submit_entrypoint.go          // submit facade entrypoint
internal/listingkit/service_submit_lease_helper.go        // shared submit lease helper
internal/listingkit/service_submit_contracts.go           // shared submit option structs / normalization helpers
internal/listingkit/service_submit_collaborators.go       // submit collaborator accessors
internal/listingkit/service_submit_routing.go             // thin submit/recovery/refresh routing delegates
internal/listingkit/service_submit_workflow_entry_helpers.go // workflow submit helpers
internal/listingkit/service_shein_publish_temporal_entrypoints.go // SHEIN publish Temporal entrypoints
internal/listingkit/service_submit_default_action_resolver_helper.go  // default SHEIN submit action resolver helper
internal/listingkit/service_submit_task_identity_helper.go // submit identity helper
internal/listingkit/service_submit_remote_context_helpers.go // submit remote context helpers
internal/listingkit/service_submit_settings_resolution_helpers.go // submit settings context helpers
internal/listingkit/service_submit_warehouse_selection_helper.go // submit warehouse helper
internal/listingkit/service_submit_wiring_support.go      // submit collaborator wiring plus config assembly helpers
internal/listingkit/service_upload_logic.go               // uploaded image logic
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
3. SHEIN submission orchestrators, including refresh/status handling, the direct-submit phase runner bridge, the shared payload-stage runner bridge, the shared remote-submit runner bridge, the shared success-persistence runner bridge, and the shared failure-persistence runner bridge,
4. Temporal lifecycle/flow/persistence/refresh collaborator initialization in its own step.

### `service_admin_collaborators.go`

Owns admin collaborator accessors:

- settings admin,
- SHEIN admin.

### `service_admin_wiring_support.go`

Owns admin collaborator wiring plus explicit config assembly for:

- settings admin,
- SHEIN admin.

### `service_child_task_retry_logic.go`

Owns root child-task retry logic:

- child-task retry entrypoint.

### `service_child_task_retry_helpers.go`

Owns shared child-task retry helpers:

- retry execution helpers,
- retry persistence/state helpers.

### `service_shein_store_settings_entrypoints.go`

Owns root SHEIN store settings entrypoints:

- store profile list / upsert / delete.

### `service_ai_client_settings_entrypoints.go`

Owns root AI client settings entrypoints:

- AI client settings fetch / update.

### `service_shein_settings_entrypoints.go`

Owns root SHEIN settings entrypoints:

- SHEIN settings fetch / update.

### `service_shein_category_search_entrypoint.go`

Owns root SHEIN category search facade delegates:

- manual SHEIN category search entrypoint.

### `service_shein_category_api_helpers.go`

Owns root SHEIN category/attribute API helpers:

- category API builder,
- attribute API builder.

### `service_shein_cookie_preview_helper.go`

Owns root SHEIN cookie preview helper:

- preview rebuild / blocker decoration entrypoint.

### `service_shein_cookie_availability_note_helper.go`

Owns SHEIN cookie availability note helper:

- cookie availability note resolution for preview/recompute surfaces.

### `service_shein_final_draft_update_entrypoint.go`

Owns root SHEIN final draft facade delegate:

- SHEIN final draft update entrypoint.

### `service_shein_data_image_regeneration_logic.go`

Owns root SHEIN data image regeneration logic:

- SHEIN data image regeneration entrypoint.

### `service_shein_pricing_preview_entrypoint.go`

Owns root SHEIN pricing preview facade delegate:

- SHEIN pricing preview entrypoint.

### `service_shein_resolution_cache_clear_entrypoint.go`

Owns root SHEIN resolution cache facade delegate:

- SHEIN resolution cache clear entrypoint.

### `service_shein_submission_event_listing_entrypoint.go`

Owns root SHEIN submission event listing facade delegate:

- SHEIN submission event listing entrypoint.

### `service_shein_store_selection_resolvers.go`

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

### `service_process_runner_helper.go`

Owns process runner helper:

- process flow object construction,
- claim/run warning-count helpers.

### `service_task_layers_logic.go`

Owns root task layer processing logic:

- standard product layer entrypoint,
- platform adaptation layer entrypoint.

### `service_task_collaborators.go`

Owns task collaborator accessors:

- task generation,
- task revision,
- task lifecycle,
- SDS baseline helper access.

### `service_task_generation_logic.go`

Owns root task generation logic delegates:

- generation task listing and actions,
- review queue / session delegates,
- generation retry and navigation dispatch delegates.

### `service_task_generation_support_helpers.go`

Owns shared generation helpers:

- temporal platform selection bridge,
- generation retry task selection/planning helpers,
- generation task/review listing helpers.

### `service_task_revision_entrypoints.go`

Owns root task revision entrypoints:

- revision history listing and detail,
- revision apply and validation delegates.

### `service_task_lifecycle_logic.go`

Owns root task lifecycle logic delegates:

- task creation and result lookup,
- task list and SDS baseline readiness delegates,
- internal inline/enqueue lifecycle helpers used by studio/task flows.

### `service_sds_baseline_warmup_entrypoint.go`

Owns root SDS baseline warmup entrypoint:

- SDS baseline warmup entrypoint.

### `service_task_preview_logic.go`

Owns task preview logic:

- task preview fetch and projection assembly.

### `service_task_preview_payload_helper.go`

Owns task preview helper:

- preview payload construction and cookie decoration bridge.

### `service_shein_store_resolution_preview_support_helper.go`

Owns SHEIN store resolution preview helper:

- store-resolution preview decoration,
- store-resolution summary/snapshot helper values.

### `service_task_export_logic.go`

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

### `service_studio_batch_draft_session_entrypoints.go`

Owns root studio batch draft/session entrypoints:

- session gallery and batch listing,
- batch draft fetch / upsert / delete,
- async job status synchronization.

### `service_studio_media_generation_entrypoints.go`

Owns root studio media generation entrypoints:

- studio design generation entrypoints,
- studio product image generation entrypoints.

### `service_studio_media_generation_helpers.go`

Owns studio media helper delegates:

- shared media input/output helper delegates.

### `service_studio_batch_entrypoints.go`

Owns root studio batch entrypoints:

- batch detail,
- batch generation lifecycle,
- batch retry / approve actions,
- batch task creation preparation and execution.

### `service_studio_batch_run_entrypoints.go`

Owns root studio batch run entrypoints:

- create batch run,
- fetch batch run,
- list batch run items,
- cancel batch run.

Current extraction checkpoint:

- `taskStudioBatchRunService` now delegates core flow to `internal/listing/studio`
- root `listingkit` still owns service entrypoints, collaborator wiring, and repo/session adapters for this seam

### `service_task_wiring.go`

Owns explicit config builders for non-submit task collaborators:

- task generation,
- task revision,
- task lifecycle,
- SDS baseline.

This keeps accessor files thin while leaving task-specific wiring visible in one place.

### `service_studio_wiring_support.go`

Owns studio collaborator wiring plus explicit config assembly for:

- studio session,
- studio batch draft,
- studio media,
- studio batch,
- studio batch run.

Studio batch generation wiring is also routed through this seam so nested studio
collaborator construction stays visible without re-expanding accessor files.
Studio batch run coordinator/executor config also stays beside the wiring support.
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

- `service_submit_entrypoint.go`: public `SubmitTask(...)` facade entrypoint only,
- `service_submit_lease_helper.go`: shared in-flight TTL and submit-lease sentinel errors,
- `service_submit_contracts.go`: shared submit option structs, target normalization, and Temporal replay detection helpers,
- `service_submit_collaborators.go`: submit/recovery/direct/Temporal/state/execution/refresh collaborator accessors,
- `service_submit_routing.go`: thin submit/recovery/requeue/refresh routing delegates that bridge facade calls to collaborators,
- `service_submit_workflow_entry_helpers.go`: root-facing SHEIN workflow submit/gating helpers,
- `service_shein_publish_temporal_entrypoints.go`: root-facing Temporal SHEIN submit and preview entrypoints,
- `service_submit_default_action_resolver_helper.go`: root-facing default SHEIN submit-action resolver,
- `service_submit_action_normalization_helper.go`: shared preferred submit-action normalization helper,
- `service_submit_task_identity_helper.go`: shared submit task identity context helper,
- `service_submit_remote_context_helpers.go`: root-facing store info, API client, and other-api submit helpers,
- `service_submit_settings_resolution_helpers.go`: root-facing submit settings and warehouse context helpers,
- `service_submit_warehouse_selection_helper.go`: shared warehouse-code selection helper,
- `service_submit_wiring_support.go`: collaborator wiring plus config assembly helpers.

`service_submit_wiring_support.go` stays focused on collaborator wiring and
config-assembly seams, while workflow
launch/gating entry helpers live in `service_submit_workflow_entry_helpers.go` and
Temporal task-loading helpers live alongside the Temporal host collaborator surface in
`service_submit_temporal_task_loader_helper.go`.

Submission-specific checkpoint:

- refresh, requeue, immediate recovery, and batch recovery orchestrations now have generic homes under `internal/listing/submission`
- root `service_submit_routing.go` remains a delegate-only facade
- root submit collaborator files should prefer adapting to submission-domain runners over adding new orchestration loops inline

### `service_upload_logic.go`

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

## 6. Internal Representation Follow-up

The task/studio/admin collaborator groups are now the preferred internal shape.

The root `service` struct still retains legacy flat collaborator fields such as
`taskGeneration`, `taskStudioBatch`, and `settingsAdmin`, but they should now be
treated as compatibility mirrors rather than the primary ownership boundary.

Current expectation:

- grouped collaborator containers (`task`, `studio`, `admin`) are the primary internal representation,
- `...OrDefault()` accessors are the only place that should synchronize grouped fields with legacy mirrors,
- grouped dependency buckets (`taskDeps`, `studioDeps`, `adminDeps`, `submissionDeps`, `workflowDeps`, `sheinRuntimeDeps`, `supportDeps`) should own resolver state first,
- service construction should seed grouped dependency buckets first; legacy root dependency mirrors should hydrate lazily through resolvers,
- legacy dependency mirrors should live under a dedicated `mirrors` bucket instead of remaining as flat root `service` fields,
- runtime-configurable submit/workflow overrides should live in a dedicated runtime bucket instead of expanding the root service surface,
- initialization stages should trigger accessors rather than re-assign grouped fields manually,
- dependency resolvers should use shared synchronization helpers instead of open-coded root/group mirror logic,
- tests should prefer asserting grouped ownership first and mirror compatibility second.

This keeps the service object behavior-compatible while reducing the amount of
duplicated root/group synchronization logic that later Phase 4 work would
otherwise need to untangle again.

## 7. Suggested Local Validation

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

## 8. Next Phase 4 Candidates

Recommended next low-risk slices:

1. Review `service_types.go` for possible internal grouping comments or dependency buckets without changing fields.
2. Continue shrinking legacy dependency mirror reads/writes so grouped dependency containers become the obvious ownership boundary.
3. Review service accessor files to ensure each accessor remains thin and delegates to concept-specific config builders.
4. Keep `service_config.go` focused on factory wiring; avoid adding new default-building logic there.
5. Continue moving default construction helpers to concept-specific files only when the move is behavior-preserving.
6. Use [legacy-store-routing-retirement-checklist.md](/D:/code/task-processor/docs/refactoring/legacy-store-routing-retirement-checklist.md) before removing `/store-routing` compatibility surfaces.

Avoid for now:

- moving `service` into a subpackage,
- changing `ServiceConfig`,
- collapsing constructor dependencies into new public DTOs,
- changing workflow client configuration semantics.

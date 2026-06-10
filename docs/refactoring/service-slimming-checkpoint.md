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
internal/listingkit/service_submission_collaborators.go   // submission collaborator container
internal/listingkit/service_task_wiring.go               // task/generation/revision collaborator config builders
internal/listingkit/service_submit.go                     // submit facade entrypoint
internal/listingkit/service_submit_primitives.go          // shared submit TTL / sentinel errors
internal/listingkit/service_submit_contracts.go           // shared submit option structs / normalization helpers
internal/listingkit/service_submit_collaborators.go       // submit collaborator accessors
internal/listingkit/service_submit_routing.go             // thin submit/recovery/refresh routing delegates
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

### `service_task_wiring.go`

Owns explicit config builders for non-submit task collaborators:

- task generation,
- task revision,
- task lifecycle.

This keeps accessor files thin while leaving task-specific wiring visible in one place.

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
- `service_submit_routing.go`: thin submit/recovery/refresh routing delegates that bridge facade calls to collaborators,
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

# App Assembly Boundaries

## Goal

This note captures the assembly-layer boundaries that are emerging in the
`internal/app` packages. The purpose is to make future high-level refactors
more consistent: instead of treating `bootstrap`, `runner`, and `consumer` as
three unrelated places, we can now describe them with one shared assembly
vocabulary.

Use `docs/architecture/project-boundaries.md` as the repository-wide entrypoint
for default ownership and dependency direction. This document narrows that
baseline specifically for app-layer assembly vocabulary and package roles; it
should not become a parallel policy source for unrelated package placement.

## Assembly Vocabulary

The app layer is converging on this sequence:

1. `build / initialize`
2. `register`
3. `start`
4. `coordinate`

This vocabulary is intentionally higher-level than the TEMU pipeline stages.
It is meant for composition roots and runtime wiring, not domain workflows.

### 1. Build / Initialize

Responsible for:

- interpreting config
- constructing resources and services
- preparing dependency bundles

Representative examples:

- `ApplicationBootstrap.initializeServices(...)`
- `ServiceManager.buildManagedComponents()`
- `ServiceManager.buildResultReporter()`
- `ServiceManager.buildLoadMonitor()`
- `processorServiceImpl.startProcessingComponents(...)`

### 2. Register

Responsible for:

- registering components into lifecycle managers
- wiring services into shared managers
- preparing runtime-managed structures

Representative examples:

- `ApplicationBootstrap.registerLifecycleComponents(...)`
- `ServiceManager.registerComponents()`

### 3. Start

Responsible for:

- starting lifecycle-managed components
- entering running state
- activating runtime services

Representative examples:

- `ApplicationBootstrap.Start(...)`
- `ServiceManager.Start(...)`
- `processorServiceImpl.startLifecycleComponents()`

### 4. Coordinate

Responsible for:

- shutdown orchestration
- signal handling
- runtime supervision
- status monitoring

Representative examples:

- `ShutdownCoordinator`
- `ServiceManager.Start(...)` signal handling setup
- `processorServiceImpl.startStatusMonitor()`

## Package Roles

### bootstrap

`bootstrap` should act as the application composition root.

It should primarily own:

- config loading
- shared service initialization
- lifecycle registration
- top-level application start / stop boundaries

It should avoid:

- platform-specific business flow logic
- long inlined construction sequences when they can be named as initialization
  steps
- runtime coordination details that belong to specialized services
- placing concrete external-client runtime state in generic type files; keep
  shared external runtime dependencies in a named seam such as
  `internal/app/httpapi/runtime_shared_deps.go`

### runner

`runner` should act as the runtime orchestration layer for processors and
schedulers.

It should primarily own:

- starting processing components
- starting lifecycle-managed runtime pieces
- runtime status and health coordination
- processor / scheduler lifecycle control

It should avoid:

- detailed assembly logic that belongs in `bootstrap`
- domain pipeline logic that belongs in platform packages
- mixing startup sequencing with low-level implementation details when a
  dedicated startup step would clarify the flow

### consumer

`consumer` should act as the messaging runtime assembly layer.

It should primarily own:

- composing messaging-related components
- lifecycle registration for consumer-side services
- queue / HTTP / reporter / shutdown coordination

It should avoid:

- platform-specific task semantics
- deeply nested component construction in the same registration method
- leaking lifecycle orchestration details into lower-level services

## Current Structural Direction

Recent refactors have pushed the app layer toward a clearer shape:

- `bootstrap` is moving toward `load -> initialize -> register`
- `consumer` is moving toward `build -> register -> coordinate`
- `runner` is moving toward `validate -> start processing -> start lifecycle -> monitor`

These are not identical flows, but they now share the same assembly idea:

- construction and initialization happen first
- registration happens explicitly
- runtime start happens separately
- coordination concerns are not mixed into every local step

## Preferred Refactor Moves

When editing the app layer, prefer these moves:

1. split construction from registration
2. split registration from runtime start
3. keep top-level entry functions orchestration-focused
4. move repeated assembly details into clearly named builders / initializers
5. keep domain-specific workflow logic out of app-layer composition code

## Naming Guidance

At the app layer, these names are usually helpful:

- `build...`
- `initialize...`
- `register...`
- `start...`
- `coordinate...`

These names are usually less helpful when they hide the role:

- `handle...`
- `process...`
- `do...`
- `setup...`

`setup...` may be acceptable for very small generic plumbing, but when a step
has a clearer role, prefer the explicit assembly vocabulary instead.

## What Not To Do

Avoid these patterns in app-layer code:

- one method both constructing and registering several unrelated components
- one method both starting services and managing shutdown coordination
- platform wiring mixed into top-level orchestration without a named assembly
  step
- broad helpers that hide assembly order instead of clarifying it

If a refactor shortens a function but makes the assembly order harder to read,
it is usually not worth it.

## Boundary Guards

App-layer assembly boundaries are guarded by:

- `TestBusinessDomainsDoNotImportAppRuntimeAssembly`
- `TestAppBootstrapManagementClientImportsStayAllowlisted`
- `TestAppTaskManagementClientImportsStayAllowlisted`
- `TestAppRunnerManagementClientImportsStayAllowlisted`
- `TestAppConsumerManagementClientImportsStayAllowlisted`
- `TestAppHTTPAPIManagementClientImportsStayAllowlisted`
- `TestAppRuntimeListingManagementClientImportsStayAllowlisted`
- `TestAppTaskStatusManagementClientImportsStayAllowlisted`
- `TestAppHTTPAPIProductImageExternalClientImportsStayAllowlisted`
- `TestHTTPAPITypesKeepExternalClientRuntimeDepsDedicated`
- `TestHTTPAPIAdaptersKeepOpenAIAssemblyDedicated`
- `TestHTTPAPIRuntimeKeepsOpenAIRuntimeAssemblyDedicated`
- `TestHTTPAPIRuntimeKeepsSharedResourceAssemblyDedicated`

This keeps business domains from depending directly on concrete
`internal/app/{bootstrap,consumer,runner,runtime}` assembly packages. If a new
transition seam is necessary, document the narrow exception and update the
allowlist in the same change.

The app-layer management client allowlists are retirement seams, not a long-term
design target. `bootstrap`, `task`, `runner`, `consumer`, `httpapi`,
`runtime/listing`, and `taskstatus` may still assemble current
management-backed runtime dependencies, but new business data access should
prefer in-repository database/repository seams owned by the relevant domain.

The `internal/app/httpapi` package also has package-local runtime seam guards.
They keep generic files such as `types.go`, `adapters.go`, and `runtime.go`
from absorbing concrete external-client state or shared resource assembly.
When a runtime dependency needs a concrete client or bootstrap resource, prefer
a named seam such as `runtime_shared_deps.go`, `adapters_openai.go`, or
`runtime_openai.go` instead of expanding the generic assembly files. The same
rule applies to ProductImage provider/external client assembly: keep provider
selection and concrete external client wiring in dedicated seams instead of
spreading them across generic app/httpapi assembly files.

## Review Questions

When reviewing app-layer changes, these questions are useful:

1. Is this logic assembly logic or domain logic?
2. Does this entry function read like orchestration?
3. Are build / initialize / register / start / coordinate concerns still mixed?
4. Did the change make lifecycle boundaries clearer?
5. Did the change keep package roles clearer rather than blurrier?

## Working Rule

The safest app-layer rule right now is:

- `bootstrap` builds and registers
- `runner` starts and supervises
- `consumer` assembles and coordinates messaging runtime

As long as new refactors reinforce that separation, the higher-level structure
should keep getting easier to extend and reason about.

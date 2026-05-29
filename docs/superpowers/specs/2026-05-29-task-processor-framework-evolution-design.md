# Task Processor Framework Evolution Design

## Goal

Evolve `task-processor` from a growing application codebase into an internal framework-style platform with:

1. a stable application kernel
2. reusable infrastructure adapters
3. business modules that register themselves instead of being hand-wired into global bootstrap code

The immediate target is not an external open-source framework. The target is an internal platform that makes it faster and safer to add:

- a new platform module such as TikTok Shop, Shopify, or AliExpress
- a new workflow such as review, publish, sync, or asset generation
- a new runtime surface such as HTTP, worker, scheduler, or Temporal host

## Why This Change

The codebase already shows platform-like traits:

- many `cmd/*` entrypoints
- shared bootstrap and lifecycle logic
- repeated service and repository patterns
- cross-cutting worker, queue, login, and workflow infrastructure
- multiple business domains that need the same runtime services

At the same time, several hotspots show that the current architecture is still application-first rather than framework-first:

- [internal/app/httpapi/modules.go](/D:/code/task-processor/internal/app/httpapi/modules.go:54) acts as a global assembly hub for multiple business domains.
- [internal/listingkit/httpapi/bootstrap.go](/D:/code/task-processor/internal/listingkit/httpapi/bootstrap.go:38) combines repository construction, handler wiring, policy injection, and runtime setup inside one module-local bootstrap surface.
- [internal/listingkit/service.go](/D:/code/task-processor/internal/listingkit/service.go:22) is healthier than before, but it still serves as the main aggregation point for a large set of collaborators and platform-specific submit concerns.
- [internal/sheinlogin/automation.go](/D:/code/task-processor/internal/sheinlogin/automation.go:140) behaves more like a subsystem than a normal service file.

The root problem is not that these files are merely large. The root problem is that stable platform concerns and unstable business concerns still meet in the same assembly and execution surfaces.

If that continues, the project will keep paying a tax every time we add a new platform or a new workflow:

- bootstrap code keeps growing
- module boundaries stay implicit
- infrastructure upgrades affect too much business code
- “framework” behavior remains accidental instead of intentional

## Non-Goals

- Do not turn the whole repository into a public general-purpose framework.
- Do not introduce a heavyweight dependency injection framework just to make wiring look cleaner.
- Do not rewrite every domain package into a new directory layout in one pass.
- Do not force all business modules to share one universal business model.
- Do not pause feature delivery for a long-lived architecture branch.

## Root Cause

The current structure mixes three different change rates:

1. **Kernel concerns** that should change slowly:
   - app lifecycle
   - module registration
   - task execution contracts
   - workflow orchestration contracts
   - shared auth, tenant, config, and observability surfaces

2. **Adapter concerns** that should be replaceable:
   - Redis
   - RabbitMQ
   - Temporal
   - object storage
   - OpenAI
   - Playwright
   - management API

3. **Business concerns** that should evolve independently:
   - Amazon
   - SHEIN
   - ListingKit
   - SDS
   - login automation

Today these concerns are partially separated by packages, but not yet separated by composition rules. That is why “adding one more capability” often still reopens the same bootstrap surfaces.

## Recommended Direction

Evolve the project toward **kernel platformization + business modularization**, not “framework extraction by rewrite”.

The recommended architecture has three layers:

1. `kernel`
   - application runtime and extension points
2. `adapters`
   - infrastructure integrations behind stable interfaces
3. `modules`
   - business capabilities that register routes, tasks, workflows, and supporting services

This gives us a framework direction without pretending all business logic is generic.

## Prior Art And Reuse

We should reuse mature patterns instead of inventing a custom architecture stack:

- **Lightweight composition root** rather than a custom DI framework.
  - Follow the repo’s existing explicit constructor style.
  - Borrow the pattern used by `wire` or `fx` conceptually, but do not adopt either until the boundaries are stable.
- **Module registries** rather than one giant bootstrap file.
  - This is a proven pattern across web frameworks, worker hosts, and plugin-based systems.
- **Adapter interfaces** instead of direct third-party SDK usage in business services.
  - This is already partially present in the repo and should be expanded.
- **Workflow registration** similar to Temporal worker registration patterns.
  - Let modules contribute workflow/activity hosts through explicit registration instead of hidden bootstrap coupling.

What we should not do:

- do not invent a new container DSL
- do not invent a custom plugin IPC system
- do not generalize business DTOs prematurely

This follows the project instruction to prefer mature open-source implementation ideas and avoid rebuilding solved problems.

## Target Architecture

### Layer 1: Kernel

The kernel owns stable concepts that every module can rely on:

- app startup and shutdown
- config loading and validation
- lifecycle registration
- module registration and enablement
- shared HTTP, worker, scheduler, and workflow hosts
- task engine contracts
- tenant/auth/request context
- logging, metrics, and tracing hooks

The kernel must not know SHEIN submit details, SDS sync details, or crawler-specific heuristics.

### Layer 2: Adapters

Adapters wrap external systems behind stable interfaces:

- `adapters/db`
- `adapters/cache`
- `adapters/mq`
- `adapters/storage`
- `adapters/llm`
- `adapters/browser`
- `adapters/management`
- `adapters/workflow`

Business modules should depend on adapter interfaces or factory abstractions, not directly on SDK packages unless the module is itself the adapter owner.

### Layer 3: Modules

Modules express business capabilities and register themselves into the kernel:

- `modules/amazon`
- `modules/shein`
- `modules/listingkit`
- `modules/sds`
- `modules/login`

Each module can provide:

- routes
- task handlers
- workflow handlers
- scheduled jobs
- module-scoped repositories and services

The application runtime decides which modules are enabled. Modules do not control global bootstrap flow.

## Directory Direction

This is the target shape, not a required day-one move:

```text
cmd/
internal/
  kernel/
    app/
    config/
    context/
    lifecycle/
    module/
    runtime/
    task/
    workflow/
    observability/
  adapters/
    browser/
    cache/
    db/
    llm/
    management/
    mq/
    storage/
    workflow/
  modules/
    amazon/
    shein/
    listingkit/
    sds/
    login/
```

Migration rule:

- first introduce kernel and adapter packages
- then move registration and composition responsibilities
- only move existing domain directories when the new boundaries are real and already in use

## Core Interfaces

The first step toward framework behavior is a stable extension surface.

### Module

```go
type Module interface {
	Name() string
	Enabled(cfg Config) bool
	Register(app *App) error
}
```

Responsibilities:

- declare module identity
- decide whether the module is enabled for a runtime
- register routes, task handlers, workflow handlers, and startup hooks

### RouteProvider

```go
type RouteProvider interface {
	RegisterRoutes(r Router)
}
```

Responsibilities:

- own module-local HTTP route registration
- keep HTTP wiring out of global bootstrap files

### TaskHandler

```go
type TaskHandler interface {
	TaskType() string
	Validate(ctx context.Context, input any) error
	Execute(ctx context.Context, task Task) (Result, error)
}
```

Responsibilities:

- declare one task type
- validate task input
- execute one task lifecycle entrypoint

### WorkflowHandler

```go
type WorkflowHandler interface {
	WorkflowName() string
	Register(worker WorkflowWorker) error
}
```

Responsibilities:

- contribute Temporal or in-process workflow capabilities through one stable registration shape

### Adapter Registration

```go
type AdapterProvider interface {
	RegisterAdapters(registry *AdapterRegistry) error
}
```

Responsibilities:

- create or attach shared infrastructure dependencies
- keep third-party setup away from business module entrypoints

## Composition Rules

To make the architecture real, not just renamed packages, we need explicit rules:

1. `cmd/*` may select modules and runtime modes, but may not manually assemble business details.
2. `kernel/*` may depend on shared abstractions, but not on concrete business modules.
3. `modules/*` may depend on `kernel/*` and `adapters/*`.
4. `modules/*` may depend on each other only through explicit interfaces or registries, not by reaching into another module’s bootstrap internals.
5. `adapters/*` may depend on external SDKs; business services should not own those SDK lifecycles directly unless that service is the adapter.

## How Current Hotspots Map To The New Model

### 1. `internal/app/httpapi/modules.go`

Current role:

- runtime dependency building
- module assembly
- login service fallback wiring
- SDS and task RPC handler construction
- HTTP bootstrap orchestration

Target role:

- thin runtime entrypoint that asks the kernel to load enabled modules and returns the assembled app surface

What moves out:

- business-specific module builders
- local-vs-remote account provider fallback logic
- module-local route/service construction

### 2. `internal/listingkit/httpapi/bootstrap.go`

Current role:

- module-local composition root
- repository construction
- service dependency mapping
- policy and hook injection
- Temporal and auth wiring

Target role:

- module registration entrypoint plus a small set of module-scoped providers

What moves out:

- shared repository builder patterns into kernel or adapters
- auth/runtime integration into kernel services
- provider groups into dedicated module files

### 3. `internal/listingkit/service.go`

Current role:

- module facade and collaborator composition root

Target role:

- remain a module facade, but depend on smaller use-case services registered through module-scoped providers

Important note:

This file does not need another large-scale rewrite first. It already moved in the right direction. The next step is to make its collaborators easier to register through the module system.

### 4. `internal/sheinlogin/automation.go`

Current role:

- browser session lifecycle
- login page automation
- verification flow
- error artifact capture
- network and page event capture

Target role:

- split between browser adapter responsibilities and SHEIN login module responsibilities

Recommended ownership split:

- generic browser/session helpers into `adapters/browser`
- SHEIN-specific page flow and verification rules into `modules/login/shein`

## Migration Plan

### Phase 1: Introduce Kernel Registration

Goal:

- create a stable registration surface without changing business behavior

Changes:

- add `internal/kernel/module`
- add `Module`, `RouteProvider`, `TaskHandler`, and `WorkflowHandler` contracts
- add a kernel app registry that can collect module contributions
- adapt existing HTTP bootstrap to ask modules to register instead of directly wiring every business area
- create initial `module.go` files for `listingkit`, `amazon`, and `sheinlogin`

Files to prioritize:

- [internal/app/httpapi/modules.go](/D:/code/task-processor/internal/app/httpapi/modules.go:54)
- [internal/app/bootstrap/app.go](/D:/code/task-processor/internal/app/bootstrap/app.go:111)
- [cmd/product-listing-api/wrappers.go](/D:/code/task-processor/cmd/product-listing-api/wrappers.go:11)

Success criteria:

- a new module can be added through registration rather than direct global bootstrap edits
- entrypoints choose module sets, not business wiring details

### Phase 2: Convert Business Areas Into Real Modules

Goal:

- move business assembly into module-owned registration paths

Changes:

- convert `listingkit` bootstrap into module-scoped providers and registrars
- give `sheinlogin` and `sdslogin` explicit module boundaries
- let modules register worker and workflow contributions
- stop letting global HTTP bootstrap own business fallback and provider logic

Files to prioritize:

- [internal/listingkit/httpapi/bootstrap.go](/D:/code/task-processor/internal/listingkit/httpapi/bootstrap.go:38)
- [internal/listingkit/service.go](/D:/code/task-processor/internal/listingkit/service.go:22)
- [internal/app/httpapi/modules.go](/D:/code/task-processor/internal/app/httpapi/modules.go:132)

Success criteria:

- most changes for a module stay inside that module
- global bootstrap becomes orchestration instead of construction

### Phase 3: Platformize Adapters

Goal:

- isolate third-party dependencies and operational lifecycles

Changes:

- move shared SDK setup behind adapter packages
- make business modules consume adapter interfaces
- centralize observability and lifecycle hooks for external systems
- separate generic browser automation helpers from SHEIN login policy and flow

Files to prioritize:

- [internal/sheinlogin/automation.go](/D:/code/task-processor/internal/sheinlogin/automation.go:140)
- `internal/infra/*` packages that already resemble adapters

Success criteria:

- infrastructure replacement has smaller blast radius
- business tests can use adapter fakes instead of full integration stacks

## Phase 1 Detailed Task Table

1. Create `internal/kernel/module` with core registration interfaces and a registry implementation.
2. Introduce a kernel app builder that owns route aggregation, worker registration, and workflow registration.
3. Convert `internal/app/httpapi/modules.go` into a thin runtime assembly path that delegates to the kernel registry.
4. Add `module.go` entrypoints for `listingkit`, `amazon`, and `sheinlogin`.
5. Move module-local route registration out of global bootstrap code.
6. Keep existing constructors and repositories working through adapter shims rather than forcing immediate package moves.
7. Add tests that prove:
   - module registration order is deterministic
   - disabled modules do not register routes or workers
   - entrypoints can assemble the same app surface through the registry path

## Risks

### 1. Premature Generalization

If we abstract business data models too early, the “framework” will become harder to use than the current code.

Mitigation:

- only abstract extension surfaces and runtime contracts first

### 2. Hidden Coupling Between Modules

Some current business flows still rely on implicit cross-package knowledge.

Mitigation:

- expose narrow interfaces at module boundaries
- refuse “just import the other module bootstrap helper” shortcuts

### 3. Framework Work Without Delivery Value

If the migration is not tied to real hotspots, we risk architecture churn without business benefit.

Mitigation:

- prioritize files already identified as central change hotspots
- measure success by faster module additions and smaller change surfaces

## Testing Strategy

We should validate framework evolution at three levels:

1. **Kernel registration tests**
   - module enable/disable behavior
   - route/task/workflow registration behavior
   - deterministic registration order

2. **Module integration tests**
   - each module can register into the kernel without global bootstrap help
   - module-local providers can be constructed without unrelated dependencies

3. **Compatibility tests**
   - existing `cmd/*` entrypoints still expose the same routes and worker surfaces
   - current feature behavior remains unchanged after registration-based assembly

## Success Criteria

The project should be considered successfully evolving toward a framework when all of the following are true:

- adding a new module does not require reopening business logic inside global bootstrap files
- entrypoints mainly choose runtime mode and enabled modules
- most infrastructure setup is owned by adapters, not by business services
- business modules register capabilities through stable kernel contracts
- the repo still supports incremental delivery without a long-lived rewrite branch

## Recommendation

Proceed with framework evolution, but treat it as an internal platform program with strict scope:

- stabilize the kernel first
- modularize business assembly second
- adapterize infrastructure third

This direction matches the current repository shape, addresses the real root cause behind the main complexity hotspots, and avoids reinventing infrastructure that mature ecosystems already know how to solve.

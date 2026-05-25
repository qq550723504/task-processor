# ListingKit HTTPAPI Module Registrar Design

## Goal

Refactor `internal/listingkit/httpapi` so `BuildService` and `BuildModule` remain stable public entrypoints while the internal assembly path is split into module-scoped registrars with narrower dependencies.

## Why This Change

The previous `listingkit` refactor reduced business logic inside the root `service`, but `internal/listingkit/httpapi/bootstrap.go` still recombines that complexity through a single oversized composition path:

- `BuildServiceInput` is a wide dependency bag that exposes all repositories and hooks to all assembly stages.
- `buildRepositories`, `resolveModuleDependencies`, `buildListingKitServiceConfig`, and `buildHandlerOptions` all understand multiple business areas at once.
- Handler assembly, service dependency resolution, auth setup, and Temporal wiring are still coordinated from one file.

That means a new task, submit, admin, or Temporal capability still tends to reopen the same centralized bootstrap surface. The root problem is not helper count. The root problem is missing module boundaries inside the composition root.

## Non-Goals

- No change to exported `BuildService`, `BuildModule`, `BuildServiceInput`, or `BuildModuleInput` signatures in this phase.
- No new DI framework or external runtime dependency.
- No package split across directories yet.
- No behavior changes to handler registration, auth setup, or Temporal worker startup.

## Prior Art And Reuse

We do not need to introduce a new framework here. The repository already uses registrar-style composition in other HTTP bootstrap work, and mature open-source systems usually solve this with a lightweight composition root plus module-local registration rather than a giant service constructor. This phase keeps that proven pattern:

- package-local module registrars, similar to route registrars already used in the repo
- explicit dependency structs instead of a container with ambient access
- a thin composition root, similar in spirit to `fx` or `wire`, without importing either framework

That lets us reuse a known assembly style instead of inventing a custom dependency system.

## Recommended Approach

Use package-local module registrars inside `internal/listingkit/httpapi`. Each registrar owns one bounded slice of assembly and exposes only the outputs the composition root needs.

Recommended module boundaries for this phase:

1. `task` module
   - owns task repositories and task/runtime-facing handler dependencies
   - owns worker-pool-facing service dependencies that are not specific to SHEIN submit or admin CRUD
2. `submit` module
   - owns SHEIN submit hook resolution, assembler creation, image upload store, and service config needed for submit/studio flows
3. `admin` module
   - owns listing-admin repositories and handler/admin wiring dependencies
4. `temporal` module
   - owns Temporal client wiring and in-process worker startup dependencies

`bootstrap.go` should remain the composition root, but it should orchestrate registrars instead of directly constructing every concrete dependency.

## Alternatives Considered

### 1. Keep extracting more helper functions in `bootstrap.go`

This is the lowest-risk short-term path, but it only changes file shape. The same root file still knows every dependency and still grows as new business capabilities are added.

### 2. Introduce a full DI framework

This would solve composition concerns eventually, but it is too large a change for the current codebase and would add framework migration cost before the boundaries are stable.

### 3. Split registrars into new sub-packages immediately

This would look cleaner on the surface, but it risks premature package churn while the boundaries are still being discovered. Package-local registrars are a safer intermediate step.

## Design

### Composition Root

`BuildService` keeps the same role:

1. validate input
2. allocate shared closer stack
3. invoke repository registrars in the required acquisition order
4. invoke dependency registrars to resolve module-scoped runtime dependencies
5. assemble `listingkit.ServiceConfig`
6. create the `listingkit` service
7. wire Temporal clients
8. assemble the `ServiceBundle`

The difference is that steps 3 through 7 become registrar calls with explicit inputs and outputs.

### Registrar Shape

Each registrar should follow the same pattern:

- package-local file
- narrow input struct
- narrow output struct
- pure build function plus any small helper methods
- no implicit reads from unrelated repositories or hooks

Example shape:

```go
type submitModuleInput struct {
	Config            *config.Config
	Logger            *logrus.Logger
	AICredentialStore aiCredentialStore
	Hooks             BuildServiceHooks
	StoreRepository   listingadmin.StoreRepository
	ResolutionCache   sheinpub.ResolutionCacheStore
}

type submitModule struct {
	Assembler                 listingkit.Assembler
	ImageUploadStore          listingkit.ImageUploadStore
	SheinCategoryResolver     sheinpub.CategoryResolver
	SheinAttributeResolver    sheinpub.AttributeResolver
	SheinSaleAttributeResolver sheinpub.SaleAttributeResolver
	SheinPricingPolicy        sheinpub.PricingPolicy
	SheinProductAPIBuilder    sheinpub.ProductAPIBuilder
	SheinImageAPIBuilder      sheinpub.ImageAPIBuilder
	SheinTranslateAPIBuilder  sheinpub.TranslateAPIBuilder
	SheinAPIClientFactory     listingkit.SheinAPIClientFactory
	StudioImageGenerator      openaiclient.ImageGenerator
	DefaultSheinStoreID       int64
}
```

The same pattern applies to `task`, `admin`, and `temporal`.

### Repository Registration

The current repository split already preserves close-order behavior by separating early core, admin, and late core acquisition. That ordering must remain unchanged in this phase.

We should evolve the repository side toward named module outputs rather than one catch-all `builtRepositories`. The recommended direction is:

- keep `builtRepositories` as the compatibility aggregate returned by the composition root
- add module-scoped views that registrars consume
- prevent registrars from reading fields they do not need

### ServiceConfig Assembly

`buildListingKitServiceConfig` should stop receiving a mixed bag of individual dependencies. Instead, it should accept module-scoped outputs and translate them into `listingkit.ServiceConfig`.

Recommended split:

- `task` contributes `listingkit.ServiceCoreDependencies`
- `submit` contributes `listingkit.ServiceSheinDependencies`
- `asset/task` shared inputs continue contributing `listingkit.ServiceAssetDependencies`
- `temporal` continues to wire workflow clients after service construction

This keeps `ServiceConfig` mapping explicit while reducing cross-module knowledge.

### Handler And Runtime Assembly

`buildHandlerOptions` and `buildModuleRuntime` should follow the same boundary rules:

- admin handler dependencies come from the `admin` registrar output
- subscription/runtime-facing task dependencies come from the `task` registrar output
- Temporal worker startup depends only on the `temporal` service surface

That prevents admin CRUD additions from reopening submit or Temporal assembly code.

## Error Handling

- Preserve current fail-fast behavior.
- Preserve closer stack semantics and reverse-order cleanup on failure.
- Registrar errors should be wrapped with the module name so failure origin is obvious.

## Testing Strategy

Add or keep tests at the composition level, not just helper-unit level:

- repository acquisition order and cleanup behavior
- registrar output mapping into `listingkit.ServiceConfig`
- bundle assembly behavior
- Temporal client wiring behavior
- handler option mapping behavior for admin and subscription dependencies

Each new registrar should have at least one focused test that proves:

- it resolves the expected dependency graph
- it does not need unrelated dependencies

## Success Criteria

- `bootstrap.go` becomes an orchestration file instead of the place that knows every concrete dependency.
- New SHEIN submit wiring changes can usually stay inside the submit registrar.
- New admin repository or handler wiring changes can usually stay inside the admin registrar.
- `BuildServiceInput` remains externally stable for now, but internal assembly stops treating it as a universal dependency container.

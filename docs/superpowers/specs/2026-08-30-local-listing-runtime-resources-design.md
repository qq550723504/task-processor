# Local listing runtime resource ownership

## Context

`internal/listingruntime/local/LocalDataProvider` is not a fallback implementation. It is the production composition object created by `BuildSharedResources`. It currently combines three responsibilities:

1. Opens and closes the GORM database and Redis clients.
2. Creates and exposes twelve concrete repositories.
3. Implements store, rules, quota, import-task, product-data, mapping, inventory, and raw-JSON APIs consumed by `LocalRuntime` and its adapters.

This makes every local adapter depend on the same broad object. It also hides lifetime management: `LocalDataProvider.Close` exists, but `SharedResources` does not retain or invoke it after building the production runtime.

The already completed SHEIN login-worker change is deliberately outside this ownership graph: it now creates only a database-backed `StoreAPI` and closes that database with the worker runtime.

## Goal

Make resource ownership explicit at the local runtime composition boundary, then migrate adapters to narrow dependencies until `LocalDataProvider` can be removed without changing its observable behavior.

## Non-goals

- Do not rewrite repository implementations or database schemas.
- Do not move unrelated packages or alter public HTTP/API contracts.
- Do not remove `LocalDataProvider` in the first migration.
- Do not change cookie-provider, daily quota, or nil-configuration behavior as a side effect.

## Proposed shape

Introduce `local.RuntimeResources` as the sole local-runtime infrastructure owner. It owns the GORM database client, the Redis client, and construction of the concrete repositories. It exposes narrow accessors for the concrete resources needed by local adapters and has one idempotent `Close()` lifecycle method.

`LocalRuntime` receives `*RuntimeResources` plus its optional `SheinCookieProvider`. Its methods use only the repositories or clients they need; adapters are progressively changed from `*LocalDataProvider` to the relevant repository/client interfaces. The bootstrap root retains `RuntimeResources` through `SharedResources` and closes it when the process exits or when a resource build fails after opening it.

During migration, `LocalDataProvider` becomes a compatibility facade backed by `RuntimeResources`. It no longer creates connections or repositories itself. This is temporary and exists only to preserve callers that have not yet been migrated. New production code must not construct it.

```
BuildSharedResources
  -> RuntimeResources (DB, Redis, repositories, Close)
  -> LocalRuntime (narrow runtime operations + cookie provider)
  -> adapter APIs / scheduler / processor

Legacy callers -> LocalDataProvider facade -> RuntimeResources (temporary)
```

## Dependency rules

- Only bootstrap creates `RuntimeResources` from configuration.
- Only the owner (`SharedResources` or a command-specific runtime) closes opened infrastructure.
- `LocalRuntime` must not expose the resources aggregate; it exposes domain APIs or the few existing repository ports required by legacy scheduler/processor integrations.
- Local adapters receive the smallest stable dependency that implements their work. For example, raw JSON receives `listingadmin.RawJsonDataAPI`; task RPC receives `*gorm.DB`; a store adapter receives a store repository plus its cookie/pause collaborators.
- Tests may create `RuntimeResources` from injected DB/Redis dependencies; no test should require a listening production Redis instance to exercise a DB-only adapter.

## Migration order

1. Add `RuntimeResources`, make bootstrap own and close it, and preserve `LocalDataProvider` as a forwarding facade. Verify failed construction closes any already-open client.
2. Migrate health validation and task RPC to the explicit resources/database dependencies. These consumers have small, easily asserted contracts.
3. Migrate raw JSON, store, rules, product data, mapping, inventory, and import-task adapters by capability group. Each group receives focused behavior tests before implementation.
4. Narrow scheduler and processor bridge interfaces where they currently leak concrete repositories. Preserve externally consumed methods until all bridges have migrated.
5. Delete the compatibility facade and the old `NewLocalDataProvider` constructor only after no production or test caller references them.

## First implementation slice

The first slice is deliberately limited to resource ownership, lifecycle, health validation, and task RPC. It will not migrate every API adapter. Acceptance criteria:

- `BuildSharedResources` no longer calls `NewLocalDataProvider`.
- A successful production runtime owns its database/Redis cleanup path.
- A partially initialized resource owner closes already-open resources on failure.
- Health validation returns the current field names and readiness semantics.
- `LocalTaskRPCProvider` accepts a database dependency directly and preserves its nil/error behavior.
- Existing local runtime and bootstrap tests pass, plus focused lifecycle tests.

## Risks and safeguards

The largest risk is an accidental semantic change from coupling behavior to the aggregate object. We mitigate this by keeping a forwarding facade during migration, using existing contract tests, and moving one capability group per change. A second risk is double-closing clients when compatibility and bootstrap coexist; therefore the owner is unique and `Close` is idempotent. A third risk is broad test interference from external local services; integration tests must use dynamically reserved loopback ports and detect early server startup failure.

## Verification

Every slice runs its package tests, the affected bootstrap/command tests, `git diff --check`, and a search proving the intended production constructor/callers have migrated. Full-repository tests are a separate signal and must not be claimed green unless they complete in this worktree.

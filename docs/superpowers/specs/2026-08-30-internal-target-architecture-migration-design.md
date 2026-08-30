# Internal target architecture migration

## Context

The repository has an approved modular-monolith target, but the current
`internal` tree is still a migration layout rather than a stable package
architecture. The current Windows/default build contains 303 Go packages under
59 top-level `internal` directories. The main structural hotspots are:

- `internal/listingkit` remains the primary complexity sink. Its root package
  contains 560 production files and directly imports 55 internal packages.
- `internal/listingadmin` combines models, DTOs, repository ports, GORM
  implementations, handlers, and management compatibility APIs for unrelated
  capabilities.
- infrastructure is split across `internal/core`, `internal/infra`, target
  placeholders under `internal/platform`, and concrete clients inside historical
  marketplace packages.
- marketplace registration is split across `platform`, `platforms`,
  `platformbase`, and `platformtask`; `internal/platforms/shein/module.go` also
  owns scheduler, prompt, watchdog, and sharding assembly.
- relative default log paths and lazy global logger initialization create
  `tmp/logs` directories beneath package working directories during tests.
- existing guards prevent several known imports, but some structure guards
  enforce file allowlists rather than actual responsibility or dependency
  boundaries.

The migration is one architecture initiative completed on one governance
branch and merged once. Implementation uses a sequence of reviewable commits so
failures can be located and a single incomplete slice can be reverted without
undoing verified lower layers.

## Goal

Converge the complete `internal` tree to the approved business-domain-first
target architecture, retire ambiguous legacy roots, and enforce the target
dependency direction in code and tests:

```text
cmd
  -> app
  -> listing / product / marketplace
  -> platform / integration
  -> shared
```

The migration is complete only when obsolete package roots have no repository
callers and are deleted. Moving files without correcting ownership or dependency
direction does not satisfy the goal.

## Non-goals

- Do not split the modular monolith into services.
- Do not redesign product behavior merely because code is being moved.
- Do not pursue directory symmetry when two marketplaces genuinely need
  different capabilities.
- Do not preserve defective internal APIs indefinitely for source compatibility.
- Do not create a second dependency-analysis framework; extend the repository's
  existing `golangci-lint` depguard rules and Go architecture tests.
- Do not combine an external protocol redesign with an unrelated package move.

## Migration principles

1. Correct defective internal design and migrate all repository callers.
2. Preserve observable behavior with characterization and contract tests before
   each move.
3. When an external HTTP, queue, Temporal, configuration, or database contract
   must change, provide versioning, dual-read/write, or an explicit migration in
   the same slice. Never make an unannounced destructive cutover.
4. Compatibility packages may translate and forward only. They must not own
   business decisions, persistence, external-client construction, or runtime
   lifecycle.
5. A compatibility entrypoint must have a named replacement, a repository-wide
   zero-reference condition, and a deletion checkpoint.
6. New code enters only the target owner. Legacy packages are migration sources,
   not valid destinations for new responsibilities.
7. Each commit leaves the repository buildable and keeps the applicable focused
   tests green.

## Target package ownership

### `internal/shared`

Owns small, stable primitives with no product or marketplace meaning, such as
generic errors, time helpers, pagination, validation primitives, and identity
envelopes that are genuinely shared. It is not a replacement for `pkg`, `core`,
or another catch-all directory.

### `internal/platform`

Owns application infrastructure: configuration loading, logging, metrics,
database bootstrap, Redis, queues, Temporal runtime support, and object storage.
It exposes infrastructure capabilities to app assembly, not business services
to domains.

### `internal/integration`

Owns adapters for systems outside the application: OpenAI, image providers, S3,
Playwright, marketplace APIs, and crawler/browser adapters. Provider SDK types
and concrete clients stay inside integration packages.

### `internal/product`

Owns canonical catalog facts, assets, product images, enrichment, and normalized
sourcing. It does not depend on listing orchestration, marketplace workspaces,
HTTP assembly, or concrete integrations.

### `internal/marketplace/<name>`

Owns marketplace-specific publishing, workspace/editor behavior, models,
validation, pricing, category, attribute, and payload policies. Runtime clients
are integration adapters. Process startup, scheduler lifecycle, and worker
registration remain app responsibilities.

### `internal/listing`

Owns cross-marketplace listing task lifecycle, workflow, preview, export,
revision, submission, studio, settings, and subscription behavior. It may use
product facts and marketplace capabilities through narrow contracts, but must
not absorb marketplace rules.

### `internal/app`

Owns process assembly for HTTP servers, workers, schedulers, consumers, and
Temporal workers. It constructs concrete platform and integration resources,
adapts them to local domain interfaces, registers routes and workers, and owns
resource lifecycle. It contains no marketplace decision rules.

### `internal/compatibility`

Owns only externally necessary legacy entrypoints and DTO translation during a
bounded cutover. Repository-internal compatibility is removed by migrating all
callers rather than by preserving forwarding aliases.

## Retired root policy

The following generic or overlapping roots are migration sources and are not
long-term owners:

- `internal/listingkit` and `internal/listingadmin`
- `internal/core`, `internal/infra`, `internal/pkg`, `internal/model`, and
  `internal/domain`
- `internal/platforms`, `internal/platformbase`, and `internal/platformtask`
- historical marketplace implementation roots such as `internal/shein`,
  `internal/temu`, and `internal/amazon`
- overlapping listing roots whose responsibilities move into `internal/listing`

Retirement is capability-based, not a blind directory rename. A file moves only
after its owner, dependency contract, callers, and behavior tests are known.

## Dependency and construction rules

- Domain packages define the smallest interfaces they consume.
- App assembly selects concrete implementations and injects them.
- A business pipeline never receives the global application configuration when
  a focused options value is sufficient.
- A business pipeline never constructs an OpenAI, database, queue, object-store,
  browser, or marketplace API client.
- Marketplace packages expose explicit constructors or services with narrow
  dependencies; they do not implement interfaces whose method signatures force
  them to import app runtime types.
- App may import marketplace packages. Marketplace packages must not import app.
- Platform and integration packages must not import listing, product, or
  marketplace business implementations.
- Compatibility may depend inward on target domains. Target domains must never
  depend back on compatibility.

The existing platform module registry will therefore be replaced by explicit
app-owned assembly. Marketplace packages may expose descriptors and constructors,
but scheduler, watchdog, prompt-store, sharding, and worker lifecycle remain in
app builders.

## Migration sequence

### Phase 1: baseline and runtime side effects

1. Add a failing filesystem-level regression test proving package tests must not
   create logs beneath `internal`.
2. Make the logger's library/test default stdout-only and require app startup to
   opt into an explicit file path under the repository runtime root or another
   configured absolute runtime directory.
3. Replace test file outputs with `t.TempDir` where file logging is under test.
4. Strengthen the repository artifact guard so it inspects the filesystem state
   relevant to a test run rather than only Git-tracked paths.

Existing ignored artifacts are reported separately. The implementation must not
delete user workspace data without explicit authorization.

### Phase 2: shared, platform, and integration foundations

1. Classify current `core`, `infra`, and technical `pkg` capabilities.
2. Move configuration, logging, metrics, database, Redis, queue, Temporal, and
   object-storage runtime ownership into `internal/platform`.
3. Move concrete provider clients and crawler adapters into
   `internal/integration`.
4. Update app construction and lifecycle ownership before migrating business
   callers.
5. Add depguard rules that block business imports of concrete adapter packages.

### Phase 3: product domain

1. Consolidate catalog, asset, product image, enrichment, and sourcing under
   `internal/product`.
2. Introduce or retain local ports at the product boundary and migrate callers.
3. Remove product-facing aliases and generic legacy models after zero-reference
   scans pass.

### Phase 4: marketplace domains

1. Migrate stable policies and payload models before runtime-heavy pipelines.
2. Separate marketplace API adapters into integration packages.
3. Replace pipeline client construction with injected local interfaces and
   focused option values.
4. Move historical SHEIN, TEMU, Amazon, and Walmart behavior to their marketplace
   owners by capability.
5. Keep runtime assembly in app, not under marketplace registration files.

### Phase 5: listing domain

1. Split `listingadmin` capabilities into their actual listing, product, or
   marketplace owners; migrate repository ports and implementations separately.
2. Move ListingKit task, workflow, preview, export, revision, submission, studio,
   settings, and subscription responsibilities into `internal/listing`.
3. Keep only externally necessary compatibility adapters during the cutover.
4. Delete the `listingadmin` and `listingkit` roots after production and test
   references reach zero.

### Phase 6: app assembly and entrypoint cutover

1. Replace `internal/platforms/*` registration with app-owned builders that call
   marketplace constructors through narrow dependencies.
2. Switch HTTP, consumer, worker, scheduler, Temporal, and command entrypoints.
3. Delete platform-registration and remaining legacy assembly roots.

### Phase 7: repository closure

1. Remove retired directories, aliases, DTOs, and temporary compatibility code.
2. Update stable architecture documents and their document tests.
3. Convert migration allowlists into permanent target-direction denials.
4. Run repository-wide validation and record any environment-dependent suites
   that cannot complete.

## Test and verification strategy

Every capability move follows this sequence:

1. Add or identify characterization tests for current observable behavior.
2. Add a failing architecture or contract test for the defective boundary.
3. Introduce the target owner and narrow contract.
4. Migrate production callers and then test callers.
5. Prove the old import or constructor has zero repository references.
6. Delete the old implementation or reduce an externally required path to a
   translation-only adapter.
7. Run focused package tests, reverse-dependency tests, architecture tests,
   `git diff --check`, and the applicable lint/build commands.

HTTP, RabbitMQ, Temporal, configuration, and database boundaries receive
contract tests independent of package locations. Full-repository test status is
reported only when the full command completes successfully.

Existing depguard rules remain the primary import enforcement mechanism. Go AST
tests are reserved for semantic constraints depguard cannot express, such as
forbidden construction calls or compatibility declarations.

## Monotonic migration guards

Before extraction begins, record production-file and internal-dependency
baselines for the largest legacy packages. During migration:

- `internal/listingkit` and `internal/listingadmin` production file counts may
  only decrease.
- their direct internal dependency sets may only shrink.
- no new package may import a root scheduled for retirement.
- no new exported declaration may be added to a compatibility package without
  an explicit external-contract justification.

These are migration ratchets, not permanent arbitrary package-size thresholds.
They are removed when the legacy package is deleted.

## Commit and rollback model

- Each commit represents one behavior-preserving capability move or one explicit
  contract migration.
- Dependency-foundation commits precede domain commits; domain commits precede
  app cutover commits.
- A failing slice is reverted independently. Verified lower-layer commits remain.
- Temporary adapters are introduced and removed in named commits within this
  initiative; they do not become an indefinite second architecture.
- The governance branch is merged once after all phases satisfy completion
  criteria.

## Risks and safeguards

### Hidden behavior in giant packages

File names do not prove ownership. Each move starts from callers, state changes,
and behavior tests rather than directory classification alone.

### Import-cycle pressure

Interfaces live with consumers and concrete construction lives in app. If a move
creates a cycle, the boundary is redesigned rather than placing a shared DTO in a
generic package to bypass the cycle.

### External protocol breakage

Protocol changes are isolated from unrelated moves and require explicit
versioning or migration coverage. The old version is removed only after its
cutover condition is met.

### Long-running branch drift

Commits remain small, target ownership is documented before each phase, and the
branch regularly rebases or merges the repository's mainline according to the
team's normal workflow. Architecture guards prevent new legacy dependencies
during the migration.

### False confidence from structure tests

File allowlists are not accepted as proof of cohesion. Guards check dependency
direction, forbidden construction behavior, zero-reference retirement, and
runtime filesystem effects.

## Completion criteria

The initiative is complete when:

- target packages own all production behavior described in this design;
- retired roots and repository-internal compatibility paths are deleted;
- app is the only runtime composition owner;
- business packages do not construct or expose concrete infrastructure clients;
- external contracts are either unchanged or have a tested migration path;
- source-directory tests no longer create runtime artifacts;
- target dependency direction is enforced by depguard and focused semantic
  tests;
- default builds, focused domain suites, architecture tests, lint, and the
  repository-wide test command complete successfully, or any environment-only
  exclusions are explicitly documented with evidence;
- stable architecture documents describe the resulting structure rather than
  the migration state.

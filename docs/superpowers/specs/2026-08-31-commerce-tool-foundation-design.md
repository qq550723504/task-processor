# Phase 2A Commerce Tool Foundation Design

## 1. Status and decision

Status: approved for design write-up on 2026-08-31. Implementation planning
starts only after this document is reviewed.

The chosen direction is to establish a small, framework-neutral Commerce Tool
contract before Product Agent implementation, then prove that contract through
one real read-only vertical slice. The Tool boundary will reuse the current
authorization, verified identity, AI capability governance, tracing, workflow,
and schema libraries. It will not create a second permission system, AI model
router, retry engine, product model, or workflow runtime.

The first real tool is a canonical-product inspection tool. It reads through a
narrow application/domain service, returns source lineage separately from
canonical facts, and computes only deterministic evidence diagnostics. It does
not mutate product state, call a paid model, prepare a marketplace payload, or
publish anything.

Implementation must be sequenced after the in-progress target-architecture
Phase 2 work is merged. That work currently changes `go.mod`, configuration,
`internal/app/httpapi`, feature-flag composition, and platform ownership. A
parallel Tool runtime change against the old base would create avoidable merge
conflicts and would encourage new dependencies on packages that are being
retired.

## 2. Context

The authoritative Phase 2A requirements are recorded in:

- `docs/product/ai-commerce-agent-platform-strategy.md`;
- `docs/refactoring/current-refactoring-status.md`;
- GitHub issues #128, #133, and #134;
- `docs/superpowers/specs/2026-08-06-ai-capability-agent-platform-design.md`;
- `docs/superpowers/specs/2026-08-13-agent-roadmap-authority-alignment-design.md`.

Those sources require a single Tool contract with:

- stable tool identity and version;
- input and output schemas;
- read, compute, propose, write, and publish risk declarations;
- tenant/user permission requirements;
- timeout, retry, idempotency, and side-effect ownership;
- cost and usage metadata;
- deterministic errors;
- audit and trace metadata;
- exact Agent allowlist binding.

Only read, compute, and propose tools may execute during Phase 2A. Write and
publish remain disabled. Agent code cannot access GORM repositories, provider
SDKs, marketplace clients, or framework-specific DTOs.

## 3. Repository evidence and reuse decisions

### 3.1 AI capability governance already exists

`internal/aicapability` already owns:

- provider-neutral AI capabilities and operations;
- model catalog and tenant model policy;
- routing, fallback, routing modes, and execution plans;
- tenant, user, business-task, trace, and idempotency metadata;
- model invocation records, usage, cost, latency, hashes, and outcomes;
- AI-specific normalized errors, including budget, Agent step limit, and Tool
  denial categories.

Commerce Tool must not duplicate those behaviors. An AI-backed Tool executor
delegates model selection, credentials, provider retries, cost calculation, and
model invocation recording to `internal/aicapability`. A deterministic Tool is
not recorded as a fake model invocation.

### 3.2 Verified identity and Casbin already exist

`internal/authidentity` stores the verified tenant, user, and roles in context
and synchronizes tenant/user scope into `internal/shared/aiidentity`.
`internal/authz` uses Casbin for ListingKit permissions. ListingKit repository
reads already fail closed on tenant and owner visibility.

Commerce Tool therefore uses injected principal and authorization ports. The
production adapters resolve only verified context identity and delegate policy
decisions to the existing Casbin authorizer. Tool JSON input never accepts
tenant ID, user ID, or roles as authority.

### 3.3 Schema and observability libraries already exist

The repository already locks:

- `github.com/santhosh-tekuri/jsonschema/v6` through `kin-openapi`;
- OpenTelemetry API and HTTP instrumentation;
- `golang.org/x/mod`, whose `semver` package can validate exact Tool versions.

The implementation should promote direct dependencies only when production
code begins importing them. It must not add a second JSON Schema validator,
custom semver parser, or tracing framework.

### 3.4 Workflow and feature-flag ownership already exist

Temporal remains the owner of durable business execution and workflow-level
retries. The target-architecture Phase 2 branch is introducing an isolated
OpenFeature runtime for feature evaluation. Product Agent rollout and tenant
allowlisting reuse that runtime in Phase 2B. Tool Registry does not become a
workflow engine or a second feature-flag implementation.

### 3.5 Existing product facts are the source of truth

The repository already has:

- `canonical.Product` and field traces;
- neutral `product/sourcing.SourceEnvelope`;
- `catalog.ProductFacts` and `asset.Facts`;
- persisted ListingKit source lineage outside canonical product facts;
- ProductEnrich and ProductImage service-facing interfaces.

Tool adapters project these existing values into versioned outputs. They do not
create an Agent-owned canonical product, source envelope, asset catalog, or
marketplace rule store.

## 4. Backlog and sequencing constraints

The whole of #134 cannot honestly be completed as one initial change:

- source evidence depends on the still-open #30 controlled 1688 loop;
- marketplace readiness depends on the still-open #34 contract;
- ProductEnrich and ProductImage propose tools semantically depend on the
  release-level AI Capability evidence in #130, even though #134 does not yet
  state that dependency;
- the target-architecture migration prohibits adding new production importers
  of retiring root packages such as `internal/listingkit`.

The correct sequence is:

1. merge target-architecture Phase 2;
2. implement the framework-neutral #133 contract and registry;
3. add one real canonical-product inspection slice through a target-direction
   service port;
4. add source/facts tools after #30 and the relevant product ownership move;
5. add ProductEnrich/ProductImage propose tools after #130 evidence;
6. add marketplace readiness/rule tools after #34 and marketplace ownership
   are stable.

Issue #134 should record #130 as a dependency before AI-backed propose tools are
claimed complete. This design does not mutate GitHub issue state.

## 5. Goals

- Give fake and future real Agent runtimes the same stable Tool contract.
- Make every executable Tool's owner, schemas, risk, permission, side effects,
  timeout, retry owner, idempotency, and usage owner explicit.
- Ensure a model can supply only business arguments, never execution authority.
- Validate both Tool input and output at the server boundary.
- Bind Agent definitions to exact Tool IDs and versions.
- Keep write and publish unavailable throughout Phase 2A.
- Reuse domain services and existing governance rather than exposing internal
  storage or clients.
- Make the first real Tool independently testable without a paid model.
- Preserve the target package dependency direction during directory migration.

## 6. Non-goals

- Implement Product Agent reasoning or an Agent framework.
- Introduce Eino, Google ADK, LangGraph, or another Agent runtime in Phase 2A.
- Replace fixed ProductEnrich, ProductImage, RabbitMQ, or Temporal flows.
- Build a dynamic plugin loader or runtime Tool installation mechanism.
- Expose a generic public HTTP Tool execution endpoint.
- Add write, save-draft, publish, or arbitrary network tools.
- Persist Agent chain-of-thought or raw sensitive Tool payloads.
- Complete #30, #34, #130, or all of #134 inside the registry change.
- Move product/listing packages as an incidental part of Tool implementation.

## 7. Ownership and dependency direction

The permanent contract belongs to a dedicated package:

```text
internal/commercetool
```

It is not placed in `internal/kernel/module` because the kernel registry is a
startup composition collector for routes, worker pools, Temporal workers, task
handlers, and workflow names. Tool lookup, authorization, schema enforcement,
risk policy, and execution auditing are runtime security behavior. Adding them
to the kernel registry would turn it into a cross-domain god registry.

The dependency direction is:

```text
Agent Runtime (Phase 2B)
  -> commercetool Registry / BoundToolSet
      -> owner-provided Tool adapter
          -> narrow domain/application service port
              -> existing domain implementation

app composition
  -> constructs owner adapters
  -> constructs authorization / identity / audit adapters
  -> creates immutable Registry

AI-backed owner adapter
  -> existing aicapability governance
  -> existing ProductEnrich or ProductImage service interface
```

`internal/commercetool` must not import:

- an Agent framework;
- Gin, Temporal, RabbitMQ, or GORM;
- provider SDKs;
- marketplace clients;
- ListingKit implementation packages;
- product, asset, or marketplace DTO packages.

Owner adapters live with their target business owner, for example under future
`internal/product/.../tools`, `internal/marketplace/.../tools`, or
`internal/listing/.../tools`. App composition may import both the contract and
owners but contains no Tool translation or business rules.

No new adapter is added under root `internal/listingkit`, and no new Tool package
imports that retiring root. If a narrow target-direction read service is not
available, the real Tool slice waits for or extracts that service rather than
creating a permanent compatibility dependency.

## 8. Contract model

The exact Go layout may be refined during implementation, but the public
semantics are fixed by this design.

### 8.1 Tool identity

A Tool definition contains:

```go
type ToolRef struct {
    ID      string
    Version string
}

type Definition struct {
    Ref           ToolRef
    Capability    string
    Owner         string
    Description   string
    InputSchema   json.RawMessage
    OutputSchema  json.RawMessage
    Risk          RiskLevel
    Permission    PermissionRequirement
    SideEffects   SideEffectPolicy
    Idempotency   IdempotencyPolicy
    Timeout       TimeoutPolicy
    Retry         RetryPolicy
    Usage         UsagePolicy
}
```

`ID` is a stable semantic name such as `product.canonical.inspect`. `Version`
is an exact semantic version with a leading `v`, validated through
`golang.org/x/mod/semver`. `Capability` is a commerce-domain grouping such as
`product.canonical`; it is not a duplicate of an AI provider capability.

An AI-backed adapter continues to use the real `aicapability.Capability` and
`aicapability.Operation` internally. Commerce Tool does not recreate provider,
model, routing, pricing, or prompt types.

### 8.2 Risk levels

Definitions recognize all long-term risk values:

```text
read
compute
propose
write
publish
```

The Phase 2A execution policy has a hard-coded ceiling of `propose`. Registering
a complete definition does not make it executable. Binding or invoking `write`
or `publish` fails with a deterministic policy error. Enabling those risks in a
later phase requires a new reviewed policy and cannot be achieved through Tool
arguments or Agent output.

Risk semantics are:

- `read`: reads existing facts through an authorized domain service;
- `compute`: pure or deterministic computation over supplied/read facts;
- `propose`: may call governed AI and return a candidate, but does not apply it;
- `write`: mutates local business state;
- `publish`: creates or changes externally visible marketplace state.

### 8.3 Side effects and idempotency

Every definition declares both fields; omission is a registration error.

Phase 2A executable Tools declare no business mutation. A `propose` Tool may
consume governed AI capacity, so it must declare the AI capability ledger as
the usage owner and require an idempotency key when the underlying governed
operation requires one.

The minimum idempotency modes are:

```text
not_applicable
deterministic
required_key
```

The Tool boundary does not implement a second idempotency store. It validates
the declaration and key, passes the key to the owning service, and relies on
that service's existing effect/ledger semantics.

### 8.4 Timeout and retry ownership

Every Tool has an end-to-end hard timeout enforced by the Tool invoker. Nested
owners may use shorter timeouts but cannot extend the Tool deadline.

Every definition declares one retry owner:

```text
none
caller
ai_capability
domain_workflow
```

The registry never automatically retries an executor. Deterministic compute
normally uses `none`; safe reads may use `caller`; AI provider attempts use
`ai_capability`; durable business retries remain with Temporal or the owning
workflow. A single failure scope cannot name two retry owners.

### 8.5 Cost and usage ownership

Usage metadata declares whether a Tool is:

- unmetered deterministic/read work;
- metered by the existing AI capability ledger;
- metered by another named existing domain ledger.

It does not contain provider pricing logic. AI-backed executors link their
child model invocations to Tool call ID and Agent run ID and let
`internal/aicapability` record model, policy, prompt, tokens, images, latency,
and cost.

## 9. Model-visible arguments versus trusted call metadata

The model-visible Tool schema covers only business arguments. For the first
Tool that is only:

```json
{"task_id":"..."}
```

The runtime constructs trusted call metadata separately:

```go
type CallMetadata struct {
    CallID        string
    AgentID       string
    AgentVersion  string
    AgentRunID    string
    BusinessTaskID string
    TraceID       string
    IdempotencyKey string
}
```

Tenant, user, and roles are resolved from verified context through a
`PrincipalResolver`. They are not fields in model arguments or output. The
future durable Agent runtime must restore a server-owned verified principal
from its Agent run record before calling a Tool; it must not trust values
generated by a model.

This separation is enforced in APIs and tests. JSON decoding of Tool arguments
must reject undeclared properties so smuggled `tenant_id`, `user_id`, `roles`,
`permission`, or `tool_version` values cannot become authority.

## 10. Authorization model

The core contract defines small ports rather than importing Casbin or an HTTP
identity package:

```go
type PrincipalResolver interface {
    ResolvePrincipal(context.Context) (Principal, error)
}

type Authorizer interface {
    Authorize(context.Context, Principal, PermissionRequirement) error
}
```

Production adapters:

- resolve tenant, user, and roles from `authidentity`/trusted restored context;
- delegate permission evaluation to the existing Casbin authorizer;
- never accept identity fallback from Tool arguments;
- fail closed on missing or partial identity.

Authorization happens twice at different responsibilities:

1. Tool Invoker verifies Agent allowlist, risk, identity, and declared
   permission.
2. The owner service/repository retains its tenant/owner visibility checks.

The second check is not redundant. It prevents an incorrectly broad Tool
permission from bypassing record ownership.

The canonical inspection tool initially reuses the existing ListingKit read
permission rather than inventing a parallel Agent role model. A new
commerce-wide permission should be introduced only when a concrete non-
ListingKit product read path needs different access semantics.

## 11. JSON Schema enforcement

Both input and output schemas are required and compiled when the registry is
constructed. Invalid or unsupported schemas prevent startup/registry creation.

The implementation uses `github.com/santhosh-tekuri/jsonschema/v6` and follows
these rules:

- one supported JSON Schema draft for every Tool;
- object inputs and outputs default to `additionalProperties: false`;
- input is validated before executor decoding;
- output is validated before it is returned to Agent Runtime;
- schema errors are normalized and do not leak Go type names or sensitive raw
  payloads;
- focused fixtures prove accepted and rejected shapes;
- typed adapter tests detect drift between Go DTO serialization and schema.

Schema generation is not introduced in the first slice. Hand-authored minimal
schemas are clearer for one Tool and avoid adding a second generator dependency.
If repetition becomes measurable, generation can be evaluated separately with
contract-diff tests.

## 12. Registry and Agent binding

The registry is immutable after construction. It is created from a finite list
of complete `Tool` values, each containing a Definition and Executor.

Construction fails on:

- empty or malformed ID/version/capability/owner;
- duplicate exact Tool references;
- missing or invalid schemas;
- missing permission, risk, side-effect, idempotency, timeout, retry, or usage
  declarations;
- a nil executor;
- inconsistent declarations, such as unmetered usage for an explicitly
  AI-governed propose Tool.

An Agent definition contains an exact allowlist:

```go
type AgentDefinition struct {
    ID           string
    Version      string
    AllowedTools []ToolRef
}
```

Binding validates every reference and returns a `BoundToolSet`. Agent Runtime
receives only that set; it cannot enumerate or invoke executors from the global
registry. Definition inspection may expose safe metadata, but no public API
returns an unguarded executor.

The Product Agent allowlist names exact versions. A new Tool version does not
silently replace the version used by an existing Agent definition.

## 13. Invocation order

The `BoundToolSet` invocation path is:

1. validate call metadata and exact Agent identity;
2. resolve the Tool only from the bound allowlist;
3. enforce the Phase 2A risk ceiling;
4. resolve the verified principal;
5. authorize the declared permission;
6. validate idempotency requirements;
7. validate JSON input schema;
8. derive an end-to-end timeout context;
9. start an OpenTelemetry span and emit safe call metadata;
10. execute the owner adapter exactly once;
11. normalize any owner error;
12. validate JSON output schema;
13. record terminal Tool audit metadata and return the structured result.

The invoker never retries step 10. Recorder/exporter failure after execution
must not cause automatic executor replay. It is surfaced through observability
and a safe audit-status field while preserving the already produced result.
Future write/publish auditing requires a separately designed durable begin/
commit protocol before those risk levels can be enabled.

## 14. Deterministic Tool errors

Tool errors are broader than AI provider errors. Forcing every deterministic
domain read into `aicapability.ErrorCategory` would incorrectly make the AI
control plane own non-AI failures. Commerce Tool therefore defines a small
boundary taxonomy:

```text
invalid_input
identity_integrity
permission_denied
tool_not_allowed
not_found
failed_precondition
conflict
deadline_exceeded
dependency_unavailable
output_invalid
budget_exceeded
unknown_execution_state
internal
```

Each code has a fixed retryability rule; adapters cannot arbitrarily label an
internal or permission error retryable. Errors returned to the model contain a
safe code and message, never provider payloads, credentials, SQL errors, or raw
marketplace responses.

AI-backed adapters deterministically map existing `aicapability.ErrorCategory`
values into this boundary taxonomy while retaining the detailed AI category in
the AI invocation ledger. Domain adapters map only documented sentinel/typed
errors and default unknown causes to `internal`.

## 15. Audit, trace, and sensitive-data handling

Tool call audit metadata includes:

- Tool call ID;
- parent Agent ID/version/run ID;
- exact Tool ID/version/capability/owner;
- tenant/user/business-task/trace identifiers from trusted context;
- risk, permission, retry owner, and usage owner;
- start/end time and latency;
- input/output hashes;
- outcome and normalized error code;
- linked AI invocation ID(s), when applicable.

Raw Tool input/output, source snapshots, prompts, provider responses,
credentials, cookies, and marketplace tokens are not stored in generic Tool
audit records. Owner-specific business storage remains authoritative for
reviewable candidate content.

OpenTelemetry owns spans and propagation. A small Tool audit recorder port may
emit structured non-sensitive events. It does not replace the AI invocation
ledger or create a second billing ledger.

## 16. First real vertical slice

### 16.1 Tool identity

```text
tool_id:       product.canonical.inspect
version:       v1.0.0
capability:    product.canonical
risk:          read
side_effects:  none
idempotency:   deterministic
retry_owner:   caller
usage_owner:   unmetered
permission:    existing ListingKit read permission
```

### 16.2 Input

The only model-visible input is a non-empty `task_id`. Tenant, user, roles,
Agent identity, Tool version, and trace values are not accepted in JSON.

### 16.3 Owner service

The adapter calls a narrow consumer-owned read port. The production port must
preserve existing tenant and owner filtering and return only the data required
for the projection:

- canonical product snapshot;
- source reference/lineage outside canonical product;
- task identity necessary for response correlation.

The Tool does not call a GORM repository or canonical cache directly. It does
not depend on the broad ListingKit service interface. If the target-direction
read port has not yet been extracted when implementation starts, extracting
that narrow port is part of the vertical slice and is verified with existing
read behavior tests.

### 16.4 Output

The output contains:

- task ID;
- the existing canonical product projection;
- source lineage as a separate field;
- deterministic diagnostics derived from field traces and existing canonical
  review helpers;
- an overall `needs_review` value.

The projection is not a second product fact model. It owns no persistence and
cannot be submitted as a canonical write payload. Contract tests prove it is a
read projection of the authoritative domain value.

Marketplace readiness is explicitly excluded. `product.canonical.inspect`
must not guess SHEIN/TEMU/Amazon category, attribute, image, or publishing
rules while #34 and the marketplace rule owners are still open.

### 16.5 Error mapping

- blank/malformed task ID -> `invalid_input`;
- missing verified identity -> `identity_integrity`;
- failed Casbin decision -> `permission_denied`;
- tenant/owner-invisible task -> `not_found`;
- task exists without canonical product -> `failed_precondition`;
- read timeout -> `deadline_exceeded`;
- unexpected service failure -> `internal`.

## 17. Later Tool adapters

Later adapters reuse the same contract and are added only when their owner and
dependency gate are ready:

| Tool family | Required owner/gate | Phase 2A behavior |
| --- | --- | --- |
| source evidence | #30 and neutral sourcing read port | read evidence and warnings |
| catalog/asset facts | product catalog/asset ownership | read normalized facts |
| ProductEnrich | #130 governed execution evidence | propose text/product patch only |
| ProductImage | #130 governed vision/image execution | analyze/propose action only |
| marketplace rules | marketplace-owned rule service | read category/attribute rules |
| readiness validator | #34 deterministic contract | compute blockers/warnings |

AI-backed adapters return proposals, evidence, confidence, unresolved issues,
and review guidance. They never apply the proposal. Domain services must
revalidate any later human-approved change.

## 18. App composition and runtime relationship

App composition constructs concrete owner Tools and the immutable registry.
It may collect a list of Tools from explicit builders, but it does not add Tool
runtime behavior to `kernel/module.Registry`.

Phase 2A does not expose a user-facing Tool execution endpoint. Tests and a
small conformance harness use the registry directly. Phase 2B Agent Runtime
will receive a bound Tool set and adapt it to the chosen framework. Framework
adapters translate:

- Tool Definition -> framework tool declaration;
- framework arguments -> JSON validated invocation;
- Tool result/error -> framework-safe structured result.

No framework type crosses into `internal/commercetool` or owner services.

The Product Agent feature flag and tenant allowlist are Phase 2B controls and
reuse OpenFeature. They do not make write/publish executable; the Tool risk
ceiling remains an independent server-side gate.

## 19. Open-source component policy

Phase 2A reuses or promotes existing components:

| Concern | Existing component/owner | Decision |
| --- | --- | --- |
| permission policy | Casbin | reuse through authorizer adapter |
| verified identity | authidentity + aiidentity | reuse; no identity in Tool JSON |
| JSON Schema | santhosh-tekuri/jsonschema/v6 | promote to direct dependency |
| semantic version validation | golang.org/x/mod/semver | reuse |
| traces | OpenTelemetry | reuse |
| durable workflow | Temporal | remains outside Tool invocation loop |
| feature flags | OpenFeature runtime from architecture Phase 2 | reuse in Phase 2B |
| AI routing/cost/ledger | internal/aicapability | reuse; no second AI control plane |

No Agent framework is introduced until Phase 2B. At that point Eino or another
candidate is evaluated as a replaceable runtime adapter against this contract,
not selected first and used to define the contract.

## 20. Delivery slices

### Slice A: #133 contract and immutable registry

- add contract types and validation;
- add JSON Schema compilation and input/output validation;
- add immutable registry and exact Agent allowlist binding;
- add principal/authorizer/audit ports;
- add risk, timeout, retry, idempotency, usage, and error policies;
- prove write/publish denial and framework import boundaries with tests;
- do not add a public endpoint or Agent framework.

### Slice B: canonical inspection vertical slice

- add/extract the target-direction canonical read port;
- add the owner Tool adapter;
- wire existing Casbin/identity and tracing adapters;
- add typed projection/schema parity tests;
- add tenant/owner isolation and no-repository-access guards;
- run through the same registry contract used by fake Agent tests.

### Slice C and later: dependency-gated adapters

- source and fact tools after #30/product ownership;
- AI proposal tools after #130;
- readiness/marketplace tools after #34 and marketplace ownership.

Slice A can close #133. Slice B records partial #134 evidence. #134 closes only
when its declared Product Agent minimum set and dependency gates are actually
complete.

## 21. Test and architecture guards

### 21.1 Contract tests

- reject incomplete definitions;
- reject invalid semantic versions and duplicate refs;
- reject invalid schemas and schema mismatches;
- reject undeclared input properties;
- reject missing verified identity and failed permission;
- reject non-allowlisted Tools and version mismatches;
- reject write/publish under the Phase 2A policy;
- enforce timeout and exactly-once executor invocation;
- prove the registry never retries an executor;
- normalize owner and AI capability errors deterministically;
- prove recorder failure does not replay a completed executor.

### 21.2 First Tool tests

- successful canonical read returns source lineage separately;
- cross-tenant and non-owner reads return not found;
- tenant admin/platform admin behavior matches existing access semantics;
- missing canonical product returns failed precondition;
- diagnostics are deterministic and require no network/model;
- input/output fixtures match compiled schemas;
- output cannot be used as a write command.

### 21.3 Import-boundary guards

- `internal/commercetool` cannot import Agent frameworks, HTTP, workflow,
  persistence, provider, marketplace, or domain implementation packages;
- owner Tool adapters cannot import GORM stores or provider SDKs;
- AI-backed adapters must call the governed capability/service boundary;
- no new production package imports root `internal/listingkit` for this work;
- app packages perform construction only.

### 21.4 Verification

Each slice runs:

- focused package tests;
- reverse-dependency tests for touched domain services;
- repository import-boundary/depguard tests;
- `go test ./... -count=1` when environment-independent;
- `go vet`/configured lint and code-health verification;
- `git diff --check`.

The exact commit and any environment-dependent exclusions must be recorded.

## 22. Failure and rollback behavior

- Registry construction failure prevents the Tool runtime from becoming
  available; it does not affect the fixed product/listing flow.
- Tool failure returns a structured error and leaves canonical product state
  unchanged.
- Product Agent remains feature-disabled until Phase 1 and Phase 2A exit
  evidence exists.
- Disabling or failing Agent Runtime leaves current fixed flows available.
- A new Tool version is opt-in through a new exact Agent allowlist entry.
- No Tool implementation requires database rollback because Phase 2A has no
  business mutation.

## 23. Rejected alternatives

### Extend `kernel/module.Registry`

Rejected because startup contribution collection and guarded runtime Tool
execution have different ownership and security semantics. Combining them
would create a god registry and couple unrelated modules to Agent policy.

### Build all #134 adapters in one change

Rejected because #30, #34, and #130 are not complete and current package owners
are moving. The result would either fake capabilities or add legacy imports.

### Let Eino/ADK Tool types become the domain contract

Rejected because it makes framework replacement expensive and lets a runtime
library dictate domain DTOs, errors, identity, and retry behavior.

### Let Agent Runtime call repositories or provider clients

Rejected because it bypasses tenant/owner checks, duplicates service logic,
leaks credentials, and creates independent retry/effect owners.

### Reuse AI invocation records for every Tool call

Rejected because deterministic reads/computations are not model invocations.
Doing so would pollute model, cost, and provider semantics. Tool audit links to
real child AI invocation records only when an AI-backed executor runs.

### Add a new generic RBAC, tracing, schema, workflow, or feature-flag library

Rejected because the repository already has Casbin, OpenTelemetry, JSON Schema,
Temporal, and an in-progress OpenFeature runtime.

## 24. Acceptance mapping

### Issue #133

This design satisfies #133 when Slice A evidence proves:

- all required metadata is mandatory and validated;
- only exact allowlisted Tool versions can be obtained by Agent Runtime;
- missing schema/permission/side-effect declarations fail construction;
- write/publish are not executable;
- no Agent framework is imported by the contract.

### Issue #134

Slice B proves the common contract works with a real owner service and provides
partial #134 evidence. Full #134 acceptance additionally requires the declared
source, facts, proposal, marketplace, and validator minimum needed by Product
Agent, each after its owner/dependency gate.

### Phase 2A exit

Phase 2A exits only when a fake and the Product Agent integration tests use the
same registry contract for read, compute, and propose behavior without a second
compatibility interface, and the Agent has no direct repository, provider SDK,
or marketplace client access.

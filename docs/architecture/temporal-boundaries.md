# Temporal Boundaries

## Goal

This note defines the stable boundary for Temporal usage in this repository.
Temporal is an orchestration runtime, not a new business domain and not a
replacement for every asynchronous path.

The main risk to avoid is letting workflow or activity code absorb HTTP API
rules, service facade behavior, repository wiring, or platform business logic.

## Stable Layers

The intended direction is:

`HTTP API -> service facade -> workflow runtime -> domain services / ports`

Each layer should stay narrow:

- `HTTP API`
  - Accepts requests, validates request shape, and calls a service facade.
  - May start, query, or signal a workflow through a facade or explicit runtime
    adapter.
  - Should not contain workflow implementation logic.
- `service facade`
  - Owns business-facing use cases and translates user actions into runtime
    commands.
  - Keeps callers independent from Temporal SDK types.
  - Decides whether a use case belongs in Temporal, RabbitMQ, or a synchronous
    service path.
- `workflow runtime`
  - Owns workflow registration, activity registration, worker startup, workflow
    IDs, retry policy, and orchestration control flow.
  - Calls domain services through ports or small adapters.
  - Should not import HTTP API packages or define product-facing request
    contracts.
- `domain services / ports`
  - Own business rules, validation, persistence decisions, and platform-specific
    behavior.
  - Should not depend on Temporal SDK packages.

## Temporal Versus RabbitMQ

Temporal is appropriate when a flow needs durable orchestration, retries across
process restarts, activity history, or explicit long-running workflow state.

RabbitMQ remains appropriate for event fan-out, simple background jobs, queue
workers that do not need workflow history, and existing task-processing paths
where queue semantics are already the stable contract.

Choosing Temporal should be an architecture decision, not the default for every
new asynchronous feature.

## Import Direction Rules

Default rules for new code:

1. HTTP API packages may depend on service facades or narrow runtime adapters.
2. Service facades should expose repository- and transport-neutral methods.
3. Workflow runtime packages may depend on domain ports and adapters.
4. Workflow runtime packages must not depend on HTTP API packages.
5. Domain packages must not import Temporal SDK packages.
6. Temporal SDK imports should stay in runtime or orchestration adapter packages.

## Current Allowed Runtime Areas

The current repository keeps Temporal-specific SDK usage in:

- `internal/app/runtime`
- `internal/listingkit/temporal`
- narrow ListingKit submission activity adapter files that bridge existing
  service behavior during the transition

These areas are allowed because they are runtime or orchestration adapters.
They should stay thin and should not become the place where new ListingKit
business behavior is implemented.

## Boundary Guards

The stable import boundaries are enforced by:

- `TestTemporalSDKImportsStayInRuntimeAndOrchestrationAdapters`
- `TestTemporalRuntimePackagesDoNotImportHTTPAPI`

Treat failures in these tests as architecture regressions, not as prompts to
broaden the allowed runtime surface without review.

## Review Questions

When reviewing a Temporal-related change, ask:

1. Is this workflow orchestration, or is it business logic that belongs in a
   service/domain package?
2. Is this flow durable and long-running enough to justify Temporal over
   RabbitMQ?
3. Does the HTTP API still call a facade instead of embedding workflow details?
4. Are Temporal SDK types kept out of domain-facing contracts?
5. Can the workflow runtime be tested without pulling in HTTP handlers or route
   builders?

## Working Rule

Use Temporal to coordinate durable workflows. Keep business decisions in domain
services, keep request/response behavior in HTTP API packages, and keep simple
queue work on RabbitMQ unless durable workflow history is the actual need.

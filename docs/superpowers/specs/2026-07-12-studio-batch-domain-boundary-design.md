# Studio Batch Domain Boundary Design

## Status

Approved direction for reducing root ListingKit ownership while improving Studio batch iteration safety.

## Goal

Create a dedicated internal/listingkit/studiobatch domain package. It will own Studio batch business concepts and deterministic draft/candidate rules. Root internal/listingkit remains the compatibility, task-orchestration, repository, and HTTP assembly shell.

## Problem

Root internal/listingkit currently contains the Studio batch public facade, persistent records, draft update flow, candidate grouping, generation lifecycle, approval, task creation, batch-run execution, retry/recovery, and service wiring. The existing internal/listingkit/service/studio package is only a documentation stub, so it does not form a usable domain boundary.

The draft path illustrates the coupling: taskStudioBatchDraftService normalizes an HTTP-facing request, derives status, mutates a legacy Studio session, writes sessions and designs, builds a batch name, reconciles batch state, logs operational metadata, and adapts a public detail response.

## Target Architecture

~~~text
internal/listingkit
  -> public service methods and current JSON DTOs
  -> legacy Studio session and GORM record adapters
  -> repository and remote Studio client wiring
  -> task creation, remote generation, Temporal, HTTP composition
  -> delegates deterministic Studio batch domain work

internal/listingkit/studiobatch
  -> domain inputs and outputs for draft/candidate behavior
  -> draft validation, normalization, status derivation
  -> candidate grouping and deterministic detail/status projection
  -> domain errors and stable contracts for later generation/run services
  -> no root ListingKit import, HTTP, GORM, Temporal, or remote client

internal/listing/studio
  -> remains the reusable Studio reference/design utility package
  -> is reused where it already owns normalization rules
~~~

The child package never imports its parent. Root ListingKit converts legacy request/session/record structures into the new domain input/output types. This avoids an import cycle and keeps public JSON and GORM models stable.

## Domain Boundaries

### studiobatch owns

- batch draft intent independent of transport and persistence;
- selected target groups and candidate selection identity;
- validation of required selection identity;
- normalization of deterministic string/list fields needed by a draft;
- draft status derivation;
- candidate grouping, status grouping, and deterministic detail projections;
- domain error categories used by adapters.

### Root ListingKit retains in this slice

- current request, record, session, and JSON/GORM fields;
- session, design, batch, batch-run, and task-link repositories;
- user and tenant context lookup, UUID allocation, optimistic concurrency checks, timestamps, logging, and database transactions;
- batch-name resolution;
- remote Studio submission, polling, materialization, approval, task creation, retry/recovery, and batch-run execution;
- HTTP/API handler and service method signatures.

## First Implementation Slice: Draft and Candidate Policy

The first implementation is limited to deterministic behavior currently mixed into taskStudioBatchDraftService and its candidate helpers.

Create internal/listingkit/studiobatch with platform-neutral draft, selection, candidate group, status, and error types. Its entrypoint accepts a draft input and returns a draft result with no persistence side effects.

PrepareDraft must:

1. reject a missing or invalid selected variant;
2. normalize the design type with the existing internal/listing/studio rule;
3. normalize deterministic string/list fields;
4. derive the same draft status as the legacy path;
5. preserve candidate and approved-design ordering and identifiers.

The root draft service continues to create/load sessions, enforce expected update timestamps, derive a batch name, persist mapped results, replace designs, log, and load the existing public detail response.

## Migration Sequence

### Phase 1 — Domain contract and pure draft/candidate policy

- characterize current draft validation, normalization, status, and candidate projection behavior;
- introduce studiobatch models, errors, and pure rules with package-local tests;
- replace root deterministic helpers with an adapter call;
- add an AST import guard prohibiting studiobatch from importing root ListingKit, HTTP, GORM, Temporal, or remote clients;
- preserve root facade and persistence behavior exactly.

### Phase 2 — Draft lifecycle service behind ports

- define repository-facing ports in studiobatch, owned by the consumer;
- move list/get/upsert/delete decision flow into the domain service;
- keep root implementations of ports over existing legacy repositories and public DTO adapters;
- preserve optimistic concurrency, batch-name behavior, and responses.

### Phase 3 — Generation, approval, and task creation

- move batch graph/status transitions and approval/task-preparation decisions;
- retain remote generation, SDS baseline calls, store validation, and CreateGenerateTask as injected root-owned collaborators;
- do not change asynchronous execution or persistence ordering.

### Phase 4 — Batch run lifecycle

- move run state transitions, cancellation/recovery decisions, and counters;
- retain worker execution, repository implementations, and HTTP route composition in root/adapters.

Each phase is a separate design/plan/verification cycle. Phase 1 must merge before Phase 2 begins.

## Compatibility Rules

- Existing ListingKit service and API method signatures remain unchanged.
- Existing JSON field names, GORM table names, records, and repository interfaces remain unchanged.
- Existing batch IDs, session IDs, timestamps, ordering, and persistence sequencing remain unchanged.
- Existing SDS, SHEIN, remote Studio, browser, Temporal, and task-creation behavior remain unchanged.
- No database migration or external dependency is introduced in Phase 1.

## Error Handling

studiobatch returns typed domain errors for invalid draft input only. The root adapter maps them to existing ListingKit errors and messages. Repository, remote, and persistence errors remain owned by root ListingKit.

## Testing Strategy

Phase 1 uses characterization and TDD:

- package-local table tests for invalid selection, design-type normalization, status derivation, ordering, and idempotent normalization;
- root adapter tests comparing the old public draft result and persisted payload shape;
- AST import guard for the new package;
- root boundary test ensuring the draft service delegates deterministic preparation instead of recreating rules;
- focused studiobatch and ListingKit tests, followed by the existing ListingKit suite.

## Non-Goals

- Migrating the entire Studio batch implementation in one change.
- Moving GORM models, database repositories, or public JSON DTOs in Phase 1.
- Changing batch naming, IDs, optimistic locking, logging, or timestamps.
- Changing remote Studio submission, polling, materialization, approval, task creation, retry, recovery, or batch-run execution.
- Changing SDS POD, SHEIN, Temporal, or HTTP behavior.
- Improving product behavior while moving code.

## Risks and Mitigations

### Import cycles

A child package that accepts root ListingKit DTOs would import its parent.

Mitigation: define narrow studiobatch values and convert in a root adapter.

### Behavioral drift in legacy draft persistence

The current service combines pure normalization with session/database mutation.

Mitigation: move only pure policy first; retain session loading, expected-update checks, UUIDs, naming, persistence, replacement, and logging in root.

### Accidental scope expansion into remote workflows

Generation and task execution include external side effects.

Mitigation: keep those flows out of Phase 1; introduce explicit ports only in later phases.

## Success Criteria

The first slice is complete when:

- internal/listingkit/studiobatch owns a tested, pure draft/candidate policy;
- root ListingKit delegates that policy through a thin adapter;
- public API/JSON, GORM records, persistence, remote execution, and task behavior are unchanged;
- the new package has no dependency on root ListingKit or runtime/infrastructure packages;
- exact legacy draft outputs remain characterized and passing;
- the root package has less deterministic Studio batch policy and a documented next migration phase.

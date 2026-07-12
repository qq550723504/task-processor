# Studio Batch Domain Boundary Design

## Status

Approved direction for reducing root ListingKit ownership while improving Studio batch iteration safety.

## Goal

Create a dedicated internal/listingkit/studiobatch domain package for ListingKit-specific Studio batch candidate semantics. Reuse the existing internal/listing/studio package for generic draft, naming, status, and batch-run rules. Root internal/listingkit remains the compatibility, task-orchestration, repository, and HTTP assembly shell.

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
  -> ListingKit-specific candidate ownership and grouping semantics
  -> stable candidate fingerprints and rejection categories
  -> deterministic candidate projections over neutral values
  -> domain errors and stable contracts for later ListingKit-specific flows
  -> no root ListingKit import, HTTP, GORM, Temporal, or remote client

internal/listing/studio
  -> owns generic batch draft, naming, status, and completion policy
  -> is reused directly; no equivalent policy is copied into studiobatch
~~~

The child package never imports its parent. Root ListingKit converts legacy request/session/record structures into the new domain input/output types. This avoids an import cycle and keeps public JSON and GORM models stable.

## Domain Boundaries

### studiobatch owns

- selected target groups and candidate selection identity;
- item-specific selection ownership and group-mode fallback;
- candidate grouping, rejection, stable fingerprinting, and deterministic projections;
- domain error categories used by adapters.

### Root ListingKit retains in this slice

- current request, record, session, and JSON/GORM fields;
- session, design, batch, batch-run, and task-link repositories;
- user and tenant context lookup, UUID allocation, optimistic concurrency checks, timestamps, logging, and database transactions;
- batch-name resolution;
- remote Studio submission, polling, materialization, approval, task creation, retry/recovery, and batch-run execution;
- HTTP/API handler and service method signatures.

## First Implementation Slice: Candidate Policy

The first implementation is limited to deterministic behavior in task_studio_batch_candidate_support.go. Generic draft behavior remains in internal/listing/studio, which already owns NormalizeBatchDesignType, ShouldDropCreateGenerationJobs, ResolveBatchName, AggregateBatchStatus, ResolveBatchStatus, and batch-run completion.

Create internal/listingkit/studiobatch with neutral selection, item, design, candidate, rejection, and error types. Its entrypoint accepts a candidate evaluation input and returns candidates plus structured rejections with no persistence side effects.

The candidate policy must:

1. retain item-specific selection ownership;
2. apply existing internal/listing/studio design-type normalization rather than duplicate it;
3. resolve group-mode fallback deterministically;
4. preserve candidate/rejection ordering and identifiers;
5. produce the same stable candidate fingerprint inputs as the legacy path.

The root service continues to convert existing batch/item/design/session structures, hydrate SDS product details, resolve durable task links, persist task links, create tasks, log, and load public details.

## Migration Sequence

### Phase 1 — Candidate contract and pure candidate policy

- characterize current item ownership, group-mode, candidate/rejection, and fingerprint behavior;
- introduce studiobatch models, errors, and pure candidate rules with package-local tests;
- replace root deterministic candidate helpers with an adapter call;
- add an AST import guard prohibiting studiobatch from importing root ListingKit, HTTP, GORM, Temporal, or remote clients;
- preserve root facade, generic studio policy, hydration, and persistence behavior exactly.

### Phase 2 — Draft lifecycle service behind ports

- define repository-facing ports for ListingKit-specific candidate lookup/claim behavior where they become necessary;
- move only candidate evaluation decision flow into the domain service;
- keep root implementations of ports over existing legacy repositories and public DTO adapters;
- preserve candidate ownership, durable-link behavior, and responses.

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
- Existing batch IDs, session IDs, candidate keys, timestamps, ordering, and persistence sequencing remain unchanged.
- Existing SDS, SHEIN, remote Studio, browser, Temporal, and task-creation behavior remain unchanged.
- No database migration or external dependency is introduced in Phase 1.

## Error Handling

studiobatch returns typed domain errors for invalid candidate input only. The root adapter maps them to existing ListingKit errors and messages. Repository, remote, hydration, and persistence errors remain owned by root ListingKit.

## Testing Strategy

Phase 1 uses characterization and TDD:

- package-local table tests for item selection ownership, design-type normalization reuse, group-mode fallback, rejection category, candidate order, and candidate fingerprint input;
- root adapter tests comparing legacy candidate/rejection outputs and task-link key inputs;
- AST import guard for the new package;
- root boundary test ensuring candidate support delegates deterministic evaluation instead of recreating rules;
- focused studiobatch and ListingKit tests, followed by the existing ListingKit suite.

## Non-Goals

- Migrating the entire Studio batch implementation in one change.
- Moving GORM models, database repositories, public JSON DTOs, or draft persistence in Phase 1.
- Changing batch naming, IDs, optimistic locking, logging, or timestamps.
- Changing remote Studio submission, polling, materialization, approval, task creation, retry, recovery, or batch-run execution.
- Changing SDS POD, SHEIN, Temporal, or HTTP behavior.
- Improving product behavior while moving code.

## Risks and Mitigations

### Import cycles

A child package that accepts root ListingKit DTOs would import its parent.

Mitigation: define narrow studiobatch values and convert in a root adapter.

### Behavioral drift in legacy candidate creation

The current candidate flow combines pure ownership rules with SDS hydration, durable-link lookup, and task persistence.

Mitigation: move only pure candidate policy first; retain hydration, durable-link lookup, persistence, task creation, and logging in root.

### Accidental scope expansion into remote workflows

Generation and task execution include external side effects.

Mitigation: keep those flows out of Phase 1; introduce explicit ports only in later phases.

## Success Criteria

The first slice is complete when:

- internal/listingkit/studiobatch owns a tested, pure ListingKit candidate policy and reuses internal/listing/studio generic policy;
- root ListingKit delegates that policy through a thin adapter;
- public API/JSON, GORM records, persistence, remote execution, and task behavior are unchanged;
- the new package has no dependency on root ListingKit or runtime/infrastructure packages;
- exact legacy candidate and rejection outputs remain characterized and passing;
- the root package has less deterministic Studio batch policy and a documented next migration phase.

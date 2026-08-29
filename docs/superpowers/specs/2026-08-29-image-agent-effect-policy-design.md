# Image Agent V3 Effect Policy Design

## 1. Status and decision

This design is approved for planning. It defines a behavior-preserving refactor of the Image Agent v3 external-effect state machine.

The decision is to add a pure domain-policy package at `internal/imageagent/effectpolicy` and make both the memory and GORM repositories delegate every v3 effect transition to that package. Repository adapters retain locking, transactions, compare-and-swap persistence, database mapping, and not-found handling. They no longer independently own phase, blocking, budget, lease, fencing, or recovery rules.

The work is split into two independently verifiable delivery slices:

1. Repair the Windows CRLF-sensitive release-boundary test harness without changing runtime behavior.
2. Extract and adopt the Image Agent v3 effect policy without changing public or persisted contracts.

## 2. Context and root cause

The repository is a modular monolith with explicit package-boundary rules. The current Image Agent design already states that:

- Temporal owns durable orchestration and retry scheduling;
- `imageagent` owns effect identity, phases, claims, fencing, and transition validation;
- `imageagent/store` owns transactional compare-and-swap persistence.

The implementation has drifted from that ownership model. `internal/imageagent/store/slot_effect_v3.go` contains both memory and GORM implementations plus domain transition predicates and validation. The two adapters repeatedly express the same business decisions around provider reservation, budget settlement, staging, publication, blocked phases, and explicit recovery.

This is not only a file-size problem. It creates three structural risks:

1. A protocol change must be applied consistently to two persistence paths.
2. Repository tests can pass while domain rules still diverge between adapters.
3. Persistence code becomes the de facto owner of recovery policy, contrary to the documented dependency direction.

The focused `internal/imageagent/...` package tests pass on the audited commit, but the repository history shows repeated corrections to recovery, authorization, budget, cancellation, and corruption handling. The repository-level `./tests` package is not green on the Windows checkout because of the separate line-ending defect described below. Centralizing the policy addresses the shared architectural cause rather than continuing to patch individual state-machine call sites.

The audit also found a separate verification defect. On Windows, the machine-level Git configuration has `core.autocrlf=true` and the repository has no `.gitattributes`. Several release-boundary tests read checked-out Markdown or YAML as bytes and search for LF-only snippets. The checked-out files contain CRLF, so the fixtures fail before exercising the policy under test. Normalizing those test inputs in memory makes the expected blocks discoverable. This is a test-harness portability problem, not a release workflow defect.

## 3. Goals

- Establish one executable owner for every Image Agent v3 effect transition.
- Make transition decisions pure, deterministic, and independent of storage technology.
- Keep memory and GORM behavior identical for state, budget, error, lease, fencing, idempotency, and recovery semantics.
- Preserve all current public, database, projection, and Temporal contracts.
- Keep database concurrency protection at least as strong as the current implementation.
- Restore release-boundary test execution on CRLF worktrees without changing runtime files.
- Add package-boundary enforcement so persistence or runtime concerns cannot leak back into the policy package.

## 4. Non-goals

- Do not change any Image Agent business behavior.
- Do not add Agent planning, automatic repair, or new workflow nodes.
- Do not redesign ProductImage providers, object storage, ListingKit publication, or AI capability routing.
- Do not change the Repository interfaces used by Temporal, HTTP, tools, or services.
- Do not change database tables, columns, indexes, records, or migration files.
- Do not change Temporal workflow names, activity names, task queues, version gates, timeouts, or history payloads.
- Do not add a generic finite-state-machine dependency.
- Do not split files merely to reduce line counts.
- Do not deploy, merge, mutate production, or update GitHub issue state as part of this design and planning work.

## 5. Open-source reuse decision

The repository already uses the appropriate open-source building blocks:

- Temporal remains the durable workflow and retry owner.
- GORM remains the transactional persistence adapter.

A generic finite-state-machine library would not replace the project-specific rules for usage reservation, provider dispatch ambiguity, immutable manifests, publication leases, fencing tokens, explicit redrive, or corrupt persisted authorization. It would introduce an additional mapping layer while leaving transaction and compare-and-swap code unchanged. No new FSM or workflow framework is introduced.

## 6. Ownership and dependency direction

The intended dependency direction is:

```text
Temporal / HTTP / tools / services
                |
                v
       imageagent repository ports
                |
                v
       imageagent/store adapters
                |
                v
      imageagent/effectpolicy
                |
                v
      imageagent model and errors
```

The package responsibilities are:

### 6.1 `internal/imageagent`

Retains existing domain types, sentinel errors, blocked codes, normalization functions, fingerprints, repository ports, and public contracts. No existing exported signature changes.

### 6.2 `internal/imageagent/effectpolicy`

Owns pure v3 transition decisions. It may import only the Go standard library and `task-processor/internal/imageagent`.

It must not import:

- GORM or database drivers;
- Temporal SDK packages;
- HTTP or Gin packages;
- configuration packages;
- object storage or provider clients;
- ListingKit, ProductImage, marketplace, app, infra, or integration implementations.

It must not accept `context.Context`, `*gorm.DB`, repository implementations, or callbacks that persist state. Time-sensitive decisions receive an explicit timestamp.

### 6.3 `internal/imageagent/store`

Retains:

- memory mutex ownership;
- GORM transaction and row-lock ownership;
- row decoding and encoding;
- not-found classification;
- SQL update mapping;
- compare-and-swap predicates and `RowsAffected == 1` checks;
- atomic persistence of effect and usage results.

It must not independently decide whether a phase change, budget change, lease renewal, fence handoff, blocked transition, or recovery restore is allowed.

### 6.4 Other Image Agent packages

Temporal, HTTP, object-store, and ProductImage adapters continue to call the existing repository ports. They do not import `effectpolicy` directly. This prevents a second transition owner from appearing in orchestration or delivery adapters.

## 7. Policy API shape

The package uses typed transition functions rather than a generic `Apply(event)` function. Typed functions keep command-specific inputs and outputs visible to reviewers and prevent invalid field combinations from being represented as a generic event map.

The common immutable result is conceptually:

```go
type EffectDecision struct {
    Attempt imageagent.SlotEffectV3Attempt
    Changed bool
}
```

Budget-aware decisions additionally return absolute accounting results rather than loose deltas:

```go
type AccountingSnapshot struct {
    Reserved  imageagent.UsageVector
    Committed imageagent.UsageVector
    Elapsed   time.Duration
}

type AccountingDecision struct {
    EffectDecision
    Accounting        AccountingSnapshot
    AccountingChanged bool
}
```

Provider reservation decisions also return whether the caller acquired the provider-dispatch right. Publication claim decisions return the normalized claim and whether the caller acquired the publication right. Lease renewal returns the new claim. Those outcomes preserve the boolean meanings of the current Repository methods.

The transition families are:

1. Provider and budget:
   - reserve a new or explicitly redispatchable provider effect;
   - record provider-not-dispatched;
   - settle provider usage;
   - release reserved provider budget;
   - mark provider budget outcome unknown.
2. Staging:
   - prepare a normalized immutable staging manifest;
   - commit the matching manifest as staged.
3. Publication:
   - claim publication with lease and fencing;
   - renew an unexpired matching lease;
   - complete publication with the exact manifest and result fingerprint.
4. Recovery:
   - block a permitted executable phase with the exact public code;
   - record and restore an explicitly redrivable recovery phase;
   - convert deterministic corrupt evidence into the canonical fail-closed fields.

Every transition returns a new value and must not mutate its input. Idempotent repeats return the existing normalized value with `Changed=false`. Invalid input, conflicting identity, stale phase, stale fence, expired lease, mismatched fingerprint, or unsupported recovery returns the same sentinel classification as the current Repository implementation.

## 8. Repository data flow and atomicity

Every Repository method follows the same sequence:

```text
normalize and validate command
  -> acquire mutex or begin transaction
  -> load effect and accounting snapshot
  -> call the typed effectpolicy transition
  -> persist the returned absolute state atomically
  -> return the normalized result
```

### 8.1 Memory adapter

The memory adapter performs the complete load, decision, and write sequence under the existing mutex. When a decision changes both an effect and usage accounting, both values are replaced before the lock is released. An error causes no map mutation. Returned values remain cloned so callers cannot mutate repository state.

### 8.2 GORM adapter

The GORM adapter retains the existing transaction and row locks. It obtains the database timestamp inside the transaction and passes it to the policy for lease and elapsed-time decisions. It persists the returned effect and accounting values in the same transaction.

SQL `WHERE` predicates may repeat identity, expected phase, owner, or fence fields as concurrency guards. Those predicates are storage-level race protection, not an independent source of business eligibility. `RowsAffected != 1` continues to return `imageagent.ErrRevisionConflict`.

### 8.3 Idempotency

If the policy returns `Changed=false`, the adapter performs no state-changing update. It returns the existing normalized attempt, claim, or accounting outcome. Repeated completion with different content remains a revision conflict; it is not converted to a successful no-op.

### 8.4 Corrupt persisted evidence

Row decoding remains a storage concern. When decoding produces deterministic, non-sensitive corrupt evidence, the adapter passes only the stable identity and marker to the policy. The policy returns the canonical recovery-blocked phase, blocked code, and corruption marker. It never reconstructs missing authorization or provider data. Repeating the same fail-closed operation is idempotent.

## 9. Error contract

The following classifications are compatibility requirements:

- invalid command or domain input: `imageagent.ErrValidation`;
- identity, fingerprint, phase, fence, lease, manifest, or result conflict: `imageagent.ErrRevisionConflict`;
- missing run or effect row: `imageagent.ErrRunNotFound`;
- unsupported or mismatched persisted phase/code policy: `imageagent.ErrInvalidPersistedPolicy`;
- corrupt persisted authorization evidence: `imageagent.ErrCorruptPersistedEffect`;
- existing budget exhaustion, quote-unavailable, elapsed, and lifecycle blocked codes remain exact.

`errors.Is` behavior must remain compatible. Public blocked codes and permitted actions remain exact. Internal error text may be consolidated only when it is not a public or tested contract.

The policy never retries, falls back, logs, or swallows persistence errors. Temporal remains the only owner of technical activity retry and durable recovery orchestration.

## 10. Compatibility contract

The refactor must not change:

- `SlotExternalEffectV3Repository` or its optional recovery/corruption capabilities;
- public Go types or method signatures used by callers;
- JSON names, DTOs, event payloads, projections, or SSE behavior;
- table definitions, column names, indexes, persisted phase strings, or historical rows;
- idempotency-key or fingerprint algorithms;
- budget quote, receipt, or usage semantics;
- publication owner, lease, or fence semantics;
- Temporal workflow/activity names, frozen v2 wires, v3 wires, task queues, version markers, or history replay;
- deployment configuration or runtime state.

No database or Temporal migration is required. If implementation discovers that any item above must change, work stops and the design returns for approval instead of expanding scope.

## 11. Windows release-boundary test portability

The test-only prerequisite introduces a helper in the `tests` package that:

1. reads a repository text fixture;
2. converts `\r\n` and any remaining bare `\r` to `\n` in memory;
3. returns the normalized text to string-based mutation and fenced-code-block tests.

Only release-boundary tests that perform exact textual matching use this helper. YAML tests that already parse YAML continue using the parser. Shell syntax checks receive the normalized fenced block. Runtime Markdown, YAML, shell scripts, workflows, and `.gitattributes` are not changed.

This slice must first preserve a failing test that demonstrates the CRLF fixture problem, then make that test and the existing failures pass. It is committed separately from the runtime refactor.

## 12. Test strategy

### 12.1 Pure policy tests

Table-driven tests in `internal/imageagent/effectpolicy` cover:

- every allowed phase transition;
- every disallowed source/target phase pair;
- exact idempotent repeats and conflicting repeats;
- reservation identity and input fingerprint mismatches;
- provider-not-dispatched redispatch;
- budget reserve, settle, release, and unknown outcomes;
- quote/receipt validation and accounting overflow/underflow;
- staging manifest normalization and fingerprint mismatch;
- publication acquisition, lease renewal, expiry, fence handoff, and completion;
- every blocked phase/code pairing and permitted recovery action;
- explicit recovery restore eligibility;
- deterministic corruption fail-closed behavior;
- no input mutation.

### 12.2 Repository conformance tests

One shared scenario suite runs against both the memory repository and the existing GORM test harness. Each scenario asserts the same:

- returned attempt and claim;
- acquisition and changed flags;
- persisted effect;
- reserved and committed usage;
- sentinel error classification;
- state after idempotent repeat;
- state after conflicting repeat;
- concurrent publication and recovery outcome where the current harness supports concurrency.

Adapter-specific tests remain for transaction rollback, SQL row mapping, database time, locking, and corruption decoding.

### 12.3 Temporal and integration regression

Existing Image Agent workflow, activity, cancellation, external recovery, history replay, HTTP, and tool tests remain unchanged except for imports required by the refactor. Replay fixtures must pass without regeneration.

### 12.4 Architecture guards

Depguard and semantic import tests enforce that:

- `effectpolicy` imports only the standard library and `internal/imageagent`;
- `imageagent` domain code does not import `store`;
- Temporal, HTTP, and tools do not import `effectpolicy` directly;
- store adapters may import `effectpolicy`, but the policy cannot import store or infrastructure.

The architecture documents' current guard baseline is updated in the same change.

### 12.5 Verification ladder

Verification proceeds from focused to broad:

```text
go test ./internal/imageagent/effectpolicy -count=1
go test ./internal/imageagent/store -count=1
go test ./internal/imageagent/temporal -count=1
go test ./internal/imageagent/... -count=1
go test ./tests -count=1
go test ./... -count=1
current CI-equivalent depguard invocation from the repository root
```

Selected `-run` patterns must be listed before use and must execute real tests. `[no tests to run]` is not evidence.

## 13. Delivery sequence

### Slice 1: test portability

- Add the failing CRLF fixture test.
- Add the normalized repository-text helper.
- Migrate only the failing release-boundary textual fixtures.
- Run the focused failures and `go test ./tests -count=1`.
- Commit as an independently reviewable test-only change.

### Slice 2: policy extraction

1. Add the empty policy package boundary guard and failing transition tests.
2. Implement provider and budget decisions, then migrate memory and GORM methods for that family.
3. Implement staging decisions, then migrate both adapters.
4. Implement publication decisions, then migrate both adapters.
5. Implement blocked, recovery, and corruption decisions, then migrate both adapters.
6. Run the shared conformance suite after every family.
7. Remove old store-owned transition predicates only after both adapters use the policy.
8. Run Temporal replay and the full verification ladder.

Each family is a reviewable commit. Behavior changes, schema changes, file-only moves, and unrelated cleanup are excluded.

## 14. Acceptance criteria

- Memory and GORM repositories produce identical transition, accounting, claim, and error outcomes for the shared matrix.
- Store code retains persistence and concurrency guards but no longer owns domain eligibility decisions.
- The policy package has no persistence, orchestration, HTTP, configuration, or external-client dependency.
- Existing public Go, JSON, database, and Temporal contracts are unchanged.
- Existing history replay fixtures pass unchanged.
- Windows CRLF worktrees execute the release-boundary tests successfully.
- Focused Image Agent tests, repository tests, full Go tests, and depguard all execute and pass on the exact implementation commit before completion is claimed.
- No deployment, production mutation, merge, or GitHub issue mutation occurs without separate authorization.

## 15. References

- `docs/superpowers/specs/2026-08-26-image-agent-workflow-design.md`
- `docs/superpowers/specs/2026-08-27-image-agent-recovery-compatibility-design.md`
- `docs/superpowers/specs/2026-08-28-image-agent-budget-authorization-design.md`
- `docs/architecture/project-boundaries.md`
- `docs/architecture/architecture-review-checklist.md`
- `internal/imageagent/slot_effect_v3.go`
- `internal/imageagent/store/slot_effect_v3.go`

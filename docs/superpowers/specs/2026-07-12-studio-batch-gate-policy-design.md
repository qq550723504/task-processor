# Studio Batch Gate Policy Boundary Design

## Goal

Move deterministic Studio batch candidate admission rules from root ListingKit into internal/listingkit/studiobatch while retaining all external checks, caching, persistence, and task orchestration in root ListingKit.

## Problem

The current studio batch gate combines two different responsibilities:

- pure admission policy: design ownership and approval, selection identity and ownership, variant compatibility, grouped-selection compatibility, and target-group matching;
- root-bound integration: tenant resolution, store validation, SDS baseline lookup, caching of external results, error propagation, and task-link persistence.

The pure portion depends only on candidate data already represented by internal/listingkit/studiobatch. Keeping it in root ListingKit duplicates the candidate domain boundary and makes rule changes depend on repository and runtime wiring.

## Existing Ownership to Reuse

internal/listingkit/studiobatch already owns neutral candidates, selections, compatibility fingerprints, candidate keys, and structured candidate rejections. It imports only the standard library and internal/listing/studio.

internal/listing/studio remains the owner of generic batch policy. The gate slice must not duplicate its generic design-type, batch-status, naming, or completion rules.

## Target Architecture

~~~text
root internal/listingkit
  -> loads/hydrates candidates and batch graph
  -> resolves tenant identity
  -> validates store through repository-backed validator
  -> checks SDS baseline through cache/repository-backed checker
  -> caches external store/baseline results
  -> persists rejected task links and creates tasks
  -> adapts root records to neutral gate input
  -> delegates pure admission decision

internal/listingkit/studiobatch
  -> evaluates design admission
  -> evaluates selection identity, ownership, and variant compatibility
  -> evaluates grouped-selection compatibility and target matching
  -> returns eligible or a stable reason code/message
  -> has no context, repository, HTTP, GORM, Temporal, SDS client, or store dependency
~~~

## Package Contract

The new pure evaluator receives neutral values. It returns a value result rather than an error:

~~~go
type GateInput struct {
    BatchID             string
    BatchGroupMode      string
    Candidate           Candidate
    Designs             []Design
    SelectionByID       map[string]GroupedSelection
    ItemSelections      []GroupedSelection
}

type GateResult struct {
    Eligible   bool
    ReasonCode string
    Message    string
}

func EvaluateGate(input GateInput) GateResult
~~~

The exact types may include the minimum additional neutral fields required to preserve existing variant compatibility checks. They must not expose root ListingKit models or external client values.

## Pure Rule Order

EvaluateGate preserves the current deterministic order:

1. Confirm the candidate design exists, belongs to the requested batch and item, is approved, and has an image.
2. Confirm selection identity is present, appears in the batch snapshot, belongs to the item when item ownership is explicit, has required SDS identity fields, and has compatible variant surface metadata.
3. Confirm compatibility fingerprint completeness and equality for grouped non-per-product selections.
4. Confirm a design target group does not conflict with its item target group.

The root gate runs this pure result before store and baseline checks. Store validation and baseline readiness remain after it and preserve existing reason codes and error behavior.

## Compatibility Rules

- Preserve all current rejection reason codes and message text.
- Preserve the admission order: pure policy before store validation before baseline validation.
- Preserve root cache keys and cache lifetime for store, baseline, and compatibility data.
- Preserve baseline query construction, tenant resolution, store lookup, external error propagation, and rejected task-link persistence.
- Preserve API, JSON, GORM records, repository interfaces, task keys, and task creation behavior.
- Do not change SDS hydration, remote Studio operations, Temporal, or persistence ordering.

## Testing Strategy

- Add package-local table tests for every pure rejection category and one eligible path.
- Add a root characterization test showing that a rejected pure result prevents store/baseline calls.
- Add a root characterization test showing store and baseline rejections remain unchanged after pure admission succeeds.
- Add an AST boundary test requiring root gate delegation to studiobatch.EvaluateGate and rejecting revived root pure admission helpers.
- Expand the studiobatch import guard if required to cover every new production file.
- Run focused studiobatch and gate tests, all ListingKit subpackage tests, and go vet for ListingKit.

## Non-Goals

- Moving store validation, SDS baseline cache reads, tenant bridge, external cache maps, or their error behavior.
- Moving rejected task-link persistence, gate evaluation orchestration, task creation, or remote Studio calls.
- Changing candidate key, fingerprint, design type, selection ownership, group mode, or rejection behavior.
- Adding a generic policy engine, dependency injection framework, external package, or database migration.

## Risks and Mitigations

### Missing neutral variant data

Selection compatibility currently reads variant surface metadata from root DTOs.

Mitigation: add only the necessary neutral variant fields and compare existing root characterization outputs before deleting root pure helpers.

### Rejection order drift

Moving separate checks can change the first visible rejection.

Mitigation: EvaluateGate follows the current design, selection, compatibility order; root tests assert store and baseline are not called when pure policy rejects.

### External side effects leaking into the child package

The current gate owns repository-backed checks and caches.

Mitigation: root remains the only caller of context-aware validators and cache maps. The child evaluator accepts no context and exposes no port interface in this slice.

## Success Criteria

- studiobatch owns all deterministic gate admission decisions.
- Root ListingKit owns only integration checks, cache state, persistence, and adaptation.
- Rejection codes, messages, ordering, and external call behavior are unchanged.
- New production code in studiobatch depends only on the standard library and existing internal/listing/studio.
- Focused gate tests and full ListingKit subpackage verification pass.

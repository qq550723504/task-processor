# Legacy Hard-Cut Policy

> Status: Active architecture policy  
> Effective: 2026-09-05  
> Scope: all refactoring, new Product/Marketplace/Agent/Tool/Console work, code review, and AI-assisted development

## 1. Decision

The repository uses **Hard-Cut + Selective Extraction** for legacy code.

Legacy code is historical implementation material and behavioral evidence. It is **not** a contract that the new architecture must preserve internally.

Current legacy handling has only two valid outcomes:

```text
Legacy code
  ├─ reusable and valid under current architecture -> EXTRACT
  └─ obsolete design / ownership / workflow          -> RETIRE
```

There is currently **no legacy compatibility class**.

Do not create compatibility layers, fallback paths, dual models, dual state machines, or bidirectional synchronization merely to preserve an obsolete internal design.

If a future task proves that an externally observable contract or persisted/runtime state truly requires temporary compatibility, that requirement must be introduced as an explicit, reviewed exception. It is not implied by this policy and must not be anticipated in advance.

---

## 2. EXTRACT — reuse behavior, not architecture

Use `EXTRACT` when legacy code contains implementation that is still correct and belongs in the current architecture, for example:

- deterministic algorithms;
- parsing/normalization logic;
- marketplace rules;
- provider/client transport logic that does not own business state;
- pure validation logic;
- image/content processing helpers;
- bounded, stateless utilities.

Required migration shape:

```text
Legacy owner
  -> identify reusable behavior
  -> move/rewrite behind current domain contract
  -> add tests against current contract
  -> switch current caller
  -> remove dependency on legacy owner
```

Preferred result:

```text
new service -> current domain capability
```

Forbidden result:

```text
new service -> legacy service -> legacy task/workflow/model
```

### Extract, do not wrap

A new package must not keep an obsolete package alive just because the old implementation already works.

Wrong:

```text
internal/product/new
  -> internal/listingkit/legacy-product-service
```

Correct:

```text
legacy reusable logic
  -> internal/product/<current-owner>

new caller
  -> internal/product/<current-owner>
```

Extraction may preserve verified business behavior. It must not preserve obsolete ownership, DTOs, workflow topology, internal state transitions, or navigation concepts unless they are independently required by the current architecture.

---

## 3. RETIRE — obsolete internal design has no compatibility obligation

Use `RETIRE` when code exists only because of a superseded architecture or product model.

Retired code must not receive:

- new features;
- new Agent/Tool integration;
- new internal compatibility adapters;
- new fallback paths;
- new state synchronization;
- new API surface for the current architecture;
- architecture changes whose only purpose is keeping the retired design alive.

Allowed actions are limited to:

1. extracting still-valid reusable behavior;
2. making the minimum bounded change needed to complete cutover safely;
3. deleting the retired path after its current replacement owns the required behavior.

Do not repair a retired abstraction merely to make new code depend on it.

---

## 4. No internal legacy fallback

New architecture code must not use legacy fallback as a safety mechanism.

Forbidden patterns include semantic equivalents of:

```text
if new path available:
    use new path
else:
    use legacy path
```

```text
read new model
if missing:
    read legacy model
```

```text
write new fact
also write legacy fact
```

```text
BusinessTask <-> old internal Task bidirectional synchronization
```

```text
new Listing -> old Listing implementation fallback
```

A hard cut means that after cutover the current owner is authoritative.

If a migration requires an explicitly bounded transition, it must have a named migration/cutover plan and removal condition; it must not silently become permanent architecture.

---

## 5. Legacy tests are not architecture authority

An existing test proves historical behavior, not automatically a current requirement.

Before preserving a failing legacy test, classify what it asserts:

### Preserve

Rewrite or move the test when it proves a still-valid business, safety, authorization, idempotency, marketplace, or deterministic behavior requirement.

### Retire

Delete or replace the test when it only fixes an obsolete implementation detail, such as:

- retired task/workflow topology;
- obsolete intermediate states;
- superseded DTO shape;
- old ProductEnrich/ProductImage runtime ownership;
- old task-first UI semantics;
- old Product/Listing Workspace assumptions;
- platform-specific Workbench structure that conflicts with current product authority.

Do not modify the new architecture merely to keep an obsolete test green.

---

## 6. Data migration rule

Do not maintain two internal facts indefinitely.

Preferred migration:

```text
old persisted representation
  -> one-time/bounded migration or backfill
  -> current authoritative representation
  -> cutover
  -> retire old read/write path
```

Forbidden by default:

- permanent dual write;
- permanent dual read with fallback;
- new code treating both old and new tables/models as equal authorities.

Any future exception requires a task-specific reviewed migration contract with a removal condition.

---

## 7. Current hard-cut examples

The following current directions are already hard-cut and must not be revived through compatibility work:

- retired ProductEnrich task/queue/worker/API architecture;
- retired ProductImage task/queue/worker/API architecture;
- task-first Product architecture;
- generic independent Listing Workspace (#27);
- old internal/customer Product Task Dashboard semantics (#35);
- separate platform Workbenches as the default TEMU/Amazon expansion model;
- independent top-level Product Center / Listing Center assumptions that conflict with the current Figma UI/IA authority;
- second Product fact source, second submission state machine, second Agent RBAC, second Tool Registry, or second durable retry owner.

Current target owners include:

- Product facts -> Product Domain;
- approved assets -> Product Asset / ImageAgent approval boundary;
- marketplace rules -> Marketplace / Listing Domain;
- AI model routing/cost/ledger -> AI Capability Control Plane;
- Agent single-run state -> Agent Runtime;
- durable workflow/retry -> existing Temporal/queue owners;
- user-visible business tasks -> BusinessTask Product Projection;
- final UI / IA -> Figma `31:463` + `docs/product/final-ui-ia-authority.md`.

---

## 8. Required decision when touching legacy code

Every implementation that encounters legacy code must answer, in order:

```text
1. Is this behavior still required by the current architecture/product?
   ├─ no  -> RETIRE / do not propagate it
   └─ yes
       ↓
2. Can the useful behavior be extracted under the current owner?
   ├─ yes -> EXTRACT
   └─ no  -> stop and identify the concrete current requirement blocking extraction
```

`Keep legacy compatibility` is not a valid third answer under the current baseline.

If a future external compatibility requirement is discovered, stop and create an explicit exception instead of introducing a hidden fallback.

---

## 9. PR / Review rules

A PR touching legacy code must state one of:

```text
Legacy decision: EXTRACT
Reusable behavior:
Current owner:
Legacy dependency removed by this PR / follow-up:
```

or:

```text
Legacy decision: RETIRE
Replacement/current owner:
Remaining cutover condition:
Deletion owner/follow-up:
```

Reviewers should reject a PR when it introduces a new legacy dependency without an explicit current architecture requirement.

The burden of proof is on preserving legacy structure, not on removing it.

---

## 10. Relationship to other authorities

Use this policy together with:

1. Figma `31:463` + `docs/product/final-ui-ia-authority.md` — final product UI/IA and product projection;
2. current Product/Architecture specs — canonical facts, ownership, safety, state machines and contracts;
3. this policy — how obsolete implementation is handled during migration;
4. `docs/refactoring/current-refactoring-status.md` — current implementation reality;
5. GitHub #137 — execution/backlog authority.

Historical issues, tests and code are evidence to inspect. They do not override these current authorities.

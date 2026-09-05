# Current Refactoring Status

> Status: active current-state document.  
> Last reviewed: 2026-09-05.  
> Calibrated against: `main` at `f1c8bad612c8026063434ccd1169e69bd4c43168`.  
> Scope: current product/repository reality, production-validation gates, Product/Marketplace/Console boundaries, Commerce Tool readiness, and the active Now / Next / Later direction for Task Processor / ListingKit / AI Commerce Agent Platform.

## 1. Current position

`task-processor` is no longer accurately described as either a generic task processor or a ListingKit-only application. The long-term product is **AI Commerce Agent Platform**, while the current commercial execution surface still depends on mature deterministic Product/Listing/SHEIN flows.

The current posture is:

```text
1. Validate and ship the current fixed-pipeline commercial baseline safely.
2. Preserve the Product/Identity/Store/Resource hard-cut boundaries already established.
3. Close the remaining 1688 + SHEIN + human-review + release-evidence gates.
4. Stabilize the AI Capability control plane.
5. Complete the minimum read/compute/propose Commerce Tool set.
6. Only then start the bounded Product Agent PoC.
7. Expand SHEIN Agent capability before broad TEMU/Amazon Agent or product expansion.
```

This is **not** a greenfield architecture phase and **not** the time for another broad directory rewrite, a second product fact model, a second workflow/state machine, or parallel platform-specific workbenches.

Use this file together with:

- `docs/product/ai-commerce-agent-platform-strategy.md`
- GitHub issue #137 — executable backlog authority
- GitHub issue #33 — Customer Trial / Production Readiness gate
- `docs/superpowers/specs/2026-09-02-shuomi-console-phase1-hard-cut-design.md`
- `docs/superpowers/specs/2026-08-27-listingkit-product-design-v1.md`
- `docs/architecture/project-boundaries.md`
- Product-domain / Store / Resource / Agent plans only as bounded design or execution evidence

### 1.1 Evidence vocabulary

Use these terms consistently:

- **Implemented**: the code path exists on the calibrated `main` baseline.
- **Repository-validated**: the exact baseline has recorded automated test/build results.
- **Local-accepted**: a real local dependency/runtime path was exercised, such as local ZITADEL/browser acceptance.
- **Staging-validated**: a controlled non-production environment exercised the real integration.
- **Production-validated**: a real production environment or real customer path is recorded in a dated validation note.
- **Deferred**: code/runtime assets may exist, but the capability is not an active product-expansion commitment.
- **Superseded**: the old issue/design abstraction has been replaced and must not be implemented as originally described.

Never treat implemented, repository-validated, local-accepted, staging-validated, and production-validated as interchangeable.

---

## 2. Current system reality

### 2.1 Product model

The current user-facing business hierarchy is:

```text
Source Product / Source Evidence
        ↓
Canonical Product / ProductSnapshot
        ↓
Platform Draft
        ↓
Listing (platform + store)
        ↓
Published Result / Submission History
```

Task and Workflow remain execution infrastructure:

```text
Workflow -> Task -> Child Task -> Queue / Retry / Adapter
```

They may appear in advanced diagnostics, but they are not the long-term primary user business object. New UI or API work must not recreate a Task-first product model.

### 2.2 Product Domain hard-cut

The Phase 3 Product Domain hard-cut is complete at the architecture level.

Current target ownership includes:

- `internal/product/catalog` — canonical `ProductSnapshot` and normalization
- `internal/product/sourcing` — source identity, envelope, lineage/warnings contract
- `internal/product/enrichment` — side-effect-free Proposal generation/validation/scoring boundary
- `internal/product/asset` — approved asset facts and approval persistence contract
- `internal/product/image` — provider-neutral image capability boundary
- ImageAgent — the single owner of product-image workflow, budget/retry/recovery/approval execution

The retired ProductEnrich/ProductImage task/queue/worker/API architecture must not be reintroduced for Agent work.

ListingKit, SDS, Amazon target/source compatibility code and future Commerce Tools consume current Product boundaries; they do not become alternative product-fact owners.

### 2.3 Identity / Organization / Console

ZITADEL remains the identity provider and trusted authentication/authorization foundation.

The repository now contains a Multi-Organization workbench identity model with:

- verified identity context;
- effective/home Organization semantics;
- organization switching;
- role-aware access;
- fail-closed authorization behavior;
- local browser acceptance for cross-Organization isolation and revoke/restore behavior;
- strict BFF/API trust boundaries.

This means the old backlog item “establish the ZITADEL role/tenant model” is no longer an architecture task. Remaining work is **staging/production acceptance and operational security evidence**, tracked through #33/#45/#48.

The Shuomi Console Phase 1 product direction is also active. It is a UI/IA hard cut, not a capability hard cut: existing production capabilities remain reachable until their new Console replacement exists.

### 2.4 Store / Resource baseline

The repository has moved beyond the old “tenant quota JSON / task limit” model.

Current resource semantics are organization-scoped and include:

- `store_renewal_period`
- `ai_point`
- `data_row`

Store binding and Store service activation are separate concepts. Resource Ledger / Store Service work owns durable value/idempotency semantics. Online RMB wallet/payment/resource acquisition remains deferred; UI must not present Renew/Reactivate as purchasable when no acquisition channel exists.

Store lifecycle code may be implemented before production authority handoff. Migration/constraints/route enablement and production execution evidence must remain explicit gates.

### 2.5 Platform maturity

| Capability | Current status | Current interpretation |
| --- | --- | --- |
| SHEIN target listing | Production main path; active stabilization/validation | The current commercial release focus. Readiness, pricing, submission, idempotency, recovery and real E2E evidence remain release-sensitive. |
| 1688 product source | Neutral source model and guarded handoff implemented; controlled closeout pending | Durable lineage + one controlled import-to-product/listing path still need acceptance evidence. |
| SDS POD | Active specialized product/design capability | Keep as POD/design semantics, not a generic source abstraction. |
| Amazon source | Source-envelope/modeling path exists | Does not mean Amazon target listing product is active. |
| TEMU target | Runtime/platform assets retained; full shared-Listing expansion deferred | Maintain correctness; do not build an independent TEMU Workbench. |
| Amazon target | Historical/target assets retained; full shared-Listing expansion deferred | Do not infer SHEIN parity. |
| Multi-Organization / Store Center | Implemented with strong repository/local evidence | Production release acceptance is still required. |
| Resource Ledger / Store Service | Core domain foundation implemented/evolving | Production authority handoff and customer-visible enablement remain explicit gates. |
| AI Capability Control Plane | Foundation exists; release-level validation pending | #126/#130 must close before Product Agent main implementation. |
| Commerce Tool Registry | Completed | #133 closed through PR #272. |
| Commerce domain tools | Partial | `product.canonical.inspect@v1.0.0` is merged; the rest of #134 remains open. |
| Product Agent | Not yet an approved production path | #131/#132 must wait for Phase 1 + Phase 2A exit evidence. |
| SHEIN Listing Agent | Later phase | Read/propose first; save-draft only after independent write safety gate. |

---

## 3. Current maintained boundaries

### 3.1 Product facts

- Product facts belong to Product Domain.
- Source lineage belongs to Product Sourcing.
- Approved assets belong to Product Asset / ImageAgent approval boundaries.
- Platform-specific representation belongs to marketplace/listing boundaries.
- Agent output is a Proposal, not an alternative source of truth.

### 3.2 Runtime / workflow

- `internal/app/*` remains assembly/lifecycle focused.
- Temporal owns durable workflow execution where already established.
- RabbitMQ/listing control-plane owners remain singular.
- Agent Runtime must not become a second durable business-task lifecycle owner.
- Marketplace submission state remains owned by the existing submission/publishing boundary.

### 3.3 Authorization

- Agent/Tool code inherits the same tenant/user/Organization identity authority as existing application code.
- No parallel Agent RBAC system.
- Every Commerce Tool adapter rechecks the domain authorization boundary; trusted Tool metadata alone is not enough.

### 3.4 Side effects

Current Product Agent preparation is read/compute/propose first.

Write/publish capabilities require separate review for:

- explicit human approval;
- proposal/base version binding;
- authorization;
- idempotency;
- remote unknown-state handling;
- audit;
- deterministic readiness immediately before the write.

---

## 4. Now

### 4.1 Close the Customer Trial / Production Readiness baseline

GitHub #33 is the umbrella release gate.

Current high-priority evidence:

1. #28 — SHEIN end-to-end commercial validation.
2. #30 — controlled 1688 path with durable lineage.
3. #36 — Proposal → Human Review → Explicit Apply.
4. #44 — release-critical E2E regression authority.
5. #47 — reusable production regression / future Agent-eval dataset.
6. #41 — production health/metrics/SLO baseline.
7. #48 — production security readiness.
8. #45 — first customer or controlled customer-like publishing rehearsal.

Do not expand later marketplace products to avoid doing these acceptance tasks.

### 4.2 Preserve current Product architecture

Required posture:

- Do not recreate old ProductEnrich/ProductImage task systems.
- New product rules belong to the current Product or Marketplace owner.
- ProductSnapshot and ApprovedAsset are authoritative read boundaries.
- Missing facts/assets fail visibly; do not silently choose a source image or hidden default.
- UI semantics should be Product/Listing-first rather than Task-first.

### 4.3 Finish Marketplace / Listing boundary hard-cut

#29 now represents remaining ownership cleanup, not a new universal MarketplaceAdapter project.

Focus on:

- moving pure platform rules out of root ListingKit where still necessary;
- keeping one submission state owner;
- preserving assembly/domain separation;
- exposing marketplace capabilities to Tools through narrow adapters;
- preventing TEMU/Amazon from copying independent Workbench/state models.

### 4.4 Stabilize deterministic validation

#34 must turn current readiness logic into a stable deterministic validator contract shared by:

- UI/Product/Listing projections;
- fixed pipeline;
- submission gates;
- Commerce Tool invocation;
- later Agent repair loops.

The model may propose repairs; it may not decide that a failed deterministic validator passed.

### 4.5 Keep operational evidence exact

For release-sensitive work:

- name the exact commit/candidate;
- link exact CI/build results;
- record controlled runtime acceptance separately;
- never mark an integration green because a provider-dependent test was skipped;
- record migration/rollback/security evidence for the exact candidate that will be deployed.

---

## 5. Next

### 5.1 AI Capability Platform — Phase 1

Complete #126/#130:

- provider-neutral model catalog/capabilities;
- tenant + capability policy;
- routing/fallback decisions;
- invocation ledger;
- usage/cost/latency/error taxonomy;
- prompt/policy/version traceability;
- health/budget/concurrency limits;
- rollback between legacy/shadow/active behavior.

Exit condition: Product Agent code never needs to select a provider, handle API keys, or become provider retry authority.

### 5.2 Commerce Tool Foundation — Phase 2A

#133 is complete. #134 remains open.

Completed:

- Tool Registry / governance contract — PR #272.
- `product.canonical.inspect@v1.0.0` — PR #295.

Still required for the minimum Product Agent PoC:

- source evidence reader;
- approved asset/product facts reader;
- enrichment analyze/propose;
- image analyze/propose through current Product Image/ImageAgent boundary;
- marketplace category/attribute rule query;
- deterministic readiness/validator Tool.

The PR #295 merge must not be interpreted as completing #134 or #128.

### 5.3 Product Agent — Phase 2B

Only after Phase 1 + Phase 2A exit evidence:

- #131 defines bounded runtime state/limits/stop reasons;
- #132 runs the Product Agent PoC against the fixed pipeline;
- #47 supplies the reusable evaluation dataset;
- #36 supplies human review/application;
- #34 supplies deterministic validation.

Required safety characteristics:

- feature flag + tenant allowlist;
- hard step/model/token/time/cost budgets;
- bounded repair loops;
- read/compute/propose tools only;
- structured evidence/confidence/unresolved issues;
- fixed pipeline remains available as fallback;
- no canonical Product or platform write on failed Agent runs.

The Agent only advances if measured quality improves without worsening risk metrics.

---

## 6. Later

### 6.1 SHEIN Listing Agent

After Product Agent/Tool contracts prove useful:

- #129/#135: diagnose SHEIN blockers and produce a reviewable Listing Proposal;
- #136: only then permit a controlled, human-approved, idempotent `save_draft` write path;
- autonomous formal publish remains out of the initial scope.

### 6.2 TEMU / Amazon expansion

#31/#32 now mean **platform capabilities in the shared Listing Center**, not separate TEMU/Amazon Workbenches.

Wait until:

- #33 commercial baseline is stable;
- #29 marketplace/listing boundary is stable;
- current SHEIN patterns are safe to reuse;
- Product/Tool/Agent boundaries are not being copied as platform-specific forks.

### 6.3 Resource acquisition / billing

Resource Ledger and Store lifecycle do not require redesign when payment arrives.

Later work should add approved acquisition sources to existing resource authority, such as Billing/Platform Finance, rather than restore the old tenant quota/task-limit model.

---

## 7. Superseded backlog abstractions

The following old abstractions must not be restarted as written:

- generic independent Listing Workspace framework (#27 closed);
- customer-facing Task Center as the primary product object (#35 closed);
- separate publish-result fact model (#38 closed; current events/results are authoritative);
- “build ZITADEL tenant/role model from scratch” (#39 closed; production acceptance moved to release gates);
- “create deployment documentation” (#40 closed; production execution evidence remains);
- old tenant quota/task-limit billing model (#42 closed);
- duplicate v0.1 Customer Trial checklist (#43 closed into #33);
- separate RC branch/release authority (#46 closed; current main/immutable-candidate/gated-release model remains).

Historical docs/issues remain useful as context, but they are not permission to reintroduce these architectures.

---

## 8. Do not do now

Do not start work that:

- recreates old ProductEnrich/ProductImage runtime ownership;
- introduces another Product fact source or asset approval source;
- makes Task/Workflow the default user navigation identity again;
- builds independent TEMU/Amazon product workbenches;
- creates another submission state machine or durable retry owner;
- lets Agent Runtime import GORM repositories, provider SDKs or marketplace clients directly;
- lets an Agent bypass deterministic readiness;
- lets an Agent write/publish without explicit independent write-safety gates;
- starts Multi-Agent before one bounded Product Agent proves measurable value;
- treats repository/local validation as production readiness;
- starts a new source/platform expansion to avoid completing current commercial evidence.

---

## 9. Current execution checklist

Before approving the next release-sensitive or architecture-sensitive PR:

```text
[ ] Exact base/main commit is named.
[ ] Current automated test/build results are visible.
[ ] Runtime/staging/production evidence is separately identified when required.
[ ] Product facts remain owned by Product Domain.
[ ] Approved assets remain owned by Product Asset/ImageAgent boundaries.
[ ] No old ProductEnrich/ProductImage workflow is reintroduced.
[ ] Task/Workflow remains execution infrastructure, not the new product IA.
[ ] Marketplace rules stay in marketplace/listing ownership.
[ ] App packages contain assembly/lifecycle logic, not new business rules.
[ ] No second submission state machine, scheduler, retry owner, RBAC system or Tool Registry is added.
[ ] Commerce Tool adapters enforce tenant/user authorization again at the domain boundary.
[ ] Agent work names its Phase 1 and Phase 2A exit evidence before Product Agent implementation.
[ ] Deterministic validation remains outside model authority.
[ ] Human approval/version/idempotency/audit gates are named before any new write side effect.
[ ] Release claims distinguish implemented/repository/local/staging/production validation.
```

---

## 10. Source of truth summary

Current order of authority:

1. `docs/product/ai-commerce-agent-platform-strategy.md` — long-term product direction and Agent phases.
2. This file — current repository/product reality and Now / Next / Later gates.
3. GitHub issue #137 — executable backlog order.
4. GitHub issue #33 — Customer Trial / Production Readiness release gate.
5. Current approved product/domain specs — bounded product/architecture contracts.
6. Dated validation notes / exact CI runs — runtime, staging and production evidence.
7. Completed implementation plans — historical execution evidence only.

When strategy and current maturity appear to conflict: **strategy determines where the platform is going; this current-status document determines what the repository is ready to do now.**
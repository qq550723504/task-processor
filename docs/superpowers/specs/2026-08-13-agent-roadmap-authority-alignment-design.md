# AI Commerce Agent Roadmap Authority Alignment Design

## Status

- Approved direction: 2026-08-13
- Repository baseline: `master` at `49815aad922d98bc4c7d12b90dcabf86b541df2a`
- Scope: documentation and GitHub backlog governance only

## Problem

The AI Commerce Agent Platform strategy currently places Product Agent PoC in
Phase 2 and Commerce Tool Contract in Phase 3. That order conflicts with the
open backlog:

- issue #132 requires the Product Agent to choose and call read-only tools;
- issue #133 requires Agent Runtime to obtain allowed tools through a registry;
- issue #134 requires the Product Agent PoC to use narrow Commerce Tool
  adapters rather than repositories, provider SDKs, or Agent-framework DTOs.

Executing the numbered phases literally would therefore require either a
temporary tool contract or direct domain access during the PoC. Both choices
would create a second boundary that must be removed later.

The documented authority chain has also drifted. The active
`docs/refactoring/current-refactoring-status.md` is calibrated to 2026-07-13
and still makes next-source selection the next growth step, while the strategy
and issue #137 now make AI capability governance and a bounded Product Agent
the next direction after the current commercial gates. In addition,
`docs/refactoring/next-phase-plan.md` describes a completed platform-aware
asset implementation but is still named as the immediate execution queue.

## Goals

1. Put the minimum Commerce Tool boundary before Product Agent implementation.
2. Keep one Tool contract instead of creating a disposable PoC interface.
3. Recalibrate the current-state authority to the latest verified repository
   baseline and distinguish merged implementation from production acceptance.
4. Make the documentation authority order and GitHub execution order agree.
5. Preserve existing issue numbers and discussion history.

## Non-goals

- No Agent Runtime, Commerce Tool, AI provider, marketplace, or persistence
  implementation.
- No new service, framework, state machine, scheduler, or workflow owner.
- No claim that SHEIN, 1688, TaskEvent V2, or Phase 0 production acceptance is
  complete.
- No closure of commercial validation issues.
- No immediate GitHub issue mutation before the documentation change is merged
  into `master`.

## Chosen Phase Model

### Phase 0: Commercial and trust baseline

Phase 0 remains unchanged in purpose. It owns the current release gates:

- SHEIN end-to-end commercial validation;
- controlled 1688 source-to-ListingKit acceptance with source lineage;
- ZITADEL tenant, role, and permission trust boundaries;
- human confirmation for AI-generated product changes;
- repeatable core-flow regression coverage and evaluation data;
- Customer Trial Readiness.

Merged code or operator tooling is not sufficient evidence for these gates.
Each gate requires the exact repository, CI, controlled-runtime, or production
evidence stated by its issue.

### Phase 1: AI Capability Platform stabilization

Phase 1 keeps the existing #126/#130 scope: provider-neutral routing, tenant
policy, invocation ledger, cost and usage evidence, fallback behavior, bounded
execution gates, and rollback semantics. Product Agent code must not select a
provider or handle an API key directly.

### Phase 2A: Minimum Commerce Tool Foundation

Issues #128, #133, and #134 move from Phase 3 to Phase 2A and become P0
prerequisites for the Product Agent PoC.

Phase 2A is intentionally narrow:

- `ToolDefinition`, version, capability, input/output schema, risk level,
  tenant/user permission requirements, timeout ownership, deterministic error
  taxonomy, audit metadata, and allowlist binding;
- only read, compute, and propose risk levels are executable;
- narrow adapters for source evidence, canonical product and facts, product
  enrichment proposals, image analysis proposals, marketplace rule lookup, and
  deterministic readiness/validation;
- no write or publish tool is enabled;
- no direct Agent access to GORM repositories, provider SDKs, marketplace
  clients, or framework-specific DTOs.

The exit gate is that fake and real Product Agent tests can use the same stable
Tool contract without adding a second compatibility interface.

### Phase 2B: Bounded Product Agent PoC

Issues #127, #131, and #132 become Phase 2B. Their dependencies are explicit:

- #127 depends on Phase 1 and the Phase 2A epic;
- #131 depends on #130 and #133 so its allowlist and tool-call model use the
  shared registry contract;
- #132 depends on #128, #131, #133, #134, #36, and #47, in addition to the AI
  Capability Platform gates.

The PoC remains feature-flagged, tenant-allowlisted, read/compute/propose-only,
bounded by step/model/token/runtime/cost limits, and evaluated against the
fixed pipeline. Failure must leave the fixed pipeline and canonical product
unchanged.

### Phase 3: Tool expansion and production hardening

Phase 3 no longer introduces the first Tool contract. It expands the proven
Phase 2A boundary only after the Product Agent PoC produces evidence. Candidate
work includes broader tool coverage, operational service-level objectives,
version migration policy, and additional marketplace adapters. Write and
publish tools remain excluded until their separate approval, idempotency, and
audit gates are satisfied.

### Phase 4 and later

The SHEIN Listing Agent remains after the Product Agent and Tool gates. Its
first release stays read/propose-only. Save-draft remains a separately approved
write boundary, and publish remains outside the initial Agent scope.

## Documentation Authority

The aligned authority order is:

1. `docs/product/ai-commerce-agent-platform-strategy.md` defines the long-term
   product direction and phase model.
2. `docs/refactoring/current-refactoring-status.md` defines what the current
   repository and production evidence allow now.
3. GitHub issue #137 maps those two documents into executable issue order.
4. Domain-specific plans and validation notes define acceptance inside their
   bounded area.
5. Completed implementation plans, including
   `docs/refactoring/next-phase-plan.md`, are historical execution evidence and
   do not override the active current-state document.

The current-state document will be recalibrated to the repository baseline
named in this design. It will record that:

- the guarded 1688 acceptance tool is merged, while controlled live acceptance
  and the complete preview/readiness evidence remain open;
- TaskEvent V2 observability and canary support are merged, while production
  rollout and the 14-day legacy-decoder observation gate remain open;
- no next product source or Product Agent implementation starts before its
  stated prerequisites are green or explicitly documented as blocked.

## Repository Changes

The implementation changes only these authority documents plus the required
design and implementation-plan records:

- `docs/product/ai-commerce-agent-platform-strategy.md`
- `docs/refactoring/current-refactoring-status.md`
- `docs/refactoring/next-phase-plan.md`
- `docs/superpowers/specs/2026-08-13-agent-roadmap-authority-alignment-design.md`
- `docs/superpowers/plans/2026-08-13-agent-roadmap-authority-alignment.md`

`next-phase-plan.md` will retain its completed implementation content. Only its
status and authority wording will change so it cannot be mistaken for the live
queue.

## GitHub Issue Changes

After the documentation PR is merged, update the existing issues without
creating replacements:

- #137: order Phase 2A before Phase 2B and describe Phase 3 as later expansion;
- #128: rename to `[EPIC][Phase 2A][P0] Commerce Tool Foundation` and narrow its
  exit gate to the minimum Product Agent foundation;
- #133: rename to `[Phase 2A][P0][Tools] ...` and retain the framework-neutral
  registry requirements;
- #134: rename to `[Phase 2A][P0][Tools] ...` and retain read/compute/propose
  adapter constraints;
- #127: rename to `[EPIC][Phase 2B][P0] Product Agent ...` and add the Phase 2A
  dependency;
- #131: rename to `[Phase 2B][P0][Agent Runtime] ...` and depend on #133;
- #132: rename to `[Phase 2B][P0][Product Agent] ...` and depend on #128, #133,
  and #134;
- #125: keep Phase 0 open and link the merged authority-alignment change as
  evidence for its documentation-authority condition.

Issue bodies must continue to distinguish repository implementation,
repository validation, controlled-runtime acceptance, and production
acceptance. No checkbox is marked complete without the corresponding evidence.

## Publication Order

1. Create the documentation changes on
   `codex/agent-roadmap-authority-alignment` from the verified `master`
   baseline.
2. Run documentation and architecture guards, placeholder scans, link/reference
   checks, and `git diff --check`.
3. Commit the design, plan, and authority-document changes in reviewable
   documentation-only commits.
4. Push the branch and open a Draft PR.
5. Merge only after review and required checks pass; merging remains a separate
   explicit authorization.
6. Re-read `master` and the live issue bodies after merge.
7. Apply the approved issue title, label, dependency, and roadmap-body changes.
8. Fetch every changed issue again and verify titles, labels, dependencies,
   phase order, and unchanged completion claims.

This order prevents GitHub from claiming an authority model that is not yet
present on the default branch.

## Validation

Repository validation for the documentation PR consists of:

```powershell
go test ./tests -count=1
git diff --check
rg -n "Phase 2|Phase 3|next-phase-plan|current-refactoring-status" `
  docs/product/ai-commerce-agent-platform-strategy.md `
  docs/refactoring/current-refactoring-status.md `
  docs/refactoring/next-phase-plan.md `
  docs/superpowers/specs/2026-08-13-agent-roadmap-authority-alignment-design.md `
  docs/superpowers/plans/2026-08-13-agent-roadmap-authority-alignment.md
rg -n "T[B]D|T[O]DO|implement l[a]ter|fill in d[e]tails" `
  docs/superpowers/specs/2026-08-13-agent-roadmap-authority-alignment-design.md `
  docs/superpowers/plans/2026-08-13-agent-roadmap-authority-alignment.md
```

The final scan must show one consistent order: Phase 0, Phase 1, Phase 2A,
Phase 2B, Phase 3, Phase 4. References to the old ordering are allowed only
when explicitly identified as the superseded state in this design.

## Rollback

Before merge, rollback is the normal branch/PR revert path. After merge, revert
the documentation commit before changing issue order again. Issue changes are
reversible by restoring the exact pre-change titles, labels, and bodies captured
during the post-merge preflight. No issue deletion, closure, or recreation is
part of this work.

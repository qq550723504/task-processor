# Architecture Documentation

## Goal

This index separates stable architecture rules from plans, runbooks, and
historical evaluations. Use the stable documents for code review and new
implementation decisions. Use plans and runbooks for context, not as newer
boundary rules unless they explicitly supersede a stable document.

## Approved Authorities by Responsibility

- [Final UI / IA](../product/final-ui-ia-authority.md): final navigation, naming
  and BusinessTask projection (#298), not current release capability.
- [Product Domain](../superpowers/specs/2026-09-01-internal-target-architecture-phase3-product-design.md),
  [Identity / Organization](../superpowers/specs/2026-08-30-shuomi-workbench-store-center-zitadel-multi-org-design.md)
  and current Store/Resource/CommerceTool contracts: facts, access and side effects.
  Preserve valid domain rules even when a document also contains older UI or
  implementation observations; explicit supersession controls the affected section.
- [Hard-Cut Policy](../refactoring/legacy-hard-cut-policy.md),
  [Legacy Register](../refactoring/legacy-register.md) and
  [Module Mapping](../refactoring/module-target-mapping.md): only EXTRACT / RETIRE;
  existing legacy is not a new dependency permission.
- [Current status](../refactoring/current-refactoring-status.md): baseline-bound
  implementation and evidence. [#137](https://github.com/qq550723504/task-processor/issues/137)
  and execution Issues own scheduling and scope under the
  [dispatch rules](../engineering/issue-driven-development.md).
- [Sourcing guide](../product/product-sourcing-handoff.md) and
  [closeout](../product/product-sourcing-mvp-plan.md) distinguish current Product
  wiring, prepared imports and target acceptance. The
  [greenfield baseline](../product/greenfield-no-legacy-migration.md)
  (**PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08**) requires a fresh installation
  and empty business data; historical migration, profile-reuse and environment
  preflight guidance is not a current acceptance prerequisite.
- [next-phase-plan.md](../refactoring/next-phase-plan.md): HISTORICAL implementation
  record, not an execution queue. The [Publication Identity draft](../refactoring/2026-09-05-publication-identity-cutover-issue-draft.md)
  is SUPERSEDED under #307, not a migration prerequisite.

ACTIVE contracts, CURRENT STATE observations and HISTORICAL/SUPERSEDED evidence
serve different purposes. An old Active label or a newer date cannot override
an approved contract. Follow the explicit responsibility/supersession above.

## Recommended Reading Order

After identifying the applicable approved authorities above, review structural rules in this order:

1. Start with `project-boundaries.md` for default package ownership,
   dependency direction, forbidden imports, and repository-wide placement rules.
2. Then open the most relevant specialized boundary document such as
   `httpapi-assembly-boundaries.md`, `app-assembly-boundaries.md`, or
   `platform-boundary-strategy.md`.
3. Use `architecture-review-checklist.md` to turn those rules into concrete PR
   review questions.
4. Use `next-steps.md` only as the current guard coverage ledger, not as a
   competing source of architecture policy.

If a specialized document appears broader than `project-boundaries.md`, treat
`project-boundaries.md` as the default review entrypoint and tighten the
specialized note instead of creating a second top-level policy.

## Stable Boundary Documents

Use these as the main source of truth for structural work:

- `project-boundaries.md`
  - default package ownership, dependency direction, forbidden imports, and
    placement rules for new code
- `product-agent-runtime-contract.md`
  - #131 bounded Product Agent execution contract and #132 consumer gaps;
    contract/fake execution does not imply real model or product availability
- `commercial-wallet-billing-contract.md`
  - canonical money/wallet, commercial offer/quote/order, and purchased-resource ownership contract for #457
- `ecosystem-services-v1.md`
  - #603 frozen IMPLEMENTATION_READY third-party onboarding, original service purchase, exact customer acceptance and channel settlement contract; real channel qualification and product acceptance remain separate gates
- `self-service-subscription-purchase-contract.md`
  - tenant self-service subscription offer/quote/order, source-bound subscription activation, settlement and recovery contract for #478/#479
- `store-center-current-application-v1.md`
  - #552 current Console Store record management, owner pools, quota, authorization and single-state hard-cut; service activation remains a separate unopened capability
- `store-center-platform-observations-v1.md`
  - #614 IMPLEMENTATION_READY readonly SHEIN products, consumer orders and logistics;
    scoped observations, original-member sync receipts and bounded recovery. Shared
    runtime readiness and real merchant/user acceptance remain separate evidence.
- `httpapi-assembly-boundaries.md`
  - HTTP API ownership, route/module builder boundaries, and app/httpapi limits
- `app-assembly-boundaries.md`
  - app-layer build/register/start/coordinate vocabulary and package roles
- `temporal-boundaries.md`
  - Temporal versus RabbitMQ responsibilities and workflow/runtime boundaries
- `platform-boundary-strategy.md`
  - historical platform, publishing, ListingKit, and platform registration
    convergence roles
- `historical-platform-migration-inventory.md`
  - retained ownership/inventory evidence for historical platform packages;
    candidate slices require current mapping and an execution Issue
- `external-client-boundary-inventory.md`
  - local-interface rules and baseline-bound adapter hotspots; suggested
    slices do not override current owners or Issue scheduling
- `compatibility-retirement.md`
  - retired compatibility paths, replacement owners, and guard tests
- `listing-preview-boundaries.md`
  - platform-neutral preview ownership, ListingKit facade limits, and guard
    tests for preview extraction
- `architecture-review-checklist.md`
  - repeatable PR review checklist for boundary-sensitive changes

## Development Boundary Documents

These documents live outside `docs/architecture`, but still define long-lived
structure rules that should be reviewed with architecture changes:

- `docs/development/repository-structure.md`
  - top-level directory ownership, local artifact placement, and repository
    layout guard tests

Use this development document when the question is mainly about repository
layout, entrypoint placement, or runtime artifact location. If the question is
mainly about package ownership or dependency direction, start from the default
project boundary entrypoint first and only then drop to the development
document.

## Current Guard Baseline

Use `docs/architecture/next-steps.md` and its `Current guard coverage` section
as the current guard coverage baseline for active import-boundary tests. Formal
review actions should still start from
`docs/architecture/architecture-review-checklist.md`. This baseline tracks what
reviewers must keep visible while the stable boundary documents remain the
source of truth for long-lived rules.

## Supporting Context

- [`ai-workbench-reports-v1.md`](./ai-workbench-reports-v1.md)
  - #628 frozen IMPLEMENTATION_READY personal report snapshots, favorites and
    exact source-version capture. Shared runtime wiring and actual user acceptance
    remain separate from feature-local implementation and fixture evidence.
- [`tool-market-v1.md`](./tool-market-v1.md)
  - Native composition and local installation limits: [current-tool-market operations](../operations/current-tool-market.md).
  - #613 frozen IMPLEMENTATION_READY feature Design Basis for enterprise-shared enablement and manual customization progress; existing capture permissions and offline quote/payment owners remain unchanged. This feature contract does not introduce repository-wide structural rules.
- [`my-supply-chain-shein-v1.md`](./my-supply-chain-shein-v1.md): #605 的已评审冻结 Design Basis，Collection → Preparation → Target → Submission 完整交付及三种官方应用类型；运行说明见 [operations](../operations/my-supply-chain-shein-v1.md)。
- [`notification-center-v1.md`](./notification-center-v1.md)
  - #608 notification-center design and current-business source mapping. Admission
    is stated in that document and the execution Issue; a draft is not permission
    to implement or a claim that notification sources have been delivered.

- [`issue-36-local-trial-runtime.md`](./issue-36-local-trial-runtime.md)
  - IMPLEMENTATION_READY #36 durable loopback runtime and independently reviewed
    post-#590 Task Center entry correction for existing Review/Apply and Listing
    completed-work paths. Product validation remains separate.
- [`issue-36-completed-work-store-scope.md`](./issue-36-completed-work-store-scope.md)
  - IMPLEMENTATION_READY #36 bounded completed-work v2 projection of Listing's
    historical Store ID and local preparation action; no Store name read,
    remote publication claim, or Product Review dependency.
- [`ai-workbench-project-center-v1.md`](./ai-workbench-project-center-v1.md)
  - APPROVED / IMPLEMENTATION_READY #624: creator-private current-enterprise
    long-term projects, manually authorized references and personal templates;
    independent persistence/runtime, receipt-first replay and slot removal.
    Does not own execution, source facts or shared project permissions.
- [`ai-workbench-chat-business-task-v1.md`](./ai-workbench-chat-business-task-v1.md)
  - APPROVED / IMPLEMENTATION_READY #576 Slice E: owner-scoped Conversation,
    no-tool Eino/eino-ext planning, immutable execution proposals and BusinessTask,
    exact model-profile/replay semantics and current AgentRun/Review projections.
    Reuses #580 title policy/organization-only credentials; native protocols and
    run-profile binding are approved implementation contracts, not deployed capability.
    Reviewed contract HEAD `620495bf03b57099e4b934f89c04e0d989d543d7` received
    independent no-major-issues verification and CI `37032794759` SUCCESS.
    Production Writer waits for PR #578 merge and execution-Issue admission;
    final-head merge checks and separate rollout/provider permissions still apply.

- [`enterprise-custom-roles-v1.md`](./enterprise-custom-roles-v1.md)
  - IMPLEMENTATION_READY #598 Figma members and enterprise custom roles, native slot inventory, scoped permission consumers and atomic mutation receipts.

- [`member-directory-query-v1.md`](./member-directory-query-v1.md)
  - #575 bounded current Membership query increment: complete display-name/login
    search, native role/state filtering, filtered totals and paging. IMPLEMENTATION_READY;
    no new IAM, persistence owner or shared runtime authority.

- [`organization-agent-configuration-v1.md`](./organization-agent-configuration-v1.md)
  - APPROVED / IMPLEMENTATION_READY #570 Slice D: enterprise Agent enablement,
    immutable templates/defaults and start snapshots, capability projections,
    same-database activation/Claim ordering and existing Product Agent consumption.
    Final reviewed contract HEAD `88b2be970b470fab45d1897d89b41de4061020ec`;
    targeted review found no major issues and CI `36566072746` completed SUCCESS
    including Required CI Gate. Production Writer starts only after PR #571 merges
    to main; rollout/provider use remain separately gated.

- [`agent-customization-v1.md`](./agent-customization-v1.md)
  - APPROVED / IMPLEMENTATION_READY #611: enterprise customization requests,
    contact consent, private files and verified-platform manual progress.
    Submission is free; proposals and fees are confirmed offline. Runtime
    assembly and user acceptance remain separate from the owner implementation.

- [`private-agent-delivery-v1.md`](./private-agent-delivery-v1.md)
  - APPROVED / IMPLEMENTATION_READY #611: fixed-version platform draft checks
    privately delivered to the original request enterprise, atomic publication,
    actor-private immutable reports and current source authorization. Existing
    Listing saved validation is reused;
    no model/provider, product mutation or second generic Agent runtime.
  - Bounded isolated trial admission: [`private-draft-offline-trial.md`](./private-draft-offline-trial.md).
    Explicit offline test scope/store and seven existing native read routes;
    test-rule reports remain distinct from real platform acceptance.

- [`product-agent-text-provider-neutral-v1.md`](./product-agent-text-provider-neutral-v1.md)
  - IMPLEMENTATION_READY #573 architecture for organization-scoped,
    provider-neutral title text admission over OpenAI-compatible routes. The
    user's new supplier decision supersedes the old GRSAI-only requirement;
    actual paid execution still requires route-specific metering, limits,
    pricing and paid-call authorization.

- [`product-agent-google-interactions-v1.md`](./product-agent-google-interactions-v1.md)
  - IMPLEMENTATION_READY #573 Google Gemini 3.8 Flash native Interactions route
    increment after independent architecture review. The
    existing provider-neutral identity, points and UNKNOWN owners remain.

- [`account-audit-summary-v1.md`](./account-audit-summary-v1.md)
  - IMPLEMENTATION_READY #478 four true Account Audit totals over the complete
    30-day window of current committed owner events, preserving live Organization
    authorization, canonical operation identity and existing list pagination.
    Independent Store/Billing history, rollout and product acceptance remain outside
    this bounded increment.

- [`account-audit-filters-v1.md`](./account-audit-filters-v1.md)
  - IMPLEMENTATION_READY #581: bounded cross-source content, time and affected
    member filters with complete matching pagination and unchanged fact owners.

- [`account-audit-usage-readers-v1.md`](./account-audit-usage-readers-v1.md)
  - IMPLEMENTATION_READY #587: two independent read-only invocation owner pools
    and fresh Account Compose ledger initialization for the existing filtered
    audit contract; no Agent execution or historical backfill.

- [`unified-base-prepaid-resources-v1.md`](./unified-base-prepaid-resources-v1.md)
  - IMPLEMENTATION_READY design basis for #478/#564: current Figma base plan, native Stores, prepaid purchases, real resource events and Product Agent point metering; supersedes subscription/quota/Token entry paths without changing repository package-boundary rules.

- [`commercial-retail-pricing-v1.md`](./commercial-retail-pricing-v1.md)
  - IMPLEMENTATION_READY #600: approved Store CNY 168 per 30-day period, AI-point purchase CNY 0.01 per point, create-only catalog installation and price-only public projection; unchanged wallet/resource/Store contracts.

These documents are useful background, but should not override stable boundary
documents unless they say so explicitly:

- [`member-resource-allocation-v1.md`](./member-resource-allocation-v1.md)
  - IMPLEMENTATION_READY #561 member Store grants, period/data allocation and
    independent Resource settlement from native Store/Product proofs, including
    official SHEIN authorization. The existing admitted Product composition
    constructs its exact-read and charge-proof adapters; candidate implementation
    does not imply runtime rollout or real merchant authorization acceptance.

- [`agent-knowledge-context-v1.md`](./agent-knowledge-context-v1.md)
  - IMPLEMENTATION_READY #555/#556 enterprise Knowledge + existing Product Agent V1:
    Organization-scoped Knowledge, bounded document lifecycle, immutable context bundles,
    exact citations, dispatch-permit disable fence and Human Review provenance. Production
    Writer starts only after PR #556 is merged to main; rollout/provider use remains gated.

- [`account-facts-invitations-v1.md`](./account-facts-invitations-v1.md)
  - IMPLEMENTATION_READY Design Basis for the bounded #551 Account Center
    delivery: user facts, region preferences, complete enterprise statistics and
    consent-based email invitations. It does not supersede repository structural
    boundaries; candidate delivery is not browser acceptance or runtime rollout.
- [`settlement-reversal-locking.md`](./settlement-reversal-locking.md)
  - IMPLEMENTATION_READY #413 repair of ordinary refund/chargeback concurrency
    while preserving immutable canonical settlement privileges and replay bounds
- [`subject-verification-tencent-esign-design.md`](./subject-verification-tencent-esign-design.md)
  - #510 bounded subject-verification design. Section 13 freezes the Tencent
    enterprise first-verification contract and section 14 freezes the Aliyun
    personal KYC contract; both are IMPLEMENTATION_READY and have merged
    implementations. Real provider trials and production rollout remain separate gates.
- [`referral-withdrawal-personal-kyc-contract.md`](./referral-withdrawal-personal-kyc-contract.md)
  - IMPLEMENTATION_READY #519 cross-domain admission contract: a new referral
    withdrawal consumes the same subject's authoritative personal KYC VERIFIED
    fact through a narrow read port; includes committed-operation replay and the
    staged v1/v2 rules rollout required by the API-first deployment order.
- [`referral-settlement-and-conduct.md`](./referral-settlement-and-conduct.md)
  - IMPLEMENTATION_READY #469 contract for a 30-day period on newly recorded
    referral earnings, immutable stored maturity deadlines during payment replay,
    and promotion conduct guidance bounded by existing processing capabilities.
- `alipay-wallet-topup-design.md`
  - DESIGNING #481 WeChat Pay and Alipay wallet top-up architecture using GoPay;
    fixed-channel attempts, typed checkout, isolated verification, posting/refund
    receipts and recovery. Historical filename; one dual-channel design.
    Desktop checkout, third-party payer, zero top-up commission and admin-approved
    refunds are approved and incrementally reviewed. Economic amount mapping remains;
    not IMPLEMENTATION_READY or a real-payment authorization. Includes exact Figma
    recharge-page and modal references.
- `2026-09-27-1688-anonymous-rate-and-egress-proxy-addendum.md`
  - Draft addendum to the #514 browser acquisition design: anonymous collection
    rate governance (the observed 1688 challenge is frequency-triggered) and the
    constraints any egress proxy must satisfy. **Proposal only — neither the
    rate-governance half nor the egress-proxy half is implemented.** Implementation
    waits on admission of this document and an explicit `IMPLEMENTATION_READY`.
- `2026-09-26-1688-server-public-browser-acquisition-design.md`
  - FROZEN BASELINE / IMPLEMENTATION_READY #514 `src2b-public-browser-v1`:
    server-side anonymous 1688 browser acquisition with automatic challenge
    handling, reusing the existing `AcquisitionEvidence` → SourceEnvelope →
    SRC-1 → Catalog chain. The design §12-A open items were resolved by user
    decision on 2026-09-26 and are recorded in §12-A'; the real-network
    acceptance result is recorded in §12-A''. Does not supersede approved
    sourcing contracts, and does not by itself authorize production wiring or
    deployment.
- `2026-10-03-acquisition-channel-pricing.md`
  - IMPLEMENTATION_READY #592 contract for 5 fen per published server-side
    1688 acquisition and free Browser Capture extension/local executor submission;
    preserves the existing Product publication and prepaid resource owners.
- `project-target-architecture.md`
  - target architecture context; use stable boundary documents for current
    review policy
- `auth-and-tenancy.md`
  - verified effective Organization, home identity distinction, route
    authorization and remaining legacy paths; supporting context, not a new IAM
- `task-status-lifecycle.md`
  - status lifecycle context; use stable boundary documents for package
    ownership and dependency rules
- `temu-architecture-patterns.md`
  - TEMU architecture pattern context; use stable boundary documents for
    cross-platform dependency rules
- `temu-pipeline-stages.md`
  - TEMU pipeline stage context; use stable boundary documents for runtime and
    assembly boundaries
- `listingkit-refactor-status.md`
  - ListingKit refactor status context; use stable boundary documents for
    long-lived ListingKit boundaries
- `amazon-crawler-runtime-flow.md`
  - Amazon crawler runtime flow context; use stable boundary documents for
    review policy
- `task-event-v2-migration.md`
  - RabbitMQ complete-task event V2 schema, compatibility window, and removal
    gate; Listing Control ID-only dispatch is out of scope
- `openmeter-shadow-metering-poc-report.md`
  - time-bounded, evidence-backed local PoC decision; it does not authorize a
    production integration, billing, payment, deployment, or data migration
- `pay-041-usage-ledger.md`
  - time-bounded PAY-041 reconciliation evidence and PAY-042 handoff; it does
    not authorize production data repair, entrypoint cutover, or billing
    integration
- `pay-042-listingkit-generation-usage-cutover.md`
  - time-bounded first-slice PAY-042 generation settlement boundary; it does
    not authorize payment-provider changes or enable the rollout flag

## Plans, runbooks, and evaluations

Documents with names such as `*-plan.md`, `*-runbook.md`, `*-evaluation.md`,
`*-checklist.md`, `*-status.md`, `*-playbook.md`, `*-validation.md`,
`*-split.md`, or `*-management.md` are normally time-bounded. Names are discovery hints, not retirement decisions. They may explain
why a decision was made. Still-approved domain contracts remain effective; link
applicable boundary rules into stable entries for review policy. Classify mixed
sections by explicit supersession and evidence, never mechanically by filename.
Every architecture document must be either indexed above or match a
time-bounded context pattern.

## Working Rule

When a structural question comes up, start with this index. If two documents
appear to disagree, first apply the approved responsibility/supersession above;
within that boundary, prefer the stable boundary document and update the older
contextual note with a link instead of creating a third interpretation.
Every stable or development boundary document must have a document test before
it is treated as a long-lived review entrypoint.

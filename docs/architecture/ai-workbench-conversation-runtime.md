# 硕米 Chat 会话管理独立运行接线

> Status: DRAFT / REVIEW_PENDING
> Design Basis: Independent Architecture (bounded runtime and authorization-consumer increment).
> Execution: Issue #619 / PR #631, one existing Writer/worktree.
> Baseline: 1994e8348e6b53b9e356b330b5ec123f89ecab29; origin/main 60d432aa8b9391491278912e4268950ba65b58f0.

## Product outcome and authority

2026-10-11 user requested 硕米Chat integration, confirmed no model configuration, and explicitly chose “先接入真实会话管理，模型待配置”. Current authorized result: an admitted trial-enterprise member can create/reopen/rename/favorite/archive/restore their own durable conversations in the retained unified program. Model messages, proposals, title execution, task mutation, paid calls and production are not opened.

Reuse current Product/Figma authority: [final-ui-ia-authority.md](../product/final-ui-ia-authority.md), Figma tg48P46SSXl6TBy9lZwg63 / 31:463, Chat 427:2467. Preserve current Chat pages and original metadata actions; no new page/design system/general-purpose chat capability. [ai-workbench-chat-business-task-v1.md](ai-workbench-chat-business-task-v1.md) remains canonical for full planning/execution. This user-selected stage only replaces its startup dependency for conversation management; full-mode requirements are unchanged.

Current defect: configuration/build requires an enabled ProductAgent, title policy, planning policy, limits and all Agent/Review/Asset dependencies before the existing Conversation store can be served. The retained installation has none of these policies or ai_workbench schema. Fabricating policies/prices/limits to make the UI visible is forbidden.

## Owner and contract → implementation → injection → consumer

- Existing ai_workbench schema/store remains sole Conversation, command, proposal and BusinessTask owner. Existing schema/column grants and optimistic revision/create receipt semantics remain unchanged. No new tables or fact owner.
- Extend trusted AIWorkbench deployment configuration with explicit conversationOnly and allowedOrganizationIds. In this mode require the existing ProductAgent logical database, separate ai_workbench_runtime user/pool <=8, a bounded unique organization allowlist, no planning policies and no enabled ProductAgent. Default/false remains full mode, with original dependency/policy validation.
- Current-application composition injects the existing live organization resolver and authorizer directly into this mode. Extract the existing freshWorkbenchIdentity behavior into a bounded shared helper; retain the same binder, signed original actor/home/effective org, token expiry, LiveWrite re-resolution, active/current membership, exact permission and allowed-org checks. ProductAgent full mode calls the same helper without semantic changes.
- Full mode still builds original governed planner/execution ports. Conversation-only mode builds only the existing Store and no planner/executor/AI credential/Commercial/Agent/Review/Asset consumer.
- Module admission uses its trusted mode allowlist; original native aiWorkbenchAvailable exposes mounted/admitted storage. PlanningReadiness and TitleReadiness are UNAVAILABLE in conversation mode and never fabricate credentials/model/price/budget.
- Existing routes/BFF remain. After normal live authorization, message/confirm/task start/resume/review reject before receipt, mutation, domain executor or external I/O. Conversation GET/POST/PATCH keep original read/use permissions and bounds. Existing task history GET may read owner intents, but absent Agent projections remain unavailable with all action flags false. Proposal product/knowledge/model details require the original owners; absence hides those details. Do not mark a stored task complete or reinterpret UNKNOWN.
- Existing Project conversation links reuse the same store; no new relationship facts. Existing task references retain unavailable projections.
- Chat creation uses existing chat.use permission independently of canPlan, since original owner Create is model-free. Model send/confirm remain readiness + existing permissions gated, with server-side rejection in this mode. Rename/favorite/archive/restore keep original revision/idempotency handling.

## State, storage and recovery

No new state machine or receipt semantics. Same org+actor scopes, same create idempotency key, same revision conflict, same archive behavior. Empty/model-disabled conversations are actual owner facts, not samples. GET results remain private/no-store, existing page/response/body/deadline limits apply.

Install existing empty ai_workbench schema into current product_agent logical DB with its owner-only installer and a newly bounded runtime role, only when that schema/role is absent. Do not reinstall Product/native/E, modify their facts, migrate/delete data or run auto-DDL in serving. Keep owner/admin secrets only in the one-time installer; serving receives only its private generated Chat manifest and bounded role password.

Add one optional final Chat overlay consuming the existing E manifest. Retained installer verifies completion/input/output identity and original secret rather than rerunning installation or manufacturing a replacement password. Interrupted fresh install fails for operator inspection; no automatic repair/retry. Preserve all original named volumes and existing stop/start layers, add only Chat installation/runtime state. No credential provisioner/provider/payment consumer is invoked.

Later full-model activation still needs original exact title/planning routes, credential versions, pricing/tariffs, approved limits, ProductAgent/Review/Asset installation and separate provider authority. conversationOnly must be disabled explicitly; do not auto-select another model or reuse a global/fixture credential. Durable Conversation facts stay in the same original store; no backfill/compatibility path.

## Threat model / scope

Must: current identity/current membership + exact read/use authorization; no cross-org/actor access; no permissions granted by flags; no paid/external mutation or fake success; existing durable facts/UNKNOWN preserved; no credentials in UI/log/repository. Full execution security/owner/recovery contracts remain frozen.

Out of Scope: provider selection/pricing, general chatbot, unsent-message draft feature, new model/Agent/Review/IAM/Commercial owner, task/AI execution, Project feature expansion, migration/cleanup, CI/protection changes, acceptance runner, production/merge/Issue close. Legacy decision: N/A; no retired abstraction or fallback.

## Necessary verification and handoff

TDD captures current startup rejection without ProductAgent/model policies and UI create blocked by readiness. Validate explicit opt-in, missing/duplicate allowlist/role/DB mismatch, no accidental full-mode relaxation. Test the extracted fresh authorization and real scoped Conversation owner CRUD/replay/revision using existing integration facilities, including other actor/org and current permission removal.

Direct message/confirm/task mutations in conversation mode must produce unavailable without changing receipts/messages/proposals/tasks/invocations or calling execution. Original full-mode tests remain valid; only affected checks run locally. Verify task/proposal/Project absent-owner projections do not panic or imply success.

Production build + ordinary Edge login/current OrgA: root AI now lands in Chat, create/reopen/rename/favorite/archive/restore a clearly named local trial conversation, empty/model-disabled state and direct no-model gate; normal stop/start retains it and all pre-existing volumes/session. Such fixture-free local trial data are authorized by this conversation-management task, not real customer data; do not delete them. Independent final increment review and exact-HEAD CI are distinct from user acceptance, which remains NOT_RUN.


# ListingKit Product Design V1 — Phase 2 Product Workspace Implementation Plan

**Date:** 2026-08-27
**Base:** `feature/listingkit-product-v1-phase1` @ `3993a852d359a30d65365d95129e0f12623cbc42`
**Branch:** `feature/listingkit-product-v1-phase2-workspace`
**Scope:** Product Workspace shell and information hierarchy only. Preserve existing SHEIN review/final-review/actions, API contracts, task/workflow routing, and tenant scope.

## Goal

Replace the current long vertical task/workflow page with a product-oriented three-column workspace while preserving the existing business execution logic.

Target composition:

```text
Product header

Left product navigation | Existing review/editor surface | AI review / attention rail

Bottom execution/action area
```

The implementation is intentionally progressive: this phase changes the workspace frame, hierarchy, and language first. It does not rewrite SHEIN review engines, Temporal actions, API contracts, or product persistence.

## Non-goals

- no database migration;
- no stable Product ID migration;
- no replacement of existing `taskId` routes;
- no rewrite of `useWorkspaceData`, `useSheinWorkspaceActions`, or platform adapters;
- no cross-tenant browsing or tenant selector inside Product Workspace;
- no generic AI chat panel;
- no TEMU/Amazon editor implementation beyond platform navigation placeholders derived from existing platform cards.

## Multi-tenant constraints

1. Workspace continues to inherit the effective tenant from the authenticated/delegated request context.
2. No tenant ID is editable inside Product Workspace.
3. No tenant filter is added to the workspace.
4. Existing workspace actions continue to execute under the originating tenant/task context.
5. Phase 2 must not modify tenant switching, ZITADEL identity propagation, or API tenant forwarding.

---

## Task 1 — Add a product workspace presentation model

### Files

Create:
- `web/listingkit-ui/src/components/listingkit/workspace/product-workspace-model.ts`
- `web/listingkit-ui/src/components/listingkit/workspace/product-workspace-model.test.ts`

### RED test first

Add pure-model tests that assert:

1. canonical navigation contains `概览 / 图片 / 基础信息 / SKU / 规格 / 属性 / 描述`;
2. platform navigation is derived from existing platform cards and preserves selected platform;
3. execution terminology is excluded from primary navigation (`Task`, `Temporal`, `Queue`, raw task ID);
4. attention summary maps review state into product language:
   - blocking → `必须处理`
   - warning/review → `建议确认`
   - resolved/trusted → `已通过` where data is available;
5. selected SHEIN platform remains compatible with existing workspace navigation targets.

Run:

```bash
cd web/listingkit-ui
npm test -- product-workspace-model.test.ts
```

Expected RED: module missing.

### Minimal implementation

Define view-model types only; no React state:

```ts
export type ProductWorkspaceSectionKey =
  | "overview"
  | "images"
  | "basic"
  | "sku"
  | "specs"
  | "attributes"
  | "description";

export type ProductWorkspaceNavItem = {
  key: string;
  label: string;
  platform?: string;
  selected?: boolean;
  status?: "ready" | "processing" | "attention" | "failed" | "idle";
};
```

Expose pure builders for canonical navigation, platform navigation, and attention summary. Keep inputs narrow so existing `workspace-screen.tsx` can adapt without changing backend types.

### GREEN verification

Run the focused test again and require zero failures before Task 2.

Commit:

```text
feat: add Product Workspace presentation model
```

---

## Task 2 — Add three-column workspace frame components

### Files

Create:
- `web/listingkit-ui/src/components/listingkit/workspace/product-workspace-shell.tsx`
- `web/listingkit-ui/src/components/listingkit/workspace/product-workspace-shell.test.tsx`
- `web/listingkit-ui/src/components/listingkit/workspace/product-workspace-navigation.tsx`
- `web/listingkit-ui/src/components/listingkit/workspace/product-workspace-navigation.test.tsx`

### RED tests first

`ProductWorkspaceShell` must expose accessible regions:

- `商品工作台导航`
- `商品工作区`
- `AI 审核`
- optional `商品操作`

Responsive behavior:

- desktop: three-column grid;
- narrow screens: stacked layout;
- no fixed viewport width that creates horizontal overflow.

`ProductWorkspaceNavigation` must render:

```text
商品资料
  概览
  图片
  基础信息
  SKU
  规格
  属性
  描述

平台资料
  <derived platforms>

历史
```

It must not display a tenant selector or Task ID.

Run focused tests and observe RED before implementation.

### Minimal implementation

Use existing Tailwind/shadcn patterns only. Do not introduce a new design system dependency.

Recommended desktop grid:

```text
xl:grid-cols-[220px_minmax(0,1fr)_320px]
```

Use sticky behavior only at `xl` sizes where it does not trap mobile scrolling.

Commit:

```text
feat: add three-column Product Workspace shell
```

---

## Task 3 — Add contextual AI review rail without a chatbot

### Files

Create:
- `web/listingkit-ui/src/components/listingkit/workspace/product-workspace-ai-review.tsx`
- `web/listingkit-ui/src/components/listingkit/workspace/product-workspace-ai-review.test.tsx`

### RED tests first

The panel must:

1. render severity groups using product language (`必须处理`, `建议确认`, `已通过`);
2. display human-readable review reason where available;
3. expose an action callback for a review item without invoking task APIs directly;
4. avoid raw task/workflow IDs;
5. render a clear empty/success state when there are no unresolved review items.

The component is presentation-only. Existing handlers such as `handleAction`, `handleRecovery`, and `handleSelectSheinBlockingItem` remain owned by `workspace-screen.tsx`.

### Minimal implementation

Prefer a compact rail of issue cards. Reuse existing `Badge`, `Button`, `Card` primitives. Do not embed `ReviewReasonsCard` directly if it exposes task-oriented language; adapt its data into the new presentation component.

Commit:

```text
feat: add contextual Product Workspace AI review rail
```

---

## Task 4 — Recompose `WorkspaceScreen` around Product Workspace

### Files

Modify:
- `web/listingkit-ui/src/components/listingkit/workspace/workspace-screen.tsx`

Create or modify tests as needed:
- `web/listingkit-ui/src/components/listingkit/workspace/workspace-screen-product-layout.test.tsx`
- existing workspace helper/view tests only when expectations intentionally change.

### RED tests first

Add an integration-level component test with mocks for existing hooks. Assert the rendered hierarchy contains:

- product workspace navigation;
- central work area;
- AI review rail;
- existing SHEIN review/final-review content in the central region;
- `查看执行记录` as a secondary/advanced action;
- no top-level `TaskStatusPanel` before the main product workspace;
- no visible raw Task ID in the product frame.

Also assert existing action hooks remain wired:

- standard product generation action key remains `run_standard_product_temporal`;
- platform adapt action key remains `run_platform_adapt_temporal`;
- SHEIN blocking-item navigation still delegates to `handleSelectSheinBlockingItem`.

### Minimal implementation strategy

Keep all current data/hooks before the `return` unchanged unless required for presentation adaptation.

Move current surfaces as follows:

| Existing surface | New placement |
| --- | --- |
| `WorkspaceHeader` | replace/wrap with product-oriented header inside shell |
| `TaskStatusPanel` | secondary execution area, not top-level primary content |
| `ReviewReasonsCard` | data source replaced by AI review rail presentation |
| `SDSRepairPanel` | remains modal/context action |
| `TaskProgressNotice` | compact contextual progress near main/action area |
| `PlatformCardRail` | represented in left platform navigation; keep only if needed for unsupported platform cards |
| `SheinFlowNav` | lightweight central platform progress indicator |
| `WorkspaceReviewView` | central work area |
| `SheinFinalReviewWorkspaceView` | central work area |
| `SheinAdvancedReviewDetails` | central advanced details where currently required |
| `WorkspaceOverviewPanel` | central overview/secondary section |
| `TaskRevisionHistoryPanel` | history/advanced section; no longer primary page bottom emphasis |

Do not delete existing components in this phase; preserve rollback capability.

Commit:

```text
feat: recompose WorkspaceScreen around Product Workspace
```

---

## Task 5 — Tenant-scope and regression verification

### Required checks

Run full frontend validation:

```bash
cd web/listingkit-ui
npm run lint
npm run typecheck
npm test
npm run build
```

Use repository CI to additionally verify backend/race/build jobs.

Review changed filenames and require:

- no backend tenant middleware/auth changes;
- no API tenant forwarding changes;
- no subscription/tenant switch logic changes;
- no database migration;
- no change to existing workspace/status route identity.

Confirm tenant-related existing tests still pass, especially:

- ZITADEL/proxy identity tests;
- tenant store tests;
- tenant-scoped AI settings tests;
- Phase 1 product-navigation tenant-context test.

### Acceptance criteria

Phase 2 is accepted when:

1. Product Workspace renders a clear three-column product-oriented hierarchy on desktop and a usable stacked layout on narrow screens;
2. existing SHEIN review/final-review behavior remains reachable and functional;
3. AI review is contextual and action-oriented, not chat-based;
4. raw task/workflow identity is secondary;
5. no tenant boundary behavior changes;
6. all CI jobs pass on the Phase 2 branch.

---

## Delivery / PR strategy

Create a stacked Draft PR:

```text
feature/listingkit-product-v1-phase2-workspace
    ↓
feature/listingkit-product-v1-phase1
    ↓
main
```

This keeps Phase 2 review focused. After Phase 1 merges, retarget/rebase Phase 2 onto `main` without changing behavior.

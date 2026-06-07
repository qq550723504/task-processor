# SHEIN Sale Attribute Card UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the `SHEIN 销售属性确认` card into an action-first review panel that surfaces one clear conclusion and one primary next action before any diagnostic detail.

**Architecture:** Keep the change scoped to the existing React card component and its tests. Reorganize the card into status, summary, primary action, and advanced-details layers without changing the broader workspace layout or backend policy. Drive the refactor with focused Vitest regression coverage for optional-secondary, required-secondary, and regeneration-first states.

**Tech Stack:** Next.js App Router, React, TypeScript, Tailwind CSS, Vitest, Testing Library

---

### Task 1: Lock The New Card Behavior With Failing Tests

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx`

- [ ] **Step 1: Write the failing tests**

Add assertions for the action-first layout states:

```tsx
expect(screen.getByText("状态 可直接确认")).toBeInTheDocument();
expect(screen.queryByText("状态 待补齐")).not.toBeInTheDocument();
expect(
  screen.getByText("当前类目没有可用的其他规格字段模板，可保持只使用主规格。"),
).toBeInTheDocument();
```

Keep the existing required-secondary regression:

```tsx
expect(
  screen.getByText("第 2 步：其他规格字段（必填） · 来源 Size"),
).toBeInTheDocument();
expect(
  screen.queryByRole("button", { name: "直接确认当前结果" }),
).not.toBeInTheDocument();
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run:

```bash
npm test -- src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx
```

Expected: FAIL because the current card still renders `状态 待补齐` and does not render the explicit “当前类目没有可用的其他规格字段模板...” message.

- [ ] **Step 3: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx
git commit -m "test: lock shein sale attribute card action-first states"
```

### Task 2: Refactor The Card Into An Action-First Layout

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx`

- [ ] **Step 1: Implement the minimal card behavior**

Update the component so optional-secondary scenarios become direct-confirm states and missing-template cases render explanation instead of a misleading select.

Key code shape:

```tsx
const statusLabel =
  !secondaryRequired && canConfirm
    ? "可直接确认"
    : current.status
      ? presentSaleReviewStatus(current.status)
      : undefined;

const secondaryTemplateUnavailable =
  !secondaryRequired &&
  !hasMatchingSecondaryTemplate &&
  !current.secondary_attribute_id &&
  Boolean(current.secondary_source_dimension?.trim());
```

Replace the secondary field selector in optional/no-template mode with a dedicated notice:

```tsx
{secondaryTemplateUnavailable ? (
  <OptionalSecondaryTemplateNotice
    label={`第 2 步：其他规格字段（选填）${current.secondary_source_dimension ? ` · 来源 ${current.secondary_source_dimension}` : ""}`}
  />
) : (
  <TemplateOptionSelect ... />
)}
```

Adjust the summary and guidance copy:

```tsx
<ResultSummaryCard
  title="其他规格"
  value={
    secondaryTemplateUnavailable
      ? "当前类目未提供可用模板"
      : formatResolvedAttributeValue(
          fallbackSecondaryAttributes[0],
          "未识别或未使用",
        )
  }
  description={
    secondaryTemplateUnavailable
      ? "当前类目没有可用的其他规格字段模板，这一步可以跳过。"
      : "系统当前识别到的其他规格"
  }
/>
```

- [ ] **Step 2: Run the focused card tests to verify they pass**

Run:

```bash
npm test -- src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx
```

Expected: PASS with all sale-attribute-card tests green.

- [ ] **Step 3: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.tsx web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx
git commit -m "feat: make shein sale attribute card action-first"
```

### Task 3: Verify The Refactor Does Not Regress Neighboring Workspace Views

**Files:**
- Test: `web/listingkit-ui/src/components/listingkit/workspace/shein-advanced-review-details.test.tsx`
- Test: `web/listingkit-ui/src/components/listingkit/workspace/workspace-screen-views.test.tsx`

- [ ] **Step 1: Run the related workspace view tests**

Run:

```bash
npm test -- src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx src/components/listingkit/workspace/shein-advanced-review-details.test.tsx src/components/listingkit/workspace/workspace-screen-views.test.tsx
```

Expected: PASS with all 3 files green.

- [ ] **Step 2: Review the diff for scope**

Run:

```bash
git diff -- web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.tsx web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx
```

Expected: The diff is limited to the sale attribute card behavior and its tests, with no workspace-wide structural changes.

- [ ] **Step 3: Commit**

```bash
git add web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.tsx web/listingkit-ui/src/components/listingkit/shein/shein-sale-attribute-review-card.test.tsx
git commit -m "test: verify shein sale attribute card workspace compatibility"
```

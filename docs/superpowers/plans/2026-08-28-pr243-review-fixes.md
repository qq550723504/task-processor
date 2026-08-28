# PR243 Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 PR#243 产品工作台 AI 审核栏对非阻断 review、SHEIN 提交 readiness 和历史回退审核原因的错误呈现与缺失处理入口。

**Architecture:** 继续由 `product-workspace-review-model.ts` 作为审核栏唯一的纯展示模型入口。工作台将当前平台的 SHEIN `submit_readiness` 显式传入模型；模型统一把 workflow issues、readiness items 和历史回退原因投影成审核项，并复用 SHEIN 现有动作归一化规则，避免在组件层追加分支。

**Tech Stack:** TypeScript, React, Vitest, Testing Library, Next.js.

**Spec:** PR#243 inline review comments `3881131812`, `3881131821`, `3881131836`.

## Global Constraints

- `blocking` 只能映射为 `blocking`；`warning` 和 `review` 映射为 `warning`。
- 只有当前活动平台的 SHEIN `submit_readiness.blocking_items` 和 `warning_items` 进入审核栏。
- 回退审核原因只能推断已有 SHEIN 类目、普通属性、销售属性动作，不得把 SDS 或其他系统错误误导到 SHEIN 处理入口。
- 保持现有 workspace actions、后端接口、租户边界和 PR#243 其他行为不变。
- 每个行为先写失败测试并确认失败，再写最小生产代码；不做无关重构。

---

### Task 1: 修正 workflow review 严重级别

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/workspace/product-workspace-review-model.test.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/workspace/product-workspace-review-model.ts:57-80`

**Interfaces:**
- Consumes: `ListingKitTaskResult.result.workflow_issues`.
- Produces: `ProductWorkspaceReviewIssue.severity` with `review` projected to `warning`.

- [ ] **Step 1: Write the failing test**

Change the existing review-severity test expectation from `blocking` to `warning`, preserving the same issue code and title:

```ts
it("treats review-severity workflow issues as suggestions", () => {
  const task = {
    status: "completed",
    result: {
      workflow_issues: [
        {
          code: "shein_review_required",
          severity: "review",
          message: "属性需要确认",
        },
      ],
    },
  } as ListingKitTaskResult;

  expect(buildProductWorkspaceReviewIssues(task, "shein")).toEqual([
    {
      id: "shein_review_required",
      severity: "warning",
      title: "属性需要确认",
    },
  ]);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- --run src/components/listingkit/workspace/product-workspace-review-model.test.ts`

Expected: FAIL because the current model converts `review` to `blocking`.

- [ ] **Step 3: Write minimal implementation**

In the workflow issue projection, use only `issue.severity === "blocking"` for the blocking branch; all other accepted non-blocking workflow severities use `warning`:

```ts
severity: issue.severity === "blocking" ? "blocking" : "warning",
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- --run src/components/listingkit/workspace/product-workspace-review-model.test.ts`

Expected: PASS with the review issue rendered as `warning`.

---

### Task 2: Project active SHEIN readiness and preserve fallback repair actions

**Files:**
- Modify: `web/listingkit-ui/src/components/listingkit/workspace/product-workspace-review-model.test.ts`
- Modify: `web/listingkit-ui/src/components/listingkit/workspace/product-workspace-review-model.ts:53-110`
- Modify: `web/listingkit-ui/src/components/listingkit/workspace/workspace-screen.tsx:287-290`

**Interfaces:**
- Consumes: `SheinSubmitReadiness`, task workflow issues, `review_reasons`, task errors, and existing `normalizeSheinWorkspaceActionKey`.
- Produces: review issues for readiness blockers/warnings and fallback reasons with safe SHEIN repair action keys.

- [ ] **Step 1: Write the failing readiness test**

Add a test that passes readiness separately and asserts both severity groups and action mapping:

```ts
it("includes active SHEIN submit readiness items", () => {
  const task = { status: "completed", result: {} } as ListingKitTaskResult;

  expect(
    buildProductWorkspaceReviewIssues(task, "shein", {
      blocking_items: [
        { key: "category_review", label: "类目未确认", message: "请确认类目。" },
      ],
      warning_items: [
        { key: "attribute_review", label: "属性建议确认", message: "请复核属性。" },
      ],
    }),
  ).toEqual([
    {
      id: "readiness-blocking-1",
      severity: "blocking",
      title: "类目未确认",
      description: "请确认类目。",
      actionKey: "category_review",
    },
    {
      id: "readiness-warning-1",
      severity: "warning",
      title: "属性建议确认",
      description: "请复核属性。",
      actionKey: "attribute_review",
    },
  ]);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- --run src/components/listingkit/workspace/product-workspace-review-model.test.ts`

Expected: FAIL because the model currently accepts only task workflow issues and has no readiness argument.

- [ ] **Step 3: Write the failing fallback-action test**

Add a test proving category/attribute/sale-attribute reasons keep direct actions while unrelated SDS errors remain non-actionable:

```ts
it("adds safe repair actions to legacy SHEIN fallback reasons", () => {
  const task = {
    status: "needs_review",
    review_reasons: [
      "SHEIN 类目解析尚未命中真实 category_id",
      "SHEIN 属性模板尚未完成真实 attribute_id 映射",
      "SHEIN 销售属性尚未完成真实 sale attribute 映射",
      "SDS 登录状态已失效，请重新登录",
    ],
    result: {},
  } as ListingKitTaskResult;

  expect(buildProductWorkspaceReviewIssues(task, "shein")).toEqual([
    expect.objectContaining({ title: expect.stringContaining("类目"), actionKey: "category" }),
    expect.objectContaining({ title: expect.stringContaining("属性模板"), actionKey: "attributes" }),
    expect.objectContaining({ title: expect.stringContaining("销售属性"), actionKey: "sale_attributes" }),
    expect.objectContaining({ title: "SDS 登录状态已失效，请重新登录", actionKey: undefined }),
  ]);
});
```

- [ ] **Step 4: Run test to verify it fails**

Run: `npm test -- --run src/components/listingkit/workspace/product-workspace-review-model.test.ts`

Expected: FAIL because fallback issues currently have no `actionKey`.

- [ ] **Step 5: Write minimal implementation**

Extend the builder with `readiness?: SheinSubmitReadiness | null`, project readiness arrays into the same issue shape, and append them without changing non-SHEIN platform behavior. Reuse `normalizeSheinWorkspaceActionKey` for readiness fields. For fallback text, allow only `category`, `attributes`, and `sale_attributes`; do not map `store_login`, pricing, image, or unrelated system errors.

Update `workspace-screen.tsx` to pass only readiness for the active SHEIN platform:

```ts
buildProductWorkspaceReviewIssues(
  taskResult.data,
  selectedPlatform,
  selectedPlatform === "shein" ? preview.data?.shein?.submit_readiness : undefined,
)
```

- [ ] **Step 6: Run focused tests to verify they pass**

Run: `npm test -- --run src/components/listingkit/workspace/product-workspace-review-model.test.ts src/components/listingkit/workspace/workspace-screen-product-layout.test.ts`

Expected: PASS with readiness items visible, correct severity labels, and fallback SHEIN action keys preserved.

---

### Task 3: Full validation and review evidence

**Files:**
- Review only: changed files from Tasks 1–2.

**Interfaces:**
- Consumes: updated pure-model and workspace integration behavior.
- Produces: verified branch diff and evidence for inline review replies.

- [ ] **Step 1: Run frontend lint**

Run: `npm run lint`

Expected: exit code 0.

- [ ] **Step 2: Run frontend typecheck**

Run: `npm run typecheck`

Expected: exit code 0.

- [ ] **Step 3: Run the full frontend test suite**

Run: `npm test -- --reporter=dot`

Expected: all test files and tests pass; record pre-existing warnings separately.

- [ ] **Step 4: Build the frontend**

Run: `npm run build`

Expected: exit code 0.

- [ ] **Step 5: Review the final diff and status**

Run: `git diff --check`; `git diff --stat`; `git status --short --branch`.

Expected: only the planned review model, workspace screen, and focused test changes are present; no tenant, backend, API, or unrelated files changed.

- [ ] **Step 6: Prepare review-thread responses**

Reply technically in the original GitHub inline threads after the branch is verified:

- `3881131812`: review now projects to warning.
- `3881131821`: active SHEIN readiness blockers/warnings now enter the AI review model.
- `3881131836`: fallback review reasons now retain safe category/attribute/sale-attribute actions, while unrelated errors remain without actions.

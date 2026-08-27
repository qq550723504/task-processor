import { describe, expect, it } from "vitest";

import { buildProductWorkspaceReviewIssues } from "@/components/listingkit/workspace/product-workspace-review-model";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

describe("Product Workspace review model", () => {
  it("preserves structured review reasons when workflow issues are absent", () => {
    const task = {
      status: "needs_review",
      review_reasons: ["类目需要人工确认"],
      result: {},
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task, "shein")).toEqual([
      {
        id: "fallback-review-1",
        severity: "blocking",
        title: "类目需要人工确认",
        actionKey: "category",
      },
    ]);
  });

  it("preserves task and child error fallbacks without inventing an action", () => {
    const task = {
      status: "needs_review",
      result: {
        child_tasks: [
          {
            kind: "source_product_enrich",
            status: "failed",
            error: "源数据转换失败",
          },
        ],
      },
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task, "shein")).toEqual([
      {
        id: "fallback-review-1",
        severity: "blocking",
        title: "源数据转换失败",
      },
    ]);
  });

  it("marks generic workflow issues as non-actionable instead of inventing a SHEIN target", () => {
    const task = {
      status: "needs_review",
      result: {
        workflow_issues: [
          {
            code: "product_enrich_failed",
            severity: "blocking",
            message: "商品补全失败",
            detail: "请检查源数据。",
          },
        ],
      },
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task, "shein")).toEqual([
      {
        id: "product_enrich_failed",
        severity: "blocking",
        title: "商品补全失败",
        description: "请检查源数据。",
      },
    ]);
  });

  it("keeps known SHEIN workflow issues actionable", () => {
    const task = {
      status: "needs_review",
      result: {
        workflow_issues: [
          {
            code: "attributes",
            severity: "blocking",
            message: "Material 缺失",
          },
        ],
      },
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task, "shein")).toEqual([
      {
        id: "attributes",
        severity: "blocking",
        title: "Material 缺失",
        actionKey: "attributes",
      },
    ]);
  });
});

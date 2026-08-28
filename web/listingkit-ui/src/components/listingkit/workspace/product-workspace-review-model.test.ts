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
            error: "SDS 登录状态已失效，请重新登录",
          },
        ],
      },
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task, "shein")).toEqual([
      {
        id: "fallback-review-1",
        severity: "blocking",
        title: "SDS 登录状态已失效，请重新登录",
      },
    ]);
  });

  it("treats review-ready fallback reasons as mandatory", () => {
    const task = {
      status: "review_ready",
      review_reasons: ["类目需要人工确认"],
      result: {},
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task, "shein")).toEqual([
      {
        id: "fallback-review-1",
        severity: "blocking",
        title: "类目需要人工确认",
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

  it("treats review-severity workflow issues as mandatory", () => {
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
        severity: "blocking",
        title: "属性需要确认",
      },
    ]);
  });

  it("keeps repeated workflow issue codes as unique render identities", () => {
    const task = {
      status: "needs_review",
      result: {
        workflow_issues: [
          {
            code: "shein_review_required",
            severity: "blocking",
            message: "类目需要确认",
          },
          {
            code: "shein_review_required",
            severity: "blocking",
            message: "属性需要确认",
          },
        ],
      },
    } as ListingKitTaskResult;

    const issues = buildProductWorkspaceReviewIssues(task, "shein");

    expect(issues).toHaveLength(2);
    expect(new Set(issues.map((issue) => issue.id)).size).toBe(2);
  });

  it("preserves structured review reasons alongside warning-only workflow issues", () => {
    const task = {
      status: "needs_review",
      review_reasons: ["类目需要人工确认"],
      result: {
        workflow_issues: [
          {
            code: "copy_warning",
            severity: "warning",
            message: "标题建议优化",
          },
        ],
      },
    } as ListingKitTaskResult;

    const issues = buildProductWorkspaceReviewIssues(task, "shein");

    expect(issues.map((issue) => issue.title)).toEqual([
      "标题建议优化",
      "类目需要人工确认",
    ]);
    expect(issues[1]).toEqual(
      expect.objectContaining({
        severity: "blocking",
      }),
    );
  });

  it("does not route unrelated SDS login issues into SHEIN store login", () => {
    const task = {
      status: "needs_review",
      result: {
        workflow_issues: [
          {
            code: "sds_auth_required",
            severity: "blocking",
            message: "SDS 登录状态已失效，请重新登录",
          },
        ],
      },
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task, "shein")).toEqual([
      {
        id: "sds_auth_required",
        severity: "blocking",
        title: "SDS 登录状态已失效，请重新登录",
      },
    ]);
  });

  it("does not route non-SHEIN workflow issues into SHEIN actions", () => {
    const task = {
      status: "needs_review",
      result: {
        workflow_issues: [
          {
            code: "image_review_required",
            stage: "product_image:amazon",
            severity: "review",
            message: "Amazon 图片需要确认",
          },
        ],
      },
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task, "shein")).toEqual([
      {
        id: "image_review_required",
        severity: "blocking",
        title: "Amazon 图片需要确认",
      },
    ]);
  });

  it("surfaces canonical field review flags when no textual reason exists", () => {
    const task = {
      status: "needs_review",
      result: {
        canonical_product: {
          title: "Canvas Tote",
          description: "Reusable canvas tote for daily use.",
          needs_review: true,
          field_traces: {
            title: {
              needs_review: true,
              review_reason: "商品标题需要确认",
            },
          },
        },
      },
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task)).toEqual([
      {
        id: "fallback-review-1",
        severity: "blocking",
        title: "商品标题需要确认",
      },
    ]);
  });

  it("retains the missing-description blocker alongside an unrelated field trace", () => {
    const task = {
      status: "needs_review",
      result: {
        canonical_product: {
          title: "Canvas Tote",
          description: "   ",
          needs_review: true,
          field_traces: {
            brand: {
              needs_review: true,
              review_reason: "品牌需要确认",
            },
          },
        },
      },
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task)).toEqual([
      {
        id: "fallback-review-1",
        severity: "blocking",
        title: "品牌需要确认",
      },
      {
        id: "fallback-review-2",
        severity: "blocking",
        title: "商品描述缺失",
      },
    ]);
  });

  it("retains the missing-title blocker alongside an unrelated field trace", () => {
    const task = {
      status: "needs_review",
      result: {
        canonical_product: {
          title: "   ",
          description: "Reusable canvas tote for daily use.",
          needs_review: true,
          field_traces: {
            brand: {
              needs_review: true,
              review_reason: "品牌需要确认",
            },
          },
        },
      },
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task)).toEqual([
      {
        id: "fallback-review-1",
        severity: "blocking",
        title: "品牌需要确认",
      },
      {
        id: "fallback-review-2",
        severity: "blocking",
        title: "商品标题缺失",
      },
    ]);
  });

  it("marks failed task error fallbacks as mandatory", () => {
    const task = {
      status: "failed",
      error: "商品生成失败",
      result: {},
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task)).toEqual([
      {
        id: "fallback-review-1",
        severity: "blocking",
        title: "商品生成失败",
      },
    ]);
  });

  it("merges canonical review reasons with blocking workflow issues without duplicates", () => {
    const task = {
      status: "needs_review",
      result: {
        workflow_issues: [
          {
            code: "title_review_required",
            severity: "blocking",
            message: "商品标题需要确认",
          },
        ],
        canonical_product: {
          title: "Canvas Tote",
          description: "Reusable canvas tote for daily use.",
          needs_review: true,
          field_traces: {
            title: {
              needs_review: true,
              review_reason: "商品标题需要确认",
            },
            selling_points: {
              needs_review: true,
              review_reason: "商品卖点需要确认",
            },
          },
        },
      },
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task)).toEqual([
      {
        id: "title_review_required",
        severity: "blocking",
        title: "商品标题需要确认",
      },
      {
        id: "fallback-review-2",
        severity: "blocking",
        title: "商品卖点需要确认",
      },
    ]);
  });
});

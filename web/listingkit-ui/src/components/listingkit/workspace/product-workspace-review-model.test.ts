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
        actionKey: "category",
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

  it("includes active SHEIN submit readiness items", () => {
    const task = { status: "completed", result: {} } as ListingKitTaskResult;

    expect(
      buildProductWorkspaceReviewIssues(task, "shein", {
        blocking_items: [
          { key: "category_review", label: "类目未确认", message: "请确认类目。" },
        ],
        warning_items: [
          { key: "attribute_review", label: "属性建议确认", message: "请复核属性。" },
          {
            key: "shein_attribute_template_freshness",
            label: "属性模板已过期",
            message: "请刷新属性模板。",
          },
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
      {
        id: "readiness-warning-2",
        severity: "warning",
        title: "属性模板已过期",
        description: "请刷新属性模板。",
        actionKey: "shein_attribute_template_freshness",
      },
    ]);
  });

  it("preserves ordinary sale-attribute readiness actions", () => {
    const task = { status: "completed", result: {} } as ListingKitTaskResult;

    expect(
      buildProductWorkspaceReviewIssues(task, "shein", {
        blocking_items: [],
        warning_items: [
          {
            key: "sale_attributes",
            label: "销售属性需要确认",
            message: "请复核销售属性。",
            taxonomy: {
              domain: "sale_attribute",
              repair_target: "sale_attribute_review",
              repair_route: "workspace.sale_attributes",
            },
          },
        ],
      }),
    ).toEqual([
      {
        id: "readiness-warning-1",
        severity: "warning",
        title: "销售属性需要确认",
        description: "请复核销售属性。",
        actionKey: "sale_attributes",
      },
    ]);
  });

  it("does not attach SHEIN fallback actions to non-SHEIN platforms", () => {
    const task = {
      status: "needs_review",
      review_reasons: ["TEMU 类目需要确认"],
      result: {},
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task, "temu")).toEqual([
      {
        id: "fallback-review-1",
        severity: "blocking",
        title: "TEMU 类目需要确认",
      },
    ]);
  });

  it("deduplicates readiness items that target the same workspace surface", () => {
    const task = { status: "completed", result: {} } as ListingKitTaskResult;

    expect(
      buildProductWorkspaceReviewIssues(task, "shein", {
        blocking_items: [
          { key: "attributes", label: "属性未确认", message: "请确认属性。" },
          { key: "attribute_review", label: "属性需要确认", message: "请确认属性。" },
        ],
      }),
    ).toEqual([
      {
        id: "readiness-blocking-1",
        severity: "blocking",
        title: "属性未确认",
        description: "请确认属性。",
        actionKey: "attributes",
      },
    ]);
  });

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
      {
        id: "fallback-review-1",
        severity: "blocking",
        title: "SHEIN 类目解析尚未命中真实 category_id",
        actionKey: "category",
      },
      {
        id: "fallback-review-2",
        severity: "blocking",
        title: "SHEIN 属性模板尚未完成真实 attribute_id 映射",
        actionKey: "attributes",
      },
      {
        id: "fallback-review-3",
        severity: "blocking",
        title: "SHEIN 销售属性尚未完成真实 sale attribute 映射",
        actionKey: "sale_attributes",
      },
      {
        id: "fallback-review-4",
        severity: "blocking",
        title: "SDS 登录状态已失效，请重新登录",
      },
    ]);
  });

  it("infers a SHEIN category action from a generic review issue message", () => {
    const task = {
      status: "needs_review",
      result: {
        workflow_issues: [
          {
            code: "shein_review_required",
            stage: "shein_review",
            severity: "blocking",
            message: "建议复核 SHEIN 类目",
          },
        ],
      },
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task, "shein")).toEqual([
      {
        id: "shein_review_required",
        severity: "blocking",
        title: "建议复核 SHEIN 类目",
        actionKey: "category",
      },
    ]);
  });

  it("keeps a SHEIN issue actionable when another platform is selected", () => {
    const task = {
      status: "needs_review",
      result: {
        workflow_issues: [
          {
            code: "shein_review_required",
            stage: "shein_review",
            severity: "blocking",
            message: "建议复核 SHEIN 类目",
          },
        ],
      },
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task, "temu")).toEqual([
      {
        id: "shein_review_required",
        severity: "blocking",
        title: "建议复核 SHEIN 类目",
        actionKey: "category",
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
        severity: "warning",
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

  it("surfaces nested attribute and variant trace blockers", () => {
    const task = {
      status: "needs_review",
      result: {
        canonical_product: {
          title: "Canvas Tote",
          description: "Reusable canvas tote for daily use.",
          attributes: {
            material: {
              value: "Cotton",
              trace: {
                needs_review: true,
                review_reason: "材质来源需要确认",
              },
            },
          },
          variants: [
            {
              sku: "TOTE-BLK-M",
              trace: {
                needs_review: true,
                review_reason: "变体事实需要确认",
              },
              attributes: {
                size: {
                  value: "M",
                  trace: {
                    needs_review: true,
                    review_reason: "变体尺码需要确认",
                  },
                },
              },
            },
          ],
        },
      },
    } as ListingKitTaskResult;

    expect(buildProductWorkspaceReviewIssues(task)).toEqual([
      {
        id: "fallback-review-1",
        severity: "blocking",
        title: "材质来源需要确认",
      },
      {
        id: "fallback-review-2",
        severity: "blocking",
        title: "变体事实需要确认",
      },
      {
        id: "fallback-review-3",
        severity: "blocking",
        title: "变体尺码需要确认",
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

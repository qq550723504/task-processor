import { describe, expect, it } from "vitest";

import {
  buildProductWorkspaceAttentionSummary,
  buildProductWorkspaceCanonicalNavigation,
  buildProductWorkspacePlatformNavigation,
} from "@/components/listingkit/workspace/product-workspace-model";

describe("Product Workspace presentation model", () => {
  it("builds product-first canonical navigation without execution language", () => {
    const items = buildProductWorkspaceCanonicalNavigation("overview");

    expect(items.map((item) => item.label)).toEqual([
      "概览",
      "图片",
      "基础信息",
      "SKU",
      "规格",
      "属性",
      "描述",
    ]);
    expect(items.find((item) => item.key === "overview")?.selected).toBe(true);
    expect(items.map((item) => item.label).join(" ")).not.toMatch(/Task|Temporal|Queue|任务 ID/i);
  });

  it("derives platform navigation while preserving the selected platform", () => {
    const items = buildProductWorkspacePlatformNavigation(
      [
        { platform: "shein", label: "SHEIN", status: "attention" },
        { platform: "temu", label: "TEMU", status: "ready" },
      ],
      "shein",
    );

    expect(items).toEqual([
      {
        key: "platform:shein",
        label: "SHEIN",
        platform: "shein",
        selected: true,
        status: "attention",
      },
      {
        key: "platform:temu",
        label: "TEMU",
        platform: "temu",
        selected: false,
        status: "ready",
      },
    ]);
  });

  it("maps review counts into operator-facing AI attention language", () => {
    const summary = buildProductWorkspaceAttentionSummary({
      blockingCount: 2,
      warningCount: 3,
      passedCount: 18,
    });

    expect(summary).toEqual([
      { severity: "blocking", label: "必须处理", count: 2 },
      { severity: "warning", label: "建议确认", count: 3 },
      { severity: "passed", label: "已通过", count: 18 },
    ]);
  });
});

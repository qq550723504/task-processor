import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ProductWorkspaceAIReview } from "@/components/listingkit/workspace/product-workspace-ai-review";
import { buildProductWorkspaceAttentionSummary } from "@/components/listingkit/workspace/product-workspace-model";

describe("ProductWorkspaceAIReview", () => {
  it("presents blocking, warning, and passed counts with contextual issue details", () => {
    render(
      <ProductWorkspaceAIReview
        summary={buildProductWorkspaceAttentionSummary({
          blockingCount: 1,
          warningCount: 2,
          passedCount: 18,
        })}
        issues={[
          {
            id: "material",
            severity: "blocking",
            title: "Material 缺失",
            description: "SHEIN 提交前必须补充材质。",
            suggestion: "Cotton",
            evidence: "来源属性 + 商品描述",
            confidence: 0.94,
          },
          {
            id: "color",
            severity: "warning",
            title: "颜色建议确认",
            description: "原始数据为 Dark Blue。",
            suggestion: "Navy Blue",
          },
        ]}
        onSelectIssue={vi.fn()}
      />,
    );

    expect(screen.getByText("AI 审核")).toBeInTheDocument();
    expect(screen.getByText("必须处理")).toBeInTheDocument();
    expect(screen.getByText("建议确认")).toBeInTheDocument();
    expect(screen.getByText("已通过")).toBeInTheDocument();
    expect(screen.getByText("Material 缺失")).toBeInTheDocument();
    expect(screen.getByText("AI 建议：Cotton")).toBeInTheDocument();
    expect(screen.getByText("依据：来源属性 + 商品描述")).toBeInTheDocument();
    expect(screen.getByText("置信度：94%")).toBeInTheDocument();
    expect(screen.queryByText(/chat|ask ai|task-1|temporal/i)).not.toBeInTheDocument();
  });

  it("delegates actionable issue actions through callbacks", async () => {
    const user = userEvent.setup();
    const onSelectIssue = vi.fn();
    const issue = {
      id: "category",
      severity: "blocking" as const,
      title: "类目需要确认",
      description: "AI 无法安全确定目标类目。",
      actionKey: "category" as const,
    };

    render(
      <ProductWorkspaceAIReview
        summary={buildProductWorkspaceAttentionSummary({
          blockingCount: 1,
          warningCount: 0,
          passedCount: 0,
        })}
        issues={[issue]}
        onSelectIssue={onSelectIssue}
      />,
    );

    await user.click(screen.getByRole("button", { name: "处理：类目需要确认" }));
    expect(onSelectIssue).toHaveBeenCalledWith(issue);
  });

  it("does not render a dead action button for non-actionable issues", () => {
    render(
      <ProductWorkspaceAIReview
        summary={buildProductWorkspaceAttentionSummary({
          blockingCount: 1,
          warningCount: 0,
          passedCount: 0,
        })}
        issues={[
          {
            id: "product_enrich_failed",
            severity: "blocking",
            title: "商品补全失败",
            description: "请检查源数据。",
          },
        ]}
        onSelectIssue={vi.fn()}
      />,
    );

    expect(screen.getByText("商品补全失败")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "处理：商品补全失败" })).not.toBeInTheDocument();
  });

  it("shows a clear success state when no unresolved issues remain", () => {
    render(
      <ProductWorkspaceAIReview
        summary={buildProductWorkspaceAttentionSummary({
          blockingCount: 0,
          warningCount: 0,
          passedCount: 27,
        })}
        issues={[]}
        onSelectIssue={vi.fn()}
      />,
    );

    expect(screen.getByText("没有需要你处理的问题")).toBeInTheDocument();
    expect(screen.getByText("AI 检查已完成，可以继续当前操作。")).toBeInTheDocument();
  });
});

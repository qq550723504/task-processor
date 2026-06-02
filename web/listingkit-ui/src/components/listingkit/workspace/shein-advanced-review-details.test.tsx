import { render, screen, within } from "@testing-library/react";

import { SheinAdvancedReviewDetails } from "@/components/listingkit/workspace/shein-advanced-review-details";

describe("SheinAdvancedReviewDetails", () => {
  it("renders refresh history with affected recompute areas", () => {
    const { container } = render(
      <SheinAdvancedReviewDetails
        open
        showCategoryReview={false}
        showAttributeReview={false}
        showSaleAttributeReview={false}
        refreshHistory={[
          {
            scope: "category",
            status: "completed",
            title: "类目模板已触发刷新",
            detail: "已触发类目模板刷新，稍后会用最新类目继续刷新属性映射。",
            affectedAreas: ["类目", "普通属性", "销售属性"],
            occurredAt: "2026-05-28T05:30:00.000Z",
          },
          {
            scope: "attribute",
            status: "running",
            title: "正在重新生成普通属性",
            detail: "正在重新生成普通属性…",
            affectedAreas: ["普通属性"],
            occurredAt: "2026-05-28T05:31:00.000Z",
          },
        ]}
        categoryReviewProps={{ taskId: "task-1" } as never}
        attributeReviewProps={{} as never}
        saleAttributeReviewProps={{} as never}
      />,
    );

    const timeline = screen.getByText("查看本次刷新轨迹（2）").closest("details");
    expect(timeline).not.toBeNull();
    if (!timeline) {
      throw new Error("refresh timeline missing");
    }

    const articles = within(timeline).getAllByRole("article");
    expect(articles).toHaveLength(2);
    expect(within(articles[0]).getByText("类目模板已触发刷新")).toBeInTheDocument();
    expect(within(articles[0]).getByText("已触发")).toBeInTheDocument();
    expect(
      within(articles[0]).getByText("当前操作 类目 · 将重算 类目 / 普通属性 / 销售属性"),
    ).toBeInTheDocument();
    expect(within(articles[1]).getByText("正在重新生成普通属性")).toBeInTheDocument();
    expect(within(articles[1]).getByText("进行中")).toBeInTheDocument();
    expect(
      within(articles[1]).getByText("当前操作 普通属性 · 将重算 普通属性"),
    ).toBeInTheDocument();
    expect(
      Array.from(container.querySelectorAll("div")).some((element) =>
        String(element.className).includes("2xl:grid-cols-2"),
      ),
    ).toBe(true);
  });
});

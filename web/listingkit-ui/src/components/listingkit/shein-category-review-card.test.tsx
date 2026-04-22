import { render, screen } from "@testing-library/react";

import { SheinCategoryReviewCard } from "@/components/listingkit/shein-category-review-card";

describe("SheinCategoryReviewCard", () => {
  it("renders current category review state and suggested category", () => {
    render(
      <SheinCategoryReviewCard
        editorContext={{
          category: {
            current: {
              category_id: 12143,
              category_path: ["家居&生活", "家庭用品", "鞋用品", "鞋配饰"],
              suggested_category: {
                category_id: 3221,
                source: "ai_category_tree",
                reason: "当前类目模板不适配销售属性结构，建议复核该候选类目",
                matched_path: ["家居&生活", "厨房&餐厅", "饮具", "真空瓶和保温杯"],
              },
            },
          },
          sale_attributes: {
            current: {
              recommend_category_review: true,
              category_review_reason: "当前类目路径与商品语义明显不一致",
            },
          },
        }}
      />,
    );

    expect(screen.getByText("SHEIN category review")).toBeInTheDocument();
    expect(screen.getByText("家居&生活 > 家庭用品 > 鞋用品 > 鞋配饰")).toBeInTheDocument();
    expect(screen.getByText("当前类目路径与商品语义明显不一致")).toBeInTheDocument();
    expect(screen.getByText("Suggested category")).toBeInTheDocument();
    expect(screen.getByText("家居&生活 > 厨房&餐厅 > 饮具 > 真空瓶和保温杯")).toBeInTheDocument();
    expect(screen.getByText("3221")).toBeInTheDocument();
  });

  it("renders review-only state without suggested category", () => {
    render(
      <SheinCategoryReviewCard
        editorContext={{
          category: {
            current: {
              category_id: 12143,
              category_path: ["家居&生活", "家庭用品", "鞋用品", "鞋配饰"],
            },
          },
          sale_attributes: {
            current: {
              recommend_category_review: true,
              category_review_reason: "当前类目销售属性模板未提供可承接款式/型号的销售属性字段",
            },
          },
        }}
      />,
    );

    expect(screen.getByText("SHEIN category review")).toBeInTheDocument();
    expect(screen.queryByText("Suggested category")).not.toBeInTheDocument();
  });

  it("returns null when there is no category review signal", () => {
    const { container } = render(
      <SheinCategoryReviewCard
        editorContext={{
          category: {
            current: {
              category_id: 12143,
              category_path: ["家居&生活", "家庭用品", "鞋用品", "鞋配饰"],
            },
          },
        }}
      />,
    );

    expect(container.firstChild).toBeNull();
  });
});

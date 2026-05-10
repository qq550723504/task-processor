import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { SheinCategoryReviewCard } from "@/components/listingkit/shein/shein-category-review-card";

describe("SheinCategoryReviewCard", () => {
  it("renders current category review state and suggested category", () => {
    render(
      <SheinCategoryReviewCard
        taskId="task-1"
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

    expect(screen.getByText("SHEIN 类目审核")).toBeInTheDocument();
    expect(
      screen.getByText("家居&生活 > 家庭用品 > 鞋用品 > 鞋配饰 (12143)"),
    ).toBeInTheDocument();
    expect(screen.getByText("当前类目路径与商品语义明显不一致")).toBeInTheDocument();
    expect(screen.getByText("建议类目")).toBeInTheDocument();
    expect(
      screen.getByText("家居&生活 > 厨房&餐厅 > 饮具 > 真空瓶和保温杯 (3221)"),
    ).toBeInTheDocument();
  });

  it("renders review-only state without suggested category", () => {
    render(
      <SheinCategoryReviewCard
        taskId="task-1"
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

    expect(screen.getByText("SHEIN 类目审核")).toBeInTheDocument();
    expect(screen.queryByText("建议类目")).not.toBeInTheDocument();
  });

  it("renders confirmed category summary when there is no review signal", () => {
    render(
      <SheinCategoryReviewCard
        taskId="task-1"
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

    expect(screen.getByText("SHEIN 类目已确认")).toBeInTheDocument();
    expect(
      screen.getByText("家居&生活 > 家庭用品 > 鞋用品 > 鞋配饰 (12143)"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "确认当前类目" })).not.toBeInTheDocument();
  });

  it("fires apply callback when suggested category action is clicked", async () => {
    const user = userEvent.setup();
    const onApplySuggestedCategory = vi.fn();

    render(
      <SheinCategoryReviewCard
        taskId="task-1"
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
        onApplySuggestedCategory={onApplySuggestedCategory}
      />,
    );

    await user.click(screen.getByRole("button", { name: "应用建议类目" }));
    expect(onApplySuggestedCategory).toHaveBeenCalledTimes(1);
  });

  it("hides apply action after the suggested category is already applied", () => {
    render(
      <SheinCategoryReviewCard
        taskId="task-1"
        editorContext={{
          category: {
            current: {
              category_id: 3221,
              category_path: ["家居&生活", "厨房&餐厅", "饮具", "真空瓶和保温杯"],
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
              recommend_category_review: false,
            },
          },
        }}
        onApplySuggestedCategory={vi.fn()}
      />,
    );

    expect(screen.getByText("已应用类目")).toBeInTheDocument();
    expect(
      screen.getByText("建议类目已经应用到当前 SHEIN 草稿。"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "应用建议类目" })).not.toBeInTheDocument();
  });
});

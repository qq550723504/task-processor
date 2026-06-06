import { render, screen } from "@testing-library/react";

import { ReviewReasonsCard } from "@/components/listingkit/review/review-reasons-card";

describe("ReviewReasonsCard", () => {
  it("renders nothing when the task does not need review", () => {
    const { container } = render(
      <ReviewReasonsCard task={{ status: "completed" }} />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("renders capped review reasons for needs-review tasks", () => {
    render(
      <ReviewReasonsCard
        limit={2}
        task={{
          status: "needs_review",
          review_reasons: ["Reason one", "Reason two", "Reason three"],
          error: "legacy fallback string",
        }}
      />,
    );

    expect(screen.getByText("审核重点")).toBeInTheDocument();
    expect(screen.getByText("Reason one")).toBeInTheDocument();
    expect(screen.getByText("Reason two")).toBeInTheDocument();
    expect(screen.queryByText("Reason three")).not.toBeInTheDocument();
    expect(screen.getByText("还有 1 条待确认原因，请在任务详情中继续查看")).toBeInTheDocument();
    expect(screen.queryByText("legacy fallback string")).not.toBeInTheDocument();
  });

  it("splits semicolon-separated reasons before applying the limit", () => {
    render(
      <ReviewReasonsCard
        limit={2}
        task={{
          status: "needs_review",
          error: "Reason one; Reason two；Reason three",
        }}
      />,
    );

    expect(screen.getByText("Reason one")).toBeInTheDocument();
    expect(screen.getByText("Reason two")).toBeInTheDocument();
    expect(screen.queryByText("Reason three")).not.toBeInTheDocument();
    expect(screen.getByText("还有 1 条待确认原因，请在任务详情中继续查看")).toBeInTheDocument();
  });

  it("renders direct review entry buttons for shein mapping reasons", () => {
    render(
      <ReviewReasonsCard
        taskId="task-1"
        task={{
          status: "needs_review",
          error:
            "SHEIN 类目解析尚未命中真实 category_id；SHEIN 属性模板尚未完成真实 attribute_id 映射；SHEIN 销售属性尚未完成真实 sale attribute 映射",
        }}
      />,
    );

    expect(screen.getByRole("link", { name: "去确认类目" })).toHaveAttribute(
      "href",
      "/listing-kits/task-1/workspace?platform=shein&section_key=general_review#shein-category-review-card",
    );
    expect(screen.getByRole("link", { name: "去确认普通属性" })).toHaveAttribute(
      "href",
      "/listing-kits/task-1/workspace?platform=shein&section_key=general_review#shein-attribute-review-card",
    );
    expect(screen.getByRole("link", { name: "去确认销售属性" })).toHaveAttribute(
      "href",
      "/listing-kits/task-1/workspace?platform=shein&section_key=general_review#shein-sale-attribute-review-card",
    );
  });
});

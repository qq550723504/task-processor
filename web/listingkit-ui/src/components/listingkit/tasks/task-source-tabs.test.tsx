import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TaskSourceTabs } from "@/components/listingkit/tasks/task-source-tabs";

describe("TaskSourceTabs", () => {
  it("renders tabs and active copy", () => {
    render(
      <TaskSourceTabs activeTab="productUrl" onTabChange={() => {}} />,
    );

    expect(screen.getByText("任务来源")).toBeInTheDocument();
    expect(
      screen.getByText(
        "适合已有商品来源时使用。粘贴 1688 或其他商品页链接，系统会按原始商品资料继续处理。",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: "商品链接" }),
    ).toHaveAttribute("aria-selected", "true");
  });

  it("switches source tabs", () => {
    const onTabChange = vi.fn();

    render(<TaskSourceTabs activeTab="imageUrls" onTabChange={onTabChange} />);

    fireEvent.click(screen.getByRole("tab", { name: "商品链接" }));

    expect(onTabChange).toHaveBeenCalledWith("productUrl");
  });
});

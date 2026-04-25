import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TaskSourceTabs } from "@/components/listingkit/tasks/task-source-tabs";

describe("TaskSourceTabs", () => {
  it("renders tabs and active copy", () => {
    render(
      <TaskSourceTabs activeTab="productUrl" onTabChange={() => {}} />,
    );

    expect(screen.getByText("Source mode")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Paste a 1688 or other product URL when you want ListingKit to start from the original listing.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: "1688 / Product URL" }),
    ).toHaveAttribute("aria-selected", "true");
  });

  it("switches source tabs", () => {
    const onTabChange = vi.fn();

    render(<TaskSourceTabs activeTab="imageUrls" onTabChange={onTabChange} />);

    fireEvent.click(screen.getByRole("tab", { name: "1688 / Product URL" }));

    expect(onTabChange).toHaveBeenCalledWith("productUrl");
  });
});

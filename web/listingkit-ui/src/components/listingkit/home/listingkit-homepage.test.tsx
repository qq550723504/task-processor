import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import Home from "@/app/page";

vi.mock("@/lib/query/use-task-list", () => ({
  useListingKitTasks: () => ({
    isLoading: false,
    isError: false,
    data: {
      total: 1,
      items: [
        {
          task_id: "task-1",
          status: "needs_review",
          platforms: ["shein"],
          title: "Botanical clock",
          image_count: 0,
          created_at: "2026-04-30T10:00:00+08:00",
          updated_at: "2026-04-30T10:00:00+08:00",
        },
      ],
    },
  }),
}));

describe("Home", () => {
  it("renders the SHEIN-first homepage layout", () => {
    render(<Home />);

    expect(screen.getByText("ListingKit")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "进入 SHEIN 工作台" }),
    ).toHaveAttribute("href", "/listing-kits/shein");
    expect(
      screen.getByRole("link", { name: "开始新的 ListingKit 任务" }),
    ).toHaveAttribute("href", "/listing-kits/new");
    expect(
      screen.getByRole("link", { name: "SHEIN 工作台" }),
    ).toHaveAttribute("href", "/listing-kits/shein");
    expect(
      screen.getByRole("link", { name: "SDS 选品" }),
    ).toHaveAttribute("href", "/listing-kits/sds");
    expect(
      screen.getByRole("link", { name: "任务列表" }),
    ).toHaveAttribute("href", "/listing-kits");
    expect(
      screen.getByRole("link", { name: "继续最近任务" }),
    ).toHaveAttribute("href", "/listing-kits/task-1/workspace?platform=shein");
    expect(screen.getAllByText("Botanical clock").length).toBeGreaterThan(0);
  });
});

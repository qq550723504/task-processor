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
  it("renders the canonical-product centered homepage layout", () => {
    render(<Home />);

    expect(screen.getByText("ListingKit")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "从商品源信息生成多平台上架资料" }),
    ).toBeInTheDocument();
    expect(screen.getByText("输入源信息")).toBeInTheDocument();
    expect(screen.getByText("生成 Canonical Product")).toBeInTheDocument();
    expect(screen.getByText("生成平台资料")).toBeInTheDocument();
    expect(screen.getByText("审核 / 上架")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "开始生成商品资料" }),
    ).toHaveAttribute("href", "/listing-kits/new");
    expect(
      screen.getByRole("link", { name: "查看 Canonical Products" }),
    ).toHaveAttribute("href", "/listing-kits/canonical-products");
    expect(
      screen.getByRole("link", { name: "新建 ListingKit 任务" }),
    ).toHaveAttribute("href", "/listing-kits/new");
    expect(
      screen.getByRole("link", { name: "SDS 源" }),
    ).toHaveAttribute("href", "/listing-kits/sds");
    expect(screen.queryByRole("link", { name: "SHEIN 上架工作台" })).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "任务列表" }),
    ).toHaveAttribute("href", "/listing-kits");
    expect(
      screen.getByRole("link", { name: "Canonical Products" }),
    ).toHaveAttribute("href", "/listing-kits/canonical-products");
    expect(
      screen.getByRole("link", { name: "继续最近任务" }),
    ).toHaveAttribute("href", "/listing-kits/task-1/workspace?platform=shein");
    expect(screen.getAllByText("Botanical clock").length).toBeGreaterThan(0);
  });
});

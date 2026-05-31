import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinStudioBatchPageShell } from "@/components/listingkit/shein-studio/shein-studio-batch-page-shell";

vi.mock("@/components/listingkit/shein-studio/shein-studio-workbench", () => ({
  SheinStudioWorkbench: ({ initialBatchId }: { initialBatchId?: string }) => (
    <div>workbench {initialBatchId}</div>
  ),
}));

describe("SheinStudioBatchPageShell", () => {
  it("wraps the batch header in a responsive page container", () => {
    const { container } = render(
      <SheinStudioBatchPageShell batchId="batch-123" />,
    );

    expect(
      screen.getByRole("heading", { name: "批次工作台" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("当前正在继续处理批次 batch-123，可以在这里继续生成、审核和创建任务。"),
    ).toBeInTheDocument();
    expect(screen.getByText("workbench batch-123")).toBeInTheDocument();

    const headerContainer = container.querySelector(
      ".mx-auto.max-w-7xl",
    ) as HTMLDivElement | null;
    expect(headerContainer).not.toBeNull();
    expect(headerContainer?.className).toContain("px-4");
  });
});

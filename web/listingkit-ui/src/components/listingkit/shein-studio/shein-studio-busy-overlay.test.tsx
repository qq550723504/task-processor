import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SheinStudioBusyOverlay } from "@/components/listingkit/shein-studio/shein-studio-busy-overlay";

describe("SheinStudioBusyOverlay", () => {
  it("keeps the advisory panel mobile-safe", () => {
    const { container } = render(
      <SheinStudioBusyOverlay message="正在生成图片" />,
    );

    expect(screen.getByText("正在生成图片")).toBeInTheDocument();

    const advisoryPanel = screen
      .getByText("当前仅锁定本次生图相关字段和提交按钮，避免重复扣费或让结果和表单状态错位。")
      .closest("div");
    expect(advisoryPanel).not.toBeNull();
    expect(advisoryPanel?.className).not.toContain("min-w-[220px]");

    const contentRow = container.querySelector(
      ".flex.flex-col.items-start.gap-4",
    ) as HTMLDivElement | null;
    expect(contentRow).not.toBeNull();
    expect(contentRow?.className).toContain("flex-col");
  });
});

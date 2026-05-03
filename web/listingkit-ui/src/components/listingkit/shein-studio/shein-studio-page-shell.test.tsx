import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinStudioPageShell } from "@/components/listingkit/shein-studio/shein-studio-page-shell";

vi.mock("@/lib/utils/live-search-params", () => ({
  useLiveSearchParams: () => new URLSearchParams(""),
}));

vi.mock("@/components/listingkit/shein-studio/shein-product-picker-modal", () => ({
  SheinProductPickerModal: () => <div>picker modal</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-studio-workbench-slot", () => ({
  SheinStudioWorkbenchSlot: () => <div>workbench slot</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-studio-step-tabs", () => ({
  SheinStudioStepTabs: () => <div>step tabs</div>,
}));

describe("SheinStudioPageShell", () => {
  it("shows a step explanation for selection before the user has chosen a product", () => {
    render(<SheinStudioPageShell />);

    expect(screen.getByText("当前步骤")).toBeInTheDocument();
    expect(screen.getByText("先选择要处理的 SDS 商品")).toBeInTheDocument();
    expect(
      screen.getByText("完成选品后，系统会带着模板和变体信息进入图片生成。"),
    ).toBeInTheDocument();
  });
});

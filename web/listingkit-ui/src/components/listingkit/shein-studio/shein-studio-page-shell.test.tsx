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

    expect(screen.getByText("SHEIN 工作室")).toBeInTheDocument();
    expect(screen.getByText("从 SDS 商品生成 SHEIN 上架任务")).toBeInTheDocument();
    expect(screen.getByText("当前步骤")).toBeInTheDocument();
    expect(screen.getByText("先选择要处理的 SDS 商品")).toBeInTheDocument();
    expect(
      screen.getByText("完成选品后，系统会带着模板和变体信息进入图片生成。"),
    ).toBeInTheDocument();
    expect(screen.getByText("workbench slot")).toBeInTheDocument();
    expect(screen.getByText("先继续最近批次，或新建一个批次再开始选品。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "继续最近批次" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "新建批次后选品" })).toBeInTheDocument();
    expect(screen.getByText("picker modal")).toBeInTheDocument();
  });

  it("allows the POD route to use POD-facing header copy", () => {
    render(
      <SheinStudioPageShell
        eyebrow="POD"
        layout="compact"
        title="从 POD 商品生成上架资料"
        description="选择 POD 商品、生成图片、审核款式，然后创建平台资料确认任务。"
      />,
    );

    expect(screen.getByText("POD")).toBeInTheDocument();
    expect(screen.getByText("从 POD 商品生成上架资料")).toBeInTheDocument();
    expect(
      screen.getByText("选择 POD 商品、生成图片、审核款式，然后创建平台资料确认任务。"),
    ).toBeInTheDocument();
    expect(screen.queryByText("当前步骤")).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "查看款式图库" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "查看 SHEIN 任务" })).not.toBeInTheDocument();
  });
});

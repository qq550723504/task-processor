import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinStudioPageShell } from "@/components/listingkit/shein-studio/shein-studio-page-shell";
import { dispatchSheinStudioRecentBatchesRecommendation } from "@/lib/shein-studio/recent-batches-focus";
import { SHEIN_STUDIO_RECENT_BATCHES_RECOMMENDATION_EVENT } from "@/lib/shein-studio/recent-batches-focus";
import { dispatchSheinStudioSectionFocus, SHEIN_STUDIO_SECTION_FOCUS_EVENT } from "@/lib/shein-studio/section-highlight";

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

  it("surfaces the recent batch risk recommendation in the homepage guidance", () => {
    render(<SheinStudioPageShell />);

    act(() => {
      window.dispatchEvent(
        new CustomEvent(SHEIN_STUDIO_RECENT_BATCHES_RECOMMENDATION_EVENT, {
          detail: { recommendedRiskLabel: "Baseline 未就绪" },
        }),
      );
    });

    expect(
      screen.getByText(
        "如果只是接着处理上一轮内容，优先从最近批次进入会更快，建议先处理“Baseline 未就绪”。",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: "继续最近批次（优先处理 Baseline 未就绪）",
      }),
    ).toBeInTheDocument();
  });

  it("promotes creating a batch when no recent batches can be resumed", () => {
    render(<SheinStudioPageShell />);

    act(() => {
      window.dispatchEvent(
        new CustomEvent(SHEIN_STUDIO_RECENT_BATCHES_RECOMMENDATION_EVENT, {
          detail: { hasRecoverableBatches: false },
        }),
      );
    });

    expect(
      screen.getByText("还没有可继续的最近批次，建议先新建一个批次再开始选品。"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: "开始新建批次并选品",
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: "继续最近批次",
      }),
    ).not.toBeInTheDocument();
  });

  it("highlights the product picker after starting a new batch from the homepage guidance", () => {
    render(<SheinStudioPageShell />);

    fireEvent.click(screen.getByRole("button", { name: "新建批次后选品" }));

    expect(screen.getByTestId("shein-studio-product-picker")).toHaveClass(
      "ring-2",
    );
  });

  it("highlights recent batches after continuing from the homepage guidance", () => {
    render(<SheinStudioPageShell />);

    fireEvent.click(screen.getByRole("button", { name: "继续最近批次" }));

    expect(screen.getByTestId("shein-studio-recent-batches")).toHaveClass(
      "ring-2",
    );
  });

  it("responds to a shared section focus event for the product picker", () => {
    render(<SheinStudioPageShell />);

    act(() => {
      dispatchSheinStudioSectionFocus({ action: "product-picker" });
    });

    expect(screen.getByTestId("shein-studio-product-picker")).toHaveClass(
      "ring-2",
    );
  });

  it("follows the shared homepage flow from recommendation to focused product picker", () => {
    render(<SheinStudioPageShell />);

    act(() => {
      dispatchSheinStudioRecentBatchesRecommendation({
        hasRecoverableBatches: false,
      });
    });

    fireEvent.click(screen.getByRole("button", { name: "开始新建批次并选品" }));

    expect(
      screen.getByText("还没有可继续的最近批次，建议先新建一个批次再开始选品。"),
    ).toBeInTheDocument();
    expect(screen.getByTestId("shein-studio-product-picker")).toHaveClass(
      "ring-2",
    );
  });
});

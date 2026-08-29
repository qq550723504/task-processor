import { render, screen, within } from "@testing-library/react";
import { vi } from "vitest";

import { WorkspaceAgentSurface, WorkspaceReviewView } from "@/components/listingkit/workspace/workspace-screen-views";

vi.mock("@/components/listingkit/shared/preview-canvas", () => ({
  PreviewCanvas: () => <div>预览画布内容</div>,
}));

vi.mock("@/components/listingkit/review/recovery-action-list", () => ({
  RecoveryActionList: () => <div>恢复建议列表</div>,
}));

vi.mock("@/components/listingkit/review/review-section-tabs", () => ({
  ReviewSectionTabs: ({ sections }: { sections?: unknown[] }) =>
    (sections?.length ?? 0) > 1 ? <div>审核分区导航</div> : null,
}));

vi.mock("@/components/listingkit/review/review-toolbar", () => ({
  ReviewToolbar: () => <div>工具栏动作</div>,
}));

vi.mock("@/components/listingkit/review/scene-preset-panel", () => ({
  ScenePresetPanel: () => <div>场景预设面板</div>,
}));

vi.mock("@/components/listingkit/review/slot-navigation-list", () => ({
  SlotNavigationList: ({ slots }: { slots?: unknown[] }) =>
    (slots?.length ?? 0) > 1 ? <div>款式槽位导航</div> : null,
}));

vi.mock("@/components/listingkit/shein/shein-data-image-gallery", () => ({
  SheinDataImageGallery: () => <div>图片画廊</div>,
}));

vi.mock("@/components/listingkit/shein/shein-final-review-panel", () => ({
  SheinFinalReviewPanel: () => <div>最终确认面板</div>,
}));

vi.mock("@/components/listingkit/shein/shein-submit-readiness-panel", () => ({
  SheinSubmitReadinessPanel: () => <div>发布检查</div>,
}));

vi.mock("@/components/listingkit/shein/shein-submission-timeline", () => ({
  SheinSubmissionTimeline: () => <div>提交时间线</div>,
}));

vi.mock("@/components/listingkit/workspace/workspace-preview-suggestion", () => ({
  WorkspacePreviewSuggestionCard: ({ suggestion }: { suggestion?: unknown }) =>
    suggestion ? <div>预览建议卡</div> : null,
}));

vi.mock("@/components/listingkit/image-agent/image-agent-workbench", () => ({
  ImageAgentWorkbench: ({ taskId, runId }: { taskId: string; runId: string }) => (
    <div data-testid="image-agent-workbench">
      场景图 scene-2 生成失败 仅重试 scene-2 {taskId} {runId}
    </div>
  ),
}));

describe("WorkspaceReviewView", () => {
  it.each(["旧预览加载中", "旧预览加载失败", "旧预览数据缺失"])(
    "keeps the explicit blocked Agent surface visible with %s",
    (legacyState) => {
      render(
        <WorkspaceAgentSurface context={{ taskId: "task-1", runId: "run-1" }}>
          <div>{legacyState}</div>
        </WorkspaceAgentSurface>,
      );
      expect(screen.getByText(/场景图 scene-2 生成失败/)).toBeInTheDocument();
      expect(screen.getByText(legacyState)).toBeInTheDocument();
    },
  );
  it("organizes shein general review into three primary stages", () => {
    const { container } = render(
      <WorkspaceReviewView
        selectedPlatform="shein"
        previewSuggestionProps={{
          suggestion: {
            slot: "detail_front" as never,
            title: "确认普通属性",
            summary: "先补齐商品属性",
            ctaLabel: "去处理",
          },
          onSelect: vi.fn(),
        }}
        reviewSectionTabsProps={{
          sections: [{ section_key: "a" }, { section_key: "b" }] as never,
          selectedKey: "a",
          onSelect: vi.fn(),
        }}
        sheinImageGalleryProps={{} as never}
        sheinFinalReviewProps={{} as never}
        previewCanvasProps={{} as never}
        slotNavigationProps={{
          slots: [{ slot: "a" }, { slot: "b" }] as never,
          onSelect: vi.fn(),
        }}
        reviewToolbarProps={{} as never}
        sheinReadinessProps={{} as never}
        sheinTimelineProps={{} as never}
        scenePresetPanelProps={{} as never}
        recoveryActionListProps={{} as never}
      />,
    );

    const repairSection = screen.getByText("审核修复").closest("section");
    const previewSection = screen.getByText("预览确认").closest("section");
    const submitSection = screen.getByText("提交准备").closest("section");

    expect(repairSection).not.toBeNull();
    expect(previewSection).not.toBeNull();
    expect(submitSection).not.toBeNull();

    expect(within(repairSection as HTMLElement).getByText("发布检查")).toBeInTheDocument();
    expect(within(repairSection as HTMLElement).getByText("预览建议卡")).toBeInTheDocument();
    expect(within(previewSection as HTMLElement).getByText("图片画廊")).toBeInTheDocument();
    expect(within(previewSection as HTMLElement).getByText("预览画布内容")).toBeInTheDocument();
    expect(within(submitSection as HTMLElement).getByText("最终确认面板")).toBeInTheDocument();
    expect(within(submitSection as HTMLElement).queryByText("来源商品面板")).not.toBeInTheDocument();

    expect(screen.getByText("更多诊断")).toBeInTheDocument();
    expect(screen.getByText("工具栏动作")).toBeInTheDocument();
    expect(container.firstElementChild).not.toHaveClass("lg:grid-cols-[minmax(0,1fr)_21rem]");
    expect(container.firstElementChild).toHaveClass("2xl:grid-cols-[minmax(0,1fr)_24rem]");
    expect(screen.getByText("最终确认草稿").closest("summary")).toHaveClass("flex-col");
    expect(screen.queryByTestId("image-agent-workbench")).not.toBeInTheDocument();
  });

  it("adds the image-agent workbench without replacing the existing review for an explicit run context", () => {
    render(
      <WorkspaceReviewView
        imageAgentContext={{ taskId: "task-1", runId: "run-1" }}
        previewSuggestionProps={{ onSelect: vi.fn() }}
        reviewSectionTabsProps={{ onSelect: vi.fn() } as never}
        sheinImageGalleryProps={{} as never}
        sheinFinalReviewProps={{} as never}
        previewCanvasProps={{} as never}
        slotNavigationProps={{ onSelect: vi.fn() } as never}
        reviewToolbarProps={{} as never}
        sheinReadinessProps={{} as never}
        sheinTimelineProps={{} as never}
        scenePresetPanelProps={{} as never}
        recoveryActionListProps={{} as never}
      />,
    );

    expect(screen.getByTestId("image-agent-workbench")).toHaveTextContent("task-1 run-1");
    expect(screen.getByText("工具栏动作")).toBeInTheDocument();
    expect(screen.getByText("更多诊断")).toBeInTheDocument();
  });
});

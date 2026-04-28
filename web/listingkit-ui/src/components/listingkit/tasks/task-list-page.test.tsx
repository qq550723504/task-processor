import { render, screen } from "@testing-library/react";

import { TaskListPage } from "@/components/listingkit/tasks/task-list-page";

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  refetch: vi.fn(),
  useListingKitTasks: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mocks.push }),
  useSearchParams: () => new URLSearchParams(""),
}));

vi.mock("@/lib/query/use-task-list", () => ({
  useListingKitTasks: (query: unknown) => mocks.useListingKitTasks(query),
}));

vi.mock("@/components/listingkit/shein/shein-settings-card", () => ({
  SheinSettingsCard: () => <div data-testid="shein-settings-card" />,
}));

describe("TaskListPage", () => {
  beforeEach(() => {
    mocks.push.mockReset();
    mocks.refetch.mockReset();
    mocks.useListingKitTasks.mockReset();
  });

  it("renders SHEIN workflow filters and row statuses in Chinese", () => {
    mocks.useListingKitTasks.mockReturnValue({
      data: {
        page: 1,
        page_size: 20,
        total: 1,
        items: [
          {
            task_id: "task-1",
            status: "completed",
            platforms: ["shein"],
            title: "测试商品",
            image_count: 4,
            shein_workflow_status: "draft_saved",
            shein_latest_submission_status: "success",
            created_at: "2026-04-27T10:00:00Z",
          },
        ],
      },
      isLoading: false,
      isError: false,
      refetch: mocks.refetch,
    });

    render(<TaskListPage />);

    expect(screen.getByRole("option", { name: "全部 SHEIN 状态" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "待确认" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "可提交" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "发布失败" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "已发布" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "草稿已保存" })).toBeInTheDocument();
    expect(screen.getAllByText("已完成").length).toBeGreaterThan(0);
    expect(screen.getAllByText("草稿已保存").length).toBeGreaterThan(0);
    expect(screen.getByText("最近提交：成功")).toBeInTheDocument();
  });

  it("renders latest SHEIN failure summary in Chinese", () => {
    mocks.useListingKitTasks.mockReturnValue({
      data: {
        page: 1,
        page_size: 20,
        total: 1,
        items: [
          {
            task_id: "task-2",
            status: "needs_review",
            platforms: ["shein"],
            title: "失败商品",
            shein_workflow_status: "publish_failed",
            shein_latest_submission_error: "方形图必须有一个",
          },
        ],
      },
      isLoading: false,
      isError: false,
      refetch: mocks.refetch,
    });

    render(<TaskListPage />);

    expect(screen.getAllByText("发布失败").length).toBeGreaterThan(0);
    expect(
      screen.getByText("最近提交：发布失败。原始错误：方形图必须有一个"),
    ).toBeInTheDocument();
  });
});

import { fireEvent, render, screen } from "@testing-library/react";

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

  it("renders the full task id in each row", () => {
    mocks.useListingKitTasks.mockReturnValue({
      data: {
        page: 1,
        page_size: 20,
        total: 1,
        items: [
          {
            task_id: "10856aa8-7e11-4257-ac11-dd095ed1593d",
            status: "completed",
            platforms: ["shein"],
            title: "完整任务 ID",
          },
        ],
      },
      isLoading: false,
      isError: false,
      refetch: mocks.refetch,
    });

    render(<TaskListPage />);

    expect(screen.getByText("任务 ID")).toBeInTheDocument();
    expect(screen.getByText("10856aa8-7e11-4257-ac11-dd095ed1593d")).toBeInTheDocument();
  });

  it("shows diagnostic timestamps and refresh guidance when the task list fails", () => {
    mocks.useListingKitTasks.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      refetch: mocks.refetch,
    });

    render(<TaskListPage />);

    expect(screen.getByText("任务列表加载失败")).toBeInTheDocument();
    expect(
      screen.getByText("后端列表接口暂时不可用，可以刷新重试。"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getAllByRole("button", { name: "刷新" })[1]);
    expect(mocks.refetch).toHaveBeenCalledTimes(1);
  });

  it("shows latest update time and completion context in the row card", () => {
    mocks.useListingKitTasks.mockReturnValue({
      data: {
        page: 1,
        page_size: 20,
        total: 1,
        items: [
          {
            task_id: "task-3",
            status: "completed",
            platforms: ["shein"],
            title: "诊断商品",
            created_at: "2026-04-27T10:00:00Z",
            updated_at: "2026-04-27T10:30:00Z",
            completed_at: "2026-04-27T11:00:00Z",
          },
        ],
      },
      isLoading: false,
      isError: false,
      refetch: mocks.refetch,
    });

    render(<TaskListPage />);

    expect(screen.getByText(/最近更新\s/)).toBeInTheDocument();
    expect(screen.getByText(/完成时间\s/)).toBeInTheDocument();
  });
});

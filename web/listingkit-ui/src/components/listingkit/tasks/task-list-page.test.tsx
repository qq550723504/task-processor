import { fireEvent, render, screen } from "@testing-library/react";

import { TaskListPage } from "@/components/listingkit/tasks/task-list-page";

const mocks = vi.hoisted(() => ({
  currentSearch: "",
  push: vi.fn(),
  refetch: vi.fn(),
  useListingKitTasks: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mocks.push }),
  useSearchParams: () => new URLSearchParams(mocks.currentSearch),
}));

vi.mock("@/lib/query/use-task-list", () => ({
  useListingKitTasks: (query: unknown) => mocks.useListingKitTasks(query),
}));

describe("TaskListPage", () => {
  beforeEach(() => {
    mocks.currentSearch = "";
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
            shein_submission_remote_status: "confirmed",
            shein_submission_remote_record_id: "record-123",
            shein_submission_remote_checked_at: "2026-04-27T10:03:00Z",
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
    expect(screen.getAllByText("生成 已完成").length).toBeGreaterThan(0);
    expect(screen.getAllByText("SHEIN 草稿已保存").length).toBeGreaterThan(0);
    expect(screen.getAllByText("远端已确认").length).toBeGreaterThan(0);
    expect(screen.getByText("最近提交：成功")).toBeInTheDocument();
    expect(screen.getByText(/SHEIN 远端：远端已确认 · record-123/)).toBeInTheDocument();
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

    expect(screen.getAllByText("SHEIN 发布失败").length).toBeGreaterThan(0);
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
            shein_store_id: 903,
            shein_store_site: "GB",
            shein_store_profile_id: 17,
            shein_store_resolved_at: "2026-05-18T08:15:00Z",
            shein_store_strategy: "country",
            shein_store_reason: "根据任务国家信息命中了对应店铺。",
            shein_store_matched_rule_kinds: ["country"],
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
    expect(screen.getByText("SHEIN 店铺 903 · GB")).toBeInTheDocument();
    expect(screen.getByText(/路由 按国家匹配/)).toBeInTheDocument();
    expect(screen.getByText(/Profile #17/)).toBeInTheDocument();
    expect(screen.getByText(/固化/)).toBeInTheDocument();
    expect(screen.getByText("SHEIN 店铺 903 · GB").getAttribute("title") ?? "").toContain(
      "根据任务国家信息命中了对应店铺。",
    );
  });

  it("renders shein overview, work queue, and action queue when present", () => {
    mocks.useListingKitTasks.mockReturnValue({
      data: {
        page: 1,
        page_size: 20,
        total: 1,
        summary: {
          shein_work_queue_counts: { repair_queue: 3 },
          shein_action_queue_counts: { final_review_queue: 2 },
          shein_blocker_counts: { final_review: 2 },
          shein_warning_counts: { manual_notes: 1 },
        },
        taxonomy: {
          shein_blockers: [
            { key: "final_review", label: "最终确认", severity: "warning" },
          ],
          shein_warnings: [
            { key: "manual_notes", label: "人工备注", severity: "warning" },
          ],
          shein_work_queues: [
            { key: "repair_queue", label: "修复队列", severity: "negative" },
          ],
          shein_action_queues: [
            { key: "final_review_queue", label: "最终确认", severity: "warning" },
          ],
        },
        items: [
          {
            task_id: "task-queue-1",
            status: "completed",
            platforms: ["shein"],
            title: "待确认商品",
            shein_workflow_status: "pending_confirmation",
            shein_work_queue: "repair_queue",
            shein_action_queue: "final_review_queue",
            shein_status_overview: {
              headline: "SHEIN 资料包暂不能直接提交",
              subheadline: "提交前必须在最终确认页核对图片、价格、属性和 SKU 后确认",
              blocking_count: 1,
              warning_count: 1,
              primary_action: "最终确认",
            },
          },
        ],
      },
      isLoading: false,
      isError: false,
      refetch: mocks.refetch,
    });

    render(<TaskListPage />);

    expect(screen.getAllByText("修复队列").length).toBeGreaterThan(0);
    expect(screen.getAllByText("最终确认").length).toBeGreaterThan(0);
    expect(screen.getByRole("option", { name: "人工备注" })).toBeInTheDocument();
    expect(screen.getByText("SHEIN 资料包暂不能直接提交")).toBeInTheDocument();
    expect(screen.getByText(/阻断 1/)).toBeInTheDocument();
    expect(screen.getByText(/待确认 1/)).toBeInTheDocument();
  });

  it("applies facet filters when summary chips are clicked", () => {
    mocks.useListingKitTasks.mockReturnValue({
      data: {
        page: 1,
        page_size: 20,
        total: 1,
        summary: {
          shein_work_queue_counts: { repair_queue: 3 },
          shein_action_queue_counts: { final_review_queue: 2 },
          shein_blocker_counts: { final_review: 2 },
          shein_warning_counts: { manual_notes: 1 },
        },
        taxonomy: {
          shein_blockers: [
            { key: "final_review", label: "最终确认", severity: "warning" },
          ],
          shein_warnings: [
            { key: "manual_notes", label: "人工备注", severity: "warning" },
          ],
          shein_work_queues: [
            { key: "repair_queue", label: "修复队列", severity: "negative" },
          ],
          shein_action_queues: [
            { key: "final_review_queue", label: "最终确认", severity: "warning" },
          ],
        },
        items: [],
      },
      isLoading: false,
      isError: false,
      refetch: mocks.refetch,
    });

    render(<TaskListPage />);

    fireEvent.click(screen.getByRole("button", { name: "修复队列 · 3" }));
    expect(mocks.push).toHaveBeenCalledWith(
      "/listing-kits?shein_work_queue=repair_queue",
    );

    fireEvent.click(screen.getByRole("button", { name: "人工备注 · 1" }));
    expect(mocks.push).toHaveBeenCalledWith(
      "/listing-kits?shein_warning_key=manual_notes",
    );
  });

  it("shows active facet state and clears it from summary controls", () => {
    mocks.currentSearch = "shein_work_queue=repair_queue";
    mocks.useListingKitTasks.mockReturnValue({
      data: {
        page: 1,
        page_size: 20,
        total: 1,
        summary: {
          shein_work_queue_counts: { repair_queue: 3 },
        },
        taxonomy: {
          shein_work_queues: [
            { key: "repair_queue", label: "修复队列", severity: "negative" },
          ],
        },
        items: [],
      },
      isLoading: false,
      isError: false,
      refetch: mocks.refetch,
    });

    render(<TaskListPage />);

    expect(
      screen.getByRole("button", { name: "修复队列 · 3" }),
    ).toHaveAttribute("aria-pressed", "true");

    fireEvent.click(screen.getByRole("button", { name: "清除" }));
    expect(mocks.push).toHaveBeenCalledWith("/listing-kits");
  });

  it("shows active filter chips and clears all active filters together", () => {
    mocks.currentSearch =
      "platform=shein&shein_work_queue=repair_queue&shein_action_queue=final_review_queue";
    mocks.useListingKitTasks.mockReturnValue({
      data: {
        page: 1,
        page_size: 20,
        total: 1,
        summary: {
          shein_work_queue_counts: { repair_queue: 3 },
          shein_action_queue_counts: { final_review_queue: 2 },
        },
        taxonomy: {
          shein_work_queues: [
            { key: "repair_queue", label: "修复队列", severity: "negative" },
          ],
          shein_action_queues: [
            { key: "final_review_queue", label: "最终确认", severity: "warning" },
          ],
        },
        items: [],
      },
      isLoading: false,
      isError: false,
      refetch: mocks.refetch,
    });

    render(<TaskListPage />);

    expect(screen.getByText("当前筛选")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "SHEIN" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "修复队列" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "最终确认" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "清空全部" }));
    expect(mocks.push).toHaveBeenCalledWith("/listing-kits");
  });

  it("renders pagination controls and updates the page query", () => {
    mocks.currentSearch = "page=2";
    mocks.useListingKitTasks.mockReturnValue({
      data: {
        page: 2,
        page_size: 20,
        total: 45,
        items: [
          {
            task_id: "task-page-2",
            status: "completed",
            platforms: ["shein"],
            title: "第二页商品",
          },
        ],
      },
      isLoading: false,
      isError: false,
      refetch: mocks.refetch,
    });

    render(<TaskListPage />);

    expect(screen.getByText("第 2 / 3 页")).toBeInTheDocument();
    expect(screen.getByText("21-40 / 45")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "上一页" }));
    expect(mocks.push).toHaveBeenCalledWith("/listing-kits");

    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    expect(mocks.push).toHaveBeenCalledWith("/listing-kits?page=3");
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

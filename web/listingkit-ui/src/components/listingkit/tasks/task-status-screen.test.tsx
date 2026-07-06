import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { saveTaskCreateDraft } from "@/components/listingkit/tasks/task-create-draft";
import { TaskStatusScreen } from "@/components/listingkit/tasks/task-status-screen";

const push = vi.fn();
const revisionHistoryMock = vi.fn();
const revisionHistoryDetailMock = vi.fn();
const executeActionMutate = vi.fn();
const retryChildTaskMutate = vi.fn();
const recoverTaskNowMutate = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push,
  }),
}));

vi.mock("@/lib/query/use-revision-history", () => ({
  useTaskRevisionHistory: (...args: unknown[]) => revisionHistoryMock(...args),
  useTaskRevisionHistoryDetail: (...args: unknown[]) => revisionHistoryDetailMock(...args),
}));

vi.mock("@/lib/query/use-action", () => ({
  useExecuteAction: () => ({
    mutate: executeActionMutate,
    isPending: false,
  }),
}));

vi.mock("@/lib/query/use-child-task-retry", () => ({
  useRetryChildTask: () => ({
    mutate: retryChildTaskMutate,
    isPending: false,
  }),
}));

vi.mock("@/lib/query/use-task-recovery", () => ({
  useRecoverTaskNow: () => ({
    mutate: recoverTaskNowMutate,
    isPending: false,
  }),
  useBulkRecoverTasks: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

describe("TaskStatusScreen", () => {
  beforeEach(() => {
    push.mockReset();
    revisionHistoryMock.mockReset();
    revisionHistoryDetailMock.mockReset();
    executeActionMutate.mockReset();
    retryChildTaskMutate.mockReset();
    recoverTaskNowMutate.mockReset();
    revisionHistoryMock.mockReturnValue({
      data: { items: [] },
      isLoading: false,
    });
    revisionHistoryDetailMock.mockReturnValue({
      data: undefined,
      isLoading: false,
    });
    window.sessionStorage.clear();
  });

  it("shows review entrypoints for completed tasks", () => {
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "completed",
        }}
      />,
    );

    expect(screen.getByText("任务已处理完成")).toBeInTheDocument();
    expect(
      screen.getByText("1.5 秒后会自动进入工作台，你也可以先留在这里查看状态。"),
    ).toBeInTheDocument();
    expect(setTimeoutSpy).toHaveBeenCalledWith(expect.any(Function), 1500);
    const callback = setTimeoutSpy.mock.calls[0]?.[0] as (() => void) | undefined;
    callback?.();
    expect(push).toHaveBeenCalledWith("/listing-kits/task_123/workspace");
    setTimeoutSpy.mockRestore();
  });

  it("treats needs-review as a terminal review state", () => {
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "needs_review",
          error:
            "SHEIN 类目解析尚未命中真实 category_id；SHEIN 属性模板尚未完成真实 attribute_id 映射；SHEIN 销售属性尚未完成真实 sale attribute 映射",
        }}
      />,
    );

    expect(screen.getAllByText("任务需要人工确认")).toHaveLength(2);
    expect(
      screen.getByText("建议先查看工作台和结果，再决定继续提交还是回退修改。"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("1.5 秒后会自动进入工作台，你也可以先留在这里查看状态。"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "去确认类目" })).toHaveAttribute(
      "href",
      "/listing-kits/task_123/workspace?platform=shein&section_key=general_review#shein-category-review-card",
    );
    expect(setTimeoutSpy).toHaveBeenCalledWith(expect.any(Function), 1500);
    setTimeoutSpy.mockRestore();
  });

  it("still allows manual workspace entry before auto-open completes", () => {
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "completed",
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "打开工作台" }));
    expect(push).toHaveBeenCalledWith("/listing-kits/task_123/workspace");
  });

  it("allows cancelling auto-open for completed tasks", () => {
    const clearTimeoutSpy = vi.spyOn(window, "clearTimeout");
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "completed",
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "取消自动跳转" }));

    expect(screen.getByText("已暂停自动跳转。")).toBeInTheDocument();
    expect(clearTimeoutSpy).toHaveBeenCalled();
    expect(push).not.toHaveBeenCalled();
    clearTimeoutSpy.mockRestore();
  });

  it("keeps processing tasks on the status page", () => {
    const { container } = render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "processing",
          created_at: "2026-05-04T10:00:00Z",
          result: {
            updated_at: "2026-05-04T10:30:00Z",
          },
        }}
      />,
    );

    expect(screen.getByText("任务状态")).toBeInTheDocument();
    expect(screen.getByText("任务 ID")).toBeInTheDocument();
    expect(screen.getAllByText("task_123").length).toBeGreaterThan(0);
    expect(screen.getAllByText("最近更新").length).toBeGreaterThan(0);
    expect(screen.getByText("任务处理中")).toBeInTheDocument();
    expect(screen.getByText("正在生成图片")).toBeInTheDocument();
    expect(container.querySelector(".sm\\:grid-cols-2")).not.toBeNull();
    expect(
      screen.queryByRole("button", { name: "打开工作台" }),
    ).not.toBeInTheDocument();
  });

  it("keeps queue and workspace entrypoints for failed tasks", () => {
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "failed",
          error:
            "product enrichment failed: 数据质量不足\n\n必需改进操作：\n1. 添加至少 3 张高质量产品图片\n2. 提供至少 50 字符的产品描述\n",
        }}
      />,
    );

    expect(screen.getByText("任务处理失败")).toBeInTheDocument();
    expect(screen.getByText("建议先处理这些问题")).toBeInTheDocument();
    expect(screen.getByText("添加至少 3 张高质量产品图片")).toBeInTheDocument();
    expect(screen.getByText("提供至少 50 字符的产品描述")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "基于当前内容重新创建任务" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "打开工作台" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "打开队列" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "基于当前内容重新创建任务" }));
    expect(push).toHaveBeenCalledWith(
      "/listing-kits/new?fromTask=task_123&focus=imageUrls&issues=imageUrls,text",
    );
  });

  it("shows the task source summary when a creation draft exists", () => {
    saveTaskCreateDraft("task_123", {
      text: "",
      imageUrls: "",
      productUrl: "https://detail.1688.com/offer/123456789.html",
      platforms: ["shein"],
    });

    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "processing",
        }}
      />,
    );

    expect(screen.getByText("任务来源")).toBeInTheDocument();
    expect(screen.getByText("来自商品链接")).toBeInTheDocument();
  });

  it("shows revision history with store resolution audit when revisions exist", () => {
    revisionHistoryMock.mockReturnValue({
      data: {
        items: [
          {
            revision_id: "rev-1",
            updated_at: "2026-05-18T08:15:00Z",
            action_type: "edit",
            timeline: {
              headline: "刷新 SHEIN 类目模板",
              relation_text: "将重算类目 / 普通属性 / 销售属性",
            },
            store_resolution: {
              store_id: 903,
              site: "GB",
            },
          },
        ],
      },
      isLoading: false,
    });
    revisionHistoryDetailMock.mockReturnValue({
      data: {
        record: {
          revision_id: "rev-1",
          updated_at: "2026-05-18T08:15:00Z",
          reason: "manual adjustment",
          action_type: "edit",
          timeline: {
            headline: "刷新 SHEIN 类目模板",
            relation_text: "将重算类目 / 普通属性 / 销售属性",
          },
          store_resolution: {
            store_id: 903,
            site: "GB",
            strategy: "country",
            reason: "根据任务国家信息命中了对应店铺。",
            matched_rule_kinds: ["country"],
            matched_profile_id: 17,
            resolved_at: "2026-05-18T08:15:00Z",
          },
        },
      },
      isLoading: false,
    });

    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "processing",
        }}
      />,
    );

    expect(screen.getByText("修订历史")).toBeInTheDocument();
    expect(screen.getAllByText("刷新 SHEIN 类目模板").length).toBeGreaterThan(0);
    expect(
      screen.getAllByText("将重算类目 / 普通属性 / 销售属性").length,
    ).toBeGreaterThan(0);
    expect(screen.getByText("店铺快照")).toBeInTheDocument();
    expect(screen.getByText("SHEIN 店铺 903 · GB")).toBeInTheDocument();
    expect(screen.getByText("Profile #17")).toBeInTheDocument();
  });

  it("uses publish-aware wording when the SHEIN task is already published", () => {
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "completed",
          shein_workflow_status: "published",
          shein_submission_remote_status: "confirmed",
        }}
      />,
    );

    expect(screen.getByText("任务生成已完成，SHEIN 已发布")).toBeInTheDocument();
    expect(
      screen.getByText("生成链路已经完成，且商品资料已经发布到 SHEIN。建议进入工作台核对最终结果和提交记录。"),
    ).toBeInTheDocument();
    expect(screen.getByText("SHEIN 已发布")).toBeInTheDocument();
    expect(screen.getByText("远端已确认")).toBeInTheDocument();
  });

  it("shows SHEIN submission failures even when generation completed", () => {
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "completed",
          shein_workflow_status: "publish_failed",
          shein_latest_submission_status: "failed",
          shein_latest_submission_error: "方形图必须有一个",
        }}
      />,
    );

    expect(screen.getByText("任务生成已完成，SHEIN 提交失败")).toBeInTheDocument();
    expect(screen.getByText("SHEIN 提交失败")).toBeInTheDocument();
    expect(screen.getByText("方形图必须有一个")).toBeInTheDocument();
  });

  it("shows SHEIN publish status for completed tasks", () => {
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "completed",
          shein_workflow_status: "published",
          shein_submission_remote_status: "confirmed",
        }}
      />,
    );

    expect(screen.getByText("SHEIN 已发布")).toBeInTheDocument();
    expect(screen.getByText("远端已确认")).toBeInTheDocument();
  });

  it("can manually trigger layered temporal actions", () => {
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "processing",
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "运行标准商品层" }));
    expect(executeActionMutate).toHaveBeenCalledWith({
      action_key: "run_standard_product_temporal",
    });

    fireEvent.click(screen.getByRole("button", { name: "运行平台适配层" }));
    expect(executeActionMutate).toHaveBeenCalledWith({
      action_key: "run_platform_adapt_temporal",
      target: {
        action_key: "run_platform_adapt_temporal",
        queue_query: {
          platform: "all",
        },
      },
    });
    expect(screen.getByRole("button", { name: "运行标准商品层" })).toHaveClass("w-full");
    expect(screen.getByRole("button", { name: "运行平台适配层" })).toHaveClass("w-full");
  });

  it("can retry the SDS design sync child task from the status screen", () => {
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "failed",
          result: {
            child_tasks: [
              {
                kind: "sds_design_sync",
                task_id: "child_1",
                status: "failed",
              },
            ],
          },
        }}
      />,
    );

    expect(screen.getByText("失败的子任务")).toBeInTheDocument();
    expect(screen.getByText("sds_design_sync")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "重试子任务" }));

    expect(retryChildTaskMutate).toHaveBeenCalledWith({
      kind: "sds_design_sync",
    });
  });

  it("can recover a blocked retryable task from the status screen", () => {
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "blocked_retryable",
          retryable_block: {
            reason_code: "worker_queue_backpressure",
            reason_message: "Worker queue is temporarily full.",
            blocked_at: "2026-06-06T04:00:00Z",
            next_retry_at: "2026-06-06T04:15:00Z",
            retry_attempts: 1,
            auto_resume_enabled: true,
          },
        }}
      />,
    );

    expect(screen.getAllByText("等待依赖恢复").length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole("button", { name: "立即恢复" }));

    expect(recoverTaskNowMutate).toHaveBeenCalledTimes(1);
  });
});

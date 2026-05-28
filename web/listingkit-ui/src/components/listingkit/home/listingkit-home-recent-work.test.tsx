import { render, screen } from "@testing-library/react";

import { ListingKitHomeRecentWork } from "@/components/listingkit/home/listingkit-home-recent-work";
import type { ListingKitTaskListItem } from "@/lib/types/listingkit/tasks";

function makeTask(
  overrides: Partial<ListingKitTaskListItem> = {},
): ListingKitTaskListItem {
  return {
    task_id: "task-1",
    status: "completed",
    platforms: ["shein"],
    title: "Task",
    image_count: 0,
    created_at: "2026-04-30T10:00:00+08:00",
    updated_at: "2026-04-30T10:00:00+08:00",
    ...overrides,
  };
}

describe("ListingKitHomeRecentWork", () => {
  it("renders a loading state while recent work is fetching", () => {
    render(<ListingKitHomeRecentWork isLoading isError={false} tasks={[]} />);

    expect(
      screen.getByRole("status", { name: "最近任务加载中" }),
    ).toBeInTheDocument();
  });

  it("prioritizes resumable SHEIN work for continue and shows only three recent tasks", () => {
    render(
      <ListingKitHomeRecentWork
        isLoading={false}
        isError={false}
        summary={{
          shein_work_queue_counts: {
            generation_queue: 9,
            repair_queue: 3,
            review_queue: 2,
            submit_ready_queue: 1,
          },
          shein_action_queue_counts: {
            final_review_queue: 4,
            pricing_queue: 2,
            manual_review_queue: 1,
          },
        }}
        taxonomy={{
          shein_work_queues: [
            { key: "submit_ready_queue", label: "待提交队列", severity: "positive" },
            { key: "repair_queue", label: "修复队列", severity: "negative" },
            { key: "review_queue", label: "复核队列", severity: "warning" },
            { key: "generation_queue", label: "生成队列", severity: "default" },
          ],
          shein_action_queues: [
            { key: "final_review_queue", label: "最终确认", severity: "warning" },
            { key: "pricing_queue", label: "价格处理", severity: "warning" },
            { key: "manual_review_queue", label: "人工备注复核", severity: "warning" },
          ],
        }}
        tasks={[
          makeTask({
            task_id: "amazon-processing",
            platforms: ["amazon"],
            status: "processing",
            title: "Amazon task",
            updated_at: "2026-04-30T11:00:00+08:00",
          }),
          makeTask({
            task_id: "resume-shein",
            status: "completed",
            shein_work_queue: "repair_queue",
            shein_status_overview: {
              headline: "待修复",
              subheadline: "补齐最终确认后可继续提交",
              blocking_count: 1,
              primary_action: "最终确认",
            },
            title: "Resume me",
            updated_at: "2026-04-30T10:00:00+08:00",
          }),
          makeTask({
            task_id: "three",
            title: "Three",
            updated_at: "2026-04-30T09:00:00+08:00",
          }),
          makeTask({
            task_id: "four",
            title: "Four",
            updated_at: "2026-04-30T08:00:00+08:00",
          }),
        ]}
      />,
    );

    expect(screen.getByRole("link", { name: "继续最近任务" })).toHaveAttribute(
      "href",
      "/listing-kits/resume-shein/workspace?platform=shein",
    );
    expect(screen.getByRole("link", { name: "待提交队列1" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "修复队列3" })).toHaveAttribute(
      "href",
      "/listing-kits?platform=shein&shein_work_queue=repair_queue",
    );
    expect(screen.getByRole("link", { name: "最终确认4" })).toHaveAttribute(
      "href",
      "/listing-kits?platform=shein&shein_action_queue=final_review_queue",
    );
    expect(screen.getByRole("link", { name: "复核队列2" })).toHaveAttribute(
      "href",
      "/listing-kits?platform=shein&shein_work_queue=review_queue",
    );
    expect(screen.getByRole("link", { name: "查看同队列" })).toHaveAttribute(
      "href",
      "/listing-kits?platform=shein&shein_work_queue=repair_queue",
    );
    expect(screen.getByRole("link", { name: "查看全部" })).toHaveAttribute(
      "href",
      "/listing-kits",
    );
    expect(screen.getAllByText("Resume me").length).toBeGreaterThan(0);
    expect(screen.getAllByText("修复队列").length).toBeGreaterThan(0);
    expect(screen.getByText("补齐最终确认后可继续提交")).toBeInTheDocument();
    expect(screen.getByText("阻断")).toBeInTheDocument();
    expect(screen.getAllByText("最终确认").length).toBeGreaterThan(0);
    expect(screen.getByText("Amazon task")).toBeInTheDocument();
    expect(screen.getByText("Three")).toBeInTheDocument();
    expect(screen.queryByText("Four")).not.toBeInTheDocument();
  });

  it("does not render action queue summary when there is no repair or review workload", () => {
    render(
      <ListingKitHomeRecentWork
        isLoading={false}
        isError={false}
        summary={{
          shein_work_queue_counts: {
            submit_ready_queue: 2,
            published_queue: 5,
          },
          shein_action_queue_counts: {
            submit_ready_action_queue: 2,
          },
        }}
        taxonomy={{
          shein_work_queues: [
            { key: "submit_ready_queue", label: "待提交队列", severity: "positive" },
            { key: "published_queue", label: "已发布队列", severity: "positive" },
          ],
          shein_action_queues: [
            { key: "submit_ready_action_queue", label: "直接提交", severity: "positive" },
          ],
        }}
        tasks={[makeTask()]}
      />,
    );

    expect(screen.getByText("工作队列")).toBeInTheDocument();
    expect(screen.queryByText("处理动作")).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "直接提交2" })).not.toBeInTheDocument();
  });

  it("routes continue to the SHEIN workspace when a resumable SHEIN task has mixed platform order", () => {
    render(
      <ListingKitHomeRecentWork
        isLoading={false}
        isError={false}
        tasks={[
          makeTask({
            task_id: "mixed-platform-task",
            status: "completed",
            title: "Mixed platform task",
            platforms: ["amazon", "shein"],
            shein_workflow_status: "draft_saved",
          }),
        ]}
      />,
    );

    expect(screen.getByRole("link", { name: "继续最近任务" })).toHaveAttribute(
      "href",
      "/listing-kits/mixed-platform-task/workspace?platform=shein",
    );
  });

  it("surfaces pod platform blockers in the continue card and links to blocker filter", () => {
    render(
      <ListingKitHomeRecentWork
        isLoading={false}
        isError={false}
        tasks={[
          makeTask({
            task_id: "resume-pod",
            title: "Resume pod task",
            shein_blocking_keys: ["pod_platform"],
            shein_status_overview: {
              headline: "POD 结果待确认",
              blocking_count: 1,
            },
          }),
        ]}
      />,
    );

    expect(screen.getAllByText("POD 平台待处理").length).toBeGreaterThan(0);
    expect(
      screen.getByText("POD 平台结果还未就绪，处理完成后才能继续正式发布。"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "查看同队列" })).toHaveAttribute(
      "href",
      "/listing-kits?platform=shein&shein_blocker_key=pod_platform",
    );
  });

  it("describes size-image fallback as a degradable pod warning", () => {
    render(
      <ListingKitHomeRecentWork
        isLoading={false}
        isError={false}
        tasks={[
          makeTask({
            task_id: "resume-size-fallback",
            title: "Resume size fallback task",
            pod_execution: {
              provider: "sds",
              dependency_mode: "optional",
              status: "failed_degraded",
              failure_reason: "size image render unavailable",
            },
          }),
        ]}
      />,
    );

    expect(screen.getAllByText("POD SDS 尺寸图已降级").length).toBeGreaterThan(0);
    expect(
      screen.getByText(
        "POD 平台尺寸图生成失败，当前会保留主图和场景图，按非 SDS 尺寸图路径继续发布。",
      ),
    ).toBeInTheDocument();
  });

  it("surfaces shein freshness blockers in the continue card and links to freshness filter", () => {
    render(
      <ListingKitHomeRecentWork
        isLoading={false}
        isError={false}
        tasks={[
          makeTask({
            task_id: "resume-auth-refresh",
            title: "Resume auth refresh task",
            shein_blocking_keys: ["shein_online_auth"],
            shein_status_overview: {
              headline: "店铺登录态待刷新",
              blocking_count: 1,
            },
          }),
        ]}
      />,
    );

    expect(screen.getAllByText("SHEIN 店铺待登录").length).toBeGreaterThan(0);
    expect(
      screen.getByText("SHEIN 提交店铺登录态已失效，刷新登录态后再继续正式发布。"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "查看同队列" })).toHaveAttribute(
      "href",
      "/listing-kits?platform=shein&shein_blocker_key=shein_online_auth",
    );
  });

  it("renders empty state when no tasks exist", () => {
    render(<ListingKitHomeRecentWork isLoading={false} isError={false} tasks={[]} />);

    expect(screen.getByText("还没有最近任务")).toBeInTheDocument();
  });

  it("renders inline error state without blocking the page when there is no stale data", () => {
    render(<ListingKitHomeRecentWork isLoading={false} isError tasks={[]} />);

    expect(screen.getByText("最近任务暂时加载失败")).toBeInTheDocument();
  });

  it("keeps rendering stale recent work while showing an inline warning", () => {
    render(
      <ListingKitHomeRecentWork
        isLoading={false}
        isError
        tasks={[
          makeTask({
            task_id: "resume-shein",
            status: "completed",
            shein_workflow_status: "draft_saved",
            title: "Resume me",
          }),
          makeTask({
            task_id: "stale-two",
            title: "Stale task",
            updated_at: "2026-04-30T09:00:00+08:00",
          }),
        ]}
      />,
    );

    expect(screen.getByText("最近任务暂时加载失败")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "继续最近任务" })).toBeInTheDocument();
    expect(screen.getAllByText("Resume me").length).toBeGreaterThan(0);
    expect(screen.getByText("Stale task")).toBeInTheDocument();
  });
});

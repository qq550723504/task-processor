import { fireEvent, render, screen } from "@testing-library/react";

import { TaskStatusPanel } from "@/components/listingkit/tasks/task-status-panel";

describe("TaskStatusPanel", () => {
  it("renders nothing for completed tasks", () => {
    const { container } = render(
      <TaskStatusPanel task={{ status: "completed" }} />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("renders failure details from the task result", () => {
    render(
      <TaskStatusPanel
        task={{
          status: "failed",
          error: "product enrichment failed",
          result: {
            child_tasks: [
              {
                kind: "product_enrich",
                task_id: "child-1",
                status: "failed",
                error: "quality score too low",
              },
            ],
          },
        }}
      />,
    );

    expect(screen.getByText("任务处理失败")).toBeInTheDocument();
    expect(screen.getByText("product enrichment failed")).toBeInTheDocument();
    expect(screen.getByText("失败的子任务")).toBeInTheDocument();
    expect(screen.getByText("product_enrich")).toBeInTheDocument();
    expect(screen.getByText("child-1")).toBeInTheDocument();
  });

  it("shows retry for failed sds design sync child tasks", () => {
    const onRetryChildTask = vi.fn();

    render(
      <TaskStatusPanel
        task={{
          status: "needs_review",
          result: {
            child_tasks: [
              {
                kind: "sds_design_sync",
                task_id: "child-1",
                status: "failed",
                error: "sync failed",
              },
            ],
          },
        }}
        onRetryChildTask={onRetryChildTask}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "重试子任务" }));

    expect(onRetryChildTask).toHaveBeenCalledWith("sds_design_sync");
  });

  it("still shows retry for completed tasks when the SDS child task failed", () => {
    const onRetryChildTask = vi.fn();

    render(
      <TaskStatusPanel
        task={{
          status: "completed",
          result: {
            child_tasks: [
              {
                kind: "sds_design_sync",
                task_id: "child-1",
                status: "failed",
                error: "sync failed",
              },
            ],
          },
        }}
        onRetryChildTask={onRetryChildTask}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "重试子任务" }));

    expect(onRetryChildTask).toHaveBeenCalledWith("sds_design_sync");
  });

  it("renders blocked retryable diagnostics and recover-now action", () => {
    const onRecoverNow = vi.fn();

    render(
      <TaskStatusPanel
        task={{
          task_id: "task-123",
          status: "blocked_retryable",
          retryable_block: {
            reason_code: "worker_queue_backpressure",
            reason_message: "Worker queue is temporarily full.",
            blocked_at: "2026-06-06T04:00:00Z",
            next_retry_at: "2026-06-06T04:15:00Z",
            retry_attempts: 2,
            max_auto_retry_attempts: 5,
            recovery_scope: "task",
            auto_resume_enabled: true,
          },
        }}
        onRecoverNow={onRecoverNow}
      />,
    );

    expect(screen.getAllByText("等待依赖恢复").length).toBeGreaterThan(0);
    expect(
      screen.getAllByText("Worker queue is temporarily full.").length,
    ).toBeGreaterThan(0);
    expect(screen.getByText("下次重试")).toBeInTheDocument();
    expect(screen.getByText("自动恢复")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "立即恢复" }));

    expect(onRecoverNow).toHaveBeenCalledTimes(1);
  });

  it("renders failed workflow stages and blocking issues", () => {
    render(
      <TaskStatusPanel
        task={{
          status: "failed",
          result: {
            workflow_stages: [
              {
                kind: "product_enrich",
                task_id: "product-task-1",
                status: "failed",
                error: "upstream timeout",
              },
            ],
            workflow_issues: [
              {
                severity: "blocking",
                stage: "product_enrich",
                code: "product_enrich_failed",
                message: "Product enrichment failed",
                detail: "upstream timeout",
              },
            ],
          },
        }}
      />,
    );

    expect(screen.getAllByText("upstream timeout").length).toBeGreaterThan(0);
    expect(
      screen.queryByText("Product enrichment failed"),
    ).not.toBeInTheDocument();
    expect(screen.getByText("失败的流程阶段")).toBeInTheDocument();
    expect(screen.getByText("product_enrich")).toBeInTheDocument();
    expect(screen.getByText("product-task-1")).toBeInTheDocument();
  });

  it("renders structured review reasons for needs-review tasks", () => {
    render(
      <TaskStatusPanel
        task={{
          status: "needs_review",
          review_reasons: [
            "The product type is 'Unknown Product'.",
            "The title is 'Unknown Product'.",
            "The IP risk level is 'medium' due to using scraped 1688 source images.",
          ],
          error: "legacy semicolon string should not be used here",
        }}
      />,
    );

    expect(screen.getByText("任务需要人工确认")).toBeInTheDocument();
    expect(screen.queryByText("需要人工确认的原因")).not.toBeInTheDocument();
    expect(
      screen.queryByText("The product type is 'Unknown Product'."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("The IP risk level is 'medium' due to using scraped 1688 source images."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("legacy semicolon string should not be used here"),
    ).not.toBeInTheDocument();
  });

  it("splits semicolon-joined review reasons into separate items", () => {
    render(
      <TaskStatusPanel
        task={{
          status: "needs_review",
          error:
            "The product type is 'Unknown Product'.; The title is 'Unknown Product'.； image pipeline uses scraped 1688 source images",
        }}
      />,
    );

    expect(
      screen.queryByText("The product type is 'Unknown Product'."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("The title is 'Unknown Product'."),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("image pipeline uses scraped 1688 source images"),
    ).not.toBeInTheDocument();
  });

  it("renders task diagnostics for in-flight tasks", () => {
    const { container } = render(
      <TaskStatusPanel
        task={{
          task_id: "task-123",
          status: "processing",
          created_at: "2026-05-04T10:00:00Z",
          result: {
            updated_at: "2026-05-04T10:30:00Z",
            shein_store_resolution: {
              store_id: 903,
              site: "GB",
              strategy: "country",
              reason: "根据任务国家信息命中了对应店铺。",
              matched_rule_kinds: ["country"],
              matched_profile_id: 17,
              resolved_at: "2026-05-18T08:15:00Z",
            },
          },
        }}
      />,
    );

    expect(screen.getByText("任务标识")).toBeInTheDocument();
    expect(screen.getByText("task-123")).toBeInTheDocument();
    expect(screen.getByText("最近更新")).toBeInTheDocument();
    expect(screen.getByText("已创建")).toBeInTheDocument();
    expect(screen.getByText("店铺解析")).toBeInTheDocument();
    expect(screen.getByText("SHEIN 店铺 903 · GB")).toBeInTheDocument();
    expect(screen.getByText("根据任务国家信息命中了对应店铺。")).toBeInTheDocument();
    expect(screen.getByText("命中规则：国家规则")).toBeInTheDocument();
    expect(screen.getByText("Profile #17")).toBeInTheDocument();
    expect(screen.getByText(/固化时间：/)).toBeInTheDocument();
    expect(container.querySelector(".sm\\:grid-cols-2")).not.toBeNull();
  });
});

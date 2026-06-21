import { render, screen, within } from "@testing-library/react";

import { TaskPodExecutionCard } from "@/components/listingkit/tasks/task-pod-execution-card";

describe("TaskPodExecutionCard", () => {
  it("renders SDS sync details for failed SDS child tasks", () => {
    render(
      <TaskPodExecutionCard
        task={{
          status: "failed",
          result: {
            sds_design_result: {
              variant_id: 89764,
              status: "failed",
            },
            child_tasks: [
              {
                kind: "sds_design_sync",
                status: "failed",
                task_id: "child-1",
              },
            ],
          },
        }}
      />,
    );

    expect(screen.getByText("POD 平台处理失败")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "重试子任务" })).not.toBeInTheDocument();
  });

  it("uses workflow stage status for the SDS process detail", () => {
    render(
      <TaskPodExecutionCard
        task={{
          status: "completed",
          result: {
            sds_design_result: {
              variant_id: 89764,
              product_id: 89764,
              status: "completed",
            },
            workflow_stages: [
              {
                kind: "sds_design_sync",
                status: "degraded",
                error: "SDS render returned blank template",
              },
            ],
            workflow_issues: [
              {
                severity: "warning",
                stage: "sds_design_sync",
                message: "SDS render returned blank template",
              },
            ],
          },
        }}
      />,
    );

    expect(screen.getByText("Workflow stage")).toBeInTheDocument();
    expect(screen.getByText("degraded")).toBeInTheDocument();
    expect(screen.getByText("SDS render returned blank template")).toBeInTheDocument();
  });

  it("shows SDS auth blockers before the raw sync error", () => {
    render(
      <TaskPodExecutionCard
        task={{
          status: "needs_review",
          result: {
            sds_design_result: {
              variant_id: 89764,
              status: "failed",
              error: "sds POST /ps/design/add_and_design auth required with status 400: 用户未登录",
            },
            workflow_stages: [
              {
                kind: "sds_design_sync",
                status: "degraded",
              },
            ],
            workflow_issues: [
              {
                code: "sds_auth_required",
                severity: "blocking",
                stage: "sds_design_sync",
                message: "SDS 登录状态已失效，需要重新登录或刷新授权后重试官方渲染",
                detail: "用户未登录",
              },
            ],
          },
        }}
      />,
    );

    expect(screen.getByText("SDS 登录状态需要处理")).toBeInTheDocument();
    expect(
      screen.getByText("SDS 登录状态已失效，需要重新登录或刷新授权后重试官方渲染"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/^sds POST/)).not.toBeInTheDocument();
  });

  it("shows the detailed SDS upstream reason together with the summarized sync error", () => {
    render(
      <TaskPodExecutionCard
        task={{
          status: "needs_review",
          result: {
            sds_design_result: {
              variant_id: 82491,
              status: "failed",
              error: "SDS render failed for selected color variants: white",
            },
            workflow_issues: [
              {
                code: "sds_variant_render_failed",
                severity: "warning",
                stage: "sds_design_sync",
                message: "SDS variant render failed",
                detail:
                  'sds POST /materials/one failed with status 400: {"ret":500,"msg":"您所属的商户总额度已使用完，请升级会员或额外购买增值服务","traceId":"0b2504be48ee144f"}',
              },
              {
                code: "sds_variant_render_failed",
                severity: "warning",
                stage: "sds_design_sync",
                message: "SDS render failed for selected color variants: white",
              },
            ],
          },
        }}
      />,
    );

    expect(
      screen.getByText("SDS render failed for selected color variants: white"),
    ).toBeInTheDocument();
    expect(screen.getByText("详细原因")).toBeInTheDocument();
    expect(
      screen.getByText(/您所属的商户总额度已使用完，请升级会员或额外购买增值服务/),
    ).toBeInTheDocument();
  });

  it("renders pod execution status even when only pod summary is available", () => {
    render(
      <TaskPodExecutionCard
        task={{
          status: "processing",
          result: {
            pod_execution: {
              provider: "sds",
              dependency_mode: "required",
              status: "processing",
            },
          },
        }}
      />,
    );

    expect(screen.getByText("POD 平台处理中")).toBeInTheDocument();
    expect(screen.getByText("POD 平台结果仍在处理中，完成后再继续正式发布。")).toBeInTheDocument();
    expect(screen.getByText("POD SDS 处理中")).toBeInTheDocument();
  });

  it("renders collapsed pod execution history in reverse chronological order", () => {
    render(
      <TaskPodExecutionCard
        task={{
          status: "failed",
          result: {
            pod_execution: {
              provider: "sds",
              dependency_mode: "required",
              status: "failed_blocking",
              history: [
                {
                  kind: "policy_decision",
                  dependency_mode: "required",
                  provider: "sds",
                  decision_source: "system_rule",
                  occurred_at: "2026-05-28T08:00:00Z",
                },
                {
                  kind: "status_transition",
                  from_status: "pending",
                  to_status: "processing",
                  occurred_at: "2026-05-28T08:10:00Z",
                },
                {
                  kind: "status_transition",
                  from_status: "processing",
                  to_status: "failed_blocking",
                  detail: "mockup sync timeout",
                  occurred_at: "2026-05-28T08:20:00Z",
                },
              ],
            },
          },
        }}
      />,
    );

    expect(screen.getByText("查看处理轨迹（3）")).toBeInTheDocument();
    const timeline = screen.getByText("查看处理轨迹（3）").closest("details");
    expect(timeline).not.toBeNull();
    if (!timeline) {
      throw new Error("timeline details missing");
    }
    const articles = within(timeline).getAllByRole("article");
    expect(articles).toHaveLength(3);
    expect(within(articles[0]).getByText("状态从 Processing 变为 Failed blocking")).toBeInTheDocument();
    expect(within(articles[0]).getByText("mockup sync timeout")).toBeInTheDocument();
    expect(within(articles[2]).getByText("策略判定为 Required")).toBeInTheDocument();
    expect(within(articles[2]).getByText("平台 SDS · system_rule")).toBeInTheDocument();
  });
});

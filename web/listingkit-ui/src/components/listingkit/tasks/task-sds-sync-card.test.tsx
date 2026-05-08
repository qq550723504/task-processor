import { render, screen } from "@testing-library/react";

import { TaskSDSSyncCard } from "@/components/listingkit/tasks/task-sds-sync-card";

describe("TaskSDSSyncCard", () => {
  it("uses workflow stage status for the SDS process detail", () => {
    render(
      <TaskSDSSyncCard
        task={{
          status: "completed",
          result: {
            sds_sync: {
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
      <TaskSDSSyncCard
        task={{
          status: "needs_review",
          result: {
            sds_sync: {
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
});

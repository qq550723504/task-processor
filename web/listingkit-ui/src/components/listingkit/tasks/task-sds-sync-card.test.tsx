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
});

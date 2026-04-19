import { render, screen } from "@testing-library/react";

import { TaskStatusPanel } from "@/components/listingkit/task-status-panel";

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

    expect(screen.getByText("Task failed")).toBeInTheDocument();
    expect(screen.getByText("product enrichment failed")).toBeInTheDocument();
    expect(screen.getByText("Failed child tasks")).toBeInTheDocument();
    expect(screen.getByText("product_enrich")).toBeInTheDocument();
    expect(screen.getByText("child-1")).toBeInTheDocument();
  });
});

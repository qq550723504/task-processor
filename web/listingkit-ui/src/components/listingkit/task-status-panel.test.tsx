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

    expect(screen.getByText("Task requires review")).toBeInTheDocument();
    expect(screen.getByText("Review reasons")).toBeInTheDocument();
    expect(
      screen.getByText("The product type is 'Unknown Product'."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("The IP risk level is 'medium' due to using scraped 1688 source images."),
    ).toBeInTheDocument();
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
      screen.getByText("The product type is 'Unknown Product'."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("The title is 'Unknown Product'."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("image pipeline uses scraped 1688 source images"),
    ).toBeInTheDocument();
  });
});

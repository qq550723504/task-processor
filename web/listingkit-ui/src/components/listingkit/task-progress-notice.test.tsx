import { render, screen } from "@testing-library/react";

import { TaskProgressNotice } from "@/components/listingkit/task-progress-notice";

describe("TaskProgressNotice", () => {
  it("renders nothing for failed tasks", () => {
    const { container } = render(<TaskProgressNotice task={{ status: "failed" }} />);

    expect(container).toBeEmptyDOMElement();
  });

  it("renders processing guidance", () => {
    render(<TaskProgressNotice task={{ status: "processing" }} />);

    expect(screen.getByText("Generation is still running")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Preview, queue, and review actions will fill in as child tasks finish. Status refreshes automatically every 5 seconds.",
      ),
    ).toBeInTheDocument();
  });

  it("renders pending guidance", () => {
    render(<TaskProgressNotice task={{ status: "pending" }} />);

    expect(screen.getByText("Waiting to start")).toBeInTheDocument();
    expect(
      screen.getByText(
        "The task has been accepted, but generation planning has not started yet. Status refreshes automatically every 5 seconds.",
      ),
    ).toBeInTheDocument();
  });
});

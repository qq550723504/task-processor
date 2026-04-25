import { render, screen } from "@testing-library/react";

import { ReviewReasonsCard } from "@/components/listingkit/review/review-reasons-card";

describe("ReviewReasonsCard", () => {
  it("renders nothing when the task does not need review", () => {
    const { container } = render(
      <ReviewReasonsCard task={{ status: "completed" }} />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("renders capped review reasons for needs-review tasks", () => {
    render(
      <ReviewReasonsCard
        limit={2}
        task={{
          status: "needs_review",
          review_reasons: ["Reason one", "Reason two", "Reason three"],
          error: "legacy fallback string",
        }}
      />,
    );

    expect(screen.getByText("Review focus")).toBeInTheDocument();
    expect(screen.getByText("Reason one")).toBeInTheDocument();
    expect(screen.getByText("Reason two")).toBeInTheDocument();
    expect(screen.queryByText("Reason three")).not.toBeInTheDocument();
    expect(screen.getByText("1 more review reason in task details")).toBeInTheDocument();
    expect(screen.queryByText("legacy fallback string")).not.toBeInTheDocument();
  });

  it("splits semicolon-separated reasons before applying the limit", () => {
    render(
      <ReviewReasonsCard
        limit={2}
        task={{
          status: "needs_review",
          error: "Reason one; Reason two；Reason three",
        }}
      />,
    );

    expect(screen.getByText("Reason one")).toBeInTheDocument();
    expect(screen.getByText("Reason two")).toBeInTheDocument();
    expect(screen.queryByText("Reason three")).not.toBeInTheDocument();
    expect(screen.getByText("1 more review reason in task details")).toBeInTheDocument();
  });
});

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TaskSourceSummary } from "@/components/listingkit/tasks/task-source-summary";

describe("TaskSourceSummary", () => {
  it("renders a product URL source summary", () => {
    render(
      <TaskSourceSummary
        draft={{
          text: "",
          imageUrls: "",
          productUrl: "https://detail.1688.com/offer/123456789.html",
          platforms: ["shein"],
        }}
      />,
    );

    expect(screen.getByText("Task source")).toBeInTheDocument();
    expect(screen.getByText("Created from product URL")).toBeInTheDocument();
    expect(
      screen.getByText(
        "This task started from a product listing URL. A 1688 link is supported in this flow.",
      ),
    ).toBeInTheDocument();
  });

  it("renders an image URL source summary", () => {
    render(
      <TaskSourceSummary
        draft={{
          text: "",
          imageUrls: "https://example.com/1.jpg\nhttps://example.com/2.jpg",
          productUrl: "",
          platforms: ["shein"],
        }}
      />,
    );

    expect(screen.getByText("Created from image URLs")).toBeInTheDocument();
    expect(
      screen.getByText("2 image URLs were submitted for this task."),
    ).toBeInTheDocument();
  });
});


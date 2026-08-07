import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { TaskPersistedSourceReference } from "@/components/listingkit/tasks/task-persisted-source-reference";

describe("TaskPersistedSourceReference", () => {
  it("shows the persisted source identity and safe external link", () => {
    render(
      <TaskPersistedSourceReference
        source={{
          type: "crawler",
          platform: "1688",
          id: "888",
          url: "https://detail.1688.com/offer/888.html",
        }}
      />,
    );

    expect(screen.getByText("任务来源")).toBeInTheDocument();
    expect(screen.getByText("来源 1688 · 888")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "查看来源" })).toHaveAttribute(
      "href",
      "https://detail.1688.com/offer/888.html",
    );
    expect(screen.getByRole("link", { name: "查看来源" })).toHaveAttribute(
      "target",
      "_blank",
    );
    expect(screen.getByRole("link", { name: "查看来源" })).toHaveAttribute(
      "rel",
      "noreferrer",
    );
  });

  it("renders nothing when the persisted reference is empty", () => {
    const { container } = render(
      <TaskPersistedSourceReference source={{}} />,
    );

    expect(container).toBeEmptyDOMElement();
  });
});

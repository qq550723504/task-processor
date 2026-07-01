import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinCreatedTasksList } from "@/components/listingkit/shein-studio/shein-created-tasks-list";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

describe("SheinCreatedTasksList", () => {
  it("shows the batch task source so reused legacy links are not confused with newly created tasks", () => {
    render(
      <SheinCreatedTasksList
        reusedTasks={[
          {
            id: "task-legacy",
            title: "Legacy task",
            designId: "design-1",
            source: "legacy_session_backfilled",
          },
        ]}
        tasks={[
          {
            id: "task-created",
            title: "Created task",
            designId: "design-2",
            source: "batch_created",
          },
        ]}
      />,
    );

    expect(screen.getByText("批次创建")).toBeInTheDocument();
    expect(screen.getByText("旧任务回填")).toBeInTheDocument();
  });
});

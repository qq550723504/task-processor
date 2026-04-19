import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TaskLauncher } from "@/components/listingkit/task-launcher";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push,
  }),
}));

describe("TaskLauncher", () => {
  beforeEach(() => {
    push.mockReset();
  });

  it("navigates to the demo workspace from the quick action", () => {
    render(<TaskLauncher />);

    fireEvent.click(screen.getByRole("button", { name: "Open Demo Workspace" }));

    expect(push).toHaveBeenCalledWith("/listing-kits/demo-task/workspace");
  });

  it("trims the entered task id before opening the queue", () => {
    render(<TaskLauncher />);

    fireEvent.change(screen.getByLabelText("Task ID"), {
      target: { value: "  task_123  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Open Queue" }));

    expect(push).toHaveBeenCalledWith("/listing-kits/task_123/queue");
  });

  it("opens the create page from the primary entrypoint", () => {
    render(<TaskLauncher />);

    fireEvent.click(screen.getByRole("button", { name: "Create New Task" }));

    expect(push).toHaveBeenCalledWith("/listing-kits/new");
  });
});

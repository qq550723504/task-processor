import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { saveTaskCreateDraft } from "@/components/listingkit/task-create-draft";
import { TaskStatusScreen } from "@/components/listingkit/task-status-screen";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push,
  }),
}));

describe("TaskStatusScreen", () => {
  beforeEach(() => {
    push.mockReset();
    window.sessionStorage.clear();
  });

  it("shows review entrypoints for completed tasks", () => {
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "completed",
        }}
      />,
    );

    expect(screen.getByText("Task completed")).toBeInTheDocument();
    expect(
      screen.getByText("Opening workspace automatically in 1.5 seconds."),
    ).toBeInTheDocument();
    expect(setTimeoutSpy).toHaveBeenCalledWith(expect.any(Function), 1500);
    const callback = setTimeoutSpy.mock.calls[0]?.[0] as (() => void) | undefined;
    callback?.();
    expect(push).toHaveBeenCalledWith("/listing-kits/task_123/workspace");
    setTimeoutSpy.mockRestore();
  });

  it("still allows manual workspace entry before auto-open completes", () => {
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "completed",
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Open workspace" }));
    expect(push).toHaveBeenCalledWith("/listing-kits/task_123/workspace");
  });

  it("allows cancelling auto-open for completed tasks", () => {
    const clearTimeoutSpy = vi.spyOn(window, "clearTimeout");
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "completed",
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Cancel auto-open" }));

    expect(screen.getByText("Auto-open paused.")).toBeInTheDocument();
    expect(clearTimeoutSpy).toHaveBeenCalled();
    expect(push).not.toHaveBeenCalled();
    clearTimeoutSpy.mockRestore();
  });

  it("keeps processing tasks on the status page", () => {
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "processing",
        }}
      />,
    );

    expect(screen.getByText("Task in progress")).toBeInTheDocument();
    expect(screen.getByText("Generation is still running")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Open workspace" }),
    ).not.toBeInTheDocument();
  });

  it("keeps queue and workspace entrypoints for failed tasks", () => {
    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "failed",
          error:
            "product enrichment failed: 数据质量不足\n\n必需改进操作：\n1. 添加至少 3 张高质量产品图片\n2. 提供至少 50 字符的产品描述\n",
        }}
      />,
    );

    expect(screen.getByText("Task failed")).toBeInTheDocument();
    expect(screen.getByText("Recommended fixes")).toBeInTheDocument();
    expect(screen.getByText("添加至少 3 张高质量产品图片")).toBeInTheDocument();
    expect(screen.getByText("提供至少 50 字符的产品描述")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Create improved task" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open workspace" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open queue" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Create improved task" }));
    expect(push).toHaveBeenCalledWith(
      "/listing-kits/new?fromTask=task_123&focus=imageUrls&issues=imageUrls,text",
    );
  });

  it("shows the task source summary when a creation draft exists", () => {
    saveTaskCreateDraft("task_123", {
      text: "",
      imageUrls: "",
      productUrl: "https://detail.1688.com/offer/123456789.html",
      platforms: ["shein"],
    });

    render(
      <TaskStatusScreen
        taskId="task_123"
        task={{
          task_id: "task_123",
          status: "processing",
        }}
      />,
    );

    expect(screen.getByText("Task source")).toBeInTheDocument();
    expect(screen.getByText("Created from product URL")).toBeInTheDocument();
  });
});

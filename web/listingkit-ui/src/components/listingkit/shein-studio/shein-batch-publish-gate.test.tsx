import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SheinBatchPublishGate } from "@/components/listingkit/shein-studio/shein-batch-publish-gate";
import { getListingKitPreview } from "@/lib/api/preview";
import { submitTask } from "@/lib/api/submit";
import { getListingKitTaskResult } from "@/lib/api/task-result";
import type { ListingKitPreview, ListingKitTaskResult } from "@/lib/types/listingkit";
import type { SheinStudioCreatedTask } from "@/lib/types/shein-studio";

vi.mock("@/lib/api/preview", () => ({
  getListingKitPreview: vi.fn(),
}));

vi.mock("@/lib/api/submit", () => ({
  submitTask: vi.fn(),
}));

vi.mock("@/lib/api/task-result", () => ({
  getListingKitTaskResult: vi.fn(),
}));

const mockedGetPreview = vi.mocked(getListingKitPreview);
const mockedGetTaskResult = vi.mocked(getListingKitTaskResult);
const mockedSubmitTask = vi.mocked(submitTask);

const tasks: SheinStudioCreatedTask[] = [
  { designId: "design-1", id: "task-1", title: "First task" },
  { designId: "design-2", id: "task-2", title: "Second task" },
  { designId: "design-3", id: "task-3", title: "Third task" },
];

function renderGate() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <SheinBatchPublishGate tasks={tasks} />
    </QueryClientProvider>,
  );
}

function completedResult(taskId: string): ListingKitTaskResult {
  return {
    task_id: taskId,
    status: "completed",
  };
}

function readyPreview(taskId: string): ListingKitPreview {
  return {
    task_id: taskId,
    status: "completed",
    shein: {
      submit_readiness: {
        ready: true,
        status: "ready",
      },
    },
  };
}

describe("SheinBatchPublishGate", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedGetTaskResult.mockImplementation((taskId) =>
      Promise.resolve(completedResult(taskId)),
    );
    mockedGetPreview.mockImplementation((taskId) =>
      Promise.resolve(readyPreview(taskId)),
    );
  });

  it("continues after one task fails and retries only failed submissions", async () => {
    const attemptsByTask = new Map<string, number>();
    mockedSubmitTask.mockImplementation((taskId) => {
      const nextAttempt = (attemptsByTask.get(taskId) ?? 0) + 1;
      attemptsByTask.set(taskId, nextAttempt);

      if (taskId === "task-2" && nextAttempt === 1) {
        return Promise.reject(new Error("SHEIN rejected task-2"));
      }

      return Promise.resolve(readyPreview(taskId));
    });

    renderGate();

    await screen.findByText("可正式发布：3");

    fireEvent.click(screen.getByRole("button", { name: "发布可用任务" }));

    await waitFor(() => {
      expect(mockedSubmitTask).toHaveBeenCalledTimes(3);
    });
    expect(mockedSubmitTask).toHaveBeenNthCalledWith(1, "task-1", {
      platform: "shein",
      action: "publish",
    });
    expect(mockedSubmitTask).toHaveBeenNthCalledWith(2, "task-2", {
      platform: "shein",
      action: "publish",
    });
    expect(mockedSubmitTask).toHaveBeenNthCalledWith(3, "task-3", {
      platform: "shein",
      action: "publish",
    });

    expect(
      await screen.findByText("已发布 2 个 SHEIN 任务，失败 1 个。"),
    ).toBeInTheDocument();
    expect(screen.getByText("Second task：SHEIN rejected task-2")).toBeInTheDocument();

    await waitFor(() => {
      for (const task of tasks) {
        expect(mockedGetTaskResult.mock.calls.filter(([id]) => id === task.id))
          .toHaveLength(2);
        expect(mockedGetPreview.mock.calls.filter(([id]) => id === task.id))
          .toHaveLength(2);
      }
    });

    fireEvent.click(screen.getByRole("button", { name: "发布可用任务" }));

    await waitFor(() => {
      expect(mockedSubmitTask).toHaveBeenCalledTimes(4);
    });
    expect(mockedSubmitTask).toHaveBeenLastCalledWith("task-2", {
      platform: "shein",
      action: "publish",
    });
    expect(
      mockedSubmitTask.mock.calls.filter(([taskId]) => taskId === "task-1"),
    ).toHaveLength(1);
    expect(
      mockedSubmitTask.mock.calls.filter(([taskId]) => taskId === "task-3"),
    ).toHaveLength(1);
    expect(
      await screen.findByText("已发布 1 个 SHEIN 任务，跳过 2 个。"),
    ).toBeInTheDocument();

    await waitFor(() => {
      expect(mockedGetTaskResult.mock.calls.filter(([id]) => id === "task-2"))
        .toHaveLength(3);
      expect(mockedGetPreview.mock.calls.filter(([id]) => id === "task-2"))
        .toHaveLength(3);
    });
  });
});

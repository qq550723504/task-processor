import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useRetryChildTask } from "@/lib/query/use-child-task-retry";

const retryChildTaskMock = vi.fn();

vi.mock("@/lib/api/child-task-retry", () => ({
  retryChildTask: (...args: unknown[]) => retryChildTaskMock(...args),
}));

describe("useRetryChildTask", () => {
  beforeEach(() => {
    retryChildTaskMock.mockReset();
    retryChildTaskMock.mockResolvedValue({ task_id: "task-1" });
  });

  it("invalidates the task result after retrying a child task", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    const { result } = renderHook(() => useRetryChildTask("task-1", "version-1"), { wrapper });

    result.current.mutate({ kind: "sds_design_sync" });

    await waitFor(() => expect(retryChildTaskMock).toHaveBeenCalled());
    await waitFor(() => expect(invalidateQueries).toHaveBeenCalled());
  });

  it("returns the queued acknowledgement from the retry endpoint", async () => {
    retryChildTaskMock.mockResolvedValue({
      task_id: "task-1",
      kind: "sds_design_sync",
      status: "queued",
    });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    const { result } = renderHook(() => useRetryChildTask("task-1", "version-1"), { wrapper });

    result.current.mutate({ kind: "sds_design_sync" });

    await waitFor(() => expect(result.current.data?.status).toBe("queued"));
  });

  it("clears the queued acknowledgement after the task result version changes", async () => {
    retryChildTaskMock.mockResolvedValue({
      task_id: "task-1",
      kind: "sds_design_sync",
      status: "queued",
    });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    const { result, rerender } = renderHook(
      ({ taskVersion }: { taskVersion: string }) =>
        useRetryChildTask("task-1", taskVersion),
      { initialProps: { taskVersion: "version-1" }, wrapper },
    );

    result.current.mutate({ kind: "sds_design_sync" });

    await waitFor(() => expect(result.current.retryQueued).toBe(true));
    rerender({ taskVersion: "version-2" });
    await waitFor(() => expect(result.current.retryQueued).toBe(false));
  });
});

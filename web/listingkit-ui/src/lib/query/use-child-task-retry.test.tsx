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

    const { result } = renderHook(() => useRetryChildTask("task-1"), { wrapper });

    result.current.mutate({ kind: "sds_design_sync" });

    await waitFor(() => expect(retryChildTaskMock).toHaveBeenCalled());
    await waitFor(() => expect(invalidateQueries).toHaveBeenCalled());
  });
});

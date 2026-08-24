import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useRetryChildTask } from "@/lib/query/use-child-task-retry";
import { listingKitKeys } from "@/lib/query/keys";

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

  it("polls the task result while a retry is queued", async () => {
    vi.useFakeTimers();
    try {
      retryChildTaskMock.mockResolvedValue({
        task_id: "task-1",
        kind: "sds_design_sync",
        status: "queued",
      });
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });
      const refetchQueries = vi
        .spyOn(queryClient, "refetchQueries")
        .mockResolvedValue(undefined);
      const wrapper = ({ children }: { children: React.ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      );

      const { result, rerender } = renderHook(
        ({ taskVersion }: { taskVersion: string }) =>
          useRetryChildTask("task-1", taskVersion),
        { initialProps: { taskVersion: "version-1" }, wrapper },
      );

      await act(async () => {
        await result.current.mutateAsync({ kind: "sds_design_sync" });
      });
      expect(result.current.retryQueued).toBe(true);

      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });
      expect(
        refetchQueries.mock.calls.some(
          ([filters]) =>
            JSON.stringify(filters) ===
            JSON.stringify({
              queryKey: ["listingkit", "task-1", "task-result"],
            }),
        ),
      ).toBe(true);

      rerender({ taskVersion: "version-2" });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5000);
      });
      expect(
        refetchQueries.mock.calls.filter(
          ([filters]) =>
            JSON.stringify(filters) ===
            JSON.stringify({
              queryKey: ["listingkit", "task-1", "task-result"],
            }),
        ),
      ).toHaveLength(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("restores queued polling from the durable retry status", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const refetchQueries = vi
      .spyOn(queryClient, "refetchQueries")
      .mockResolvedValue(undefined);
    const invalidateQueries = vi
      .spyOn(queryClient, "invalidateQueries")
      .mockResolvedValue(undefined);
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    const { result, rerender } = renderHook(
      ({ retries }: { retries: Array<{ kind: string; status: string }> }) =>
        useRetryChildTask("task-1", "version-1", retries),
      {
        initialProps: {
          retries: [{ kind: "sds_design_sync", status: "queued" }],
        },
        wrapper,
      },
    );

    expect(result.current.retryQueued).toBe(true);
    rerender({ retries: [{ kind: "sds_design_sync", status: "exhausted" }] });
    expect(result.current.retryQueued).toBe(false);
    expect(refetchQueries).not.toHaveBeenCalled();
    expect(invalidateQueries).toHaveBeenCalled();
  });

  it("invalidates workspace caches when a durable retry reaches a terminal state", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    const { rerender } = renderHook(
      ({ status }: { status: string }) =>
        useRetryChildTask("task-1", "version-1", [
          { kind: "sds_design_sync", status },
        ]),
      { initialProps: { status: "queued" }, wrapper },
    );

    rerender({ status: "exhausted" });

    await waitFor(() => expect(invalidateQueries).toHaveBeenCalled());
    const filters = invalidateQueries.mock.calls.at(-1)?.[0];
    if (!filters || typeof filters.predicate !== "function") {
      throw new Error("workspace cache invalidation predicate was not provided");
    }
    expect(filters.predicate({ queryKey: listingKitKeys.reviewSession("task-1", {}) } as never)).toBe(true);
    expect(filters.predicate({ queryKey: listingKitKeys.reviewPreview("task-1", {}) } as never)).toBe(true);
    expect(filters.predicate({ queryKey: listingKitKeys.preview("task-1") } as never)).toBe(true);
    expect(filters.predicate({ queryKey: ["listingkit", "other-task", "review-session"] } as never)).toBe(false);
  });
});


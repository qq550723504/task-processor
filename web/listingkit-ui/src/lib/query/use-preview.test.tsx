import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { getListingKitPreview } from "@/lib/api/preview";
import { useListingKitPreview } from "@/lib/query/use-preview";
import type { ListingKitPreview } from "@/lib/types/listingkit";

vi.mock("@/lib/api/preview", () => ({
  getListingKitPreview: vi.fn(),
}));

const mockedGetPreview = vi.mocked(getListingKitPreview);

function wrapper({ children }: PropsWithChildren) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

function preview(taskId: string): ListingKitPreview {
  return {
    task_id: taskId,
    status: "completed",
    shein: {},
  };
}

describe("useListingKitPreview", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedGetPreview.mockImplementation((taskId) =>
      Promise.resolve(preview(taskId)),
    );
  });

  it("refetches preview when the task result freshness key changes", async () => {
    const { rerender } = renderHook(
      ({ freshnessKey }) => useListingKitPreview("task-1", freshnessKey),
      {
        initialProps: { freshnessKey: "updated-1" },
        wrapper,
      },
    );

    await waitFor(() => expect(mockedGetPreview).toHaveBeenCalledTimes(1));

    rerender({ freshnessKey: "updated-2" });

    await waitFor(() => expect(mockedGetPreview).toHaveBeenCalledTimes(2));
    expect(mockedGetPreview).toHaveBeenNthCalledWith(1, "task-1");
    expect(mockedGetPreview).toHaveBeenNthCalledWith(2, "task-1");
  });
});

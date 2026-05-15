import { beforeEach, describe, expect, it, vi } from "vitest";

import { apiRequest } from "@/lib/api/client";
import { getGenerationQueue } from "@/lib/api/queue";

vi.mock("@/lib/api/client", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api/client")>(
    "@/lib/api/client",
  );
  return {
    ...actual,
    apiRequest: vi.fn(),
  };
});

const mockedApiRequest = vi.mocked(apiRequest);

describe("getGenerationQueue", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("normalizes numeric pagination and summary fields", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      task_id: "task-1",
      page: "1",
      page_size: "20",
      total: "2",
      summary: {
        total_items: "2",
        ready_items: "1",
        fallback_items: "0",
        missing_items: "0",
        queued_items: "0",
        running_items: "0",
        completed_items: "1",
        failed_items: "0",
        retryable_items: "0",
        previewable_items: "1",
        approved_sections: "0",
        deferred_sections: "0",
        review_pending_sections: "1",
      },
      items: [{ task_id: "asset-1", preview_capabilities: ["main"] }],
    });

    await expect(getGenerationQueue("task-1", {})).resolves.toMatchObject({
      page: 1,
      page_size: 20,
      total: 2,
      summary: {
        total_items: 2,
        completed_items: 1,
        review_pending_sections: 1,
      },
    });
  });

  it("rejects queue pages without a string task id", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      task_id: 123,
      page: 1,
      page_size: 20,
      total: 0,
    });

    await expect(getGenerationQueue("task-1", {})).rejects.toMatchObject({
      message: "ListingKit API returned an unexpected generation queue response",
      status: 502,
    });
  });
});

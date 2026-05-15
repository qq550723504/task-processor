import { beforeEach, describe, expect, it, vi } from "vitest";

import { apiRequest } from "@/lib/api/client";
import { getReviewSession } from "@/lib/api/review-session";

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

describe("getReviewSession", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("normalizes numeric review section counts", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      task_id: "task-1",
      session: {
        sections: [
          {
            section_key: "images",
            title: "Images",
            item_count: "3",
          },
        ],
      },
    });

    await expect(getReviewSession("task-1", {})).resolves.toMatchObject({
      task_id: "task-1",
      session: {
        sections: [{ section_key: "images", item_count: 3 }],
      },
    });
  });

  it("rejects review sections without an item count", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      task_id: "task-1",
      session: {
        sections: [
          {
            section_key: "images",
          },
        ],
      },
    });

    await expect(getReviewSession("task-1", {})).rejects.toMatchObject({
      message: "ListingKit API returned an unexpected review session response",
      status: 502,
    });
  });
});

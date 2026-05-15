import { beforeEach, describe, expect, it, vi } from "vitest";

import { apiRequest } from "@/lib/api/client";
import { getListingKitTasks } from "@/lib/api/task-list";

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

describe("getListingKitTasks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("normalizes numeric pagination fields from the API response", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      page: "2",
      page_size: "25",
      total: "41",
      items: [
        {
          task_id: "task-1",
          platforms: ["shein"],
        },
      ],
    });

    await expect(getListingKitTasks()).resolves.toMatchObject({
      page: 2,
      page_size: 25,
      total: 41,
      items: [{ task_id: "task-1", platforms: ["shein"] }],
    });
  });

  it("rejects task list items without a string task id", async () => {
    mockedApiRequest.mockResolvedValueOnce({
      page: 1,
      page_size: 20,
      total: 1,
      items: [{ task_id: 123 }],
    });

    await expect(getListingKitTasks()).rejects.toMatchObject({
      message: "ListingKit API returned an unexpected task list response",
      status: 502,
    });
  });
});

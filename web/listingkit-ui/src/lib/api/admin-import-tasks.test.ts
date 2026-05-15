import { afterEach, describe, expect, it, vi } from "vitest";

import {
  batchCreateListingImportTasks,
  getListingImportTasks,
  parseImportTaskPageResponse,
} from "@/lib/api/admin-import-tasks";

describe("parseImportTaskPageResponse", () => {
  it("accepts the ListingKit import task page shape", () => {
    expect(
      parseImportTaskPageResponse({
        items: [
          {
            id: 1,
            tenantId: 101,
            storeId: 11,
            platform: "Amazon",
            region: "US",
            categoryId: 22,
            productId: "B001",
            status: 0,
            retryCount: 0,
            maxRetryCount: 3,
            priority: 8,
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
    ).toMatchObject({
      total: 1,
      items: [{ id: 1, productId: "B001", platform: "Amazon" }],
    });
  });

  it("rejects invalid page payloads", () => {
    expect(() =>
      parseImportTaskPageResponse({ items: [{}], total: "1" }),
    ).toThrow("ListingKit API returned an unexpected import task page response");
  });
});

describe("admin import task API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests import task pages through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          items: [],
          total: 0,
          page: 2,
          page_size: 10,
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      getListingImportTasks({ page: 2, page_size: 10, platform: "Amazon" }),
    ).resolves.toMatchObject({ page: 2, page_size: 10 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/import-tasks?page=2&page_size=10&platform=Amazon",
    );
  });

  it("batch creates import tasks", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          createdCount: 2,
          items: [
            {
              id: 1,
              tenantId: 101,
              storeId: 11,
              platform: "Amazon",
              region: "US",
              categoryId: 22,
              productId: "B001",
              status: 0,
              retryCount: 0,
              maxRetryCount: 3,
              priority: 8,
            },
          ],
        }),
        {
          status: 201,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      batchCreateListingImportTasks({
        storeId: 11,
        categoryId: 22,
        platform: "Amazon",
        region: "US",
        priority: 8,
        productIds: ["B001", "B002"],
      }),
    ).resolves.toMatchObject({ createdCount: 2 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/import-tasks/batch",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});

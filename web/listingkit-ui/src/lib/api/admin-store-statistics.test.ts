import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getListingStoreStatistics,
  parseStoreStatisticsResponse,
} from "@/lib/api/admin-store-statistics";

describe("parseStoreStatisticsResponse", () => {
  it("accepts ListingKit store statistics rows", () => {
    expect(
      parseStoreStatisticsResponse([
        {
          id: 1,
          storeId: "SHEIN-US",
          tenantId: 101,
          name: "SHEIN US",
          platform: "SHEIN",
          dailyLimit: 10,
          dailyLimitType: "fixed",
          completedCount: 6,
          remainingCount: 2,
          holdCount: 1,
          queuedCount: 3,
          remainingQuota: 4,
          progressPercentage: 60,
          status: 0,
        },
      ]),
    ).toMatchObject([{ id: 1, name: "SHEIN US", progressPercentage: 60 }]);
  });

  it("rejects invalid statistics payloads", () => {
    expect(() => parseStoreStatisticsResponse([{ id: "1" }])).toThrow(
      "ListingKit API returned an unexpected store statistics response",
    );
  });
});

describe("admin store statistics API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests store statistics through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify([]), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      getListingStoreStatistics({ date: "2026-05-15" }),
    ).resolves.toEqual([]);

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/store-statistics?date=2026-05-15",
    );
  });
});

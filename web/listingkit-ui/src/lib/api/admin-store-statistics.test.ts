import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getListingStoreStatistics,
  getPlatformStoreStatistics,
  parseStoreStatisticsResponse,
} from "@/lib/api/admin-store-statistics";

describe("parseStoreStatisticsResponse", () => {
  it("accepts paginated store statistics with a summary", () => {
    expect(
      parseStoreStatisticsResponse({
        items: [
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
        ],
        total: 1,
        page: 2,
        page_size: 20,
        summary: {
          completed_count: 6,
          daily_limit: 10,
          remaining_count: 2,
          queued_count: 3,
          hold_count: 1,
        },
      }),
    ).toMatchObject({
      total: 1,
      page: 2,
      page_size: 20,
      summary: {
        completed_count: 6,
        daily_limit: 10,
        remaining_count: 2,
        queued_count: 3,
        hold_count: 1,
      },
      items: [{ id: 1, name: "SHEIN US", progressPercentage: 60 }],
    });
  });

  it("rejects legacy bare array payloads", () => {
    expect(() =>
      parseStoreStatisticsResponse([
        {
          id: 1,
          tenantId: 101,
          name: "SHEIN US",
          dailyLimit: 10,
          completedCount: 6,
          remainingCount: 2,
          holdCount: 1,
          queuedCount: 3,
          remainingQuota: 4,
          progressPercentage: 60,
          status: 0,
        },
      ]),
    ).toThrow(
      "ListingKit API returned an unexpected store statistics response",
    );
  });
});

describe("admin store statistics API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests paginated store statistics through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          items: [],
          total: 0,
          page: 3,
          page_size: 50,
          summary: {
            completed_count: 0,
            daily_limit: 0,
            remaining_count: 0,
            queued_count: 0,
            hold_count: 0,
          },
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      getListingStoreStatistics({ date: "2026-05-15", page: 3, page_size: 50 }),
    ).resolves.toMatchObject({
      page: 3,
      page_size: 50,
      summary: {
        completed_count: 0,
        daily_limit: 0,
      },
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/store-statistics?date=2026-05-15&page=3&page_size=50",
    );
  });

  it("requests platform store statistics through the platform API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          items: [],
          total: 0,
          page: 1,
          page_size: 20,
          summary: {
            completed_count: 0,
            daily_limit: 0,
            remaining_count: 0,
            queued_count: 0,
            hold_count: 0,
          },
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await getPlatformStoreStatistics({ date: "2026-05-15" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/platform/store-statistics?date=2026-05-15&page=1&page_size=20",
    );
  });
});

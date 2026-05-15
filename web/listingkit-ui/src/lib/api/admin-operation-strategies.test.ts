import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createListingOperationStrategy,
  getListingOperationStrategies,
  parseOperationStrategyPageResponse,
} from "@/lib/api/admin-operation-strategies";

describe("parseOperationStrategyPageResponse", () => {
  it("accepts the ListingKit operation strategy page shape", () => {
    expect(
      parseOperationStrategyPageResponse({
        items: [
          {
            id: 1,
            tenantId: 101,
            storeId: 11,
            name: "SHEIN stock guard",
            platform: "SHEIN",
            status: 0,
            stockChangeThreshold: 10,
            stockChangeAction: "按比例更新",
            outOfStockAction: "自动下架",
            minProfitRate: 0.2,
            lowProfitAction: "暂停上架",
            priceUpdateMultiplier: 1.1,
            stockUpdateRatio: 0.8,
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
    ).toMatchObject({
      total: 1,
      items: [{ id: 1, name: "SHEIN stock guard", platform: "SHEIN" }],
    });
  });

  it("rejects invalid page payloads", () => {
    expect(() =>
      parseOperationStrategyPageResponse({ items: [{}], total: "1" }),
    ).toThrow(
      "ListingKit API returned an unexpected operation strategy page response",
    );
  });
});

describe("admin operation strategy API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests operation strategy pages through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({ items: [], total: 0, page: 2, page_size: 10 }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      getListingOperationStrategies({
        page: 2,
        page_size: 10,
        platform: "SHEIN",
      }),
    ).resolves.toMatchObject({ page: 2, page_size: 10 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/operation-strategies?page=2&page_size=10&platform=SHEIN",
    );
  });

  it("creates operation strategies", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 1,
          tenantId: 101,
          storeId: 11,
          name: "SHEIN stock guard",
          platform: "SHEIN",
          status: 0,
        }),
        { status: 201, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      createListingOperationStrategy({
        storeId: 11,
        name: "SHEIN stock guard",
        platform: "SHEIN",
        status: 0,
      }),
    ).resolves.toMatchObject({ id: 1, platform: "SHEIN" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/operation-strategies",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});

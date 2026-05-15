import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createListingProductData,
  getListingProductData,
  parseProductDataPageResponse,
} from "@/lib/api/admin-product-data";

describe("parseProductDataPageResponse", () => {
  it("accepts the ListingKit product data page shape", () => {
    expect(
      parseProductDataPageResponse({
        items: [
          {
            id: 1,
            tenantId: 101,
            storeId: 11,
            platform: "SHEIN",
            region: "US",
            productId: "B001",
            title: "Cotton shirt",
            originalPrice: 19.99,
            specialPrice: 15.99,
            stock: "12",
            status: 1,
            platformProductId: "SPU-001",
            shelfStatus: 2,
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
    ).toMatchObject({
      total: 1,
      items: [{ id: 1, productId: "B001", platformProductId: "SPU-001" }],
    });
  });

  it("rejects invalid page payloads", () => {
    expect(() =>
      parseProductDataPageResponse({ items: [{}], total: "1" }),
    ).toThrow(
      "ListingKit API returned an unexpected product data page response",
    );
  });
});

describe("admin product data API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests product data pages through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({ items: [], total: 0, page: 2, page_size: 10 }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      getListingProductData({
        page: 2,
        page_size: 10,
        platform: "SHEIN",
        productId: "B001",
      }),
    ).resolves.toMatchObject({ page: 2, page_size: 10 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/product-data?page=2&page_size=10&platform=SHEIN&productId=B001",
    );
  });

  it("creates product data rows", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 1,
          tenantId: 101,
          productId: "B001",
          platform: "SHEIN",
          status: 1,
        }),
        { status: 201, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      createListingProductData({
        productId: "B001",
        platform: "SHEIN",
        status: 1,
      }),
    ).resolves.toMatchObject({ id: 1, productId: "B001" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/product-data",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});

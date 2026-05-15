import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createListingProductImportMapping,
  getListingProductImportMappings,
  parseProductImportMappingPageResponse,
} from "@/lib/api/admin-product-import-mappings";

describe("parseProductImportMappingPageResponse", () => {
  it("accepts the ListingKit product import mapping page shape", () => {
    expect(
      parseProductImportMappingPageResponse({
        items: [
          {
            id: 1,
            tenantId: 101,
            importTaskId: 1001,
            storeId: 11,
            platform: "SHEIN",
            region: "US",
            productId: "B001",
            sku: "SKU-001",
            salePriceMultiplier: 1.8,
            discountPriceMultiplier: 1.2,
            status: 1,
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
    ).toMatchObject({
      total: 1,
      items: [{ id: 1, productId: "B001", sku: "SKU-001" }],
    });
  });

  it("rejects invalid page payloads", () => {
    expect(() =>
      parseProductImportMappingPageResponse({ items: [{}], total: "1" }),
    ).toThrow(
      "ListingKit API returned an unexpected product import mapping page response",
    );
  });
});

describe("admin product import mapping API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests product import mapping pages through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({ items: [], total: 0, page: 2, page_size: 10 }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      getListingProductImportMappings({
        page: 2,
        page_size: 10,
        platform: "SHEIN",
        productId: "B001",
      }),
    ).resolves.toMatchObject({ page: 2, page_size: 10 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/product-import-mappings?page=2&page_size=10&platform=SHEIN&productId=B001",
    );
  });

  it("creates product import mappings", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 1,
          tenantId: 101,
          importTaskId: 1001,
          storeId: 11,
          platform: "SHEIN",
          region: "US",
          productId: "B001",
          salePriceMultiplier: 1,
          discountPriceMultiplier: 1,
          status: 0,
        }),
        { status: 201, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      createListingProductImportMapping({
        importTaskId: 1001,
        storeId: 11,
        platform: "SHEIN",
        region: "US",
        productId: "B001",
      }),
    ).resolves.toMatchObject({ id: 1, productId: "B001" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/product-import-mappings",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});

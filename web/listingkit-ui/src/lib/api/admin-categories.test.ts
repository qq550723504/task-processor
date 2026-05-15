import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createListingCategory,
  getListingCategories,
  parseCategoryListResponse,
} from "@/lib/api/admin-categories";

describe("parseCategoryListResponse", () => {
  it("accepts the ListingKit category list shape", () => {
    expect(
      parseCategoryListResponse([
        {
          id: 1,
          tenantId: 101,
          name: "Apparel",
          code: "APPAREL",
          parentId: 0,
          level: 1,
          sort: 10,
          status: 1,
        },
      ]),
    ).toMatchObject([{ id: 1, name: "Apparel", code: "APPAREL" }]);
  });

  it("rejects invalid list payloads", () => {
    expect(() => parseCategoryListResponse([{ id: "1" }])).toThrow(
      "ListingKit API returned an unexpected category list response",
    );
  });
});

describe("admin category API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests category lists through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify([]), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      getListingCategories({ name: "Apparel", status: "1" }),
    ).resolves.toEqual([]);

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/categories?name=Apparel&status=1",
    );
  });

  it("creates categories", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 1,
          tenantId: 101,
          name: "Apparel",
          code: "APPAREL",
          parentId: 0,
          level: 1,
          sort: 10,
          status: 1,
        }),
        { status: 201, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      createListingCategory({
        name: "Apparel",
        code: "APPAREL",
        parentId: 0,
        level: 1,
        sort: 10,
        status: 1,
      }),
    ).resolves.toMatchObject({ id: 1, code: "APPAREL" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/categories",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});

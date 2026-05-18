import { describe, expect, it, vi, afterEach } from "vitest";

import {
  createListingStore,
  extendListingStoreValidity,
  getDeletedListingStores,
  getListingStores,
  permanentlyDeleteListingStore,
  parseStorePageResponse,
  restoreListingStore,
  updateListingStoreStatus,
} from "@/lib/api/admin-stores";

describe("parseStorePageResponse", () => {
  it("accepts the ListingKit store page shape", () => {
    expect(
      parseStorePageResponse({
        items: [
          {
            id: 1,
            tenantId: 101,
            name: "SHEIN US",
            username: "shein-us",
            platform: "SHEIN",
            shopType: "semi",
            region: "US",
            status: 0,
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
    ).toMatchObject({
      total: 1,
      items: [{ id: 1, name: "SHEIN US", platform: "SHEIN" }],
    });
  });

  it("rejects invalid page payloads", () => {
    expect(() => parseStorePageResponse({ items: [{}], total: "1" })).toThrow(
      "ListingKit API returned an unexpected store page response",
    );
  });
});

describe("admin store API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests store pages through the ListingKit API proxy", async () => {
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
      getListingStores({ page: 2, page_size: 10, platform: "SHEIN" }),
    ).resolves.toMatchObject({ page: 2, page_size: 10 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/stores?page=2&page_size=10&platform=SHEIN",
    );
  });

  it("creates stores with the admin store endpoint", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 1,
          tenantId: 101,
          name: "SHEIN US",
          username: "shein-us",
          platform: "SHEIN",
          shopType: "semi",
          region: "US",
          status: 0,
        }),
        {
          status: 201,
          headers: { "content-type": "application/json" },
        },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      createListingStore({
        name: "SHEIN US",
        username: "shein-us",
        password: "secret",
        platform: "SHEIN",
        shopType: "semi",
        region: "US",
      }),
    ).resolves.toMatchObject({ id: 1, name: "SHEIN US" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/stores",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });

  it("requests deleted stores through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(JSON.stringify([]), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(getDeletedListingStores()).resolves.toEqual([]);

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/stores/deleted",
    );
  });

  it("restores and permanently deletes stores", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            id: 1,
            name: "SHEIN US",
            username: "shein-us",
            shopType: "semi",
            platform: "SHEIN",
            status: 0,
          }),
          { status: 200, headers: { "content-type": "application/json" } },
        ),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ deleted: true }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    await expect(restoreListingStore(1)).resolves.toMatchObject({ id: 1 });
    await expect(permanentlyDeleteListingStore(1)).resolves.toBeUndefined();

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/stores/1/restore",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("PUT");
    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      "/api/listing-kits/admin/stores/1/permanent",
    );
    expect(fetchMock.mock.calls[1]?.[1]?.method).toBe("DELETE");
  });

  it("extends store validity", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 1,
          name: "SHEIN US",
          username: "shein-us",
          shopType: "semi",
          platform: "SHEIN",
          validUntil: "2026-06-14T00:00:00Z",
          status: 0,
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(extendListingStoreValidity(1, 30)).resolves.toMatchObject({
      validUntil: "2026-06-14T00:00:00Z",
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/stores/1/extend-validity?days=30",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("PUT");
  });

  it("updates store status through the admin store status endpoint", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 1,
          name: "SHEIN US",
          username: "shein-us",
          shopType: "semi",
          platform: "SHEIN",
          status: 1,
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(updateListingStoreStatus(1, 1, "平台手动禁用店铺")).resolves.toMatchObject({
      id: 1,
      status: 1,
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/stores/1/status",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("PATCH");
  });
});

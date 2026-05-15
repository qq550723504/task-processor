import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createListingFilterRule,
  getListingFilterRules,
  parseFilterRulePageResponse,
} from "@/lib/api/admin-filter-rules";

describe("parseFilterRulePageResponse", () => {
  it("accepts the ListingKit filter rule page shape", () => {
    expect(
      parseFilterRulePageResponse({
        items: [
          {
            id: 1,
            tenantId: 101,
            name: "Amazon basic",
            ruleCode: "FR-AMZ",
            storeId: 11,
            priceMin: 1,
            priceMax: 99,
            stockMin: 10,
            ratingMin: 4.2,
            reviewCountMin: 20,
            status: 1,
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
    ).toMatchObject({
      total: 1,
      items: [{ id: 1, name: "Amazon basic", ruleCode: "FR-AMZ" }],
    });
  });

  it("rejects invalid page payloads", () => {
    expect(() =>
      parseFilterRulePageResponse({ items: [{}], total: "1" }),
    ).toThrow("ListingKit API returned an unexpected filter rule page response");
  });
});

describe("admin filter rule API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests filter rule pages through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({ items: [], total: 0, page: 2, page_size: 10 }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      getListingFilterRules({ page: 2, page_size: 10, ruleCode: "FR" }),
    ).resolves.toMatchObject({ page: 2, page_size: 10 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/filter-rules?page=2&page_size=10&ruleCode=FR",
    );
  });

  it("creates filter rules", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 1,
          tenantId: 101,
          name: "Amazon basic",
          ruleCode: "FR-AMZ",
          priceMin: 1,
          priceMax: 99,
          stockMin: 10,
          ratingMin: 4.2,
          reviewCountMin: 20,
          status: 1,
        }),
        { status: 201, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      createListingFilterRule({
        name: "Amazon basic",
        ruleCode: "FR-AMZ",
        priceMin: 1,
        priceMax: 99,
      }),
    ).resolves.toMatchObject({ id: 1, ruleCode: "FR-AMZ" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/filter-rules",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});

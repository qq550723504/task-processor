import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createListingProfitRule,
  getListingProfitRules,
  parseProfitRulePageResponse,
} from "@/lib/api/admin-profit-rules";

describe("parseProfitRulePageResponse", () => {
  it("accepts the ListingKit profit rule page shape", () => {
    expect(
      parseProfitRulePageResponse({
        items: [
          {
            id: 1,
            tenantId: 101,
            name: "SHEIN margin",
            ruleCode: "PR-SHEIN",
            storeId: 11,
            salePriceMultiplier: 3,
            discountPriceMultiplier: 2.5,
            status: 1,
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
    ).toMatchObject({
      total: 1,
      items: [{ id: 1, name: "SHEIN margin", ruleCode: "PR-SHEIN" }],
    });
  });

  it("rejects invalid page payloads", () => {
    expect(() =>
      parseProfitRulePageResponse({ items: [{}], total: "1" }),
    ).toThrow("ListingKit API returned an unexpected profit rule page response");
  });
});

describe("admin profit rule API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests profit rule pages through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({ items: [], total: 0, page: 2, page_size: 10 }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      getListingProfitRules({ page: 2, page_size: 10, ruleCode: "PR" }),
    ).resolves.toMatchObject({ page: 2, page_size: 10 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/profit-rules?page=2&page_size=10&ruleCode=PR",
    );
  });

  it("creates profit rules", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 1,
          tenantId: 101,
          name: "SHEIN margin",
          ruleCode: "PR-SHEIN",
          salePriceMultiplier: 3,
          discountPriceMultiplier: 2.5,
          status: 1,
        }),
        { status: 201, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      createListingProfitRule({
        name: "SHEIN margin",
        ruleCode: "PR-SHEIN",
        salePriceMultiplier: 3,
        discountPriceMultiplier: 2.5,
      }),
    ).resolves.toMatchObject({ id: 1, ruleCode: "PR-SHEIN" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/profit-rules",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});

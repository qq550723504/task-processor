import { afterEach, describe, expect, it, vi } from "vitest";

import {
  createListingPricingRule,
  getListingPricingRules,
  parsePricingRulePageResponse,
} from "@/lib/api/admin-pricing-rules";

describe("parsePricingRulePageResponse", () => {
  it("accepts the ListingKit pricing rule page shape", () => {
    expect(
      parsePricingRulePageResponse({
        items: [
          {
            id: 1,
            tenantId: 101,
            name: "SHEIN auto price",
            ruleCode: "AR-SHEIN",
            storeId: 11,
            priceMin: 1,
            priceMax: 99,
            ruleType: "multiple_fixed",
            ruleValue: 1.8,
            fixedValue: 2.5,
            status: 1,
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
    ).toMatchObject({
      total: 1,
      items: [{ id: 1, name: "SHEIN auto price", ruleCode: "AR-SHEIN" }],
    });
  });

  it("rejects invalid page payloads", () => {
    expect(() =>
      parsePricingRulePageResponse({ items: [{}], total: "1" }),
    ).toThrow("ListingKit API returned an unexpected pricing rule page response");
  });
});

describe("admin pricing rule API", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("requests pricing rule pages through the ListingKit API proxy", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({ items: [], total: 0, page: 2, page_size: 10 }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      getListingPricingRules({ page: 2, page_size: 10, ruleCode: "AR" }),
    ).resolves.toMatchObject({ page: 2, page_size: 10 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/pricing-rules?page=2&page_size=10&ruleCode=AR",
    );
  });

  it("creates pricing rules", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          id: 1,
          tenantId: 101,
          name: "SHEIN auto price",
          ruleCode: "AR-SHEIN",
          priceMin: 1,
          priceMax: 99,
          ruleType: "multiple_fixed",
          ruleValue: 1.8,
          fixedValue: 2.5,
          status: 1,
        }),
        { status: 201, headers: { "content-type": "application/json" } },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      createListingPricingRule({
        name: "SHEIN auto price",
        ruleCode: "AR-SHEIN",
        ruleType: "multiple_fixed",
        ruleValue: 1.8,
        fixedValue: 2.5,
      }),
    ).resolves.toMatchObject({ id: 1, ruleCode: "AR-SHEIN" });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/api/listing-kits/admin/pricing-rules",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});

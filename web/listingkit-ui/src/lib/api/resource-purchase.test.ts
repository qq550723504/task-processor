import { afterEach, expect, it, vi } from "vitest";
import {
  createResourceQuote,
  createResourceOrder,
  getResourceOffers,
  parseResourceQuote,
} from "./resource-purchase";
const quote = {
  quote_id: "quote-1",
  organization_id: "org-B",
  offer_id: "offer-1",
  product_kind: "AI_POINT",
  resource_type: "ai_point",
  resource_quantity: "10",
  currency: "CNY",
  total_minor: "70",
  pricing_version: "synthetic-v1",
  expires_at: "2026-09-29T01:00:00Z",
  fingerprint: "frozen",
  created_at: "2026-09-29T00:00:00Z",
  amount_minor: "73",
  unit_price_minor: "7",
  remainder_minor: "3",
};
afterEach(() => vi.unstubAllGlobals());
it("checks frozen budget arithmetic with exact int64 quantities", () => {
  expect(parseResourceQuote(quote)).toEqual(quote);
  for (const changed of [
    { ...quote, total_minor: "71" },
    { ...quote, remainder_minor: "4" },
    { ...quote, amount_minor: undefined },
    { ...quote, resource_type: "data_row" },
    { ...quote, unit_price_minor: "invalid" },
  ]) {
    expect(() => parseResourceQuote(changed)).not.toThrow();
    expect(parseResourceQuote(changed)).toBeNull();
  }
});
it("sends exactly one quantity or budget and binds the original order to its quote", async () => {
  const fetcher = vi
    .fn()
    .mockResolvedValue(Response.json(quote, { status: 201 }));
  vi.stubGlobal("fetch", fetcher);
  await expect(
    createResourceQuote("user-B", "org-B", "offer-1", { amountMinor: "73" }),
  ).resolves.toEqual(quote);
  expect(JSON.parse(fetcher.mock.calls[0][1].body)).toEqual({
    offer_id: "offer-1",
    amount_minor: "73",
  });
  await expect(
    createResourceQuote("user-B", "org-B", "offer-1", {
      amountMinor: "73",
      quantity: "2",
    } as never),
  ).rejects.toMatchObject({ code: "INVALID_REQUEST" });
  expect(fetcher).toHaveBeenCalledTimes(1);
  const order = {
    order_id: "order-1",
    organization_id: "org-B",
    kind: "RESOURCE_PURCHASE",
    description: "Synthetic",
    quote_id: "other-quote",
    currency: "CNY",
    total_minor: "70",
    status: "PENDING",
    items: [],
    product_kind: "AI_POINT",
    created_at: quote.created_at,
    updated_at: quote.created_at,
  };
  fetcher.mockResolvedValue(Response.json(order, { status: 201 }));
  await expect(
    createResourceOrder("user-B", "org-B", "quote-1", "original-key"),
  ).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
});
it("exposes unconfigured offers as an empty catalog and rejects cross-tenant results", async () => {
  const fetcher = vi
    .fn()
    .mockResolvedValue(Response.json({ organization_id: "org-B", items: [] }));
  vi.stubGlobal("fetch", fetcher);
  expect((await getResourceOffers("user-B", "org-B")).items).toEqual([]);
  fetcher.mockResolvedValue(
    Response.json({ organization_id: "org-A", items: [] }),
  );
  await expect(getResourceOffers("user-B", "org-B")).rejects.toMatchObject({
    code: "INVALID_UPSTREAM_RESPONSE",
  });
});

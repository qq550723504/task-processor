import { afterEach, expect, it, vi } from "vitest";
import { createSubscriptionOrder, createSubscriptionQuote, getSubscriptionOffers, getSubscriptionOrder, SubscriptionPurchaseError } from "./subscription-purchase";

const observedAt = "2026-09-25T00:00:00Z";
const offer = { offer_id: "offer-1", plan_code: "专业版", plan_name: "专业版", term_months: "1", settlement_mode: "WALLET", currency: "CNY", total_minor: "9007199254740993", pricing_version: "v1", availability: "available" };
const quote = { quote_id: "quote-1", organization_id: "org-A", offer_id: "offer-1", product_kind: "SUBSCRIPTION_PLAN", plan_code: "专业版", plan_fingerprint: "a".repeat(64), term_months: "1", settlement_mode: "WALLET", currency: "CNY", total_minor: "9007199254740993", pricing_version: "v1", expires_at: "2026-09-25T01:00:00Z", fingerprint: "b".repeat(64), created_at: observedAt };
const order = { order_id: "order-1", organization_id: "org-A", kind: "SUBSCRIPTION_PURCHASE", description: "专业版", quote_id: "quote-1", currency: "CNY", total_minor: "9007199254740993", status: "FULFILLED", items: [], product_kind: "SUBSCRIPTION_PLAN", plan_code: "专业版", plan_fingerprint: "a".repeat(64), term_months: "1", settlement_mode: "WALLET", activation_proof: { operation_id: "subscription-activate:order-1", request_fingerprint: "c".repeat(64), outcome: "ACTIVATED", subscription_id: "1", starts_at: observedAt, expires_at: "2026-10-25T00:00:00Z", entitlement_set_fingerprint: "d".repeat(64), decided_at: observedAt }, created_at: observedAt, updated_at: observedAt };

afterEach(() => vi.unstubAllGlobals());

it("reads only current-organization server offers and preserves exact minor-unit strings", async () => {
  const fetcher = vi.fn().mockResolvedValue(Response.json({ organization_id: "org-A", items: [offer] }));
  vi.stubGlobal("fetch", fetcher);
  await expect(getSubscriptionOffers("actor", "org-A")).resolves.toMatchObject({ items: [{ total_minor: "9007199254740993" }] });
  expect(fetcher.mock.calls[0][0]).toBe("/api/workbench/commercial/subscription-offers");
  expect(new Headers(fetcher.mock.calls[0][1].headers).get("X-Expected-Organization-ID")).toBe("org-A");
  fetcher.mockResolvedValue(Response.json({ organization_id: "org-B", items: [offer] }));
  await expect(getSubscriptionOffers("actor", "org-A")).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
});

it("submits only offer or quote identity and preserves the original order key", async () => {
  const fetcher = vi.fn().mockResolvedValueOnce(Response.json(quote, { status: 201 })).mockResolvedValueOnce(Response.json(order, { status: 201 })).mockResolvedValueOnce(Response.json(order));
  vi.stubGlobal("fetch", fetcher);
  await expect(createSubscriptionQuote("actor", "org-A", "offer-1")).resolves.toMatchObject({ quote_id: "quote-1", total_minor: offer.total_minor });
  await expect(createSubscriptionOrder("actor", "org-A", "quote-1", "order-key-1")).resolves.toMatchObject({ order_id: "order-1" });
  await expect(getSubscriptionOrder("actor", "org-A", "order-1")).resolves.toMatchObject({ activation_proof: { outcome: "ACTIVATED" } });
  expect(JSON.parse(fetcher.mock.calls[0][1].body)).toEqual({ offer_id: "offer-1" });
  expect(JSON.parse(fetcher.mock.calls[1][1].body)).toEqual({ quote_id: "quote-1" });
  expect(new Headers(fetcher.mock.calls[1][1].headers).get("Idempotency-Key")).toBe("order-key-1");
  expect(fetcher.mock.calls[2][0]).toBe("/api/workbench/commercial/subscription-orders/order-1");
});

it("does not turn timeout or reconciliation into a terminal purchase failure", async () => {
  const fetcher = vi.fn().mockResolvedValueOnce(Response.json({ code: "RECONCILIATION_REQUIRED", message: "unknown", requestId: "req-1", fieldErrors: [] }, { status: 409 })).mockResolvedValueOnce(Response.json({ code: "DEADLINE_EXCEEDED", message: "timeout", requestId: "req-2", fieldErrors: [] }, { status: 504 }));
  vi.stubGlobal("fetch", fetcher);
  await expect(createSubscriptionOrder("actor", "org-A", "quote-1", "order-key-1")).rejects.toMatchObject({ code: "RECONCILIATION_REQUIRED" });
  await expect(createSubscriptionOrder("actor", "org-A", "quote-1", "order-key-1")).rejects.toMatchObject({ code: "DEADLINE_EXCEEDED" });
  expect(fetcher.mock.calls.map(([, init]) => new Headers(init.headers).get("Idempotency-Key"))).toEqual(["order-key-1", "order-key-1"]);
});

it("rejects malformed offer, quote and order facts rather than inventing a safe-looking purchase", async () => {
  const fetcher = vi.fn().mockResolvedValueOnce(Response.json({ organization_id: "org-A", items: [{ ...offer, total_minor: "1.00" }] })).mockResolvedValueOnce(Response.json({ ...quote, organization_id: "org-B" }, { status: 201 })).mockResolvedValueOnce(Response.json({ ...order, status: "FULFILLED", activation_proof: undefined }, { status: 201 }));
  vi.stubGlobal("fetch", fetcher);
  await expect(getSubscriptionOffers("actor", "org-A")).rejects.toBeInstanceOf(SubscriptionPurchaseError);
  await expect(createSubscriptionQuote("actor", "org-A", "offer-1")).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
  await expect(createSubscriptionOrder("actor", "org-A", "quote-1", "order-key-1")).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
});

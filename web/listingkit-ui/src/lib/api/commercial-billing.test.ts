import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CommercialBillingReadError,
  getCommercialOrder,
  getCommercialOrderSummary,
  getCommercialOrders,
  getCommercialWallet,
  getCommercialWalletEntries,
  parseCommercialOrderPage,
  parseCommercialOrderSummary,
  parseCommercialWallet,
  parseCommercialWalletEntries,
} from "./commercial-billing";

const observedAt = "2026-09-23T10:00:00Z";
const wallet = { organization_id: "org-A", currency: "CNY", available_minor: "12000", reserved_minor: "3000", debt_minor: "0", lifetime_topup_minor: "30000", lifetime_spend_minor: "18000", version: "4", observed_at: observedAt };
const walletEntries = { organization_id: "org-A", items: [{ entry_id: "entry-1", currency: "CNY", entry_type: "PURCHASE_COMMIT", available_delta_minor: "0", reserved_delta_minor: "-3000", debt_delta_minor: "0", available_after_minor: "12000", reserved_after_minor: "0", debt_after_minor: "0", order_id: "order-1", source_id: "commercial-order:order-1", occurred_at: observedAt }], next_cursor: "" };
const order = { order_id: "order-1", organization_id: "org-A", kind: "RESOURCE_PURCHASE", description: "AI 点数 × 3", quote_id: "quote-1", currency: "CNY", total_minor: "3000", status: "FULFILLED", items: [{ order_item_id: "item-1", product_kind: "AI_POINT", resource_type: "ai_point", resource_quantity: "3", amount_minor: "3000" }], created_at: observedAt, updated_at: observedAt };
const subscriptionOrder = { order_id: "order-subscription-1", organization_id: "org-A", kind: "SUBSCRIPTION_PURCHASE", description: "专业版 · 12 个月", quote_id: "quote-subscription-1", currency: "CNY", total_minor: "600", status: "FULFILLED", items: [], product_kind: "SUBSCRIPTION_PLAN", plan_code: "professional", plan_fingerprint: "a".repeat(64), term_months: "12", settlement_mode: "WALLET", activation_proof: { operation_id: "subscription-activate:order-subscription-1", request_fingerprint: "b".repeat(64), outcome: "ACTIVATED", subscription_id: "42", starts_at: observedAt, expires_at: "2027-09-23T10:00:00Z", entitlement_set_fingerprint: "c".repeat(64), decided_at: observedAt }, created_at: observedAt, updated_at: observedAt };
const orders = { organization_id: "org-A", items: [order], next_cursor: "" };
const summary = { organization_id: "org-A", currency: "CNY", from: "2026-08-24T10:00:00Z", until: observedAt, spend_minor: "3000", store_renewal_spend_minor: "0", ai_point_spend_minor: "3000", data_row_spend_minor: "0", other_spend_minor: "0", observed_at: observedAt };

afterEach(() => vi.unstubAllGlobals());

describe("commercial billing read contracts", () => {
  it("accepts owner snapshots and exact money strings without Number conversion", () => {
    expect(parseCommercialWallet({ ...wallet, available_minor: "9007199254740993" })?.available_minor).toBe("9007199254740993");
    expect(parseCommercialWallet({ ...wallet, organization_id: "org-B" })?.organization_id).toBe("org-B");
    expect(parseCommercialWallet({ ...wallet, available_minor: "01" })).toBeNull();
    expect(parseCommercialWalletEntries(walletEntries)?.items[0].order_id).toBe("order-1");
    expect(parseCommercialWalletEntries({ ...walletEntries, extra: true })).toBeNull();
    expect(parseCommercialOrderPage(orders)?.items[0].status).toBe("FULFILLED");
    expect(parseCommercialOrderPage({ ...orders, items: [subscriptionOrder] })?.items[0]).toMatchObject({ kind: "SUBSCRIPTION_PURCHASE", plan_code: "professional", settlement_mode: "WALLET" });
    expect(parseCommercialOrderPage({ ...orders, items: [{ ...order, total_minor: "-1" }] })).toBeNull();
    expect(parseCommercialOrderPage({ ...orders, items: [{ ...order, organization_id: "org-B" }] })).toBeNull();
    expect(parseCommercialOrderSummary(summary)?.spend_minor).toBe("3000");
    expect(parseCommercialOrderSummary({ ...summary, currency: "USD" })).toBeNull();
  });

  it("binds every read to both the expected identity and organization", async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(Response.json(wallet)).mockResolvedValueOnce(Response.json(orders)).mockResolvedValueOnce(Response.json(summary)).mockResolvedValueOnce(Response.json(walletEntries)).mockResolvedValueOnce(Response.json({ ...wallet, organization_id: "org-B" })).mockResolvedValueOnce(Response.json({ ...order, order_id: "order-other" }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(getCommercialWallet("user-A", "org-A")).resolves.toMatchObject({ organization_id: "org-A" });
    await getCommercialOrders("user-A", "org-A", { query: "renewal", kind: "RESOURCE_PURCHASE", status: "FULFILLED", from: "2026-09-01T00:00:00Z", until: observedAt });
    await getCommercialOrderSummary("user-A", "org-A");
    await getCommercialWalletEntries("user-A", "org-A");
    expect(fetchMock.mock.calls.map(([url]) => String(url))).toEqual([
      "/api/workbench/commercial/wallet",
      expect.stringContaining("/api/workbench/commercial/orders?"),
      "/api/workbench/commercial/orders/summary",
      expect.stringContaining("/api/workbench/commercial/wallet/entries?limit=50"),
    ]);
    expect(fetchMock.mock.calls.every(([, init]) => (init as RequestInit).headers && new Headers((init as RequestInit).headers).get("X-Expected-User-ID") === "user-A" && new Headers((init as RequestInit).headers).get("X-Expected-Organization-ID") === "org-A")).toBe(true);
    const detailFailure = await getCommercialOrder("user-A", "org-A", "order-1").catch(error => error);
    expect(detailFailure).toMatchObject({ status: 502, code: "INVALID_UPSTREAM_RESPONSE" });
    const failure = await getCommercialWallet("user-A", "org-A").catch(error => error);
    expect(failure).toBeInstanceOf(CommercialBillingReadError);
    expect(failure).toMatchObject({ status: 502, code: "INVALID_UPSTREAM_RESPONSE" });
  });
});

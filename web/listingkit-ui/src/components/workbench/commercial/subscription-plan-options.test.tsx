import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { commercialOverviewFixture } from "@/test/fixtures/commercial-overview";
import type { SubscriptionOffer } from "@/lib/api/subscription-purchase";
import { SubscriptionPlanOptions } from "./subscription-plan-options";

const api = vi.hoisted(() => ({ quote: vi.fn(), order: vi.fn(), readOrder: vi.fn(), readOverview: vi.fn(), wallet: vi.fn() }));
vi.mock("@/lib/api/subscription-purchase", async original => ({ ...await original<typeof import("@/lib/api/subscription-purchase")>(), createSubscriptionQuote: api.quote, createSubscriptionOrder: api.order, getSubscriptionOrder: api.readOrder }));
vi.mock("@/lib/api/commercial", async original => ({ ...await original<typeof import("@/lib/api/commercial")>(), getCommercialOverview: api.readOverview }));
vi.mock("@/lib/api/commercial-billing", async original => ({ ...await original<typeof import("@/lib/api/commercial-billing")>(), getCommercialWallet: api.wallet }));

const offer = { offer_id: "offer-1", plan_code: "professional", plan_name: "专业版", term_months: "1", settlement_mode: "WALLET" as const, currency: "CNY" as const, total_minor: "12000", pricing_version: "v1", availability: "available" as const };
const quote = { quote_id: "quote-1", organization_id: "org-B", offer_id: "offer-1", product_kind: "SUBSCRIPTION_PLAN" as const, plan_code: "professional", plan_fingerprint: "a".repeat(64), term_months: "1", settlement_mode: "WALLET" as const, currency: "CNY" as const, total_minor: "12000", pricing_version: "v1", expires_at: "2099-09-25T01:00:00Z", fingerprint: "b".repeat(64), created_at: "2099-09-25T00:00:00Z" };
const order = { order_id: "order-1", organization_id: "org-B", kind: "SUBSCRIPTION_PURCHASE" as const, description: "专业版", quote_id: "quote-1", currency: "CNY" as const, total_minor: "12000", status: "FULFILLED" as const, items: [], product_kind: "SUBSCRIPTION_PLAN" as const, plan_code: "professional", plan_fingerprint: "a".repeat(64), term_months: "1", settlement_mode: "WALLET" as const, activation_proof: { operation_id: "subscription-activate:order-1", request_fingerprint: "c".repeat(64), outcome: "ACTIVATED" as const, subscription_id: "1", starts_at: "2026-09-25T00:00:00Z", expires_at: "2026-10-25T00:00:00Z", entitlement_set_fingerprint: "d".repeat(64), decided_at: "2026-09-25T00:00:00Z" }, created_at: "2026-09-25T00:00:00Z", updated_at: "2026-09-25T00:00:00Z" };
const initial = { ...commercialOverviewFixture(), subscription: null, entitlements: [] };
const activated = { ...commercialOverviewFixture(), subscription: { ...commercialOverviewFixture().subscription!, plan_code: "professional", plan_name: "专业版" } };
let client: QueryClient;

function tree(options: { organizationId?: string; roles?: string[]; items?: SubscriptionOffer[] } = {}) {
  const organizationId = options.organizationId ?? "org-B";
  return <QueryClientProvider client={client}><SubscriptionPlanOptions userId="actor" organizationId={organizationId} organizationName="企业乙" roles={options.roles ?? ["listingkit_admin"]} overview={{ ...initial, organization_id: organizationId }} offers={{ organization_id: organizationId, items: options.items ?? [offer] }} /></QueryClientProvider>;
}

beforeEach(() => {
  sessionStorage.clear();
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  api.quote.mockReset().mockResolvedValue(quote);
  api.order.mockReset().mockResolvedValue(order);
  api.readOrder.mockReset().mockResolvedValue(order);
  api.readOverview.mockReset().mockResolvedValue(activated);
  api.wallet.mockReset().mockResolvedValue({ organization_id: "org-B", currency: "CNY", available_minor: "12000", reserved_minor: "0", debt_minor: "0", lifetime_topup_minor: "12000", lifetime_spend_minor: "0", version: "1", observed_at: "2026-09-25T00:00:00Z" });
});
afterEach(() => { cleanup(); client.clear(); sessionStorage.clear(); });

it("quotes a server offer, confirms exact money and organization, then reads canonical entitlements", async () => {
  const user = userEvent.setup();
  render(tree());
  expect(await screen.findByRole("button", { name: "立即开通" })).toBeVisible();
  await user.click(screen.getByRole("button", { name: "立即开通" }));
  const confirmation = await screen.findByRole("region", { name: "报价确认" });
  expect(within(confirmation).getByText("企业乙（org-B）")).toBeVisible();
  expect(within(confirmation).getByText("¥120.00")).toBeVisible();
  expect(within(confirmation).getByText(/2099/)).toBeVisible();
  expect(api.quote).toHaveBeenCalledWith("actor", "org-B", "offer-1", expect.any(AbortSignal));
  await user.click(within(confirmation).getByRole("button", { name: "确认开通" }));
  expect(await screen.findByText(/权益已生效/)).toBeVisible();
  expect(api.order).toHaveBeenCalledWith("actor", "org-B", "quote-1", expect.any(String), expect.any(AbortSignal));
  expect(api.readOverview).toHaveBeenCalledWith("org-B", expect.any(AbortSignal));
  expect(screen.getByRole("link", { name: "查看我的权益" })).toHaveAttribute("href", "/workbench/plans/entitlements");
  expect(screen.getByRole("link", { name: "查看订单" })).toHaveAttribute("href", "/workbench/plans/orders/order-1");
});

it("keeps one durable operation identity after unknown POST outcome and resumes it after reload", async () => {
  const user = userEvent.setup();
  api.order.mockRejectedValueOnce({ code: "DEADLINE_EXCEEDED" }).mockResolvedValueOnce(order);
  const view = render(tree());
  await screen.findByRole("button", { name: "立即开通" });
  await user.click(screen.getByRole("button", { name: "立即开通" }));
  await user.click(within(await screen.findByRole("region", { name: "报价确认" })).getByRole("button", { name: "确认开通" }));
  expect(await screen.findByText(/结果尚未确认/)).toBeVisible();
  const originalKey = api.order.mock.calls[0][3];
  expect(sessionStorage.getItem('subscription-purchase.pending:["actor","org-B"]')).toContain(originalKey);
  view.unmount();
  render(tree());
  expect(await screen.findByRole("button", { name: "恢复原订单" })).toBeVisible();
  expect(screen.queryByRole("button", { name: "立即开通" })).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "恢复原订单" }));
  expect(await screen.findByText(/权益已生效/)).toBeVisible();
  expect(api.order.mock.calls[1][3]).toBe(originalKey);
  expect(api.quote).toHaveBeenCalledTimes(1);
});

it("blocks unavailable, read-only, insufficient-wallet and no-offer purchase controls", async () => {
  const view = render(tree({ roles: ["listingkit_operator"], items: [offer] }));
  expect(await screen.findByText("¥120.00")).toBeVisible();
  expect(screen.queryByRole("button", { name: "立即开通" })).not.toBeInTheDocument();
  view.rerender(tree({ items: [{ ...offer, availability: "payment_unavailable", settlement_mode: "EXTERNAL_PAYMENT" }] }));
  expect(await screen.findByText("支付方式暂未开放")).toBeVisible();
  expect(screen.queryByRole("button", { name: "立即开通" })).not.toBeInTheDocument();
  view.rerender(tree({ items: [] }));
  expect(await screen.findByText("暂无正式可售套餐")).toBeVisible();
  expect(screen.queryByRole("button", { name: "立即开通" })).not.toBeInTheDocument();
  api.wallet.mockResolvedValue({ organization_id: "org-B", currency: "CNY", available_minor: "11999", reserved_minor: "0", debt_minor: "0", lifetime_topup_minor: "12000", lifetime_spend_minor: "0", version: "1", observed_at: "2026-09-25T00:00:00Z" });
  client.clear();
  view.unmount();
  render(tree());
  expect(await screen.findByText("余额不足")).toBeVisible();
  expect(screen.queryByRole("button", { name: "立即开通" })).not.toBeInTheDocument();
});

it("does not show success for an unconfirmed entitlement readback or another organization's pending operation", async () => {
  const user = userEvent.setup();
  api.readOverview.mockResolvedValue(initial);
  const view = render(tree());
  await screen.findByRole("button", { name: "立即开通" });
  await user.click(screen.getByRole("button", { name: "立即开通" }));
  await user.click(within(await screen.findByRole("region", { name: "报价确认" })).getByRole("button", { name: "确认开通" }));
  expect(await screen.findByText(/订单已完成，但权益回读尚未确认/)).toBeVisible();
  expect(screen.queryByText(/权益已生效/)).not.toBeInTheDocument();
  expect(sessionStorage.getItem('subscription-purchase.pending:["actor","org-B"]')).toContain("order-1");
  view.unmount();
  render(tree({ organizationId: "org-C", items: [] }));
  await waitFor(() => expect(screen.getByText("暂无正式可售套餐")).toBeVisible());
  expect(screen.queryByRole("button", { name: "查询原订单" })).not.toBeInTheDocument();
});

it("ends a purchase attempt only when the owner reports a terminal pre-effect or cancelled outcome", async () => {
  const user = userEvent.setup();
  api.order.mockRejectedValueOnce({ code: "INSUFFICIENT_FUNDS" });
  render(tree());
  await screen.findByRole("button", { name: "立即开通" });
  await user.click(screen.getByRole("button", { name: "立即开通" }));
  await user.click(within(await screen.findByRole("region", { name: "报价确认" })).getByRole("button", { name: "确认开通" }));
  expect(await screen.findByText("企业钱包余额不足。")).toBeVisible();
  expect(sessionStorage.getItem('subscription-purchase.pending:["actor","org-B"]')).toBeNull();
  expect(screen.queryByRole("region", { name: "原订单恢复" })).not.toBeInTheDocument();
});

it("fails closed on malformed persisted purchase recovery data", async () => {
  sessionStorage.setItem('subscription-purchase.pending:["actor","org-B"]', "{bad-json");
  render(tree());
  expect(await screen.findByText(/原订单恢复信息无效/)).toBeVisible();
  expect(screen.queryByRole("button", { name: "立即开通" })).not.toBeInTheDocument();
});

it("allows only a server-declared zero-price offer without reading or changing the wallet", async () => {
  const user = userEvent.setup();
  const free = { ...offer, settlement_mode: "ZERO_PRICE" as const, total_minor: "0" };
  api.quote.mockResolvedValue({ ...quote, settlement_mode: "ZERO_PRICE", total_minor: "0" });
  api.order.mockResolvedValue({ ...order, settlement_mode: "ZERO_PRICE", total_minor: "0" });
  render(tree({ items: [free] }));
  expect(await screen.findByRole("button", { name: "立即开通" })).toBeVisible();
  expect(api.wallet).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "立即开通" }));
  const confirmation = await screen.findByRole("region", { name: "报价确认" });
  expect(within(confirmation).getByText("¥0.00")).toBeVisible();
  await user.click(within(confirmation).getByRole("button", { name: "确认开通" }));
  expect(await screen.findByText(/权益已生效/)).toBeVisible();
  expect(api.order).toHaveBeenCalledTimes(1);
});

it("retains the original order during reconciliation and never offers a second purchase", async () => {
  const user = userEvent.setup();
  api.order.mockResolvedValue({ ...order, status: "RECONCILIATION_REQUIRED", activation_proof: undefined });
  api.readOrder.mockResolvedValue({ ...order, status: "RECONCILIATION_REQUIRED", activation_proof: undefined });
  render(tree());
  await screen.findByRole("button", { name: "立即开通" });
  await user.click(screen.getByRole("button", { name: "立即开通" }));
  await user.click(within(await screen.findByRole("region", { name: "报价确认" })).getByRole("button", { name: "确认开通" }));
  expect(await screen.findByText(/正在核对原订单/)).toBeVisible();
  expect(screen.queryByRole("button", { name: "立即开通" })).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "查询原订单" }));
  expect(api.readOrder).toHaveBeenCalledWith("actor", "org-B", "order-1", expect.any(AbortSignal));
  expect(api.order).toHaveBeenCalledTimes(1);
});

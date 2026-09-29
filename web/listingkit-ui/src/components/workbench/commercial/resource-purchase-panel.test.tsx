import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  render,
  screen,
  within,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, afterEach, it, expect, vi } from "vitest";
import { ResourcePurchasePanel } from "./resource-purchase-panel";
import { commercialResourcesFixture } from "@/test/fixtures/commercial-resources";
const api = vi.hoisted(() => ({
  quote: vi.fn(),
  order: vi.fn(),
  readOrder: vi.fn(),
  resources: vi.fn(),
  wallet: vi.fn(),
}));
vi.mock("@/lib/api/resource-purchase", async (original) => ({
  ...(await original<typeof import("@/lib/api/resource-purchase")>()),
  createResourceQuote: api.quote,
  createResourceOrder: api.order,
}));
vi.mock("@/lib/api/commercial-billing", async (original) => ({
  ...(await original<typeof import("@/lib/api/commercial-billing")>()),
  getCommercialOrder: api.readOrder,
  getCommercialResources: api.resources,
  getCommercialWallet: api.wallet,
}));
const offer = {
  offer_id: "offer-1",
  product_kind: "AI_POINT" as const,
  resource_type: "ai_point" as const,
  currency: "CNY" as const,
  unit_price_minor: "7",
  min_quantity: "1",
  max_quantity: "1000",
  pricing_version: "synthetic-v1",
};
const quote = {
  quote_id: "quote-1",
  organization_id: "org-B",
  offer_id: "offer-1",
  product_kind: "AI_POINT" as const,
  resource_type: "ai_point" as const,
  resource_quantity: "10",
  currency: "CNY" as const,
  total_minor: "70",
  pricing_version: "synthetic-v1",
  expires_at: "2099-09-29T01:00:00Z",
  fingerprint: "frozen",
  created_at: "2099-09-29T00:00:00Z",
  amount_minor: "73",
  unit_price_minor: "7",
  remainder_minor: "3",
};
const order = {
  order_id: "order-1",
  organization_id: "org-B",
  kind: "RESOURCE_PURCHASE" as const,
  description: "Synthetic points",
  quote_id: "quote-1",
  currency: "CNY" as const,
  total_minor: "70",
  status: "FULFILLED" as const,
  items: [],
  product_kind: "AI_POINT" as const,
  created_at: "2026-09-29T00:00:00Z",
  updated_at: "2026-09-29T00:00:00Z",
};
let client: QueryClient;
function tree(org = "org-B", items = [offer], roles = ["listingkit_admin"]) {
  return (
    <QueryClientProvider client={client}>
      <ResourcePurchasePanel
        userId="actor"
        organizationId={org}
        organizationName="企业乙"
        roles={roles}
        offers={{ organization_id: org, items }}
      />
    </QueryClientProvider>
  );
}
beforeEach(() => {
  sessionStorage.clear();
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  api.quote.mockReset().mockResolvedValue(quote);
  api.order.mockReset().mockResolvedValue(order);
  api.readOrder.mockReset().mockResolvedValue(order);
  api.resources
    .mockReset()
    .mockResolvedValue(commercialResourcesFixture("org-B"));
  api.wallet
    .mockReset()
    .mockResolvedValue({
      organization_id: "org-B",
      currency: "CNY",
      available_minor: "1000",
      reserved_minor: "0",
      debt_minor: "0",
      lifetime_topup_minor: "1000",
      lifetime_spend_minor: "0",
      version: "1",
      observed_at: order.created_at,
    });
});
afterEach(() => {
  cleanup();
  client.clear();
  sessionStorage.clear();
  vi.restoreAllMocks();
});
async function confirm() {
  const user = userEvent.setup();
  const region = await screen.findByRole("region", { name: "AI 点数购买" });
  await user.selectOptions(within(region).getByLabelText("购买方式"), "amount");
  await user.clear(within(region).getByLabelText("金额（元）"));
  await user.type(within(region).getByLabelText("金额（元）"), "0.73");
  await user.click(within(region).getByRole("button", { name: "获取报价" }));
  return {
    user,
    confirmation: await screen.findByRole("region", { name: "报价确认" }),
  };
}
it("confirms exact server quantity, amount and remainder then reads resource balances", async () => {
  render(tree());
  const { user, confirmation } = await confirm();
  expect(within(confirmation).getByText("企业乙（org-B）")).toBeVisible();
  expect(within(confirmation).getByText("10 点")).toBeVisible();
  expect(within(confirmation).getByText("¥0.70")).toBeVisible();
  expect(within(confirmation).getByText(/¥0.03/)).toBeVisible();
  expect(api.quote).toHaveBeenCalledWith(
    "actor",
    "org-B",
    "offer-1",
    { amountMinor: "73" },
    expect.any(AbortSignal),
  );
  await user.click(
    within(confirmation).getByRole("button", { name: "确认购买" }),
  );
  expect(await screen.findByText(/资源已到账/)).toBeVisible();
  expect(api.resources).toHaveBeenCalledWith(
    "actor",
    "org-B",
    expect.any(AbortSignal),
  );
});
it("restores the original command after an unknown result without making another quote", async () => {
  api.order.mockRejectedValueOnce({ code: "DEADLINE_EXCEEDED" });
  const view = render(tree());
  const { user, confirmation } = await confirm();
  await user.click(
    within(confirmation).getByRole("button", { name: "确认购买" }),
  );
  expect(await screen.findByText(/结果尚未确认/)).toBeVisible();
  const key = api.order.mock.calls[0][3];
  expect(
    sessionStorage.getItem('resource-purchase.pending:["actor","org-B"]'),
  ).toContain(key);
  view.unmount();
  render(tree());
  await user.click(await screen.findByRole("button", { name: "恢复原订单" }));
  expect(await screen.findByText(/资源已到账/)).toBeVisible();
  expect(api.order.mock.calls[1][3]).toBe(key);
  expect(api.quote).toHaveBeenCalledTimes(1);
});
it("does not let a late old-enterprise response overwrite a persisted original operation", async () => {
  let complete: (v: typeof order) => void = () => {};
  api.order.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        complete = resolve;
      }),
  );
  const view = render(tree());
  const { user, confirmation } = await confirm();
  await user.click(
    within(confirmation).getByRole("button", { name: "确认购买" }),
  );
  await waitFor(() => expect(api.order).toHaveBeenCalledTimes(1));
  const original = sessionStorage.getItem(
    'resource-purchase.pending:["actor","org-B"]',
  );
  view.rerender(tree("org-C", []));
  complete(order);
  await waitFor(() =>
    expect((api.order.mock.calls[0][4] as AbortSignal).aborted).toBe(true),
  );
  expect(
    sessionStorage.getItem('resource-purchase.pending:["actor","org-B"]'),
  ).toBe(original);
  expect(api.resources).not.toHaveBeenCalled();
  expect(screen.queryByText(/资源已到账/)).not.toBeInTheDocument();
});
it("keeps the original order while readback is unavailable and fails closed on invalid recovery storage", async () => {
  api.resources.mockRejectedValue({ code: "DEPENDENCY_UNAVAILABLE" });
  const view = render(tree());
  const { user, confirmation } = await confirm();
  await user.click(
    within(confirmation).getByRole("button", { name: "确认购买" }),
  );
  expect(await screen.findByText(/余额回读尚未确认/)).toBeVisible();
  expect(
    sessionStorage.getItem('resource-purchase.pending:["actor","org-B"]'),
  ).toContain("order-1");
  view.unmount();
  sessionStorage.setItem('resource-purchase.pending:["actor","org-B"]', "{bad");
  render(tree());
  expect(await screen.findByText(/原订单恢复信息无效/)).toBeVisible();
  expect(
    screen
      .getAllByRole("button", { name: "获取报价" })
      .every((button) => (button as HTMLButtonElement).disabled),
  ).toBe(true);
});
it("shows no configured price or purchase action for an empty catalog", async () => {
  render(tree("org-B", []));
  expect(screen.getAllByText("价格未配置")).toHaveLength(3);
  expect(api.quote).not.toHaveBeenCalled();
  expect(api.wallet).not.toHaveBeenCalled();
});

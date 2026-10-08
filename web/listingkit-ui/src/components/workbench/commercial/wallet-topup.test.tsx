import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { WalletTopUpError, type TopUpOrder } from "@/lib/api/wallet-topup";
import { TopUpPaymentPanel, WalletTopUpEntry } from "./wallet-topup";

const api = vi.hoisted(() => ({
  options: vi.fn(),
  create: vi.fn(),
  read: vi.fn(),
  checkout: vi.fn(),
  cancel: vi.fn(),
  push: vi.fn(),
}));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: api.push }) }));
vi.mock("@/lib/api/wallet-topup", async (original) => ({
  ...(await original<typeof import("@/lib/api/wallet-topup")>()),
  getTopUpOptions: api.options,
  createTopUp: api.create,
  checkoutTopUp: api.checkout,
  cancelTopUp: api.cancel,
}));
vi.mock("@/lib/api/commercial-billing", async (original) => ({
  ...(await original<typeof import("@/lib/api/commercial-billing")>()),
  getCommercialOrder: api.read,
}));
let client: QueryClient;
const id = "a8615321-2f62-42ad-948c-52e090714e45";
const pendingKey = 'wallet-topup.pending:["admin-1","org-1"]';
const order = (): TopUpOrder => ({
  order_id: id,
  organization_id: "org-1",
  kind: "WALLET_TOP_UP",
  description: "企业钱包充值",
  currency: "CNY",
  total_minor: "1500",
  status: "PENDING",
  items: [],
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
  top_up: {
    provider: "WECHAT_PAY",
    phase: "CREATED",
    attempt_id: "attempt-1",
    version: "1",
    expires_at: new Date(Date.now() + 600000).toISOString(),
    close_requested: false,
    late_payment_corrected: false,
  },
});
const wrap = (child: React.ReactNode) => (
  <QueryClientProvider client={client}>{child}</QueryClientProvider>
);
beforeEach(() => {
  vi.clearAllMocks();
  sessionStorage.clear();
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  HTMLDialogElement.prototype.showModal = function () {
    this.open = true;
  };
  HTMLDialogElement.prototype.close = function () {
    this.open = false;
  };
  api.options.mockResolvedValue({
    organization_id: "org-1",
    currency: "CNY",
    min_minor: "500",
    max_minor: "20000",
    quick_amounts_minor: ["1500", "7000"],
    channels: [
      {
        provider: "ALIPAY",
        product: "PAGE_PAY",
        available: false,
        reason: "NEW_PAYMENTS_DISABLED",
      },
      {
        provider: "WECHAT_PAY",
        product: "NATIVE",
        available: true,
        reason: "",
      },
    ],
  });
  api.read.mockResolvedValue(order());
});
afterEach(() => {
  cleanup();
  client.clear();
});

it("uses configured amounts and preserves the original intent across a lost creation response", async () => {
  const user = userEvent.setup();
  api.create
    .mockRejectedValueOnce(new WalletTopUpError("DEADLINE_EXCEEDED"))
    .mockResolvedValueOnce(order());
  render(
    wrap(
      <WalletTopUpEntry
        userId="admin-1"
        organizationId="org-1"
        permissions={["workbench.commercial.wallet_topup"]}
      />,
    ),
  );
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "充值钱包" })).toBeEnabled(),
  );
  await user.click(screen.getByRole("button", { name: "充值钱包" }));
  expect(screen.getByRole("radio", { name: /支付宝/ })).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "¥15.00" }));
  await user.click(screen.getByRole("radio", { name: /微信支付/ }));
  await user.click(screen.getByRole("button", { name: "确认充值" }));
  const saved = JSON.parse(sessionStorage.getItem(pendingKey)!);
  expect(saved).toMatchObject({ provider: "WECHAT_PAY", amount: "1500" });
  await user.click(await screen.findByRole("button", { name: "恢复原订单" }));
  await waitFor(() =>
    expect(api.push).toHaveBeenCalledWith(`/workbench/plans/orders/${id}`),
  );
  expect(api.create).toHaveBeenCalledTimes(2);
  for (const call of api.create.mock.calls)
    expect(call.slice(0, 5)).toEqual([
      "admin-1",
      "org-1",
      "WECHAT_PAY",
      "1500",
      saved.key,
    ]);
  expect(JSON.parse(sessionStorage.getItem(pendingKey)!)).toEqual({
    ...saved,
    orderId: id,
  });
});

it("does not dispatch on page load and removes checkout material when the original order completes", async () => {
  const user = userEvent.setup();
  const initial = order();
  api.read.mockResolvedValue(initial);
  api.checkout.mockResolvedValue({
    organization_id: "org-1",
    order_id: id,
    attempt_id: "attempt-1",
    provider: "WECHAT_PAY",
    kind: "QR_CODE",
    expires_at: initial.top_up!.expires_at,
    payload: "weixin://wxpay/bizpayurl?pr=private-checkout",
  });
  sessionStorage.setItem(
    pendingKey,
    JSON.stringify({
      key: crypto.randomUUID(),
      provider: "WECHAT_PAY",
      amount: "1500",
      orderId: id,
    }),
  );
  render(
    wrap(
      <TopUpPaymentPanel
        userId="admin-1"
        organizationId="org-1"
        permissions={["workbench.commercial.wallet_topup"]}
        initialOrder={initial}
      />,
    ),
  );
  await waitFor(() =>
    expect(
      screen.getByRole("button", { name: "显示微信付款码" }),
    ).toBeEnabled(),
  );
  expect(api.checkout).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "显示微信付款码" }));
  expect(
    (await screen.findByTitle("请用手机微信扫描原订单付款码")).closest("svg"),
  ).toBeVisible();
  expect(sessionStorage.getItem(pendingKey)).not.toContain("private-checkout");
  expect(
    JSON.stringify(
      client.getQueryData([
        "workbench",
        "org-1",
        "commercial-order",
        "admin-1",
        id,
      ]),
    ),
  ).not.toContain("private-checkout");
  api.read.mockResolvedValue({
    ...initial,
    status: "FULFILLED",
    top_up: { ...initial.top_up, phase: "COMPLETED", version: "2" },
  });
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "查询原订单" })).toBeEnabled(),
  );
  await user.click(screen.getByRole("button", { name: "查询原订单" }));
  expect(await screen.findByText("钱包入账处理完成")).toBeVisible();
  expect(
    screen.queryByTitle("请用手机微信扫描原订单付款码"),
  ).not.toBeInTheDocument();
  await waitFor(() => expect(sessionStorage.getItem(pendingKey)).toBeNull());
  expect(
    client.getQueryData([
      "workbench",
      "org-1",
      "commercial-order",
      "admin-1",
      id,
    ]),
  ).toMatchObject({ status: "FULFILLED" });
});

it("keeps missing channel configuration and non-admin actors unavailable", async () => {
  api.options.mockResolvedValue({
    organization_id: "org-1",
    currency: "CNY",
    min_minor: "0",
    max_minor: "0",
    quick_amounts_minor: [],
    channels: [],
  });
  render(
    wrap(
      <WalletTopUpEntry userId="reader" organizationId="org-1" permissions={[]} />,
    ),
  );
  expect(await screen.findByText("仅企业管理员可发起充值。")).toBeVisible();
  expect(screen.getByRole("button", { name: "充值钱包" })).toBeDisabled();
  expect(api.create).not.toHaveBeenCalled();
});

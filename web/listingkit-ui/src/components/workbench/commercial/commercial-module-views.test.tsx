import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { commercialOverviewFixture } from "@/test/fixtures/commercial-overview";
import {
  CommercialOverviewView,
  UsageDetailsView,
} from "./commercial-module-views";
import { OrdersView, WalletView } from "./commercial-billing-views";
vi.mock("./wallet-topup", () => ({
  WalletTopUpEntry: () => <button disabled>充值钱包</button>,
}));

afterEach(cleanup);

it("renders unified base and real service observations without a subscription gate", () => {
  render(<CommercialOverviewView data={commercialOverviewFixture()} />);
  expect(screen.getByText("基础方案")).toBeVisible();
  expect(screen.getByText("2 家生效中")).toBeVisible();
  expect(screen.getByRole("link", { name: "购买资源" })).toHaveAttribute(
    "href",
    "/workbench/plans/options",
  );
  expect(screen.queryByText(/¥168|未提供|专业版/)).not.toBeInTheDocument();
});
it("filters actual resource ledger rows on the server and preserves exact resource units", async () => {
  const onFilter = vi.fn();
  render(
    <UsageDetailsView
      page={{ organization_id: "org-B", items: [], next_cursor: null }}
      onFilter={onFilter}
      onNext={() => undefined}
    />,
  );
  expect(screen.getByText("暂无匹配的资源流水。")).toBeVisible();
  await userEvent.selectOptions(screen.getByLabelText("用量类型"), "ai_point");
  await userEvent.click(screen.getByRole("button", { name: "筛选" }));
  expect(onFilter).toHaveBeenCalledWith({
    resourceType: "ai_point",
    from: undefined,
    until: undefined,
  });
});

it("renders only real wallet data and delegates recharge to the configured payment entry", () => {
  render(
    <WalletView
      userId="reader"
      roles={["listingkit_admin"]}
      wallet={{
        organization_id: "org-A",
        currency: "CNY",
        available_minor: "12000",
        reserved_minor: "3000",
        debt_minor: "0",
        lifetime_topup_minor: "30000",
        lifetime_spend_minor: "18000",
        version: "4",
        observed_at: "2026-09-23T10:00:00Z",
      }}
      entries={{
        organization_id: "org-A",
        items: [
          {
            entry_id: "entry-1",
            currency: "CNY",
            entry_type: "PURCHASE_COMMIT",
            available_delta_minor: "0",
            reserved_delta_minor: "-3000",
            debt_delta_minor: "0",
            available_after_minor: "12000",
            reserved_after_minor: "0",
            debt_after_minor: "0",
            order_id: "order-1",
            source_id: "commercial-order:order-1",
            occurred_at: "2026-09-23T10:00:00Z",
          },
        ],
        next_cursor: "",
      }}
      onNext={() => undefined}
    />,
  );
  expect(screen.getByText("¥120.00")).toBeVisible();
  expect(screen.getByText("¥30.00")).toBeVisible();
  expect(screen.getByRole("button", { name: "充值钱包" })).toBeDisabled();
  expect(
    screen.getByText(/有欠款时先偿债；渠道手续费由平台承担/),
  ).toBeVisible();
  expect(screen.getByRole("link", { name: "order-1" })).toHaveAttribute(
    "href",
    "/workbench/plans/orders/order-1",
  );
  expect(screen.queryByText("¥5,000.00")).not.toBeInTheDocument();
});

it("renders owner-backed orders and submits bounded search and date/type/status filters", async () => {
  const onFilter = vi.fn();
  render(
    <OrdersView
      summary={{
        organization_id: "org-A",
        currency: "CNY",
        from: "2026-08-24T10:00:00Z",
        until: "2026-09-23T10:00:00Z",
        spend_minor: "3000",
        store_renewal_spend_minor: "0",
        ai_point_spend_minor: "3000",
        data_row_spend_minor: "0",
        other_spend_minor: "0",
        observed_at: "2026-09-23T10:00:00Z",
      }}
      page={{
        organization_id: "org-A",
        items: [
          {
            order_id: "order-1",
            organization_id: "org-A",
            kind: "RESOURCE_PURCHASE",
            description: "AI 点数 × 3",
            quote_id: "quote-1",
            currency: "CNY",
            total_minor: "3000",
            status: "FULFILLED",
            product_kind: "AI_POINT",
            items: [
              {
                order_item_id: "item-1",
                product_kind: "AI_POINT",
                resource_type: "ai_point",
                resource_quantity: "3",
                amount_minor: "3000",
              },
            ],
            created_at: "2026-09-23T10:00:00Z",
            updated_at: "2026-09-23T10:00:00Z",
          },
        ],
        next_cursor: "next",
      }}
      onFilter={onFilter}
      onNext={() => undefined}
    />,
  );
  expect(screen.getAllByText("¥30.00")).toHaveLength(3);
  expect(screen.getByText("AI 点数 × 3")).toBeVisible();
  expect(screen.getByRole("link", { name: "详情" })).toHaveAttribute(
    "href",
    "/workbench/plans/orders/order-1",
  );
  await userEvent.type(screen.getByLabelText("搜索订单"), " renewal ");
  await userEvent.selectOptions(
    screen.getByLabelText("订单类型"),
    "RESOURCE_PURCHASE",
  );
  await userEvent.selectOptions(screen.getByLabelText("订单状态"), "FULFILLED");
  fireEvent.change(screen.getByLabelText("开始日期"), {
    target: { value: "2026-09-01" },
  });
  fireEvent.change(screen.getByLabelText("结束日期"), {
    target: { value: "2026-09-23" },
  });
  await userEvent.click(screen.getByRole("button", { name: "筛选" }));
  expect(onFilter).toHaveBeenCalledWith({
    query: "renewal",
    kind: "RESOURCE_PURCHASE",
    status: "FULFILLED",
    from: "2026-08-31T16:00:00.000Z",
    until: "2026-09-23T16:00:00.000Z",
  });
  expect(
    screen.getByText(/不提供购买、退款、发票开具或导出写操作/),
  ).toBeVisible();
});

it("restores applied ledger filters when a query remounts the view", () => {
  render(
    <UsageDetailsView
      page={{ organization_id: "org-B", items: [], next_cursor: null }}
      filters={{
        resourceType: "ai_point",
        from: "2026-09-01T00:00:00.000Z",
        until: "2026-10-01T00:00:00.000Z",
      }}
      onFilter={() => undefined}
      onNext={() => undefined}
    />,
  );
  expect(screen.getByLabelText("用量类型")).toHaveValue("ai_point");
  expect(screen.getByLabelText("开始日期（UTC）")).toHaveValue("2026-09-01");
  expect(screen.getByLabelText("结束日期（UTC）")).toHaveValue("2026-09-30");
});

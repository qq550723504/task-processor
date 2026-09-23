import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { commercialOverviewFixture } from "@/test/fixtures/commercial-overview";
import { CommercialOverviewView, UsageDetailsView } from "./commercial-module-views";
import { OrdersView, WalletView } from "./commercial-billing-views";

afterEach(cleanup);

it("renders the overview entry points from real commercial observations without fixture amounts", () => {
  render(<CommercialOverviewView data={commercialOverviewFixture()} />);

  expect(screen.getByRole("heading", { name: "当前方案" })).toBeVisible();
  expect(screen.getByText("企业实际合同")).toBeVisible();
  expect(screen.getByText("充值中心")).toBeVisible();
  expect(screen.getAllByText("账单与订单").length).toBeGreaterThan(0);
  expect(screen.getByText("0 家")).toBeVisible();
  expect(screen.getByText(/钱包余额、资金流水、账单汇总与订单由独立商业 owner 提供/)).toBeVisible();
  expect(screen.queryByText(/8,650|300,000|¥5,000/)).not.toBeInTheDocument();
});

it("keeps usage units and unknown state explicit while filtering the returned ledger client-side", async () => {
  render(<UsageDetailsView data={commercialOverviewFixture()} />);

  expect(screen.getByText("9,007,199,254,740,993 字节")).toBeVisible();
  expect(screen.getAllByText("未知（作业次）").length).toBeGreaterThan(0);
  expect(screen.getAllByText("未提供").length).toBeGreaterThan(0);
  await userEvent.selectOptions(screen.getByLabelText("用量类型"), "storage_bytes_current");
  expect(screen.getByText("当前保留存储")).toBeVisible();
  expect(screen.queryByText("资料生成作业")).not.toBeInTheDocument();
});

it("renders only real wallet data and keeps payment writes unavailable", () => {
  render(<WalletView wallet={{ organization_id: "org-A", currency: "CNY", available_minor: "12000", reserved_minor: "3000", debt_minor: "0", lifetime_topup_minor: "30000", lifetime_spend_minor: "18000", version: "4", observed_at: "2026-09-23T10:00:00Z" }} entries={{ organization_id: "org-A", items: [{ entry_id: "entry-1", currency: "CNY", entry_type: "PURCHASE_COMMIT", available_delta_minor: "0", reserved_delta_minor: "-3000", debt_delta_minor: "0", available_after_minor: "12000", reserved_after_minor: "0", debt_after_minor: "0", order_id: "order-1", source_id: "commercial-order:order-1", occurred_at: "2026-09-23T10:00:00Z" }], next_cursor: "" }} onNext={() => undefined} />);
  expect(screen.getByText("¥120.00")).toBeVisible();
  expect(screen.getByText("¥30.00")).toBeVisible();
  expect(screen.getByRole("button", { name: "充值暂未开放" })).toBeDisabled();
  expect(screen.getByText(/不会提交充值、购买或资金变更/)).toBeVisible();
  expect(screen.getByRole("link", { name: "order-1" })).toHaveAttribute("href", "/workbench/plans/orders/order-1");
  expect(screen.queryByText("¥5,000.00")).not.toBeInTheDocument();
});

it("renders owner-backed orders and submits bounded search and date/type/status filters", async () => {
  const onFilter = vi.fn();
  render(<OrdersView summary={{ organization_id: "org-A", currency: "CNY", from: "2026-08-24T10:00:00Z", until: "2026-09-23T10:00:00Z", spend_minor: "3000", store_renewal_spend_minor: "0", ai_point_spend_minor: "3000", data_row_spend_minor: "0", other_spend_minor: "0", observed_at: "2026-09-23T10:00:00Z" }} page={{ organization_id: "org-A", items: [{ order_id: "order-1", organization_id: "org-A", kind: "RESOURCE_PURCHASE", description: "AI 点数 × 3", quote_id: "quote-1", currency: "CNY", total_minor: "3000", status: "FULFILLED", items: [{ order_item_id: "item-1", product_kind: "AI_POINT", resource_type: "ai_point", resource_quantity: "3", amount_minor: "3000" }], created_at: "2026-09-23T10:00:00Z", updated_at: "2026-09-23T10:00:00Z" }], next_cursor: "next" }} onFilter={onFilter} onNext={() => undefined} />);
  expect(screen.getAllByText("¥30.00")).toHaveLength(3);
  expect(screen.getByText("AI 点数 × 3")).toBeVisible();
  expect(screen.getByRole("link", { name: "详情" })).toHaveAttribute("href", "/workbench/plans/orders/order-1");
  await userEvent.type(screen.getByLabelText("搜索订单"), " renewal ");
  await userEvent.selectOptions(screen.getByLabelText("订单类型"), "RESOURCE_PURCHASE");
  await userEvent.selectOptions(screen.getByLabelText("订单状态"), "FULFILLED");
  fireEvent.change(screen.getByLabelText("开始日期"), { target: { value: "2026-09-01" } });
  fireEvent.change(screen.getByLabelText("结束日期"), { target: { value: "2026-09-23" } });
  await userEvent.click(screen.getByRole("button", { name: "筛选" }));
  expect(onFilter).toHaveBeenCalledWith({ query: "renewal", kind: "RESOURCE_PURCHASE", status: "FULFILLED", from: "2026-09-01T00:00:00Z", until: "2026-09-24T00:00:00.000Z" });
  expect(screen.getByText(/不提供购买、退款、发票开具或导出写操作/)).toBeVisible();
});

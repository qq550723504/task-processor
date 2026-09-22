import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it } from "vitest";
import { commercialOverviewFixture } from "@/test/fixtures/commercial-overview";
import { CapabilityGatedView, CommercialOverviewView, UsageDetailsView } from "./commercial-module-views";

afterEach(cleanup);

it("renders the overview entry points from real commercial observations without fixture amounts", () => {
  render(<CommercialOverviewView data={commercialOverviewFixture()} />);

  expect(screen.getByRole("heading", { name: "当前方案" })).toBeVisible();
  expect(screen.getByText("企业实际合同")).toBeVisible();
  expect(screen.getByText("充值中心")).toBeVisible();
  expect(screen.getAllByText("账单与订单").length).toBeGreaterThan(0);
  expect(screen.getByText("0 家")).toBeVisible();
  expect(screen.getAllByText("当前 owner 未接入").length).toBeGreaterThan(0);
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

it("renders wallet and billing as explicit capability gates without creating money facts", () => {
  render(<CapabilityGatedView page="top-up" />);
  expect(screen.getByRole("heading", { name: "钱包与充值暂未开放" })).toBeVisible();
  expect(screen.getByText("可用余额")).toBeVisible();
  expect(screen.getAllByText("未提供").length).toBeGreaterThan(0);
  expect(screen.queryByText("¥5,000.00")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "账户充值暂未开放" })).toBeDisabled();
});

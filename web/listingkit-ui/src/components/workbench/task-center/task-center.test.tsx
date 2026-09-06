import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { TaskCenterLayout } from "./task-center-layout";
import { CompletedWorkResults, WorkResultDetail } from "./completed-work-results";

// Approved #340 C340-H1 synthetic component DTO, not a backend or runtime fallback.
const fixture = {
  projection_version: "1", coverage: "listing-local-preparation-only",
  items: ["12345678-1234-1234-1234-123456789abc", "12345678-1234-1234-1234-123456789abd"].map((id, index) => ({
    source_type: "listing-local-preparation", source_kind: "shein-local-record",
    title: "准备商品上架资料", summary: "本地资料已创建；诊断和发布是后续独立操作",
    platform: "SHEIN", work_scope: "general", completion_basis: "local_record_committed",
    source_record_id: id, product_key: `synthetic-product-${index}`, snapshot_version: "9007199254740993",
    country: "US", language: "en", created_at: "2026-09-06T00:00:00Z",
    result: { kind: "shein-diagnostic", href: `/workbench/shein-records/${id}/diagnostic` },
  })), next_cursor: null,
};
// Presentation slots only. Production projection DTO/client remain exclusively #340-owned.
const entries = fixture.items.map((item) => ({
  key: item.source_record_id, heading: item.title, caption: item.product_key,
  content: <span>{item.summary}</span>,
  detail: <WorkResultDetail title={item.title} summary={item.summary}>
    <dl><dt>来源商品</dt><dd>{item.product_key}</dd><dt>快照版本</dt><dd>{item.snapshot_version}</dd></dl>
    <a href={item.result.href}>查看诊断</a>
  </WorkResultDetail>,
}));
afterEach(cleanup);

it("states bounded coverage and does not fabricate global metrics or lifecycle actions", () => {
  render(<TaskCenterLayout completed><p>结果区域</p></TaskCenterLayout>);
  expect(screen.getByRole("heading", { level: 1, name: "任务中心" })).toBeVisible();
  expect(screen.getByText(/仅覆盖本地资料准备完成记录/)).toBeVisible();
  expect(screen.getByText(/不代表诊断通过或可发布/)).toBeVisible();
  expect(screen.getByRole("link", { name: "已完成" })).toHaveAttribute("aria-current", "page");
  expect(screen.getByRole("button", { name: /搜索任务/ })).toBeDisabled();
  expect(screen.getByRole("button", { name: /向硕米发起任务/ })).toBeDisabled();
  expect(screen.queryByText(/3 \/ 5|5 \/ 5/)).not.toBeInTheDocument();
});

it("selects a real result reference with keyboard and returns focus to its row", async () => {
  const user = userEvent.setup();
  render(<CompletedWorkResults entries={entries} />);
  expect(screen.queryByRole("link", { name: "查看诊断" })).not.toBeInTheDocument();
  const second = screen.getByRole("button", { name: /synthetic-product-1/ });
  second.focus(); await user.keyboard("{Enter}");
  const detail = screen.getByRole("region", { name: "工作记录详情" });
  expect(within(detail).getByText("9007199254740993")).toBeVisible();
  expect(within(detail).getByRole("link", { name: "查看诊断" })).toHaveAttribute("href", fixture.items[1].result.href);
  expect(screen.getByRole("heading", { name: "工作记录详情" })).toHaveFocus();
  await user.click(within(detail).getByRole("button", { name: "返回记录列表" }));
  expect(second).toHaveFocus();
  expect(screen.queryByRole("link", { name: "查看诊断" })).not.toBeInTheDocument();
});

it("cannot retain detail when the selected entry leaves the returned page", async () => {
  const view = render(<CompletedWorkResults entries={entries} />);
  await userEvent.click(screen.getByRole("button", { name: /synthetic-product-0/ }));
  view.rerender(<CompletedWorkResults entries={entries.slice(1)} />);
  expect(screen.queryByRole("link", { name: "查看诊断" })).not.toBeInTheDocument();
  expect(screen.getByText("选择一条工作记录")).toBeVisible();
  view.rerender(<CompletedWorkResults entries={entries} />);
  expect(screen.queryByRole("link", { name: "查看诊断" })).not.toBeInTheDocument();
});

it("distinguishes unavailable from a successfully read empty page", () => {
  const view = render(<TaskCenterLayout completed><CompletedWorkResults entries={[]} /></TaskCenterLayout>);
  expect(screen.getByText("当前授权范围内暂无本地资料准备记录")).toBeVisible();
  view.rerender(<TaskCenterLayout completed />);
  expect(screen.getByText("暂未启用")).toBeVisible();
  expect(screen.queryByText("当前授权范围内暂无本地资料准备记录")).not.toBeInTheDocument();
});

it("keeps the toolbar refresh operable without introducing a write action", async () => {
  const refresh = vi.fn();
  render(<TaskCenterLayout completed onRefresh={refresh}><CompletedWorkResults entries={[]} /></TaskCenterLayout>);
  await userEvent.click(screen.getByRole("button", { name: "刷新记录" }));
  expect(refresh).toHaveBeenCalledOnce();
  expect(screen.queryByRole("button", { name: /发布|生成|编辑|Apply/ })).not.toBeInTheDocument();
});

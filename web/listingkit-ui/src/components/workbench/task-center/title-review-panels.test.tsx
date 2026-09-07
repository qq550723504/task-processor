import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { TitleReviewPanels } from "./title-review-panels";

afterEach(cleanup);
const entries = [
  { key: "first", heading: "标题提案", caption: "示例商品 A", statusLabel: "待审核", accepted: false, metadata: <span>基线 9007199254740993</span> },
  { key: "second", heading: "标题提案", caption: "示例商品 B", statusLabel: "已接受，待应用", accepted: true, metadata: <span>基线 2</span> },
];
it("presents pending and accepted separately without inventing a global count or task progress", () => {
  render(<TitleReviewPanels entries={entries} onSelect={vi.fn()} onClose={vi.fn()} />);
  expect(screen.getByText("待审核")).toBeVisible();
  expect(screen.getByText("已接受，待应用")).toBeVisible();
  expect(screen.getByText("基线 9007199254740993")).toBeVisible();
  expect(screen.queryByText(/共 2|已发布|执行中|3 \/ 5/)).not.toBeInTheDocument();
  expect(screen.getByText("选择一条标题提案")).toBeVisible();
});

it("requests selection explicitly, focuses detail and restores focus when closing", async () => {
  const onSelect = vi.fn(), onClose = vi.fn();
  const view = render(<TitleReviewPanels entries={entries} onSelect={onSelect} onClose={onClose} />);
  const row = screen.getByRole("button", { name: /示例商品 A/ });
  row.focus(); await userEvent.keyboard("{Enter}");
  expect(onSelect).toHaveBeenCalledExactlyOnceWith("first");
  expect(screen.getByRole("heading", { name: "标题提案详情" })).toHaveFocus();
  view.rerender(<TitleReviewPanels entries={entries} selectedKey="first" detail={<p>真实读取结果位置</p>} onSelect={onSelect} onClose={onClose} />);
  await userEvent.click(screen.getByRole("button", { name: "返回提案列表" }));
  expect(onClose).toHaveBeenCalledOnce();
  expect(row).toHaveFocus();
});

it("can display an authorized terminal detail outside the actionable collection", () => {
  render(<TitleReviewPanels entries={[]} selectedKey="terminal" detail={<p>已应用回执</p>} onSelect={vi.fn()} onClose={vi.fn()} />);
  expect(screen.getByText("当前授权范围内暂无待处理标题提案")).toBeVisible();
  expect(screen.getByText("已应用回执")).toBeVisible();
});

import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { TitleComparison, TitleApplyConfirmation, TitleEditor } from "./title-review-presentation";

// Presentation-only inputs; these are not a Product Review wire fixture.
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
beforeEach(() => {
  // jsdom does not implement native dialog; real focus trapping is checked in Playwright.
  Object.defineProperty(HTMLDialogElement.prototype, "showModal", { configurable: true, value: function (this: HTMLDialogElement) { this.open = true; } });
  Object.defineProperty(HTMLDialogElement.prototype, "close", { configurable: true, value: function (this: HTMLDialogElement) { this.open = false; } });
});

it("shows the exact before, edited proposal and original title as text", async () => {
  render(<TitleComparison before="原商品标题" after={'<script>alert("source")</script>'} originalTitle="原始建议" />);
  expect(screen.getByText("原商品标题")).toBeVisible();
  expect(screen.getByText('<script>alert("source")</script>')).toBeVisible();
  await userEvent.click(screen.getByText("查看原始建议标题"));
  expect(screen.getByText("原始建议")).toBeVisible();
  expect(document.querySelector("script")).toBeNull();
  expect(screen.queryByRole("link")).not.toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "修改后" })).toBeVisible();
  expect(screen.queryByText(/修改后.*待确认/)).not.toBeInTheDocument();
});

it("requires a separate confirmation and preserves large versions exactly", async () => {
  const onConfirm = vi.fn();
  render(<TitleApplyConfirmation productKey="controlled-product" baseVersion="9007199254740993" revision="17" pending={false} onCancel={vi.fn()} onConfirm={onConfirm} />);
  expect(onConfirm).not.toHaveBeenCalled();
  expect(screen.getByRole("dialog", { name: "应用到标准商品" })).toBeVisible();
  expect(screen.getByText("9007199254740993")).toBeVisible();
  expect(screen.getByText(/仅修改标题/)).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "确认应用" }));
  expect(onConfirm).toHaveBeenCalledOnce();
});

it("disables confirmation while a write is pending and never replays on rerender", async () => {
  const onConfirm = vi.fn();
  const props = { productKey: "controlled-product", baseVersion: "1", revision: "2", pending: true, onCancel: vi.fn(), onConfirm };
  const view = render(<TitleApplyConfirmation {...props} />);
  await userEvent.click(screen.getByRole("button", { name: "确认应用" }));
  view.rerender(<TitleApplyConfirmation {...props} />);
  expect(onConfirm).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: "确认应用" })).toBeDisabled();
});

it("cancel is an explicit dismissal and never applies", async () => {
  const onConfirm = vi.fn(), onCancel = vi.fn();
  render(<TitleApplyConfirmation productKey="controlled-product" baseVersion="1" revision="2" pending={false} onCancel={onCancel} onConfirm={onConfirm} />);
  await userEvent.click(screen.getByRole("button", { name: "返回审核" }));
  expect(onCancel).toHaveBeenCalledOnce();
  expect(onConfirm).not.toHaveBeenCalled();
});

it("editing preserves exact text and saving only asks for a new pending revision", async () => {
  const onSave = vi.fn();
  render(<TitleEditor title="原始标题" pending={false} onCancel={vi.fn()} onSave={onSave} />);
  const input = screen.getByRole("textbox", { name: "编辑标题" });
  await userEvent.clear(input);
  await userEvent.type(input, "  修改后的标题  ");
  expect(screen.getByText(/保存后旧批准失效/)).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "保存为待审核提案" }));
  expect(onSave).toHaveBeenCalledExactlyOnceWith("  修改后的标题  ");
  expect(screen.queryByRole("button", { name: /接受|应用/ })).not.toBeInTheDocument();
});

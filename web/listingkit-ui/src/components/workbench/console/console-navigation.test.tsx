import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it } from "vitest";
import { ConsoleNavigation } from "./console-navigation";

afterEach(cleanup);

it("reveals the selected account page after navigation from another module", async () => {
  const view = render(<ConsoleNavigation pathname="/workbench/stores" ariaLabel="主导航" />);
  view.rerender(<ConsoleNavigation pathname="/workbench/account/organization/members" ariaLabel="主导航" />);
  expect(screen.getByRole("link", { name: "成员与权限" })).toHaveAttribute("aria-current", "page");
  await userEvent.click(screen.getByRole("button", { name: "收起企业空间" }));
  expect(screen.queryByRole("link", { name: "成员与权限" })).not.toBeInTheDocument();
  view.rerender(<ConsoleNavigation pathname="/workbench/account/organization/audit" ariaLabel="主导航" />);
  expect(screen.getByRole("link", { name: "操作记录" })).toHaveAttribute("aria-current", "page");
  view.rerender(<ConsoleNavigation pathname="/workbench/account/organization/members" ariaLabel="主导航" />);
  expect(screen.getByRole("link", { name: "成员与权限" })).toHaveAttribute("aria-current", "page");
});

import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import { AccountShell } from "./account-shell";

afterEach(cleanup);
it.each([
  ["/workbench/account/organization/members", "成员与权限"],
  ["/workbench/account/organization/resources", "资源与额度"],
  ["/workbench/account/organization/audit", "操作记录"],
  ["/workbench/account/organization/resources/source-accounts", "源账号"],
])("provides the canonical account breadcrumb for %s", (pathname, title) => {
  render(<AccountShell pathname={pathname} title={title}><p>真实内容</p></AccountShell>);
  const breadcrumb = within(screen.getByRole("navigation", { name: "面包屑" }));
  expect(breadcrumb.getByRole("link", { name: "我的账户" })).toHaveAttribute("href", "/workbench/account");
  expect(breadcrumb.getByRole("link", { name: "企业空间" })).toHaveAttribute("href", "/workbench/account/organization");
  expect(breadcrumb.getByText(title)).toHaveAttribute("aria-current", "page");
  expect(screen.getByRole("heading", { level: 1, name: title })).toBeVisible();
});

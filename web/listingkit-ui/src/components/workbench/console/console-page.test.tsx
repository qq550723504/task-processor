import { render, screen } from "@testing-library/react";
import { expect, it } from "vitest";
import { ConsolePage, ConsoleState } from "./console-page";

it("composes one heading, a named toolbar, contextual breadcrumbs and a distinct unavailable state", () => {
  render(<ConsolePage title="我的店铺" description="已授权的本地店铺" breadcrumbs={[{ label: "店铺中心" }, { label: "我的店铺" }]} actions={<button>真实操作</button>}><ConsoleState kind="unavailable" title="暂未启用">接口尚未接入</ConsoleState></ConsolePage>);
  expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
  expect(screen.getByRole("navigation", { name: "面包屑" })).toHaveTextContent("店铺中心");
  expect(screen.getByRole("group", { name: "页面操作" })).toHaveTextContent("真实操作");
  expect(screen.getByRole("status")).toHaveTextContent("暂未启用");
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

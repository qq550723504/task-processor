import { describe, expect, it } from "vitest";
import { consoleNavigation, findConsoleRoute } from "./console-navigation";

describe("Figma Console navigation contract", () => {
  it("connects the delivered members page without activating pending sibling leaves", () => {
    const route = findConsoleRoute("/workbench/account/organization/members");
    expect(route?.node.availability).toBe("connected");
    expect(route?.trail.map(node => node.label)).toEqual(["我的账户", "企业空间", "成员与权限"]);
    for (const path of ["organization/resources", "organization/audit", "referrals"]) {
      expect(findConsoleRoute(`/workbench/account/${path}`)?.node.availability).toBe("unavailable");
    }
  });
  it("contains only the ten visible primary modules, never archived or backend names", () => {
    expect(consoleNavigation.map((item) => item.label)).toEqual(["运营驾驶舱", "AI工作台", "供应市场", "智能市场", "工具市场", "生态服务", "数据服务", "店铺中心", "套餐与权益", "我的账户"]);
  });
  it("maps exact descendants and internal diagnostics without inventing Store ownership", () => {
    expect(findConsoleRoute("/workbench/stores/123")?.trail.map((item) => item.label)).toEqual(["店铺中心", "我的店铺", "店铺详情"]);
    expect(findConsoleRoute("/workbench/ai/chat/recent")?.trail.map((item) => item.label)).toEqual(["AI工作台", "硕米Chat", "最近会话"]);
    expect(findConsoleRoute("/workbench/stores-evil")).toBeUndefined();
    expect(findConsoleRoute("/workbench/shein-records/123/diagnostic")?.trail.map((item) => item.label)).toEqual(["SHEIN 资料诊断"]);
  });
  it("distinguishes unimplemented functions from a live page whose BFF still authorizes", () => {
    expect(findConsoleRoute("/workbench/account")?.node.availability).toBe("connected");
    expect(findConsoleRoute("/workbench/ai/chat")?.node.availability).toBe("unavailable");
    expect(findConsoleRoute("/workbench/stores")?.node.availability).toBe("connected");
    expect(findConsoleRoute("/workbench/store-products")?.node.availability).toBe("unavailable");
    expect(findConsoleRoute("/workbench/ai/tasks/completed")?.node.availability).toBe("connected");
    expect(findConsoleRoute("/workbench/ai/tasks/running")?.node.availability).toBe("unavailable");
  });
});

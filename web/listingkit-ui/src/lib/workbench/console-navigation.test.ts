import { describe, expect, it } from "vitest";
import { consoleNavigation, findConsoleRoute } from "./console-navigation";

describe("Figma Console navigation contract", () => {
  it("keeps profile, resources and audit connected together while undelivered leaves remain pending", () => {
    for (const path of ["/workbench/account/profile", "/workbench/account/organization/resources", "/workbench/account/organization/audit"])
      expect(findConsoleRoute(path)?.node.availability).toBe("connected");
    for (const path of ["/workbench/account/organization/members", "/workbench/account/referrals"])
      expect(findConsoleRoute(path)?.node.availability).toBe("unavailable");
  });
  it("exposes the bounded audit page without activating other pending account leaves", () => {
    const route = findConsoleRoute("/workbench/account/organization/audit");
    expect(route?.node.availability).toBe("connected");
    expect(route?.trail.map(node => node.label)).toEqual(["我的账户", "企业空间", "操作记录"]);
    expect(route?.node.children).toBeUndefined();
    expect(findConsoleRoute("/workbench/account/organization/members")?.node.availability).toBe("unavailable");
  });
  it("exposes the resource page and its existing source-account breadcrumb while members remain pending", () => {
    const path = "/workbench/account/organization/resources";
    expect(findConsoleRoute(path)?.node.availability).toBe("connected");
    expect(findConsoleRoute(`${path}/source-accounts`)?.trail.map(node => node.label)).toEqual(["我的账户", "企业空间", "资源与额度", "源账号"]);
    expect(findConsoleRoute(`${path}/source-accounts`)?.node.availability).toBe("connected");
    expect(findConsoleRoute(path)?.node.children).toBeUndefined();
    for (const leaf of ["members"]) expect(findConsoleRoute(`/workbench/account/organization/${leaf}`)?.node.availability).toBe("unavailable");
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

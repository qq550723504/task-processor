import { describe, expect, it } from "vitest";
import { consoleNavigation, findConsoleRoute } from "./console-navigation";

describe("Figma Console navigation contract", () => {
  it("models every My Account entry as the Figma three-level hierarchy", () => {
    const account = consoleNavigation.find((item) => item.label === "我的账户");
    expect(account?.children?.map((item) => item.label)).toEqual(["账户资料", "企业空间", "推广与收益"]);
    expect(account?.children?.map((item) => item.children?.map((child) => child.label))).toEqual([
      ["账户设置", "经营画像", "认证信息"],
      ["成员与权限", "资源与额度", "操作记录"],
      ["推广中心", "收益明细", "提现管理", "推广规则"],
    ]);
  });

  it("resolves every My Account third-level page to its full breadcrumb", () => {
    const pages = [
      ["/workbench/account/profile/settings", ["我的账户", "账户资料", "账户设置"]],
      ["/workbench/account/profile/business", ["我的账户", "账户资料", "经营画像"]],
      ["/workbench/account/profile/verification", ["我的账户", "账户资料", "认证信息"]],
      ["/workbench/account/organization/members", ["我的账户", "企业空间", "成员与权限"]],
      ["/workbench/account/organization/resources", ["我的账户", "企业空间", "资源与额度"]],
      ["/workbench/account/organization/audit", ["我的账户", "企业空间", "操作记录"]],
      ["/workbench/account/referrals/center", ["我的账户", "推广与收益", "推广中心"]],
      ["/workbench/account/referrals/earnings", ["我的账户", "推广与收益", "收益明细"]],
      ["/workbench/account/referrals/withdrawals", ["我的账户", "推广与收益", "提现管理"]],
      ["/workbench/account/referrals/rules", ["我的账户", "推广与收益", "推广规则"]],
    ] as const;
    for (const [path, trail] of pages) {
      expect(findConsoleRoute(path)?.trail.map((item) => item.label), path).toEqual(trail);
      expect(findConsoleRoute(path)?.node.availability, path).toBe("connected");
    }
  });

  it("connects all delivered account leaves", () => {
    for (const leaf of ["members", "resources", "audit"])
      expect(findConsoleRoute(`/workbench/account/organization/${leaf}`)?.node.availability).toBe("connected");
    expect(findConsoleRoute("/workbench/account/referrals")?.node.availability).toBe("connected");
  });
  it("connects the delivered members page with its sibling leaves", () => {
    const route = findConsoleRoute("/workbench/account/organization/members");
    expect(route?.node.availability).toBe("connected");
    expect(route?.trail.map(node => node.label)).toEqual(["我的账户", "企业空间", "成员与权限"]);
    expect(findConsoleRoute("/workbench/account/referrals")?.node.availability).toBe("connected");
  });
  it("keeps the profile and enterprise leaves connected together", () => {
    for (const path of ["/workbench/account/profile", "/workbench/account/organization/resources", "/workbench/account/organization/audit"])
      expect(findConsoleRoute(path)?.node.availability).toBe("connected");
    for (const path of ["/workbench/account/referrals", "/workbench/account/organization/members"])
      expect(findConsoleRoute(path)?.node.availability).toBe("connected");
  });
  it("connects the delivered personal referral page and derives only its completion breadcrumb", () => {
    const route = findConsoleRoute("/workbench/account/referrals");
    expect(route?.node.availability).toBe("connected");
    expect(route?.trail.map(node => node.label)).toEqual(["我的账户", "推广与收益"]);
    expect(findConsoleRoute("/workbench/account/referrals/complete")?.trail.map(node => node.label)).toEqual(["我的账户", "推广与收益", "完成注册"]);
    expect(findConsoleRoute("/workbench/account/referrals/complete")?.node.availability).toBe("connected");
    expect(findConsoleRoute("/workbench/account/referrals/other")).toBeUndefined();
  });
  it("exposes the bounded audit page alongside the other account leaves", () => {
    const route = findConsoleRoute("/workbench/account/organization/audit");
    expect(route?.node.availability).toBe("connected");
    expect(route?.trail.map(node => node.label)).toEqual(["我的账户", "企业空间", "操作记录"]);
    expect(route?.node.children).toBeUndefined();
    expect(findConsoleRoute("/workbench/account/organization/members")?.node.availability).toBe("connected");
  });
  it("exposes the resource page and its existing source-account breadcrumb", () => {
    const path = "/workbench/account/organization/resources";
    expect(findConsoleRoute(path)?.node.availability).toBe("connected");
    expect(findConsoleRoute(`${path}/source-accounts`)?.trail.map(node => node.label)).toEqual(["我的账户", "企业空间", "资源与额度", "源账号"]);
    expect(findConsoleRoute(`${path}/source-accounts`)?.node.availability).toBe("connected");
    expect(findConsoleRoute(path)?.node.children).toBeUndefined();
    expect(findConsoleRoute("/workbench/account/referrals")?.node.availability).toBe("connected");
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

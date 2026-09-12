// Visible, non-archived Figma 31:463 navigation. A route is not a permission grant.
export type ConsoleNavNode = { label: string; href: string; availability: "connected" | "unavailable"; children?: readonly ConsoleNavNode[] };
const pending = (label: string, href: string, children?: readonly ConsoleNavNode[]): ConsoleNavNode => ({ label, href: `/workbench/${href}`, availability: "unavailable", children });
export const consoleNavigation: readonly ConsoleNavNode[] = [
  { label: "运营驾驶舱", href: "/workbench", availability: "unavailable", children: [pending("目标管理", "overview/goals"), pending("店铺矩阵", "overview/stores"), pending("经营预警", "overview/alerts"), pending("经营建议", "overview/advice")] },
  pending("AI工作台", "ai", [
    pending("硕米Chat", "ai/chat", [pending("新建会话", "ai/chat/new"), pending("最近会话", "ai/chat/recent"), pending("收藏会话", "ai/chat/saved")]),
    { label: "任务中心", href: "/workbench/ai/tasks", availability: "connected", children: [pending("进行中", "ai/tasks/running"), { label: "待确认", href: "/workbench/ai/tasks/pending", availability: "connected" }, { label: "已完成", href: "/workbench/ai/tasks/completed", availability: "connected" }, pending("异常任务", "ai/tasks/errors")] },
    pending("项目中心", "ai/projects"), pending("知识库", "ai/knowledge"), pending("我的报告", "ai/reports"),
  ]),
  pending("供应市场", "supply", [pending("硕米自营", "supply/official"), pending("硕米优选", "supply/selected"), pending("货盘集成", "supply/catalogs"), pending("我的供应链", "supply/mine"), pending("优选申请", "supply/applications")]),
  pending("智能市场", "agents", [pending("智能体市场", "agents/market"), pending("我的智能体", "agents/mine"), pending("智能体定制", "agents/custom")]),
  pending("工具市场", "tools", [pending("官方工具", "tools/official"), pending("我的工具", "tools/mine"), pending("工具定制", "tools/custom")]),
  pending("生态服务", "services", [pending("服务市场", "services/market"), pending("我的服务", "services/mine"), pending("申请加入", "services/join")]),
  pending("数据服务", "data", [pending("数据市场", "data/market"), pending("API管理", "data/api"), pending("我的数据", "data/mine")]),
  pending("店铺中心", "store-center", [{ label: "我的店铺", href: "/workbench/stores", availability: "connected" }, pending("店铺商品", "store-products"), pending("订单履约", "store-orders")]),
  pending("套餐与权益", "plans", [{ label: "套餐方案", href: "/workbench/plans/options", availability: "connected" }, { label: "我的权益", href: "/workbench/plans/entitlements", availability: "connected" }, pending("用量明细", "plans/usage"), pending("充值中心", "plans/top-up"), pending("账单与订单", "plans/orders")]),
  { label: "我的账户", href: "/workbench/account", availability: "connected", children: [{ label: "账户资料", href: "/workbench/account/profile", availability: "connected" }, { label: "企业空间", href: "/workbench/account/organization", availability: "connected", children: [{ label: "成员与权限", href: "/workbench/account/organization/members", availability: "connected" }, pending("资源与额度", "account/organization/resources"), pending("操作记录", "account/organization/audit")] }, pending("推广与收益", "account/referrals")] },
];

export type ConsoleRoute = { node: ConsoleNavNode; trail: readonly ConsoleNavNode[] };
export function findConsoleRoute(pathname: string): ConsoleRoute | undefined {
  function visit(nodes: readonly ConsoleNavNode[], parents: readonly ConsoleNavNode[]): ConsoleRoute | undefined {
    for (const node of nodes) {
      const trail = [...parents, node];
      if (node.href === pathname) return { node, trail };
      const child = node.children && visit(node.children, trail);
      if (child) return child;
    }
  }
  const exact = visit(consoleNavigation, []);
  if (exact) return exact;
  if (pathname === "/workbench/account/organization/resources/source-accounts") {
    const parent = findConsoleRoute("/workbench/account/organization/resources")!;
    const node: ConsoleNavNode = { label: "源账号", href: pathname, availability: parent.node.availability };
    return { node, trail: [...parent.trail, node] };
  }
  if (/^\/workbench\/stores\/[^/]+$/.test(pathname)) {
    const parent = findConsoleRoute("/workbench/stores")!;
    const node: ConsoleNavNode = { label: pathname.endsWith("/new") ? "新建店铺" : "店铺详情", href: pathname, availability: "connected" };
    return { node, trail: [...parent.trail, node] };
  }
  if (/^\/workbench\/shein-records\/[^/]+\/diagnostic$/.test(pathname)) {
    const node: ConsoleNavNode = { label: "SHEIN 资料诊断", href: pathname, availability: "connected" };
    return { node, trail: [node] };
  }
}

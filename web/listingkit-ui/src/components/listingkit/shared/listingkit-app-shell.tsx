"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import {
  Boxes,
  ChevronRight,
  ClipboardList,
  Database,
  FileCog,
  GalleryHorizontal,
  Home,
  KeyRound,
  LayoutDashboard,
  ListChecks,
  LogOut,
  PackageCheck,
  PackagePlus,
  PanelTop,
  Timer,
  Settings,
  ShieldAlert,
  ShoppingBag,
  SlidersHorizontal,
  Store,
  Tags,
  UserCog,
  type LucideIcon,
} from "lucide-react";

import { ThemeToggleButton } from "@/components/listingkit/shared/theme-toggle-button";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  AppUpdateBanner,
  resolveAppUpdatePollIntervalMs,
} from "@/components/listingkit/shared/app-update-banner";
import {
  type ZitadelClientIdentity,
  useZitadelIdentity,
} from "@/components/providers/zitadel-auth-gate";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
  useSidebar,
} from "@/components/ui/sidebar";

type NavItem = {
  label: string;
  href: string;
  icon: LucideIcon;
  match: "exact" | "prefix";
  requiresIdentity?: boolean;
  requiredRoles?: readonly string[];
};

type ExternalNavItem = {
  label: string;
  href: string;
  icon: LucideIcon;
  external: true;
  requiresIdentity?: boolean;
  requiredRoles?: readonly string[];
};

type NavSection = {
  label: string;
  icon: LucideIcon;
  children: readonly NavTreeItem[];
  requiresIdentity?: boolean;
  requiredRoles?: readonly string[];
};

type NavTreeItem = NavItem | ExternalNavItem | NavSection;

const PRIMARY_NAV_ITEMS = [
  { label: "首页", href: "/", icon: Home, match: "exact" },
  { label: "新建任务", href: "/listing-kits/new", icon: PackagePlus, match: "exact" },
  { label: "POD", href: "/listing-kits/sds", icon: Boxes, match: "prefix" },
  {
    label: "款式图库",
    href: "/listing-kits/style-gallery",
    icon: GalleryHorizontal,
    match: "prefix",
  },
  {
    label: "标准商品",
    href: "/listing-kits/canonical-products",
    icon: PackageCheck,
    match: "prefix",
  },
  { label: "任务列表", href: "/listing-kits", icon: ClipboardList, match: "exact" },
] as const satisfies readonly NavItem[];

const MENU_ROLES = {
  operator: ["listingkit_operator", "listingkit_admin", "platform_admin", "admin"],
  admin: ["listingkit_admin", "platform_admin", "admin"],
} as const;

const ZITADEL_CONSOLE_URL =
  process.env.NEXT_PUBLIC_ZITADEL_CONSOLE_URL?.trim() || "";
const APP_UPDATE_POLL_INTERVAL_MS = resolveAppUpdatePollIntervalMs(
  process.env.NEXT_PUBLIC_LISTINGKIT_UPDATE_POLL_INTERVAL_MS,
);

const ADMIN_NAV_ITEMS = [
  {
    label: "业务运营",
    icon: Store,
    requiresIdentity: true,
    children: [
      { label: "我的店铺配置", href: "/listing-kits/stores", icon: Store, match: "prefix" },
      {
        label: "SHEIN 活动报名",
        href: "/listing-kits/shein-enrollment",
        icon: ShoppingBag,
        match: "prefix",
      },
      {
        label: "SHEIN 同步商品",
        href: "/listing-kits/shein-products",
        icon: PackageCheck,
        match: "prefix",
      },
      {
        label: "我的上架统计",
        href: "/listing-kits/store-statistics",
        icon: LayoutDashboard,
        match: "prefix",
      },
      {
        label: "平台店铺管理",
        href: "/listing-kits/admin/stores",
        icon: Store,
        match: "prefix",
        requiredRoles: MENU_ROLES.operator,
      },
      {
        label: "上架统计",
        href: "/listing-kits/admin/store-statistics",
        icon: LayoutDashboard,
        match: "prefix",
        requiredRoles: MENU_ROLES.operator,
      },
    ],
  },
  {
    label: "调度与导入",
    icon: Timer,
    requiredRoles: MENU_ROLES.operator,
    children: [
      {
        label: "定时任务配置",
        href: "/listing-kits/admin/scheduled-task-configs",
        icon: Timer,
        match: "prefix",
        requiredRoles: MENU_ROLES.admin,
      },
      {
        label: "调度事件",
        href: "/listing-kits/admin/dispatch-events",
        icon: ListChecks,
        match: "prefix",
      },
      {
        label: "任务导入",
        href: "/listing-kits/admin/import-tasks",
        icon: FileCog,
        match: "prefix",
      },
    ],
  },
  {
    label: "数据字典",
    icon: Database,
    requiredRoles: MENU_ROLES.operator,
    children: [
      { label: "分类", href: "/listing-kits/admin/categories", icon: Tags, match: "prefix" },
      {
        label: "商品数据",
        href: "/listing-kits/admin/product-data",
        icon: Database,
        match: "prefix",
      },
      {
        label: "导入映射",
        href: "/listing-kits/admin/product-import-mappings",
        icon: ListChecks,
        match: "prefix",
      },
    ],
  },
  {
    label: "策略规则",
    icon: SlidersHorizontal,
    requiredRoles: MENU_ROLES.admin,
    children: [
      {
        label: "筛选规则",
        href: "/listing-kits/admin/filter-rules",
        icon: SlidersHorizontal,
        match: "prefix",
      },
      {
        label: "运营策略",
        href: "/listing-kits/admin/operation-strategies",
        icon: SlidersHorizontal,
        match: "prefix",
      },
      {
        label: "利润规则",
        href: "/listing-kits/admin/profit-rules",
        icon: PanelTop,
        match: "prefix",
      },
      {
        label: "核价规则",
        href: "/listing-kits/admin/pricing-rules",
        icon: PanelTop,
        match: "prefix",
      },
      {
        label: "敏感词",
        href: "/listing-kits/admin/sensitive-words",
        icon: ShieldAlert,
        match: "prefix",
      },
      {
        label: "生成禁用主题",
        href: "/listing-kits/admin/generation-topic-policies",
        icon: ShieldAlert,
        match: "prefix",
      },
    ],
  },
  {
    label: "账号与系统",
    icon: Settings,
    children: [
      {
        label: "SHEIN 登录",
        href: "/listing-kits/shein-login",
        icon: KeyRound,
        match: "prefix",
        requiredRoles: MENU_ROLES.admin,
      },
      {
        label: "SDS 登录",
        href: "/listing-kits/sds-login",
        icon: KeyRound,
        match: "prefix",
        requiredRoles: MENU_ROLES.admin,
      },
      {
        label: "提示词",
        href: "/listing-kits/prompts",
        icon: FileCog,
        match: "prefix",
        requiredRoles: MENU_ROLES.admin,
      },
      { label: "设置", href: "/listing-kits/settings", icon: Settings, match: "prefix" },
      {
        label: "当前租户订阅",
        href: "/listing-kits/subscription",
        icon: PackageCheck,
        match: "prefix",
      },
      {
        label: "租户订阅管理",
        href: "/listing-kits/platform/subscriptions",
        icon: UserCog,
        match: "prefix",
        requiredRoles: MENU_ROLES.admin,
      },
      {
        label: "套餐管理",
        href: "/listing-kits/platform/subscription-plans",
        icon: PanelTop,
        match: "prefix",
        requiredRoles: MENU_ROLES.admin,
      },
      ...(ZITADEL_CONSOLE_URL
        ? [{
            label: "用户管理",
            href: ZITADEL_CONSOLE_URL,
            icon: UserCog,
            external: true as const,
            requiredRoles: MENU_ROLES.admin,
          }]
        : []),
    ],
  },
] as const satisfies readonly NavSection[];

const NAV_GROUPS = [
  { label: "主流程", items: PRIMARY_NAV_ITEMS },
  { label: "管理后台", items: ADMIN_NAV_ITEMS },
] as const satisfies readonly { label: string; items: readonly NavTreeItem[] }[];

const APP_RAIL_CLASS = "mx-auto w-full max-w-[1600px] px-4 sm:px-6 lg:px-8";

function isActiveNavItem(pathname: string, item: NavItem) {
  if (item.match === "prefix") {
    return pathname === item.href || pathname.startsWith(`${item.href}/`);
  }
  return pathname === item.href;
}

function isNavItem(item: NavTreeItem): item is NavItem {
  return "match" in item;
}

function isExternalNavItem(item: NavTreeItem): item is ExternalNavItem {
  return "external" in item;
}

function isActiveNavTreeItem(pathname: string, item: NavTreeItem): boolean {
  if (isNavItem(item)) {
    return isActiveNavItem(pathname, item);
  }
  if (isExternalNavItem(item)) {
    return false;
  }
  return item.children.some((child) => isActiveNavTreeItem(pathname, child));
}

function hasAnyRole(
  requiredRoles: readonly string[] | undefined,
  identity: ZitadelClientIdentity | null | undefined,
) {
  if (!requiredRoles?.length) {
    return true;
  }
  const roles = identity?.roles ?? [];
  return requiredRoles.some((role) => roles.includes(role));
}

function canAccessNavTreeItem(
  item: NavTreeItem,
  identity: ZitadelClientIdentity | null | undefined,
): boolean {
  if (item.requiresIdentity && !identity) {
    return false;
  }
  if (!hasAnyRole(item.requiredRoles, identity)) {
    return false;
  }
  if (isNavItem(item) || isExternalNavItem(item)) {
    return true;
  }
  return item.children.some((child) => canAccessNavTreeItem(child, identity));
}

function filterNavTreeItem(
  item: NavTreeItem,
  identity: ZitadelClientIdentity | null | undefined,
): NavTreeItem | null {
  if (!canAccessNavTreeItem(item, identity)) {
    return null;
  }
  if (isNavItem(item) || isExternalNavItem(item)) {
    return item;
  }
  const children = item.children
    .map((child) => filterNavTreeItem(child, identity))
    .filter((child): child is NavTreeItem => child !== null);
  if (children.length === 0) {
    return null;
  }
  return { ...item, children };
}

function NavLink({
  item,
  pathname,
}: {
  item: NavItem;
  pathname: string;
}) {
  const active = isActiveNavItem(pathname, item);
  const Icon = item.icon;

  return (
    <SidebarMenuItem>
      <SidebarMenuButton asChild isActive={active}>
        <Link href={item.href} aria-current={active ? "page" : undefined}>
          <Icon data-icon="inline-start" />
          <span>{item.label}</span>
        </Link>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

function ExternalNavLink({ item }: { item: ExternalNavItem }) {
  const Icon = item.icon;

  return (
    <SidebarMenuItem>
      <SidebarMenuButton asChild>
        <a href={item.href} target="_blank" rel="noreferrer">
          <Icon data-icon="inline-start" />
          <span>{item.label}</span>
        </a>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

function NavSectionItem({
  item,
  pathname,
}: {
  item: NavSection;
  pathname: string;
}) {
  const active = isActiveNavTreeItem(pathname, item);
  const [open, setOpen] = useState(active);
  const { open: sidebarOpen } = useSidebar();
  const Icon = item.icon;

  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        aria-expanded={open}
        isActive={active}
        onClick={() => setOpen((current) => !current)}
        type="button"
      >
        <Icon data-icon="inline-start" />
        <span>{item.label}</span>
        <ChevronRight
          aria-hidden="true"
          className={`ml-auto size-4 shrink-0 transition-transform group-data-[state=collapsed]/sidebar:sr-only ${
            open ? "rotate-90" : ""
          }`}
        />
      </SidebarMenuButton>
      {open && sidebarOpen ? (
        <ul className="ml-4 mt-1 flex min-w-0 flex-col gap-1 border-l border-sidebar-border pl-2 group-data-[state=collapsed]/sidebar:hidden">
          {item.children.map((child) => (
            <NavTreeNode key={child.label} item={child} pathname={pathname} />
          ))}
        </ul>
      ) : null}
    </SidebarMenuItem>
  );
}

function NavTreeNode({
  item,
  pathname,
}: {
  item: NavTreeItem;
  pathname: string;
}) {
  if (isExternalNavItem(item)) {
    return <ExternalNavLink item={item} />;
  }
  if (isNavItem(item)) {
    return <NavLink item={item} pathname={pathname} />;
  }
  return <NavSectionItem item={item} pathname={pathname} />;
}

function summarizeIdentity(identity: ZitadelClientIdentity | null | undefined) {
  const username = identity?.username?.trim();
  if (username) {
    return username;
  }
  const userId =
    identity?.userId === undefined || identity.userId === null
      ? ""
      : String(identity.userId).trim();
  if (userId) {
    return userId;
  }
  return "账号";
}

function summarizeTenant(identity: ZitadelClientIdentity | null | undefined) {
  const tenantId =
    identity?.tenantId === undefined || identity.tenantId === null
      ? ""
      : String(identity.tenantId).trim();
  return tenantId || "未识别租户";
}

export function ListingKitAppShell({
  children,
  identity: identityOverride,
}: Readonly<{
  children: React.ReactNode;
  identity?: ZitadelClientIdentity | null;
}>) {
  const pathname = usePathname();
  const identityFromContext = useZitadelIdentity();
  const identity = identityOverride ?? identityFromContext;
  const [accountMenuOpen, setAccountMenuOpen] = useState(false);
  const navGroups = NAV_GROUPS.map((group) => ({
    ...group,
    items: group.items
      .map((item) => filterNavTreeItem(item, identity))
      .filter((item): item is NavTreeItem => item !== null),
  })).filter((group) => group.items.length > 0);

  return (
    <SidebarProvider>
      <Sidebar aria-label="ListingKit">
        <SidebarHeader>
          <div className="flex min-w-0 items-center gap-2 px-2 py-1 group-data-[state=collapsed]/sidebar:justify-center group-data-[state=collapsed]/sidebar:px-0">
            <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-primary text-primary-foreground">
              <ShoppingBag data-icon="inline-start" />
            </div>
            <div className="min-w-0 group-data-[state=collapsed]/sidebar:sr-only">
              <p className="truncate text-sm font-semibold text-foreground">
                ListingKit
              </p>
              <p className="truncate text-xs text-muted-foreground">
                源信息 -&gt; 标准商品 -&gt; 平台资料
              </p>
            </div>
            <SidebarTrigger className="ml-auto hidden md:inline-flex group-data-[state=collapsed]/sidebar:ml-0" />
          </div>
        </SidebarHeader>

        <SidebarContent>
          <nav aria-label="ListingKit 侧边栏导航" className="flex flex-col gap-2">
            {navGroups.map((group) => (
              <SidebarGroup key={group.label}>
                <SidebarGroupLabel>{group.label}</SidebarGroupLabel>
                <SidebarGroupContent>
                  <SidebarMenu>
                    {group.items.map((item) => (
                      <NavTreeNode key={item.label} item={item} pathname={pathname} />
                    ))}
                  </SidebarMenu>
                </SidebarGroupContent>
              </SidebarGroup>
            ))}
          </nav>
        </SidebarContent>

        <SidebarRail />
      </Sidebar>

      <SidebarInset>
        <div className={APP_RAIL_CLASS}>
          <AppUpdateBanner pollIntervalMs={APP_UPDATE_POLL_INTERVAL_MS} />
          <div className="flex min-h-14 items-center justify-between gap-3 border-b border-border py-3">
            <div className="flex min-w-0 items-center gap-3">
              <SidebarTrigger className="md:hidden" />
              <p className="min-w-0 text-sm text-muted-foreground">
                当前页面
                <Badge className="ml-2 rounded-full px-2.5 py-1 font-mono text-xs" variant="neutral">
                  {pathname}
                </Badge>
              </p>
            </div>
            <div className="flex items-center gap-2">
              <ThemeToggleButton />
              <div className="relative">
                <Button
                  aria-expanded={accountMenuOpen}
                  aria-haspopup="menu"
                  className="h-auto min-w-[200px] justify-between rounded-xl px-3 py-2"
                  onClick={() => setAccountMenuOpen((current) => !current)}
                  size="sm"
                  variant="outline"
                >
                  <span className="flex min-w-0 flex-col items-start text-left">
                    <span className="truncate text-sm font-medium text-foreground">
                      {summarizeIdentity(identity)}
                    </span>
                    <span className="truncate text-xs text-muted-foreground">
                      {summarizeTenant(identity)}
                    </span>
                  </span>
                  <UserCog className="size-4 shrink-0 text-muted-foreground" />
                </Button>
                {accountMenuOpen ? (
                  <div
                    className="absolute right-0 top-full z-50 mt-2 w-[280px] rounded-2xl border border-border bg-background p-3 shadow-xl"
                    role="menu"
                  >
                    <div className="space-y-3">
                      <div className="space-y-1">
                        <p className="text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
                          当前账号
                        </p>
                        <p className="text-sm font-medium text-foreground">{summarizeIdentity(identity)}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
                          当前租户
                        </p>
                        <p className="break-all text-sm text-foreground">{summarizeTenant(identity)}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
                          角色
                        </p>
                        <p className="break-all text-sm text-foreground">
                          {identity?.roles?.length ? identity.roles.join(", ") : "未识别角色"}
                        </p>
                      </div>
                      <Button asChild className="w-full justify-center" size="sm" variant="outline">
                        <a href="/api/zitadel-auth/logout">
                          <LogOut data-icon="inline-start" />
                          <span>退出登录</span>
                        </a>
                      </Button>
                    </div>
                  </div>
                ) : null}
              </div>
            </div>
          </div>
        </div>
        <main className={`${APP_RAIL_CLASS} flex min-h-screen flex-col py-2`}>
          {children}
        </main>
      </SidebarInset>
    </SidebarProvider>
  );
}

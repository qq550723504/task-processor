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
  Settings,
  ShieldAlert,
  ShoppingBag,
  SlidersHorizontal,
  Store,
  Tags,
  UserCog,
  type LucideIcon,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import {
  type ZitadelClientIdentity,
  useZitadelIdentity,
} from "@/components/providers/zitadel-auth-gate";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
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
    label: "SHEIN 工作台",
    href: "/listing-kits/shein",
    icon: ShoppingBag,
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
  operator: ["listingkit_admin", "listingkit_operator"],
  admin: ["listingkit_admin"],
} as const;

const ZITADEL_CONSOLE_URL =
  process.env.NEXT_PUBLIC_ZITADEL_CONSOLE_URL?.trim() || "";

const OPERATIONS_NAV_ITEMS = [
  {
    label: "店铺运营",
    icon: Store,
    requiresIdentity: true,
    children: [
      { label: "店铺", href: "/listing-kits/admin/stores", icon: Store, match: "prefix" },
      {
        label: "上架统计",
        href: "/listing-kits/admin/store-statistics",
        icon: LayoutDashboard,
        match: "prefix",
      },
    ],
  },
  {
    label: "数据配置",
    icon: Database,
    requiresIdentity: true,
    children: [
      { label: "分类", href: "/listing-kits/admin/categories", icon: Tags, match: "prefix" },
      {
        label: "任务导入",
        href: "/listing-kits/admin/import-tasks",
        icon: FileCog,
        match: "prefix",
      },
      {
        label: "导入映射",
        href: "/listing-kits/admin/product-import-mappings",
        icon: ListChecks,
        match: "prefix",
      },
      {
        label: "商品数据",
        href: "/listing-kits/admin/product-data",
        icon: Database,
        match: "prefix",
      },
    ],
  },
  {
    label: "规则策略",
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
        label: "运营策略",
        href: "/listing-kits/admin/operation-strategies",
        icon: SlidersHorizontal,
        match: "prefix",
      },
      {
        label: "敏感词",
        href: "/listing-kits/admin/sensitive-words",
        icon: ShieldAlert,
        match: "prefix",
      },
      {
        label: "SHEIN 登录",
        href: "/listing-kits/shein-login",
        icon: KeyRound,
        match: "prefix",
      },
    ],
  },
] as const satisfies readonly NavSection[];

const SYSTEM_NAV_ITEMS = [
  {
    label: "订阅计费",
    icon: PackageCheck,
    requiredRoles: MENU_ROLES.admin,
    children: [
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
      },
      {
        label: "套餐管理",
        href: "/listing-kits/platform/subscription-plans",
        icon: PanelTop,
        match: "prefix",
      },
    ],
  },
  {
    label: "系统配置",
    icon: Settings,
    requiredRoles: MENU_ROLES.admin,
    children: [
      { label: "提示词", href: "/listing-kits/prompts", icon: FileCog, match: "prefix" },
      { label: "设置", href: "/listing-kits/settings", icon: Settings, match: "prefix" },
      ...(ZITADEL_CONSOLE_URL
        ? [{ label: "用户管理", href: ZITADEL_CONSOLE_URL, icon: UserCog, external: true as const }]
        : []),
    ],
  },
] as const satisfies readonly NavSection[];

const NAV_GROUPS = [
  { label: "主流程", items: PRIMARY_NAV_ITEMS },
  { label: "运营管理", items: OPERATIONS_NAV_ITEMS },
  { label: "系统", items: SYSTEM_NAV_ITEMS },
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

export function ListingKitAppShell({
  children,
  identity: identityOverride,
}: Readonly<{
  children: React.ReactNode;
  identity?: ZitadelClientIdentity | null;
}>) {
  const pathname = usePathname();
  useZitadelIdentity();
  void identityOverride;
  const navGroups = NAV_GROUPS;

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

        <SidebarFooter>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton asChild>
                <a href="/api/zitadel-auth/logout">
                  <LogOut data-icon="inline-start" />
                  <span>退出登录</span>
                </a>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
        <SidebarRail />
      </Sidebar>

      <SidebarInset>
        <div className={APP_RAIL_CLASS}>
          <div className="flex min-h-14 items-center gap-3 border-b border-border py-3">
            <SidebarTrigger className="md:hidden" />
            <p className="min-w-0 text-sm text-muted-foreground">
              当前页面
              <Badge className="ml-2 rounded-full px-2.5 py-1 font-mono text-xs" variant="neutral">
                {pathname}
              </Badge>
            </p>
          </div>
        </div>
        <main className={`${APP_RAIL_CLASS} flex min-h-screen flex-col py-2`}>
          {children}
        </main>
      </SidebarInset>
    </SidebarProvider>
  );
}

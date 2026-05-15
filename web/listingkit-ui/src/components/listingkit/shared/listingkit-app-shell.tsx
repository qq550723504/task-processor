"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

type NavItem = {
  label: string;
  href: string;
  match: "exact" | "prefix";
};

const PRIMARY_NAV_ITEMS = [
  { label: "首页", href: "/", match: "exact" },
  { label: "新建任务", href: "/listing-kits/new", match: "exact" },
  { label: "POD", href: "/listing-kits/sds", match: "prefix" },
  { label: "款式图库", href: "/listing-kits/style-gallery", match: "prefix" },
  { label: "SHEIN 工作台", href: "/listing-kits/shein", match: "prefix" },
  {
    label: "标准商品",
    href: "/listing-kits/canonical-products",
    match: "prefix",
  },
  { label: "任务列表", href: "/listing-kits", match: "exact" },
] as const satisfies readonly NavItem[];

const OPERATIONS_NAV_ITEMS = [
  { label: "店铺", href: "/listing-kits/admin/stores", match: "prefix" },
  {
    label: "上架统计",
    href: "/listing-kits/admin/store-statistics",
    match: "prefix",
  },
  { label: "分类", href: "/listing-kits/admin/categories", match: "prefix" },
  {
    label: "任务导入",
    href: "/listing-kits/admin/import-tasks",
    match: "prefix",
  },
  {
    label: "导入映射",
    href: "/listing-kits/admin/product-import-mappings",
    match: "prefix",
  },
  {
    label: "商品数据",
    href: "/listing-kits/admin/product-data",
    match: "prefix",
  },
  {
    label: "筛选规则",
    href: "/listing-kits/admin/filter-rules",
    match: "prefix",
  },
  {
    label: "利润规则",
    href: "/listing-kits/admin/profit-rules",
    match: "prefix",
  },
  {
    label: "核价规则",
    href: "/listing-kits/admin/pricing-rules",
    match: "prefix",
  },
  {
    label: "运营策略",
    href: "/listing-kits/admin/operation-strategies",
    match: "prefix",
  },
  {
    label: "敏感词",
    href: "/listing-kits/admin/sensitive-words",
    match: "prefix",
  },
  { label: "SHEIN 登录", href: "/listing-kits/shein-login", match: "prefix" },
] as const satisfies readonly NavItem[];

const SYSTEM_NAV_ITEMS = [
  { label: "订阅", href: "/listing-kits/subscription", match: "prefix" },
  {
    label: "平台订阅",
    href: "/listing-kits/platform/subscriptions",
    match: "prefix",
  },
  {
    label: "套餐管理",
    href: "/listing-kits/platform/subscription-plans",
    match: "prefix",
  },
  { label: "设置", href: "/listing-kits/settings", match: "prefix" },
] as const satisfies readonly NavItem[];

const ZITADEL_CONSOLE_URL =
  process.env.NEXT_PUBLIC_ZITADEL_CONSOLE_URL ??
  "http://localhost:8080/ui/console";

const NAV_GROUPS = [
  { label: "主流程", items: PRIMARY_NAV_ITEMS },
  { label: "运营管理", items: OPERATIONS_NAV_ITEMS },
  { label: "系统", items: SYSTEM_NAV_ITEMS },
] as const;

const APP_RAIL_CLASS = "mx-auto w-full max-w-[1600px] px-4 sm:px-6 lg:px-8";

function isActiveNavItem(pathname: string, item: NavItem) {
  if (item.match === "prefix") {
    return pathname === item.href || pathname.startsWith(`${item.href}/`);
  }
  return pathname === item.href;
}

function isActiveGroup(pathname: string, group: (typeof NAV_GROUPS)[number]) {
  return group.items.some((item) => isActiveNavItem(pathname, item));
}

function NavLink({
  item,
  pathname,
}: {
  item: NavItem;
  pathname: string;
}) {
  const active = isActiveNavItem(pathname, item);

  return (
    <Link
      href={item.href}
      aria-current={active ? "page" : undefined}
      className={[
        "inline-flex h-9 shrink-0 items-center justify-center rounded-lg border px-3 text-sm font-medium transition",
        active
          ? "border-zinc-950 bg-zinc-950 text-white"
          : "border-zinc-200 bg-white text-zinc-700 hover:border-zinc-300 hover:text-zinc-950",
      ].join(" ")}
    >
      {item.label}
    </Link>
  );
}

function ExternalNavLink({ href, label }: { href: string; label: string }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="inline-flex h-9 shrink-0 items-center justify-center rounded-lg border border-zinc-200 bg-white px-3 text-sm font-medium text-zinc-700 transition hover:border-zinc-300 hover:text-zinc-950"
    >
      {label}
    </a>
  );
}

export function ListingKitAppShell({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const pathname = usePathname();

  return (
    <div className="min-h-full">
      <div className={APP_RAIL_CLASS}>
        <header className="mb-4 rounded-lg border border-zinc-200 bg-white px-5 py-4 shadow-sm">
          <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
            <div className="min-w-0">
              <p className="text-[11px] font-semibold uppercase text-zinc-500">
                ListingKit
              </p>
              <p className="mt-1 text-sm font-medium text-zinc-950">
                源信息 -&gt; 标准商品 -&gt; 平台资料
              </p>
              <p className="text-sm text-zinc-500">
                当前页面
                <span className="ml-2 rounded-full bg-zinc-100 px-2.5 py-1 font-mono text-xs text-zinc-700">
                  {pathname}
                </span>
              </p>
            </div>

            <nav
              aria-label="ListingKit 主导航"
              className="flex w-full flex-col gap-3 xl:max-w-[980px]"
            >
              <span className="text-xs font-semibold text-zinc-500">
                主流程
              </span>
              <div className="flex flex-wrap items-center gap-2">
                {PRIMARY_NAV_ITEMS.map((item) => (
                  <NavLink key={item.href} item={item} pathname={pathname} />
                ))}
                <details className="group relative shrink-0">
                  <summary className="inline-flex h-9 cursor-pointer list-none items-center justify-center rounded-lg border border-zinc-200 bg-white px-3 text-sm font-medium text-zinc-700 transition hover:border-zinc-300 hover:text-zinc-950">
                    更多
                    <span className="ml-1 text-xs text-zinc-400 transition group-open:rotate-180">
                      v
                    </span>
                  </summary>
                  <div className="absolute left-0 z-20 mt-2 grid w-72 gap-3 rounded-lg border border-zinc-200 bg-white p-3 shadow-lg md:left-auto md:right-0">
                    {NAV_GROUPS.slice(1).map((group) => (
                      <div key={group.label} className="grid gap-2">
                        <span className="text-xs font-semibold text-zinc-500">
                          {group.label}
                        </span>
                        <div className="grid grid-cols-2 gap-2">
                          {group.items.map((item) => (
                            <NavLink key={item.href} item={item} pathname={pathname} />
                          ))}
                          {group.label === "系统" ? (
                            <ExternalNavLink
                              href={ZITADEL_CONSOLE_URL}
                              label="用户管理"
                            />
                          ) : null}
                        </div>
                      </div>
                    ))}
                    <div className="grid gap-2 border-t border-zinc-100 pt-3">
                      <Link
                        href="/api/zitadel-auth/logout"
                        className="inline-flex h-9 items-center justify-center rounded-lg border border-zinc-200 bg-white px-3 text-sm font-medium text-zinc-700 transition hover:border-zinc-300 hover:text-zinc-950"
                      >
                        退出登录
                      </Link>
                    </div>
                  </div>
                </details>
              </div>
              {NAV_GROUPS.slice(1).map((group) => {
                if (!isActiveGroup(pathname, group)) {
                  return null;
                }

                return (
                  <div
                    key={group.label}
                    className="flex min-w-0 items-center gap-2 border-t border-zinc-100 pt-3"
                  >
                    <span className="shrink-0 text-xs font-semibold text-zinc-500">
                      {group.label}
                    </span>
                    <div className="flex flex-wrap items-center gap-2">
                      {group.items.map((item) => (
                        <NavLink key={item.href} item={item} pathname={pathname} />
                      ))}
                      {group.label === "系统" ? (
                        <ExternalNavLink
                          href={ZITADEL_CONSOLE_URL}
                          label="用户管理"
                        />
                      ) : null}
                    </div>
                  </div>
                );
              })}
            </nav>
          </div>
        </header>
      </div>
      <main className={`${APP_RAIL_CLASS} flex min-h-screen flex-col py-2`}>
        {children}
      </main>
    </div>
  );
}

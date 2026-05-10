"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const NAV_ITEMS = [
  { label: "首页", href: "/", match: "exact" },
  { label: "新建任务", href: "/listing-kits/new", match: "exact" },
  { label: "SDS 源", href: "/listing-kits/sds", match: "prefix" },
  { label: "款式图库", href: "/listing-kits/style-gallery", match: "prefix" },
  {
    label: "Canonical Products",
    href: "/listing-kits/canonical-products",
    match: "prefix",
  },
  { label: "任务列表", href: "/listing-kits", match: "exact" },
  { label: "设置", href: "/listing-kits/settings", match: "prefix" },
] as const;

const APP_RAIL_CLASS = "mx-auto w-full max-w-[1600px] px-4 sm:px-6 lg:px-8";

function isActiveNavItem(
  pathname: string,
  item: (typeof NAV_ITEMS)[number],
) {
  if (item.match === "prefix") {
    return pathname === item.href || pathname.startsWith(`${item.href}/`);
  }
  return pathname === item.href;
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
                源信息 -&gt; Canonical Product -&gt; 平台资料
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
              className="flex flex-wrap items-center gap-2"
            >
              {NAV_ITEMS.map((item) => {
                const active = isActiveNavItem(pathname, item);

                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    aria-current={active ? "page" : undefined}
                    className={[
                      "inline-flex h-9 items-center justify-center rounded-lg border px-3 text-sm font-medium transition",
                      active
                        ? "border-zinc-950 bg-zinc-950 text-white"
                        : "border-zinc-200 bg-white text-zinc-700 hover:border-zinc-300 hover:text-zinc-950",
                    ].join(" ")}
                  >
                    {item.label}
                  </Link>
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

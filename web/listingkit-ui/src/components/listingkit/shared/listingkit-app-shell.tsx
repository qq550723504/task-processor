"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

export function ListingKitAppShell({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const pathname = usePathname();

  return (
    <div className="min-h-full">
      <header className="mx-auto mb-4 flex w-full max-w-[1600px] flex-wrap items-center justify-between gap-3 rounded-[1.75rem] border border-white/70 bg-white/82 px-5 py-4 shadow-[0_12px_40px_rgba(24,24,27,0.06)] backdrop-blur">
        <div className="min-w-0">
          <p className="text-[11px] font-semibold uppercase tracking-[0.26em] text-zinc-500">
            ListingKit
          </p>
          <div className="mt-1 flex flex-wrap items-center gap-3">
            <Link
              href="/"
              className="inline-flex h-9 items-center justify-center rounded-xl border border-zinc-200 bg-white px-3 text-sm font-medium text-zinc-900 transition hover:bg-zinc-50"
            >
              返回首页
            </Link>
            <p className="text-sm text-zinc-500">
              当前页面
              <span className="ml-2 rounded-full bg-zinc-100 px-2.5 py-1 font-mono text-xs text-zinc-700">
                {pathname}
              </span>
            </p>
          </div>
        </div>
      </header>
      <main className="mx-auto flex min-h-screen w-full max-w-[1600px] flex-col px-4 py-2 sm:px-6 lg:px-8">
        {children}
      </main>
    </div>
  );
}

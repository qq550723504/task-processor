"use client";

import Link from "next/link";
import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";

import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { getTenantListingStores } from "@/lib/api/tenant-stores";
import { useSheinLoginAccounts } from "@/lib/query/use-shein-login";

export function SheinEnrollmentDashboardPage() {
  const stores = useQuery({
    queryKey: ["listingkit", "tenant-stores", "shein-enrollment-dashboard"],
    queryFn: () =>
      getTenantListingStores({
        page: 1,
        page_size: 100,
        platform: "SHEIN",
      }),
  });
  const sheinLoginAccounts = useSheinLoginAccounts();
  const loginMap = useMemo(
    () =>
      new Map(
        (sheinLoginAccounts.data ?? []).map((item) => [
          item.account.store_id,
          item,
        ]),
      ),
    [sheinLoginAccounts.data],
  );

  return (
    <ListingKitPageShell backgroundClassName="overflow-hidden rounded-lg bg-[linear-gradient(180deg,#f6f1e1_0%,#ffffff_100%)]" contentClassName="gap-6 px-4 py-4 sm:px-6 sm:py-6">
      <section className="rounded-3xl border border-white/70 bg-white/85 p-6 shadow-sm">
        <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-amber-700">
          SHEIN ENROLLMENT
        </p>
        <h1 className="mt-3 text-3xl font-semibold tracking-tight text-zinc-950">
          SHEIN 活动报名
        </h1>
        <p className="mt-2 max-w-3xl text-sm leading-6 text-zinc-600">
          先选店，再进入单店工作台执行同步、补成本价、刷新候选和手动报名。首版先用真实可跑通的数据链，不做假统计卡片。
        </p>
      </section>

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {stores.isLoading ? (
          <div className="rounded-2xl border border-zinc-200 bg-white px-4 py-6 text-sm text-zinc-500">
            正在加载店铺...
          </div>
        ) : null}
        {(stores.data?.items ?? []).map((store) => {
          const login = loginMap.get(store.id);
          return (
            <article key={store.id} className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h2 className="text-lg font-semibold text-zinc-950">{store.name}</h2>
                  <p className="text-sm text-zinc-500">{store.username}</p>
                </div>
                <Badge variant={login?.has_cookie ? "success" : "neutral"}>
                  {login?.has_cookie ? "已登录" : "需检查登录"}
                </Badge>
              </div>
              <dl className="mt-4 grid gap-2 text-sm text-zinc-600">
                <div>地区：{store.region || "-"}</div>
                <div>店铺类型：{store.shopType || "-"}</div>
                <div>自动上架：{store.enableAutoListing ? "启用" : "关闭"}</div>
              </dl>
              <div className="mt-5 flex gap-2">
                <Button asChild className="flex-1">
                  <Link href={`/listing-kits/shein-enrollment/${store.id}`}>进入工作台</Link>
                </Button>
                <Button asChild type="button" variant="outline">
                  <Link href={`/listing-kits/shein-login?store_id=${store.id}`}>登录</Link>
                </Button>
              </div>
            </article>
          );
        })}
      </section>
    </ListingKitPageShell>
  );
}

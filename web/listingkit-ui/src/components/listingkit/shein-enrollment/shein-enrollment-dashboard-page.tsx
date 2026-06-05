"use client";

import Link from "next/link";

import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import { Button } from "@/components/ui/button";
import { useSheinEnrollmentDashboard } from "@/lib/query/use-shein-enrollment";

export function SheinEnrollmentDashboardPage() {
  const dashboard = useSheinEnrollmentDashboard({
    activity_type: "PROMOTION",
  });

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
          先选店，再进入单店工作台执行同步、补成本价、刷新候选和手动报名。总览页现在直接展示当前活动类型下的真实同步与候选统计。
        </p>
      </section>

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {dashboard.isLoading ? (
          <div className="rounded-2xl border border-zinc-200 bg-white px-4 py-6 text-sm text-zinc-500">
            正在加载店铺...
          </div>
        ) : null}
        {(dashboard.data?.items ?? []).map((store) => {
          return (
            <article key={store.store_id} className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h2 className="text-lg font-semibold text-zinc-950">{store.store_name}</h2>
                  <p className="text-sm text-zinc-500">{store.store_username}</p>
                </div>
                <div className="rounded-full bg-zinc-100 px-3 py-1 text-xs text-zinc-600">
                  {store.last_sync_status || "未同步"}
                </div>
              </div>
              <div className="mt-4 grid grid-cols-2 gap-3 text-sm text-zinc-600">
                <div className="rounded-2xl bg-zinc-50 px-3 py-2">
                  <div className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">已同步</div>
                  <div className="mt-1 text-lg font-semibold text-zinc-950">{store.synced_product_count ?? 0}</div>
                </div>
                <div className="rounded-2xl bg-zinc-50 px-3 py-2">
                  <div className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">缺成本</div>
                  <div className="mt-1 text-lg font-semibold text-zinc-950">{store.missing_cost_count ?? 0}</div>
                </div>
                <div className="rounded-2xl bg-zinc-50 px-3 py-2">
                  <div className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">待审核</div>
                  <div className="mt-1 text-lg font-semibold text-zinc-950">{store.pending_review_count ?? 0}</div>
                </div>
                <div className="rounded-2xl bg-zinc-50 px-3 py-2">
                  <div className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">可报名</div>
                  <div className="mt-1 text-lg font-semibold text-zinc-950">{store.ready_to_enroll_count ?? 0}</div>
                </div>
              </div>
              <dl className="mt-4 grid gap-2 text-sm text-zinc-600">
                <div>地区：{store.region || "-"}</div>
                <div>最近同步：{store.last_sync_at || "-"}</div>
                <div>最近报名：{store.last_enrollment_at || "-"}</div>
              </dl>
              <div className="mt-5 flex gap-2">
                <Button asChild className="flex-1">
                  <Link href={`/listing-kits/shein-enrollment/${store.store_id}`}>进入工作台</Link>
                </Button>
                <Button asChild type="button" variant="outline">
                  <Link href={`/listing-kits/shein-login?store_id=${store.store_id}`}>登录</Link>
                </Button>
              </div>
            </article>
          );
        })}
      </section>
    </ListingKitPageShell>
  );
}

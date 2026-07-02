"use client";

import Link from "next/link";

import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import { Button } from "@/components/ui/button";
import { useSheinEnrollmentDashboard } from "@/lib/query/use-shein-enrollment";

export function SheinProductsDashboardPage() {
  const dashboard = useSheinEnrollmentDashboard({
    activity_type: "PROMOTION",
  });
  const dashboardItems = dashboard.data?.items ?? [];
  const dashboardErrorMessage =
    dashboard.error instanceof Error
      ? dashboard.error.message
      : "SHEIN products dashboard failed to load.";

  return (
    <ListingKitPageShell
      backgroundClassName="overflow-hidden rounded-lg bg-zinc-50"
      contentClassName="gap-6 px-4 py-4 sm:px-6 sm:py-6"
    >
      <section className="rounded-3xl border border-zinc-200 bg-white p-6 shadow-sm">
        <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
          SHEIN PRODUCTS
        </p>
        <h1 className="mt-3 text-3xl font-semibold tracking-tight text-zinc-950">
          SHEIN Synced Products
        </h1>
        <p className="mt-2 max-w-3xl text-sm leading-6 text-zinc-600">
          Pick a store to review synced on-shelf products, price snapshots,
          inventory snapshots, and POD/SDS cost maintenance.
        </p>
      </section>

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {dashboard.isLoading ? (
          <div className="rounded-2xl border border-zinc-200 bg-white px-4 py-6 text-sm text-zinc-500">
            Loading stores...
          </div>
        ) : null}
        {dashboard.isError ? (
          <div className="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-6 text-sm text-rose-700">
            {dashboardErrorMessage}
          </div>
        ) : null}
        {!dashboard.isLoading && !dashboard.isError && dashboardItems.length === 0 ? (
          <div className="rounded-2xl border border-zinc-200 bg-white px-4 py-6 text-sm text-zinc-500">
            No SHEIN stores are available for the current tenant.
          </div>
        ) : null}
        {dashboardItems.map((store) => (
          <article
            key={store.store_id}
            className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm"
          >
            <div className="flex items-start justify-between gap-3">
              <div>
                <h2 className="text-lg font-semibold text-zinc-950">
                  {store.store_name}
                </h2>
                <p className="text-sm text-zinc-500">{store.store_username}</p>
                <p className="text-xs text-zinc-400">Store ID: {store.store_id}</p>
              </div>
              <div className="rounded-full bg-zinc-100 px-3 py-1 text-xs text-zinc-600">
                {store.last_sync_status || "Not synced"}
              </div>
            </div>
            <div className="mt-4 grid grid-cols-2 gap-3 text-sm text-zinc-600">
              <div className="rounded-2xl bg-zinc-50 px-3 py-2">
                <div className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">
                  Synced
                </div>
                <div className="mt-1 text-lg font-semibold text-zinc-950">
                  {store.synced_product_count ?? 0}
                </div>
              </div>
              <div className="rounded-2xl bg-zinc-50 px-3 py-2">
                <div className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">
                  Missing Cost
                </div>
                <div className="mt-1 text-lg font-semibold text-zinc-950">
                  {store.missing_cost_count ?? 0}
                </div>
              </div>
            </div>
            <dl className="mt-4 grid gap-2 text-sm text-zinc-600">
              <div>Region: {store.region || "-"}</div>
              <div>Last Sync: {store.last_sync_at || "-"}</div>
            </dl>
            <div className="mt-5 flex gap-2">
              <Button asChild className="flex-1">
                <Link href={`/listing-kits/shein-products/${store.store_id}`}>
                  View Products
                </Link>
              </Button>
            </div>
          </article>
        ))}
      </section>
    </ListingKitPageShell>
  );
}

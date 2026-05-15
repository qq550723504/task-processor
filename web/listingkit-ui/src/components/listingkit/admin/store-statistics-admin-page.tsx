"use client";

import { useQuery } from "@tanstack/react-query";
import { BarChart3, RefreshCw, Search } from "lucide-react";
import { FormEvent, useMemo, useState } from "react";

import {
  getListingStoreStatistics,
  type ListingStoreStatistics,
} from "@/lib/api/admin-store-statistics";

const TODAY = new Date().toISOString().slice(0, 10);

const STATUS_TEXT: Record<number, string> = {
  0: "启用",
  1: "禁用",
};

const LIMIT_TYPE_TEXT: Record<string, string> = {
  fixed: "固定",
  dynamic: "动态",
};

export function StoreStatisticsAdminPage() {
  const [date, setDate] = useState(TODAY);

  const query = useMemo(() => ({ date }), [date]);
  const statisticsQuery = useQuery({
    queryKey: ["listingkit-admin-store-statistics", query],
    queryFn: () => getListingStoreStatistics(query),
    refetchInterval: 30_000,
  });

  const items = statisticsQuery.data ?? [];
  const loading = statisticsQuery.isLoading || statisticsQuery.isFetching;
  const visibleError =
    statisticsQuery.error instanceof Error ? statisticsQuery.error.message : "";
  const totals = summarize(items);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void statisticsQuery.refetch();
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">上架统计</h1>
            <p className="mt-1 text-sm text-zinc-500">
              共 {items.length} 个自动上架店铺，完成 {totals.completed} /{" "}
              {totals.limit}，待处理 {totals.pending}。
            </p>
          </div>
          <form className="flex flex-wrap gap-2" onSubmit={handleSubmit}>
            <label className="mb-3 block text-xs font-medium text-zinc-500">
              日期
              <input
                type="date"
                value={date}
                onChange={(event) => setDate(event.target.value)}
                className="mt-1 h-9 rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
              />
            </label>
            <button
              type="submit"
              className="mt-5 inline-flex h-9 items-center gap-2 rounded-md border border-zinc-200 px-3 text-sm font-medium text-zinc-700 hover:border-zinc-300"
            >
              {loading ? (
                <RefreshCw className="size-4 animate-spin" />
              ) : (
                <Search className="size-4" />
              )}
              刷新
            </button>
          </form>
        </div>
        {visibleError ? (
          <div className="mt-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {visibleError}
          </div>
        ) : null}
      </section>

      <section className="grid gap-3 md:grid-cols-4">
        <StatTile label="完成数" value={String(totals.completed)} />
        <StatTile label="待处理" value={String(totals.pending)} />
        <StatTile label="队列中" value={String(totals.queued)} />
        <StatTile label="挂起" value={String(totals.hold)} />
      </section>

      <section className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-zinc-200 text-sm">
            <thead className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
              <tr>
                <th className="px-4 py-3">店铺</th>
                <th className="px-4 py-3">平台</th>
                <th className="px-4 py-3">额度</th>
                <th className="px-4 py-3">任务</th>
                <th className="px-4 py-3">进度</th>
                <th className="px-4 py-3">状态</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {loading && items.length === 0 ? (
                <tr>
                  <td className="px-4 py-6 text-zinc-500" colSpan={6}>
                    加载中...
                  </td>
                </tr>
              ) : items.length === 0 ? (
                <tr>
                  <td className="px-4 py-6 text-zinc-500" colSpan={6}>
                    暂无统计数据
                  </td>
                </tr>
              ) : (
                items.map((item) => (
                  <tr key={item.id} className="align-top">
                    <td className="px-4 py-3">
                      <div className="font-medium text-zinc-950">
                        {item.name}
                      </div>
                      <div className="font-mono text-xs text-zinc-500">
                        {item.storeId || `#${item.id}`}
                      </div>
                      <div className="text-xs text-zinc-500">
                        租户 {item.tenantId}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-zinc-700">
                      {item.platform || "-"}
                    </td>
                    <td className="px-4 py-3 text-zinc-700">
                      <div>{item.dailyLimit}</div>
                      <div className="text-xs text-zinc-500">
                        {LIMIT_TYPE_TEXT[item.dailyLimitType ?? ""] ??
                          item.dailyLimitType ??
                          "-"}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-zinc-700">
                      <div>{item.completedCount} / {item.dailyLimit}</div>
                      <div className="text-xs text-zinc-500">
                        待处理 {item.remainingCount}，队列 {item.queuedCount}，
                        挂起 {item.holdCount}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex min-w-44 items-center gap-2">
                        <div className="h-2 flex-1 rounded-full bg-zinc-100">
                          <div
                            className="h-2 rounded-full bg-zinc-950"
                            style={{
                              width: `${Math.min(
                                100,
                                Math.max(0, item.progressPercentage),
                              )}%`,
                            }}
                          />
                        </div>
                        <span className="w-12 text-right text-xs font-medium text-zinc-700">
                          {formatPercent(item.progressPercentage)}
                        </span>
                      </div>
                      <div className="mt-1 text-xs text-zinc-500">
                        剩余额度 {item.remainingQuota}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className="rounded-full bg-zinc-100 px-2 py-1 text-xs font-medium text-zinc-700">
                        {STATUS_TEXT[item.status] ?? `状态 ${item.status}`}
                      </span>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

function summarize(items: ListingStoreStatistics[]) {
  return items.reduce(
    (total, item) => ({
      completed: total.completed + item.completedCount,
      limit: total.limit + item.dailyLimit,
      pending: total.pending + item.remainingCount,
      queued: total.queued + item.queuedCount,
      hold: total.hold + item.holdCount,
    }),
    { completed: 0, limit: 0, pending: 0, queued: 0, hold: 0 },
  );
}

function formatPercent(value: number) {
  if (Number.isInteger(value)) {
    return `${value}%`;
  }
  return `${value.toFixed(2)}%`;
}

function StatTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
      <div className="mb-2 flex items-center gap-2 text-xs font-medium text-zinc-500">
        <BarChart3 className="size-4" />
        {label}
      </div>
      <div className="text-2xl font-semibold text-zinc-950">{value}</div>
    </div>
  );
}

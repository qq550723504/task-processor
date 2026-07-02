"use client";

import { Button } from "@/components/ui/button";
import type { SheinEnrollmentStoreSummary } from "@/lib/types/listingkit/shein-enrollment";

export function SheinProductsStoreHeader({
  onSync,
  summary,
  syncError,
  syncPending,
}: {
  onSync: () => void;
  summary?: SheinEnrollmentStoreSummary;
  syncError?: unknown;
  syncPending: boolean;
}) {
  const storeIDLabel =
    summary?.store_id !== undefined ? String(summary.store_id) : "-";
  const storeName =
    summary?.store_name ||
    (summary?.store_id !== undefined ? `店铺 ${String(summary.store_id)}` : "店铺");
  const latestSyncStatus =
    summary?.last_sync_job?.status || summary?.last_sync_status || "";
  const latestSyncError =
    latestSyncStatus === "failed"
      ? summary?.last_sync_job?.error_summary?.trim()
      : "";
  const syncRequestError = formatSheinProductsError(syncError);
  const visibleSyncError = syncRequestError || latestSyncError;
  const syncStatusLabel = formatSheinSyncStatus(latestSyncStatus);
  const syncErrorTitle = syncRequestError ? "同步请求失败" : "最近同步失败";
  const syncIsRunning =
    syncPending || latestSyncStatus === "pending" || latestSyncStatus === "running";

  return (
    <section className="rounded-3xl border border-zinc-200 bg-white p-6 shadow-sm">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-zinc-500">
            SHEIN PRODUCTS
          </p>
          <div>
            <h1 className="text-2xl font-semibold tracking-tight text-zinc-950">
              {storeName}
            </h1>
            <p className="mt-1 text-sm text-zinc-600">
              管理 SHEIN 已同步商品、库存价格快照和 POD/SDS 成本价。
            </p>
          </div>
          <div className="flex flex-wrap gap-2 text-xs text-zinc-500">
            <span>店铺 ID {storeIDLabel}</span>
            <span>平台 {summary?.platform || "SHEIN"}</span>
            <span>地区 {summary?.region || "-"}</span>
          </div>
          <div className="grid gap-2 sm:grid-cols-2">
            <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-3 py-2">
              <div className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">
                已同步
              </div>
              <div className="mt-1 text-xl font-semibold text-zinc-950">
                {summary?.synced_product_count ?? 0}
              </div>
            </div>
            <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-3 py-2">
              <div className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">
                缺成本
              </div>
              <div className="mt-1 text-xl font-semibold text-zinc-950">
                {summary?.missing_cost_count ?? 0}
              </div>
            </div>
          </div>
        </div>

        <div className="flex flex-col gap-3 rounded-2xl border border-zinc-200 bg-zinc-50 p-4">
          <div className="space-y-1 text-xs text-zinc-500">
            <div>
              最近同步：{summary?.last_sync_at || "-"}
              {syncStatusLabel ? ` · ${syncStatusLabel}` : ""}
            </div>
          </div>
          {syncIsRunning && !visibleSyncError ? (
            <div className="rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">
              同步任务执行中，页面会自动刷新同步结果。
            </div>
          ) : null}
          {visibleSyncError ? (
            <div
              className="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-xs leading-5 text-red-700"
              role="alert"
            >
              <div className="font-semibold text-red-800">{syncErrorTitle}</div>
              <div className="mt-1 whitespace-pre-wrap break-words">{visibleSyncError}</div>
            </div>
          ) : null}
          <Button
            className="rounded-xl"
            disabled={syncPending}
            onClick={onSync}
            type="button"
          >
            {syncPending ? "同步中..." : "立即同步"}
          </Button>
        </div>
      </div>
    </section>
  );
}

function formatSheinSyncStatus(status: string) {
  const labels: Record<string, string> = {
    pending: "等待同步",
    running: "同步中",
    succeeded: "同步成功",
    partially_succeeded: "部分成功",
    failed: "同步失败",
  };
  return labels[status] ?? status;
}

function formatSheinProductsError(error: unknown) {
  if (!error) {
    return "";
  }
  if (error instanceof Error) {
    return error.message;
  }
  if (typeof error === "string") {
    return error;
  }
  return "同步请求失败，请稍后重试。";
}

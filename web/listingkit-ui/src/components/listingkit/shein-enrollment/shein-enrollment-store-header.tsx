"use client";

import Link from "next/link";

import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import type { SheinEnrollmentStoreSummary } from "@/lib/types/listingkit/shein-enrollment";

export function SheinEnrollmentStoreHeader({
  activityType,
  onActivityTypeChange,
  onRefreshCandidates,
  onSync,
  refreshPending,
  summary,
  syncPending,
}: {
  activityType: string;
  onActivityTypeChange: (value: string) => void;
  onRefreshCandidates: () => void;
  onSync: () => void;
  refreshPending: boolean;
  summary?: SheinEnrollmentStoreSummary;
  syncPending: boolean;
}) {
  const storeIDLabel =
    summary?.store_id !== undefined ? String(summary.store_id) : "-";
  const storeName =
    summary?.store_name ||
    (summary?.store_id !== undefined ? `店铺 ${String(summary.store_id)}` : "店铺");

  return (
    <section className="rounded-3xl border border-zinc-200 bg-[linear-gradient(135deg,#fff9ec_0%,#ffffff_75%)] p-6 shadow-sm">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-2">
          <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-amber-700">
            SHEIN ENROLLMENT
          </p>
          <div>
            <h1 className="text-2xl font-semibold tracking-tight text-zinc-950">
              {storeName}
            </h1>
            <p className="mt-1 text-sm text-zinc-600">
              {summary?.store_username || "未识别店铺账号"} · 候选池和报名执行依赖活动类型，先选活动，再刷新候选。
            </p>
          </div>
          <div className="flex flex-wrap gap-2 text-xs text-zinc-500">
            <span>店铺 ID {storeIDLabel}</span>
            <span>平台 {summary?.platform || "SHEIN"}</span>
            <span>地区 {summary?.region || "-"}</span>
          </div>
          <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
            <div className="rounded-2xl border border-zinc-200 bg-white/90 px-3 py-2">
              <div className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">
                已同步
              </div>
              <div className="mt-1 text-xl font-semibold text-zinc-950">
                {summary?.synced_product_count ?? 0}
              </div>
            </div>
            <div className="rounded-2xl border border-zinc-200 bg-white/90 px-3 py-2">
              <div className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">
                缺成本
              </div>
              <div className="mt-1 text-xl font-semibold text-zinc-950">
                {summary?.missing_cost_count ?? 0}
              </div>
            </div>
            <div className="rounded-2xl border border-zinc-200 bg-white/90 px-3 py-2">
              <div className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">
                待审核
              </div>
              <div className="mt-1 text-xl font-semibold text-zinc-950">
                {summary?.pending_review_count ?? 0}
              </div>
            </div>
            <div className="rounded-2xl border border-zinc-200 bg-white/90 px-3 py-2">
              <div className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">
                可报名
              </div>
              <div className="mt-1 text-xl font-semibold text-zinc-950">
                {summary?.ready_to_enroll_count ?? 0}
              </div>
            </div>
          </div>
        </div>

        <div className="flex flex-col gap-3 rounded-2xl border border-zinc-200 bg-white/80 p-4">
          <label className="text-xs font-semibold uppercase tracking-[0.14em] text-zinc-500">
            活动类型
            <Select
              className="mt-2 h-10 min-w-48 rounded-xl border-zinc-200 bg-white"
              onChange={(event) => onActivityTypeChange(event.target.value)}
              value={activityType}
            >
              <option value="PROMOTION">促销活动</option>
              <option value="TIME_LIMITED">限时活动</option>
              <option value="MIXED">混合活动</option>
            </Select>
          </label>
          <div className="space-y-1 text-xs text-zinc-500">
            <div>最近同步：{summary?.last_sync_at || "-"}</div>
            <div>最近报名：{summary?.last_enrollment_at || "-"}</div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              className="rounded-xl"
              disabled={syncPending}
              onClick={onSync}
              type="button"
            >
              {syncPending ? "同步中..." : "立即同步"}
            </Button>
            <Button
              className="rounded-xl"
              disabled={refreshPending}
              onClick={onRefreshCandidates}
              type="button"
              variant="secondary"
            >
              {refreshPending ? "刷新中..." : "刷新候选池"}
            </Button>
            <Button asChild className="rounded-xl" type="button" variant="outline">
              <Link href={`/listing-kits/shein-login?store_id=${summary?.store_id ?? ""}`}>去检查登录</Link>
            </Button>
          </div>
        </div>
      </div>
    </section>
  );
}

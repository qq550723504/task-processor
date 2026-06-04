"use client";

import Link from "next/link";

import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import type { ListingStore } from "@/lib/api/admin-stores";

export function SheinEnrollmentStoreHeader({
  activityType,
  onActivityTypeChange,
  onRefreshCandidates,
  onSync,
  refreshPending,
  store,
  syncPending,
}: {
  activityType: string;
  onActivityTypeChange: (value: string) => void;
  onRefreshCandidates: () => void;
  onSync: () => void;
  refreshPending: boolean;
  store?: ListingStore;
  syncPending: boolean;
}) {
  const storeIDLabel = store?.id !== undefined ? String(store.id) : "-";
  const storeName = store?.name || (store?.id !== undefined ? `店铺 ${String(store.id)}` : "店铺");

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
              {store?.username || "未识别店铺账号"} · 候选池和报名执行依赖活动类型，先选活动，再刷新候选。
            </p>
          </div>
          <div className="flex flex-wrap gap-2 text-xs text-zinc-500">
            <span>店铺 ID {storeIDLabel}</span>
            <span>平台 {store?.platform || "SHEIN"}</span>
            <span>地区 {store?.region || "-"}</span>
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
              <Link href={`/listing-kits/shein-login?store_id=${store?.id ?? ""}`}>去检查登录</Link>
            </Button>
          </div>
        </div>
      </div>
    </section>
  );
}

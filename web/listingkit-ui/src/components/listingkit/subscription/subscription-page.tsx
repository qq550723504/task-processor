"use client";

import { useQuery } from "@tanstack/react-query";
import { CheckCircle2, RefreshCw, XCircle } from "lucide-react";

import {
  formatSubscriptionApiError,
  getCurrentSubscription,
  type SubscriptionEntitlementView,
  type SubscriptionStatus,
} from "@/lib/api/subscription";

const STATUS_LABEL: Record<SubscriptionStatus, string> = {
  active: "已开通",
  trialing: "试用中",
  expired: "已过期",
  disabled: "已停用",
};

export function SubscriptionPage() {
  const query = useQuery({
    queryKey: ["listingkit-subscription"],
    queryFn: getCurrentSubscription,
  });

  const summary = query.data;
  const visibleError = query.error ? formatSubscriptionApiError(query.error) : "";

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">订阅</h1>
            <p className="mt-1 text-sm text-zinc-500">
              当前租户 {summary?.tenant_id ?? "-"}，按模块开通 ListingKit 能力。
            </p>
          </div>
          <button
            type="button"
            onClick={() => void query.refetch()}
            className="inline-flex h-9 items-center gap-2 rounded-md border border-zinc-200 px-3 text-sm font-medium text-zinc-700 hover:border-zinc-300"
          >
            <RefreshCw className={`size-4 ${query.isFetching ? "animate-spin" : ""}`} />
            刷新
          </button>
        </div>
        {visibleError ? (
          <div className="mt-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {visibleError}
          </div>
        ) : null}
      </section>

      <section className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-zinc-200 text-sm">
            <thead className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
              <tr>
                <th className="px-4 py-3">模块</th>
                <th className="px-4 py-3">状态</th>
                <th className="px-4 py-3">有效期</th>
                <th className="px-4 py-3">额度</th>
                <th className="px-4 py-3">用量</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {query.isLoading ? (
                <tr>
                  <td className="px-4 py-6 text-zinc-500" colSpan={5}>
                    加载中...
                  </td>
                </tr>
              ) : (summary?.entitlements ?? []).length === 0 ? (
                <tr>
                  <td className="px-4 py-6 text-zinc-500" colSpan={5}>
                    暂无模块
                  </td>
                </tr>
              ) : (
                summary?.entitlements.map((view) => (
                  <tr key={view.module.code} className="align-top">
                    <td className="px-4 py-3">
                      <div className="font-medium text-zinc-950">{view.module.name}</div>
                      <div className="font-mono text-xs text-zinc-500">
                        {view.module.code}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge view={view} />
                    </td>
                    <td className="px-4 py-3 text-zinc-700">
                      {formatDate(view.entitlement?.expires_at)}
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-zinc-600">
                      {formatRecord(view.limits)}
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-zinc-600">
                      {formatRecord(view.used)}
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

function StatusBadge({ view }: { view: SubscriptionEntitlementView }) {
  const active = view.allowed;
  return (
    <span
      className={[
        "inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-medium",
        active ? "bg-emerald-50 text-emerald-700" : "bg-zinc-100 text-zinc-600",
      ].join(" ")}
    >
      {active ? <CheckCircle2 className="size-3.5" /> : <XCircle className="size-3.5" />}
      {view.entitlement ? STATUS_LABEL[view.entitlement.status] : "未开通"}
    </span>
  );
}

function formatDate(value?: string) {
  if (!value) {
    return "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function formatRecord(value?: Record<string, number>) {
  if (!value || Object.keys(value).length === 0) {
    return "-";
  }
  return Object.entries(value)
    .map(([key, count]) => `${key}: ${count}`)
    .join(", ");
}

"use client";

import { useQuery } from "@tanstack/react-query";
import { CheckCircle2, Power, RefreshCw, Save, XCircle } from "lucide-react";
import { FormEvent, useState } from "react";

import {
  formatSubscriptionApiError,
  getCurrentSubscription,
  updateSubscriptionEntitlement,
  type SubscriptionEntitlementView,
  type SubscriptionStatus,
} from "@/lib/api/subscription";

const STATUS_OPTIONS: SubscriptionStatus[] = [
  "active",
  "trialing",
  "expired",
  "disabled",
];

const STATUS_LABEL: Record<SubscriptionStatus, string> = {
  active: "已开通",
  trialing: "试用中",
  expired: "已过期",
  disabled: "已停用",
};

export function SubscriptionPage() {
  const [editingModule, setEditingModule] = useState("");
  const [status, setStatus] = useState<SubscriptionStatus>("active");
  const [expiresAt, setExpiresAt] = useState("");
  const [limitsText, setLimitsText] = useState("{}");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const query = useQuery({
    queryKey: ["listingkit-subscription"],
    queryFn: getCurrentSubscription,
  });

  const summary = query.data;
  const visibleError =
    error || (query.error ? formatSubscriptionApiError(query.error) : "");

  function beginEdit(view: SubscriptionEntitlementView) {
    setEditingModule(view.module.code);
    setStatus(view.entitlement?.status ?? "active");
    setExpiresAt(toLocalDateTimeInput(view.entitlement?.expires_at));
    setLimitsText(JSON.stringify(view.entitlement?.limits ?? {}, null, 2));
    setError("");
  }

  async function handleSave(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editingModule) {
      return;
    }
    setSaving(true);
    setError("");
    try {
      const parsedLimits = parseLimits(limitsText);
      await updateSubscriptionEntitlement(editingModule, {
        status,
        expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined,
        limits: parsedLimits,
      });
      setEditingModule("");
      await query.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

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

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_380px]">
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-zinc-200 text-sm">
              <thead className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
                <tr>
                  <th className="px-4 py-3">模块</th>
                  <th className="px-4 py-3">状态</th>
                  <th className="px-4 py-3">有效期</th>
                  <th className="px-4 py-3">额度</th>
                  <th className="px-4 py-3">用量</th>
                  <th className="px-4 py-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-100">
                {query.isLoading ? (
                  <tr>
                    <td className="px-4 py-6 text-zinc-500" colSpan={6}>
                      加载中...
                    </td>
                  </tr>
                ) : (summary?.entitlements ?? []).length === 0 ? (
                  <tr>
                    <td className="px-4 py-6 text-zinc-500" colSpan={6}>
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
                      <td className="px-4 py-3 text-right">
                        <button
                          type="button"
                          onClick={() => beginEdit(view)}
                          className="inline-flex h-8 items-center gap-2 rounded-md border border-zinc-200 px-3 text-sm font-medium text-zinc-700 hover:border-zinc-300"
                        >
                          <Power className="size-4" />
                          配置
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>

        <form
          onSubmit={handleSave}
          className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm"
        >
          <div className="mb-4">
            <h2 className="text-base font-semibold text-zinc-950">模块开通</h2>
          </div>
          <label className="mb-3 block text-xs font-medium text-zinc-500">
            模块
            <input
              value={editingModule}
              readOnly
              className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-zinc-50 px-3 font-mono text-sm text-zinc-900"
              placeholder="选择模块"
            />
          </label>
          <label className="mb-3 block text-xs font-medium text-zinc-500">
            状态
            <select
              value={status}
              onChange={(event) => setStatus(event.target.value as SubscriptionStatus)}
              className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
            >
              {STATUS_OPTIONS.map((option) => (
                <option key={option} value={option}>
                  {STATUS_LABEL[option]}
                </option>
              ))}
            </select>
          </label>
          <label className="mb-3 block text-xs font-medium text-zinc-500">
            过期时间
            <input
              type="datetime-local"
              value={expiresAt}
              onChange={(event) => setExpiresAt(event.target.value)}
              className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
            />
          </label>
          <label className="mb-3 block text-xs font-medium text-zinc-500">
            额度 JSON
            <textarea
              value={limitsText}
              onChange={(event) => setLimitsText(event.target.value)}
              rows={7}
              className="mt-1 w-full rounded-md border border-zinc-200 px-3 py-2 font-mono text-xs text-zinc-900"
            />
          </label>
          <button
            type="submit"
            disabled={!editingModule || saving}
            className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-md bg-zinc-950 px-3 text-sm font-medium text-white hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-400"
          >
            {saving ? <RefreshCw className="size-4 animate-spin" /> : <Save className="size-4" />}
            保存配置
          </button>
        </form>
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

function toLocalDateTimeInput(value?: string) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const offset = date.getTimezoneOffset() * 60000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

function parseLimits(value: string): Record<string, number> {
  const parsed = JSON.parse(value || "{}") as unknown;
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("额度必须是 JSON object");
  }
  const out: Record<string, number> = {};
  for (const [key, raw] of Object.entries(parsed)) {
    if (typeof raw !== "number" || raw < 0) {
      throw new Error(`额度 ${key} 必须是非负数字`);
    }
    out[key] = raw;
  }
  return out;
}

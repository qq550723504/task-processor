"use client";

import { useQuery } from "@tanstack/react-query";
import { CheckCircle2, Power, RefreshCw, Save, Search, XCircle } from "lucide-react";
import { FormEvent, useMemo, useState } from "react";

import {
  formatSubscriptionApiError,
  getPlatformTenantSubscriptionAuditLogs,
  getPlatformTenantSubscriptions,
  getPlatformTenantSubscription,
  updatePlatformTenantSubscriptionUsage,
  updatePlatformTenantSubscriptionEntitlement,
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

export function PlatformSubscriptionPage() {
  const [tenantInput, setTenantInput] = useState("");
  const [tenantId, setTenantId] = useState("");
  const [editingModule, setEditingModule] = useState("");
  const [status, setStatus] = useState<SubscriptionStatus>("active");
  const [expiresAt, setExpiresAt] = useState("");
  const [limitsText, setLimitsText] = useState("{}");
  const [usageMetric, setUsageMetric] = useState("");
  const [usagePeriod, setUsagePeriod] = useState(currentPeriodKey());
  const [usageUsed, setUsageUsed] = useState("0");
  const [usageReason, setUsageReason] = useState("");
  const [saving, setSaving] = useState(false);
  const [savingUsage, setSavingUsage] = useState(false);
  const [error, setError] = useState("");

  const normalizedTenantId = useMemo(() => tenantId.trim(), [tenantId]);
  const query = useQuery({
    queryKey: ["listingkit-platform-subscription", normalizedTenantId],
    queryFn: () => getPlatformTenantSubscription(normalizedTenantId),
    enabled: Boolean(normalizedTenantId),
  });
  const tenantListQuery = useQuery({
    queryKey: ["listingkit-platform-subscriptions"],
    queryFn: getPlatformTenantSubscriptions,
  });
  const auditQuery = useQuery({
    queryKey: ["listingkit-platform-subscription-audit", normalizedTenantId],
    queryFn: () => getPlatformTenantSubscriptionAuditLogs(normalizedTenantId),
    enabled: Boolean(normalizedTenantId),
  });

  const summary = query.data;
  const visibleError =
    error ||
    (query.error ? formatSubscriptionApiError(query.error) : "") ||
    (tenantListQuery.error
      ? formatSubscriptionApiError(tenantListQuery.error)
      : "") ||
    (auditQuery.error
      ? formatSubscriptionApiError(auditQuery.error)
      : "");

  function handleLoad(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setEditingModule("");
    setTenantId(tenantInput);
  }

  function beginEdit(view: SubscriptionEntitlementView) {
    setEditingModule(view.module.code);
    setStatus(view.entitlement?.status ?? "active");
    setExpiresAt(toLocalDateTimeInput(view.entitlement?.expires_at));
    setLimitsText(JSON.stringify(view.entitlement?.limits ?? {}, null, 2));
    const firstLimit = Object.keys(view.entitlement?.limits ?? {})[0] ?? "";
    const firstUsage = view.usage.find((item) => item.metric === firstLimit) ?? view.usage[0];
    setUsageMetric(firstUsage?.metric ?? firstLimit);
    setUsagePeriod(firstUsage?.period_key ?? currentPeriodKey());
    setUsageUsed(String(firstUsage?.used ?? 0));
    setUsageReason("");
    setError("");
  }

  async function handleSave(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editingModule || !normalizedTenantId) {
      return;
    }
    setSaving(true);
    setError("");
    try {
      await updatePlatformTenantSubscriptionEntitlement(
        normalizedTenantId,
        editingModule,
        {
          status,
          expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined,
          limits: parseLimits(limitsText),
        },
      );
      setEditingModule("");
      await tenantListQuery.refetch();
      await query.refetch();
      await auditQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleUsageSave(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editingModule || !normalizedTenantId || !usageMetric.trim()) {
      return;
    }
    const used = Number(usageUsed);
    if (!Number.isInteger(used) || used < 0) {
      setError("用量必须是非负整数");
      return;
    }
    setSavingUsage(true);
    setError("");
    try {
      await updatePlatformTenantSubscriptionUsage(normalizedTenantId, editingModule, {
        period_key: usagePeriod || currentPeriodKey(),
        metric: usageMetric,
        used,
        reason: usageReason || undefined,
      });
      await query.refetch();
      await auditQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSavingUsage(false);
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">平台订阅</h1>
            <p className="mt-1 text-sm text-zinc-500">
              租户 {summary?.tenant_id || normalizedTenantId || "-"}
            </p>
          </div>
          <form onSubmit={handleLoad} className="flex flex-col gap-2 sm:flex-row">
            <input
              value={tenantInput}
              onChange={(event) => setTenantInput(event.target.value)}
              className="h-9 min-w-[260px] rounded-md border border-zinc-200 px-3 font-mono text-sm text-zinc-900"
              placeholder="ZITADEL resource owner id"
            />
            <button
              type="submit"
              disabled={!tenantInput.trim()}
              className="inline-flex h-9 items-center justify-center gap-2 rounded-md bg-zinc-950 px-3 text-sm font-medium text-white hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-400"
            >
              <Search className="size-4" />
              查询
            </button>
            <button
              type="button"
              disabled={!normalizedTenantId}
              onClick={() => void query.refetch()}
              className="inline-flex h-9 items-center justify-center gap-2 rounded-md border border-zinc-200 px-3 text-sm font-medium text-zinc-700 hover:border-zinc-300 disabled:cursor-not-allowed disabled:text-zinc-400"
            >
              <RefreshCw className={`size-4 ${query.isFetching ? "animate-spin" : ""}`} />
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

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_380px]">
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
          <div className="border-b border-zinc-200 bg-zinc-50 px-4 py-3">
            <div className="flex items-center justify-between gap-3">
              <h2 className="text-sm font-semibold text-zinc-900">已配置租户</h2>
              <button
                type="button"
                onClick={() => void tenantListQuery.refetch()}
                className="inline-flex h-8 items-center gap-2 rounded-md border border-zinc-200 bg-white px-3 text-xs font-medium text-zinc-700 hover:border-zinc-300"
              >
                <RefreshCw
                  className={`size-3.5 ${tenantListQuery.isFetching ? "animate-spin" : ""}`}
                />
                刷新
              </button>
            </div>
            <div className="mt-3 flex gap-2 overflow-x-auto pb-1">
              {tenantListQuery.isLoading ? (
                <span className="text-sm text-zinc-500">加载中...</span>
              ) : (tenantListQuery.data ?? []).length === 0 ? (
                <span className="text-sm text-zinc-500">暂无租户</span>
              ) : (
                tenantListQuery.data?.map((tenant) => (
                  <button
                    key={tenant.tenant_id}
                    type="button"
                    onClick={() => {
                      setTenantInput(tenant.tenant_id);
                      setTenantId(tenant.tenant_id);
                      setEditingModule("");
                      setError("");
                    }}
                    className={[
                      "shrink-0 rounded-md border px-3 py-2 text-left text-xs",
                      tenant.tenant_id === normalizedTenantId
                        ? "border-zinc-900 bg-zinc-950 text-white"
                        : "border-zinc-200 bg-white text-zinc-700 hover:border-zinc-300",
                    ].join(" ")}
                  >
                    <div className="font-mono">{tenant.tenant_id}</div>
                    <div className="mt-1 opacity-80">
                      {tenant.active_count}/{tenant.entitlement_count} 已开通
                    </div>
                  </button>
                ))
              )}
            </div>
          </div>
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

        <div className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
          <form onSubmit={handleSave}>
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
              disabled={!editingModule || !normalizedTenantId || saving}
              className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-md bg-zinc-950 px-3 text-sm font-medium text-white hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-400"
            >
              {saving ? <RefreshCw className="size-4 animate-spin" /> : <Save className="size-4" />}
              保存配置
            </button>
          </form>
          <div className="my-4 border-t border-zinc-200" />
          <div className="mb-3">
            <h2 className="text-base font-semibold text-zinc-950">用量调整</h2>
          </div>
          <form onSubmit={handleUsageSave} className="space-y-3">
            <label className="block text-xs font-medium text-zinc-500">
              周期
              <input
                value={usagePeriod}
                onChange={(event) => setUsagePeriod(event.target.value)}
                className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 font-mono text-sm text-zinc-900"
                placeholder="YYYY-MM"
              />
            </label>
            <label className="block text-xs font-medium text-zinc-500">
              指标
              <input
                value={usageMetric}
                onChange={(event) => setUsageMetric(event.target.value)}
                className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 font-mono text-sm text-zinc-900"
                placeholder="design_jobs"
              />
            </label>
            <label className="block text-xs font-medium text-zinc-500">
              已用
              <input
                type="number"
                min={0}
                value={usageUsed}
                onChange={(event) => setUsageUsed(event.target.value)}
                className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
              />
            </label>
            <label className="block text-xs font-medium text-zinc-500">
              原因
              <input
                value={usageReason}
                onChange={(event) => setUsageReason(event.target.value)}
                className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
                placeholder="运营调整"
              />
            </label>
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                disabled={!editingModule || !normalizedTenantId || savingUsage}
                onClick={() => setUsageUsed("0")}
                className="inline-flex h-9 items-center justify-center rounded-md border border-zinc-200 px-3 text-sm font-medium text-zinc-700 hover:border-zinc-300 disabled:cursor-not-allowed disabled:text-zinc-400"
              >
                重置为 0
              </button>
              <button
                type="submit"
                disabled={!editingModule || !normalizedTenantId || savingUsage}
                className="inline-flex h-9 items-center justify-center gap-2 rounded-md bg-zinc-950 px-3 text-sm font-medium text-white hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-400"
              >
                {savingUsage ? <RefreshCw className="size-4 animate-spin" /> : <Save className="size-4" />}
                保存用量
              </button>
            </div>
          </form>
        </div>
      </section>
      {normalizedTenantId ? (
        <section className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="text-base font-semibold text-zinc-950">审计日志</h2>
            <button
              type="button"
              onClick={() => void auditQuery.refetch()}
              className="inline-flex h-8 items-center gap-2 rounded-md border border-zinc-200 px-3 text-xs font-medium text-zinc-700 hover:border-zinc-300"
            >
              <RefreshCw className={`size-3.5 ${auditQuery.isFetching ? "animate-spin" : ""}`} />
              刷新
            </button>
          </div>
          <div className="divide-y divide-zinc-100 text-sm">
            {auditQuery.isLoading ? (
              <div className="py-3 text-zinc-500">加载中...</div>
            ) : (auditQuery.data ?? []).length === 0 ? (
              <div className="py-3 text-zinc-500">暂无日志</div>
            ) : (
              auditQuery.data?.map((item) => (
                <div key={item.id} className="grid gap-1 py-3 md:grid-cols-[180px_1fr_160px]">
                  <div className="text-zinc-500">{formatDate(item.created_at)}</div>
                  <div>
                    <span className="font-medium text-zinc-950">{item.action}</span>
                    {item.module_code ? (
                      <span className="ml-2 font-mono text-xs text-zinc-500">{item.module_code}</span>
                    ) : null}
                    {item.reason ? (
                      <div className="mt-1 text-xs text-zinc-500">{item.reason}</div>
                    ) : null}
                  </div>
                  <div className="font-mono text-xs text-zinc-500">{item.actor_id || "-"}</div>
                </div>
              ))
            )}
          </div>
        </section>
      ) : null}
    </div>
  );
}

function StatusBadge({ view }: { view: SubscriptionEntitlementView }) {
  return (
    <span
      className={[
        "inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-medium",
        view.allowed ? "bg-emerald-50 text-emerald-700" : "bg-zinc-100 text-zinc-600",
      ].join(" ")}
    >
      {view.allowed ? <CheckCircle2 className="size-3.5" /> : <XCircle className="size-3.5" />}
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

function currentPeriodKey() {
  return new Date().toISOString().slice(0, 7);
}

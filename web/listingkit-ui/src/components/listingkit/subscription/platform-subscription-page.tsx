"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { useQuery } from "@tanstack/react-query";
import { CheckCircle2, Power, RefreshCw, Save, Search, XCircle } from "lucide-react";
import { FormEvent, useMemo, useState } from "react";

import {
  applyPlatformTenantSubscriptionPlan,
  formatSubscriptionApiError,
  getPlatformSubscriptionPlans,
  getPlatformTenantSubscriptionAuditLogs,
  getPlatformTenantSubscriptions,
  getPlatformTenantSubscription,
  updatePlatformTenantSubscriptionUsage,
  updatePlatformTenantSubscriptionEntitlement,
  type SubscriptionEntitlementView,
  type SubscriptionStatus,
} from "@/lib/api/subscription";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Select } from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";

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

const OSS_STORAGE_LIMIT_PRESETS = [
  { label: "1 GB", bytes: 1 * 1024 * 1024 * 1024 },
  { label: "10 GB", bytes: 10 * 1024 * 1024 * 1024 },
  { label: "100 GB", bytes: 100 * 1024 * 1024 * 1024 },
];

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
  const [selectedPlan, setSelectedPlan] = useState("");
  const [planExpiresAt, setPlanExpiresAt] = useState("");
  const [saving, setSaving] = useState(false);
  const [savingUsage, setSavingUsage] = useState(false);
  const [applyingPlan, setApplyingPlan] = useState(false);
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
  const planQuery = useQuery({
    queryKey: ["listingkit-platform-subscription-plans"],
    queryFn: getPlatformSubscriptionPlans,
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
    (planQuery.error ? formatSubscriptionApiError(planQuery.error) : "") ||
    (auditQuery.error
      ? formatSubscriptionApiError(auditQuery.error)
      : "");

  function handleLoad(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setEditingModule("");
    setTenantId(tenantInput);
  }

  async function handlePlanApply(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!normalizedTenantId || !selectedPlan) {
      return;
    }
    setApplyingPlan(true);
    setError("");
    try {
      await applyPlatformTenantSubscriptionPlan(normalizedTenantId, {
        plan_code: selectedPlan,
        status: "active",
        expires_at: planExpiresAt ? new Date(planExpiresAt).toISOString() : undefined,
      });
      await tenantListQuery.refetch();
      await query.refetch();
      await auditQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setApplyingPlan(false);
    }
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
      <Card>
        <CardHeader className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <CardTitle className="text-2xl">平台订阅</CardTitle>
            <CardDescription className="mt-1">
              租户 {summary?.tenant_id || normalizedTenantId || "-"}
            </CardDescription>
          </div>
          <form onSubmit={handleLoad} className="flex flex-col gap-2 sm:flex-row">
            <Input
              value={tenantInput}
              onChange={(event) => setTenantInput(event.target.value)}
              className="h-9 min-w-[260px] font-mono"
              placeholder="ZITADEL resource owner id"
            />
            <Button
              type="submit"
              disabled={!tenantInput.trim()}
              className="h-9 gap-2 px-3"
            >
              <Search className="size-4" />
              查询
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={!normalizedTenantId}
              onClick={() => void query.refetch()}
              className="h-9 gap-2 px-3"
            >
              <RefreshCw className={`size-4 ${query.isFetching ? "animate-spin" : ""}`} />
              刷新
            </Button>
          </form>
        </CardHeader>
        {visibleError ? (
          <CardContent>
            <Alert variant="destructive">
              <AlertDescription>{visibleError}</AlertDescription>
            </Alert>
          </CardContent>
        ) : null}
      </Card>

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_380px]">
        <Card className="overflow-hidden p-0">
          <div className="border-b border-zinc-200 bg-zinc-50 px-4 py-3">
            <div className="flex items-center justify-between gap-3">
              <h2 className="text-sm font-semibold text-zinc-900">已配置租户</h2>
              <Button
                type="button"
                variant="outline"
                onClick={() => void tenantListQuery.refetch()}
                className="h-8 gap-2 px-3 text-xs"
              >
                <RefreshCw
                  className={`size-3.5 ${tenantListQuery.isFetching ? "animate-spin" : ""}`}
                />
                刷新
              </Button>
            </div>
            <div className="mt-3 flex gap-2 overflow-x-auto pb-1">
              {tenantListQuery.isLoading ? (
                <span className="text-sm text-zinc-500">加载中...</span>
              ) : (tenantListQuery.data ?? []).length === 0 ? (
                <span className="text-sm text-zinc-500">暂无租户</span>
              ) : (
                tenantListQuery.data?.map((tenant) => (
                  <Button
                    key={tenant.tenant_id}
                    type="button"
                    variant={tenant.tenant_id === normalizedTenantId ? "default" : "outline"}
                    onClick={() => {
                      setTenantInput(tenant.tenant_id);
                      setTenantId(tenant.tenant_id);
                      setEditingModule("");
                      setError("");
                    }}
                    className={[
                      "h-auto shrink-0 justify-start px-3 py-2 text-left text-xs",
                      tenant.tenant_id === normalizedTenantId
                        ? "text-white"
                        : "text-zinc-700",
                    ].join(" ")}
                  >
                    <div className="font-mono">{tenant.tenant_id}</div>
                    <div className="mt-1 opacity-80">
                      {tenant.active_count}/{tenant.entitlement_count} 已开通
                    </div>
                  </Button>
                ))
              )}
            </div>
          </div>
          <Table>
            <TableHeader>
              <TableRow className="bg-zinc-50 text-xs font-semibold uppercase text-zinc-500 hover:bg-zinc-50">
                <TableHead className="px-4 py-3">模块</TableHead>
                <TableHead className="px-4 py-3">状态</TableHead>
                <TableHead className="px-4 py-3">有效期</TableHead>
                <TableHead className="px-4 py-3">额度</TableHead>
                <TableHead className="px-4 py-3">用量</TableHead>
                <TableHead className="px-4 py-3 text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
                {query.isLoading ? (
                  <TableRow>
                    <TableCell className="px-4 py-6 text-zinc-500" colSpan={6}>
                      加载中...
                    </TableCell>
                  </TableRow>
                ) : (summary?.entitlements ?? []).length === 0 ? (
                  <TableRow>
                    <TableCell className="px-4 py-6 text-zinc-500" colSpan={6}>
                      暂无模块
                    </TableCell>
                  </TableRow>
                ) : (
                  summary?.entitlements.map((view) => (
                    <TableRow key={view.module.code} className="align-top">
                      <TableCell className="px-4 py-3">
                        <div className="font-medium text-zinc-950">{view.module.name}</div>
                        <div className="font-mono text-xs text-zinc-500">
                          {view.module.code}
                        </div>
                      </TableCell>
                      <TableCell className="px-4 py-3">
                        <StatusBadge view={view} />
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {formatDate(view.entitlement?.expires_at)}
                      </TableCell>
                      <TableCell className="px-4 py-3 font-mono text-xs text-zinc-600">
                        {formatRecord(view.limits)}
                      </TableCell>
                      <TableCell className="px-4 py-3 font-mono text-xs text-zinc-600">
                        {formatRecord(view.used)}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-right">
                        <Button
                          type="button"
                          variant="outline"
                          onClick={() => beginEdit(view)}
                          className="h-8 gap-2 px-3"
                        >
                          <Power className="size-4" />
                          配置
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
            </TableBody>
          </Table>
        </Card>

        <div className="space-y-4">
          <Card>
            <form onSubmit={handlePlanApply}>
              <CardHeader className="p-4 pb-0">
                <CardTitle className="text-base">套餐开通</CardTitle>
                <CardDescription className="mt-1 text-xs">
                  当前套餐：{summary?.current_plan?.plan.name ?? "未配置"}
                </CardDescription>
              </CardHeader>
              <CardContent className="p-4">
              <Label className="mb-3 block text-xs text-zinc-500">
                套餐
                <Select
                  value={selectedPlan}
                  onChange={(event) => setSelectedPlan(event.target.value)}
                  className="mt-1 h-9"
                >
                  <option value="">选择套餐</option>
                  {(planQuery.data ?? []).map((bundle) => (
                    <option key={bundle.plan.code} value={bundle.plan.code}>
                      {bundle.plan.name}
                    </option>
                  ))}
                </Select>
              </Label>
              <Label className="mb-3 block text-xs text-zinc-500">
                过期时间
                <Input
                  type="datetime-local"
                  value={planExpiresAt}
                  onChange={(event) => setPlanExpiresAt(event.target.value)}
                  className="mt-1 h-9"
                />
              </Label>
              <Button
                type="submit"
                disabled={!normalizedTenantId || !selectedPlan || applyingPlan}
                className="h-9 w-full gap-2 px-3"
              >
                {applyingPlan ? <RefreshCw className="size-4 animate-spin" /> : <Save className="size-4" />}
                应用套餐
              </Button>
              </CardContent>
            </form>
          </Card>

        <Card>
          <form onSubmit={handleSave}>
            <CardHeader className="p-4 pb-0">
              <CardTitle className="text-base">模块开通</CardTitle>
            </CardHeader>
            <CardContent className="p-4">
            <Label className="mb-3 block text-xs text-zinc-500">
              模块
              <Input
                value={editingModule}
                readOnly
                className="mt-1 h-9 bg-zinc-50 font-mono"
                placeholder="选择模块"
              />
            </Label>
            <Label className="mb-3 block text-xs text-zinc-500">
              状态
              <Select
                value={status}
                onChange={(event) => setStatus(event.target.value as SubscriptionStatus)}
                className="mt-1 h-9"
              >
                {STATUS_OPTIONS.map((option) => (
                  <option key={option} value={option}>
                    {STATUS_LABEL[option]}
                  </option>
                ))}
              </Select>
            </Label>
            <Label className="mb-3 block text-xs text-zinc-500">
              过期时间
              <Input
                type="datetime-local"
                value={expiresAt}
                onChange={(event) => setExpiresAt(event.target.value)}
                className="mt-1 h-9"
              />
            </Label>
            <div className="mb-3">
              <Label
                htmlFor="subscription-limits-json"
                className="block text-xs text-zinc-500"
              >
                额度 JSON
              </Label>
              {editingModule === "oss_storage" ? (
                <div className="mt-2 flex flex-wrap gap-2">
                  {OSS_STORAGE_LIMIT_PRESETS.map((preset) => (
                    <Button
                      key={preset.label}
                      type="button"
                      variant="outline"
                      onClick={() =>
                        setLimitsText(setStorageLimitPresetBytes(limitsText, preset.bytes))
                      }
                      className="h-8 px-3 text-xs"
                    >
                      {preset.label}
                    </Button>
                  ))}
                </div>
              ) : null}
              <Textarea
                id="subscription-limits-json"
                value={limitsText}
                onChange={(event) => setLimitsText(event.target.value)}
                rows={7}
                className="mt-1 font-mono text-xs"
              />
            </div>
            <Button
              type="submit"
              disabled={!editingModule || !normalizedTenantId || saving}
              className="h-9 w-full gap-2 px-3"
            >
              {saving ? <RefreshCw className="size-4 animate-spin" /> : <Save className="size-4" />}
              保存配置
            </Button>
            </CardContent>
          </form>
          <Separator className="my-4" />
          <CardHeader className="p-4 pb-0">
            <CardTitle className="text-base">用量调整</CardTitle>
          </CardHeader>
          <CardContent className="p-4">
          <form onSubmit={handleUsageSave} className="space-y-3">
            <Label className="block text-xs text-zinc-500">
              周期
              <Input
                value={usagePeriod}
                onChange={(event) => setUsagePeriod(event.target.value)}
                className="mt-1 h-9 font-mono"
                placeholder="YYYY-MM"
              />
            </Label>
            <Label className="block text-xs text-zinc-500">
              指标
              <Input
                value={usageMetric}
                onChange={(event) => setUsageMetric(event.target.value)}
                className="mt-1 h-9 font-mono"
                placeholder="design_jobs"
              />
            </Label>
            <Label className="block text-xs text-zinc-500">
              已用
              <Input
                type="number"
                min={0}
                value={usageUsed}
                onChange={(event) => setUsageUsed(event.target.value)}
                className="mt-1 h-9"
              />
            </Label>
            <Label className="block text-xs text-zinc-500">
              原因
              <Input
                value={usageReason}
                onChange={(event) => setUsageReason(event.target.value)}
                className="mt-1 h-9"
                placeholder="运营调整"
              />
            </Label>
            <div className="grid grid-cols-2 gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={!editingModule || !normalizedTenantId || savingUsage}
                onClick={() => setUsageUsed("0")}
                className="h-9 px-3"
              >
                重置为 0
              </Button>
              <Button
                type="submit"
                disabled={!editingModule || !normalizedTenantId || savingUsage}
                className="h-9 gap-2 px-3"
              >
                {savingUsage ? <RefreshCw className="size-4 animate-spin" /> : <Save className="size-4" />}
                保存用量
              </Button>
            </div>
          </form>
          </CardContent>
        </Card>
        </div>
      </section>
      {normalizedTenantId ? (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between p-4 pb-0">
            <CardTitle className="text-base">审计日志</CardTitle>
            <Button
              type="button"
              variant="outline"
              onClick={() => void auditQuery.refetch()}
              className="h-8 gap-2 px-3 text-xs"
            >
              <RefreshCw className={`size-3.5 ${auditQuery.isFetching ? "animate-spin" : ""}`} />
              刷新
            </Button>
          </CardHeader>
          <CardContent className="divide-y divide-zinc-100 p-4 text-sm">
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
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}

function setStorageLimitPresetBytes(limitsText: string, bytes: number) {
  const limits = parseLimits(limitsText);
  return JSON.stringify({ ...limits, storage_bytes: bytes }, null, 2);
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
    .map(([key, count]) => `${key}: ${formatMetricValue(key, count)}`)
    .join(", ");
}

function formatMetricValue(key: string, value: number) {
  if (key === "storage_bytes" || key.endsWith("_bytes")) {
    return formatBytes(value);
  }
  return String(value);
}

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) {
    return "0 B";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  const maximumFractionDigits = unitIndex === 0 ? 0 : 1;
  return `${new Intl.NumberFormat("zh-CN", { maximumFractionDigits }).format(size)} ${units[unitIndex]}`;
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

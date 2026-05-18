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
import {
  formatSubscriptionDate,
  formatSubscriptionRecord,
  subscriptionMetricDisplayName,
  subscriptionMetricUnit,
  subscriptionModuleSummary,
} from "@/components/listingkit/subscription/subscription-display";

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

const MODULE_GUIDANCE: Record<string, { recommendedMetrics?: Array<{ key: string; label: string; unit?: string }> }> = {
  store_management: {
  },
  task_import: {
  },
  rules: {
  },
  operation_strategy: {
  },
  studio: {
    recommendedMetrics: [{ key: "design_jobs", label: "设计任务额度", unit: "次" }],
  },
  oss_storage: {
    recommendedMetrics: [{ key: "storage_bytes", label: "存储额度", unit: "字节" }],
  },
};

export function PlatformSubscriptionPage() {
  const [tenantInput, setTenantInput] = useState("");
  const [tenantId, setTenantId] = useState("");
  const [editingModule, setEditingModule] = useState("");
  const [status, setStatus] = useState<SubscriptionStatus>("active");
  const [expiresAt, setExpiresAt] = useState("");
  const [limitsText, setLimitsText] = useState("{}");
  const [limitDraft, setLimitDraft] = useState<Record<string, number>>({});
  const [newLimitKey, setNewLimitKey] = useState("");
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
  const editingView = summary?.entitlements.find((view) => view.module.code === editingModule);
  const editingGuidance = editingModule ? MODULE_GUIDANCE[editingModule] : undefined;
  const recommendedMetrics = editingGuidance?.recommendedMetrics ?? [];
  const tenantOptions = tenantListQuery.data ?? [];
  const tenantKeyword = tenantInput.trim().toLowerCase();
  const visibleTenants = tenantKeyword
    ? tenantOptions.filter((tenant) => {
        const displayName = tenant.tenant_display_name?.toLowerCase() ?? "";
        return displayName.includes(tenantKeyword) || tenant.tenant_id.toLowerCase().includes(tenantKeyword);
      })
    : tenantOptions;
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
    const selectedPlanName =
      planQuery.data?.find((bundle) => bundle.plan.code === selectedPlan)?.plan.name ?? selectedPlan;
    if (
      typeof window !== "undefined" &&
      !window.confirm(`确认给租户 ${normalizedTenantId} 应用套餐“${selectedPlanName}”吗？`)
    ) {
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
    const nextLimits = view.entitlement?.limits ?? {};
    setLimitDraft(nextLimits);
    setLimitsText(JSON.stringify(nextLimits, null, 2));
    setNewLimitKey("");
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
    const nextLimits = parseLimits(limitsText);
    if (
      typeof window !== "undefined" &&
      !window.confirm(`确认保存租户 ${normalizedTenantId} 的模块 ${editingModule} 配置吗？`)
    ) {
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
          limits: nextLimits,
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
    if (
      typeof window !== "undefined" &&
      !window.confirm(
        `确认把租户 ${normalizedTenantId} 的模块 ${editingModule} 在周期 ${usagePeriod || currentPeriodKey()} 的指标 ${usageMetric} 用量更新为 ${used} 吗？`,
      )
    ) {
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

  function updateLimitDraft(nextDraft: Record<string, number>) {
    setLimitDraft(nextDraft);
    setLimitsText(JSON.stringify(nextDraft, null, 2));
  }

  function handleLimitValueChange(key: string, rawValue: string) {
    const nextValue = Number(rawValue);
    updateLimitDraft({
      ...limitDraft,
      [key]: Number.isFinite(nextValue) && nextValue >= 0 ? nextValue : 0,
    });
  }

  function handleLimitKeyRename(oldKey: string, nextKey: string) {
    const normalizedKey = nextKey.trim();
    const nextDraft = { ...limitDraft };
    const currentValue = nextDraft[oldKey];
    delete nextDraft[oldKey];
    if (normalizedKey) {
      nextDraft[normalizedKey] = currentValue;
    }
    updateLimitDraft(nextDraft);
  }

  function handleAddLimitMetric() {
    const normalizedKey = newLimitKey.trim();
    if (!normalizedKey || normalizedKey in limitDraft) {
      return;
    }
    updateLimitDraft({ ...limitDraft, [normalizedKey]: 0 });
    setNewLimitKey("");
  }

  function handleRemoveLimitMetric(key: string) {
    const nextDraft = { ...limitDraft };
    delete nextDraft[key];
    updateLimitDraft(nextDraft);
  }

  function handleLimitsTextChange(value: string) {
    setLimitsText(value);
    try {
      setLimitDraft(parseLimits(value));
    } catch {
      // Keep the structured editor on the last valid value until JSON is fixed.
    }
  }

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <CardTitle className="text-2xl">租户订阅管理</CardTitle>
            <CardDescription className="mt-1">
              推荐先按套餐开通；只有在补差或排障时，再单独调整模块额度和用量。
            </CardDescription>
          </div>
          <form onSubmit={handleLoad} className="flex flex-col gap-2 sm:flex-row">
            <Input
              value={tenantInput}
              onChange={(event) => setTenantInput(event.target.value)}
              className="h-9 min-w-[260px] font-mono"
              placeholder="搜索或输入租户 ID"
              aria-label="租户 ID"
              list="listingkit-tenant-options"
            />
            <datalist id="listingkit-tenant-options">
              {tenantOptions.map((tenant) => (
                <option key={tenant.tenant_id} value={tenant.tenant_id} />
              ))}
            </datalist>
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
        <CardContent className="pt-0">
          <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-600">
            如果你不知道租户 ID，优先通过输入框搜索或从下方列表选择；只有列表里没有时，再手动输入租户 ID。
          </div>
        </CardContent>
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
              ) : tenantOptions.length === 0 ? (
                <span className="text-sm text-zinc-500">暂无租户</span>
              ) : visibleTenants.length === 0 ? (
                <span className="text-sm text-zinc-500">没有匹配的租户，请继续手动输入完整租户 ID。</span>
              ) : (
                visibleTenants.map((tenant) => (
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
                    <div className="font-medium">
                      {tenantDisplayName(tenant.tenant_display_name, tenant.tenant_id)}
                    </div>
                    {tenant.tenant_display_name ? (
                      <div className="mt-1 font-mono opacity-70">{tenant.tenant_id}</div>
                    ) : null}
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
                        <div className="mt-1 text-xs text-zinc-500">
                          {subscriptionModuleSummary(view.module.code, view.module.description)}
                        </div>
                      </TableCell>
                      <TableCell className="px-4 py-3">
                        <StatusBadge view={view} />
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {formatSubscriptionDate(view.entitlement?.expires_at)}
                      </TableCell>
                      <TableCell className="px-4 py-3 font-mono text-xs text-zinc-600">
                        {formatSubscriptionRecord(view.limits)}
                      </TableCell>
                      <TableCell className="px-4 py-3 font-mono text-xs text-zinc-600">
                        {formatSubscriptionRecord(view.used)}
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
              <p className="mb-3 text-xs text-zinc-500">
                适合整租户开通。应用套餐会按套餐定义覆盖该租户的模块开通状态与默认额度。
              </p>
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
              <CardDescription className="mt-1 text-xs">
                仅用于个别模块补差或临时调整。优先使用套餐开通。
              </CardDescription>
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
            {editingView ? (
              <div className="mb-3 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-xs text-zinc-600">
                <div className="font-medium text-zinc-800">{editingView.module.name}</div>
                <div className="mt-1">{subscriptionModuleSummary(editingView.module.code, editingView.module.description)}</div>
              </div>
            ) : null}
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
              <Label className="block text-xs text-zinc-500">额度配置</Label>
              <div className="mt-2 space-y-2 rounded-lg border border-zinc-200 bg-zinc-50 p-3">
                <p className="text-xs text-zinc-500">
                  可以直接维护“指标 + 数值”。只有复杂场景才需要展开 JSON 高级模式。
                </p>
                {recommendedMetrics.length > 0 ? (
                  <div className="flex flex-wrap gap-2">
                    {recommendedMetrics.map((metric) => (
                      <Button
                        key={metric.key}
                        type="button"
                        variant="outline"
                        onClick={() => {
                          if (!(metric.key in limitDraft)) {
                            updateLimitDraft({ ...limitDraft, [metric.key]: 0 });
                          }
                        }}
                        className="h-8 px-3 text-xs"
                        disabled={metric.key in limitDraft}
                      >
                        添加 {metric.label}
                      </Button>
                    ))}
                  </div>
                ) : null}
                {(Object.keys(limitDraft).length > 0 ||
                  editingGuidance?.recommendedMetrics?.length) ? (
                  <div className="space-y-2">
                    {Object.entries(limitDraft).map(([key, value]) => (
                      <div key={key} className="grid gap-2 md:grid-cols-[minmax(0,1fr)_160px_84px]">
                        <div className="rounded-lg border border-zinc-200 bg-white px-3 py-2">
                            <div className="text-xs font-medium text-zinc-800">
                            {subscriptionMetricDisplayName(key)}
                          </div>
                          <div className="mt-1 font-mono text-[11px] text-zinc-500">{key}</div>
                        </div>
                        <Input
                          aria-label={`额度值 ${key}`}
                          type="number"
                          min={0}
                          value={String(value)}
                          onChange={(event) => handleLimitValueChange(key, event.target.value)}
                          className="h-9"
                          placeholder={subscriptionMetricUnit(key) ? `单位：${subscriptionMetricUnit(key)}` : undefined}
                        />
                        <Button
                          type="button"
                          variant="outline"
                          onClick={() => handleRemoveLimitMetric(key)}
                          className="h-9 px-3 text-xs"
                        >
                          删除
                        </Button>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-xs text-zinc-500">当前模块暂未配置额度项，可按需新增。</p>
                )}
                <div className="grid gap-2 md:grid-cols-[minmax(0,1fr)_96px]">
                  <Input
                    aria-label="新增额度指标"
                    value={newLimitKey}
                    onChange={(event) => setNewLimitKey(event.target.value)}
                    className="h-9 font-mono text-xs"
                    placeholder="输入内部指标 key"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    onClick={handleAddLimitMetric}
                    disabled={!newLimitKey.trim()}
                    className="h-9 px-3 text-xs"
                  >
                    新增指标
                  </Button>
                </div>
              </div>
              {editingModule === "oss_storage" ? (
                <div className="mt-2 flex flex-wrap gap-2">
                  {OSS_STORAGE_LIMIT_PRESETS.map((preset) => (
                    <Button
                      key={preset.label}
                      type="button"
                      variant="outline"
                      onClick={() => updateLimitDraft(setStorageLimitPresetBytes(limitDraft, preset.bytes))}
                      className="h-8 px-3 text-xs"
                    >
                      {preset.label}
                    </Button>
                  ))}
                </div>
              ) : null}
              <details className="mt-3 rounded-lg border border-dashed border-zinc-200 bg-white p-3">
                <summary className="cursor-pointer text-xs font-medium text-zinc-600">
                  高级模式：直接编辑 JSON
                </summary>
                <Label
                  htmlFor="subscription-limits-json"
                  className="mt-3 block text-xs text-zinc-500"
                >
                  额度 JSON
                </Label>
                <Textarea
                  id="subscription-limits-json"
                  value={limitsText}
                  onChange={(event) => handleLimitsTextChange(event.target.value)}
                  rows={7}
                  className="mt-1 font-mono text-xs"
                />
              </details>
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
          <div className="p-4 pt-0">
            <details className="rounded-lg border border-dashed border-zinc-200 bg-zinc-50 p-3">
              <summary className="cursor-pointer text-sm font-medium text-zinc-700">
                高级操作：用量调整
              </summary>
              <p className="mt-2 text-xs text-zinc-500">
                仅用于纠正计数异常或人工补录，不建议作为日常运营入口。
              </p>
              <form onSubmit={handleUsageSave} className="mt-3 space-y-3">
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
                    placeholder={recommendedMetrics[0]?.key ?? "design_jobs"}
                  />
                  {usageMetric ? (
                    <div className="mt-1 text-xs text-zinc-500">
                      {subscriptionMetricDisplayName(usageMetric)}
                      {subscriptionMetricUnit(usageMetric) ? `，单位：${subscriptionMetricUnit(usageMetric)}` : ""}
                    </div>
                  ) : null}
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
            </details>
          </div>
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
                  <div className="text-zinc-500">{formatSubscriptionDate(item.created_at)}</div>
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

function setStorageLimitPresetBytes(limits: Record<string, number>, bytes: number) {
  return { ...limits, storage_bytes: bytes };
}

function tenantDisplayName(displayName: string | undefined, tenantId: string) {
  const normalizedDisplayName = displayName?.trim();
  return normalizedDisplayName || tenantId;
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

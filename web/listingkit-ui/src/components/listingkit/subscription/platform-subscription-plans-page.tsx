"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { useQuery } from "@tanstack/react-query";
import { Plus, RefreshCw, Save, Trash2 } from "lucide-react";
import { FormEvent, useMemo, useState } from "react";

import {
  deletePlatformSubscriptionPlanModule,
  formatSubscriptionApiError,
  getPlatformSubscriptionPlans,
  getPlatformSubscriptionPlanAuditLogs,
  getPlatformSubscriptionPlanTenants,
  getSubscriptionModules,
  setPlatformSubscriptionPlanStatus,
  type SubscriptionPlanBundle,
  updatePlatformSubscriptionPlan,
  updatePlatformSubscriptionPlanModule,
  upsertPlatformSubscriptionPlan,
} from "@/lib/api/subscription";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";

type PlanFormState = {
  code: string;
  name: string;
  description: string;
  sortOrder: string;
  active: boolean;
};

const EMPTY_PLAN: PlanFormState = {
  code: "",
  name: "",
  description: "",
  sortOrder: "0",
  active: true,
};

function formatLimits(limits?: Record<string, number>) {
  if (!limits || Object.keys(limits).length === 0) {
    return "{}";
  }
  return JSON.stringify(limits);
}

function parseLimits(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return {};
  }
  const parsed = JSON.parse(trimmed) as Record<string, unknown>;
  return Object.fromEntries(
    Object.entries(parsed).map(([key, raw]) => [key, Number(raw)]),
  );
}

export function PlatformSubscriptionPlansPage() {
  const [selectedCode, setSelectedCode] = useState("");
  const [planForm, setPlanForm] = useState<PlanFormState>(EMPTY_PLAN);
  const [moduleCode, setModuleCode] = useState("");
  const [moduleSortOrder, setModuleSortOrder] = useState("0");
  const [moduleLimits, setModuleLimits] = useState("{}");
  const [savingPlan, setSavingPlan] = useState(false);
  const [savingModule, setSavingModule] = useState(false);
  const [error, setError] = useState("");

  const planQuery = useQuery({
    queryKey: ["listingkit-platform-subscription-plans"],
    queryFn: getPlatformSubscriptionPlans,
  });
  const moduleQuery = useQuery({
    queryKey: ["listingkit-subscription-modules"],
    queryFn: getSubscriptionModules,
  });
  const planTenantsQuery = useQuery({
    queryKey: ["listingkit-platform-subscription-plan-tenants", selectedCode],
    queryFn: () => getPlatformSubscriptionPlanTenants(selectedCode),
    enabled: Boolean(selectedCode),
  });
  const planAuditQuery = useQuery({
    queryKey: ["listingkit-platform-subscription-plan-audit", selectedCode],
    queryFn: () => getPlatformSubscriptionPlanAuditLogs(selectedCode),
    enabled: Boolean(selectedCode),
  });

  const plans = useMemo(() => planQuery.data ?? [], [planQuery.data]);
  const selectedPlan = useMemo(
    () => plans.find((item) => item.plan.code === selectedCode),
    [plans, selectedCode],
  );
  const visibleError =
    error ||
    (planQuery.error ? formatSubscriptionApiError(planQuery.error) : "") ||
    (moduleQuery.error ? formatSubscriptionApiError(moduleQuery.error) : "") ||
    (planTenantsQuery.error
      ? formatSubscriptionApiError(planTenantsQuery.error)
      : "") ||
    (planAuditQuery.error
      ? formatSubscriptionApiError(planAuditQuery.error)
      : "");

  function selectPlan(bundle: SubscriptionPlanBundle) {
    setSelectedCode(bundle.plan.code);
    setPlanForm({
      code: bundle.plan.code,
      name: bundle.plan.name,
      description: bundle.plan.description ?? "",
      sortOrder: String(bundle.plan.sort_order),
      active: bundle.plan.active,
    });
    setModuleCode(bundle.modules[0]?.module_code ?? "");
    setModuleSortOrder(String(bundle.modules[0]?.sort_order ?? 0));
    setModuleLimits(formatLimits(bundle.modules[0]?.limits));
    setError("");
  }

  async function refreshPlans() {
    await planQuery.refetch();
  }

  async function handlePlanSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSavingPlan(true);
    setError("");
    try {
      const input = {
        code: planForm.code.trim(),
        name: planForm.name.trim(),
        description: planForm.description.trim() || undefined,
        sort_order: Number(planForm.sortOrder || 0),
        active: planForm.active,
        modules: selectedPlan?.modules.map((module) => ({
          module_code: module.module_code,
          limits: module.limits,
          sort_order: module.sort_order,
        })),
      };
      const result = selectedCode
        ? await updatePlatformSubscriptionPlan(selectedCode, input)
        : await upsertPlatformSubscriptionPlan(input);
      setSelectedCode(result.plan.code);
      await refreshPlans();
      await planAuditQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSavingPlan(false);
    }
  }

  async function handleStatusChange(bundle: SubscriptionPlanBundle) {
    setError("");
    try {
      await setPlatformSubscriptionPlanStatus(
        bundle.plan.code,
        !bundle.plan.active,
      );
      await refreshPlans();
      await planAuditQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  async function handleModuleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedCode || !moduleCode) {
      return;
    }
    setSavingModule(true);
    setError("");
    try {
      await updatePlatformSubscriptionPlanModule(selectedCode, moduleCode, {
        limits: parseLimits(moduleLimits),
        sort_order: Number(moduleSortOrder || 0),
      });
      await refreshPlans();
      await planAuditQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSavingModule(false);
    }
  }

  async function handleModuleDelete(moduleCodeToDelete: string) {
    if (!selectedCode) {
      return;
    }
    setError("");
    try {
      await deletePlatformSubscriptionPlanModule(
        selectedCode,
        moduleCodeToDelete,
      );
      await refreshPlans();
      await planAuditQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  return (
    <div className="space-y-5">
      <section className="flex flex-col gap-3 border-b border-zinc-200 pb-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <p className="text-sm font-medium text-zinc-500">租户订阅管理</p>
          <h1 className="mt-1 text-2xl font-semibold text-zinc-950">
            套餐管理
          </h1>
        </div>
        <Button
          type="button"
          variant="outline"
          onClick={() => {
            setSelectedCode("");
            setPlanForm(EMPTY_PLAN);
            setModuleCode("");
            setModuleSortOrder("0");
            setModuleLimits("{}");
          }}
          className="h-9 w-full gap-2 px-3 sm:w-auto"
        >
          <Plus className="size-4" />
          新建套餐
        </Button>
      </section>

      {visibleError ? (
        <Alert variant="destructive">
          <AlertDescription>{visibleError}</AlertDescription>
        </Alert>
      ) : null}

      <section className="grid gap-4 2xl:grid-cols-[minmax(0,1fr)_420px]">
        <Card className="overflow-hidden p-0">
          <div className="overflow-x-auto">
          <div className="grid min-w-[40rem] grid-cols-[1fr_96px_88px_190px] border-b border-zinc-100 px-4 py-2 text-xs font-semibold text-zinc-500">
            <span>套餐</span>
            <span>状态</span>
            <span>模块数</span>
            <span className="text-right">操作</span>
          </div>
          {plans.map((bundle) => (
            <div
              key={bundle.plan.code}
              className="grid min-w-[40rem] grid-cols-[1fr_96px_88px_190px] items-center gap-3 border-b border-zinc-100 px-4 py-3 last:border-b-0"
            >
              <div className="min-w-0">
                <p className="font-medium text-zinc-950">{bundle.plan.name}</p>
                <p className="truncate font-mono text-xs text-zinc-500">
                  {bundle.plan.code}
                </p>
                {bundle.plan.description ? (
                  <p className="mt-1 text-sm text-zinc-500">
                    {bundle.plan.description}
                  </p>
                ) : null}
              </div>
              <span className="text-sm text-zinc-700">
                {bundle.plan.active ? "启用" : "禁用"}
              </span>
              <span className="text-sm text-zinc-700">
                {bundle.modules.length}
              </span>
              <div className="flex justify-end gap-2">
                <Button
                  type="button"
                  variant="outline"
                  aria-label={`编辑 ${bundle.plan.code}`}
                  onClick={() => selectPlan(bundle)}
                  className="h-8 px-3"
                >
                  编辑
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  aria-label={`${bundle.plan.active ? "禁用" : "启用"} ${bundle.plan.code}`}
                  onClick={() => handleStatusChange(bundle)}
                  className="h-8 px-3"
                >
                  {bundle.plan.active ? "禁用" : "启用"}
                </Button>
              </div>
            </div>
          ))}
          </div>
        </Card>

        <div className="space-y-4">
          <form
            onSubmit={handlePlanSubmit}
            className="block"
          >
            <Card>
              <CardHeader className="p-4 pb-0">
                <CardTitle className="text-base">套餐资料</CardTitle>
              </CardHeader>
              <CardContent className="grid gap-3 p-4">
              <Label className="text-xs text-zinc-500">
                套餐编码
                <Input
                  value={planForm.code}
                  onChange={(event) =>
                    setPlanForm((current) => ({
                      ...current,
                      code: event.target.value,
                    }))
                  }
                  disabled={Boolean(selectedCode)}
                  className="mt-1 h-9 disabled:bg-zinc-100"
                />
              </Label>
              <Label className="text-xs text-zinc-500">
                套餐名称
                <Input
                  value={planForm.name}
                  onChange={(event) =>
                    setPlanForm((current) => ({
                      ...current,
                      name: event.target.value,
                    }))
                  }
                  className="mt-1 h-9"
                />
              </Label>
              <Label className="text-xs text-zinc-500">
                描述
                <Textarea
                  value={planForm.description}
                  onChange={(event) =>
                    setPlanForm((current) => ({
                      ...current,
                      description: event.target.value,
                    }))
                  }
                  rows={2}
                  className="mt-1 min-h-20"
                />
              </Label>
              <Label className="text-xs text-zinc-500">
                排序
                <Input
                  type="number"
                  value={planForm.sortOrder}
                  onChange={(event) =>
                    setPlanForm((current) => ({
                      ...current,
                      sortOrder: event.target.value,
                    }))
                  }
                  className="mt-1 h-9"
                />
              </Label>
              <Label className="inline-flex items-center gap-2 text-sm text-zinc-700">
                <Checkbox
                  checked={planForm.active}
                  onChange={(event) =>
                    setPlanForm((current) => ({
                      ...current,
                      active: event.target.checked,
                    }))
                  }
                />
                启用套餐
              </Label>
            <Button
              type="submit"
              disabled={savingPlan || !planForm.code.trim() || !planForm.name.trim()}
              className="h-9 w-full gap-2 px-3"
            >
              {savingPlan ? (
                <RefreshCw className="size-4 animate-spin" />
              ) : (
                <Save className="size-4" />
              )}
              保存套餐
            </Button>
              </CardContent>
            </Card>
          </form>

          <form
            onSubmit={handleModuleSubmit}
            className="block"
          >
            <Card>
              <CardHeader className="p-4 pb-0">
                <CardTitle className="text-base">套餐模块</CardTitle>
              </CardHeader>
              <CardContent className="grid gap-3 p-4">
              <Label className="text-xs text-zinc-500">
                模块
                <Select
                  value={moduleCode}
                  onChange={(event) => setModuleCode(event.target.value)}
                  className="mt-1 h-9"
                >
                  <option value="">选择模块</option>
                  {(moduleQuery.data ?? []).map((module) => (
                    <option key={module.code} value={module.code}>
                      {module.name}
                    </option>
                  ))}
                </Select>
              </Label>
              <Label className="text-xs text-zinc-500">
                模块排序
                <Input
                  type="number"
                  value={moduleSortOrder}
                  onChange={(event) => setModuleSortOrder(event.target.value)}
                  className="mt-1 h-9"
                />
              </Label>
              <Label className="text-xs text-zinc-500">
                模块额度 JSON
                <Textarea
                  value={moduleLimits}
                  onChange={(event) => setModuleLimits(event.target.value)}
                  rows={3}
                  className="mt-1 font-mono"
                />
              </Label>
            <Button
              type="submit"
              disabled={savingModule || !selectedCode || !moduleCode}
              className="h-9 w-full gap-2 px-3"
            >
              {savingModule ? (
                <RefreshCw className="size-4 animate-spin" />
              ) : (
                <Save className="size-4" />
              )}
              保存模块
            </Button>
            {selectedPlan ? (
              <div className="mt-4 space-y-2">
                {selectedPlan.modules.map((module) => (
                  <div
                    key={module.module_code}
                    className="flex items-center justify-between gap-3 rounded-md border border-zinc-100 px-3 py-2"
                  >
                    <div className="min-w-0">
                      <p className="font-mono text-xs text-zinc-800">
                        {module.module_code}
                      </p>
                      <p className="truncate font-mono text-xs text-zinc-500">
                        {formatLimits(module.limits)}
                      </p>
                    </div>
                    <Button
                      type="button"
                      variant="outline"
                      aria-label={`移除 ${module.module_code}`}
                      onClick={() => handleModuleDelete(module.module_code)}
                      className="size-8 p-0 text-zinc-600 hover:border-red-200 hover:text-red-600"
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                ))}
              </div>
            ) : null}
              </CardContent>
            </Card>
          </form>

          {selectedCode ? (
            <Card>
              <CardHeader className="p-4 pb-0">
                <CardTitle className="text-base">使用租户</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2 p-4">
                {(planTenantsQuery.data ?? []).length === 0 ? (
                  <p className="text-sm text-zinc-500">暂无租户使用该套餐</p>
                ) : (
                  (planTenantsQuery.data ?? []).map((tenant) => (
                    <div
                      key={tenant.tenant_id}
                      className="rounded-md border border-zinc-100 px-3 py-2"
                    >
                      <p className="font-mono text-xs text-zinc-900">
                        {tenant.tenant_id}
                      </p>
                      <p className="mt-1 text-xs text-zinc-500">
                        {tenant.status}
                        {tenant.expires_at ? ` · ${tenant.expires_at}` : ""}
                      </p>
                    </div>
                  ))
                )}
              </CardContent>
            </Card>
          ) : null}

          {selectedCode ? (
            <Card>
              <CardHeader className="p-4 pb-0">
                <CardTitle className="text-base">套餐审计</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2 p-4">
                {(planAuditQuery.data ?? []).length === 0 ? (
                  <p className="text-sm text-zinc-500">暂无套餐变更记录</p>
                ) : (
                  (planAuditQuery.data ?? []).map((log) => (
                    <div
                      key={log.id}
                      className="rounded-md border border-zinc-100 px-3 py-2"
                    >
                      <p className="text-sm font-medium text-zinc-900">
                        {log.action}
                      </p>
                      <p className="mt-1 font-mono text-xs text-zinc-500">
                        {log.tenant_id || log.module_code || log.reason}
                      </p>
                    </div>
                  ))
                )}
              </CardContent>
            </Card>
          ) : null}
        </div>
      </section>
    </div>
  );
}

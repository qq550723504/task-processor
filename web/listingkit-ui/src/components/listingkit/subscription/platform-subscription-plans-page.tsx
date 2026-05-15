"use client";

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
      <section className="flex flex-col gap-3 border-b border-zinc-200 pb-4 md:flex-row md:items-end md:justify-between">
        <div>
          <p className="text-sm font-medium text-zinc-500">平台订阅</p>
          <h1 className="mt-1 text-2xl font-semibold text-zinc-950">
            套餐管理
          </h1>
        </div>
        <button
          type="button"
          onClick={() => {
            setSelectedCode("");
            setPlanForm(EMPTY_PLAN);
            setModuleCode("");
            setModuleSortOrder("0");
            setModuleLimits("{}");
          }}
          className="inline-flex h-9 items-center justify-center gap-2 rounded-md border border-zinc-200 bg-white px-3 text-sm font-medium text-zinc-800 hover:border-zinc-300"
        >
          <Plus className="size-4" />
          新建套餐
        </button>
      </section>

      {visibleError ? (
        <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {visibleError}
        </div>
      ) : null}

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_420px]">
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white">
          <div className="grid grid-cols-[1fr_96px_88px_190px] border-b border-zinc-100 px-4 py-2 text-xs font-semibold text-zinc-500">
            <span>套餐</span>
            <span>状态</span>
            <span>模块数</span>
            <span className="text-right">操作</span>
          </div>
          {plans.map((bundle) => (
            <div
              key={bundle.plan.code}
              className="grid grid-cols-[1fr_96px_88px_190px] items-center gap-3 border-b border-zinc-100 px-4 py-3 last:border-b-0"
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
                <button
                  type="button"
                  aria-label={`编辑 ${bundle.plan.code}`}
                  onClick={() => selectPlan(bundle)}
                  className="inline-flex h-8 items-center justify-center rounded-md border border-zinc-200 px-3 text-sm text-zinc-700 hover:border-zinc-300"
                >
                  编辑
                </button>
                <button
                  type="button"
                  aria-label={`${bundle.plan.active ? "禁用" : "启用"} ${bundle.plan.code}`}
                  onClick={() => handleStatusChange(bundle)}
                  className="inline-flex h-8 items-center justify-center rounded-md border border-zinc-200 px-3 text-sm text-zinc-700 hover:border-zinc-300"
                >
                  {bundle.plan.active ? "禁用" : "启用"}
                </button>
              </div>
            </div>
          ))}
        </div>

        <div className="space-y-4">
          <form
            onSubmit={handlePlanSubmit}
            className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm"
          >
            <h2 className="text-base font-semibold text-zinc-950">套餐资料</h2>
            <div className="mt-4 grid gap-3">
              <label className="text-xs font-medium text-zinc-500">
                套餐编码
                <input
                  value={planForm.code}
                  onChange={(event) =>
                    setPlanForm((current) => ({
                      ...current,
                      code: event.target.value,
                    }))
                  }
                  disabled={Boolean(selectedCode)}
                  className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900 disabled:bg-zinc-100"
                />
              </label>
              <label className="text-xs font-medium text-zinc-500">
                套餐名称
                <input
                  value={planForm.name}
                  onChange={(event) =>
                    setPlanForm((current) => ({
                      ...current,
                      name: event.target.value,
                    }))
                  }
                  className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
                />
              </label>
              <label className="text-xs font-medium text-zinc-500">
                描述
                <textarea
                  value={planForm.description}
                  onChange={(event) =>
                    setPlanForm((current) => ({
                      ...current,
                      description: event.target.value,
                    }))
                  }
                  rows={2}
                  className="mt-1 w-full rounded-md border border-zinc-200 px-3 py-2 text-sm text-zinc-900"
                />
              </label>
              <label className="text-xs font-medium text-zinc-500">
                排序
                <input
                  type="number"
                  value={planForm.sortOrder}
                  onChange={(event) =>
                    setPlanForm((current) => ({
                      ...current,
                      sortOrder: event.target.value,
                    }))
                  }
                  className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
                />
              </label>
              <label className="inline-flex items-center gap-2 text-sm text-zinc-700">
                <input
                  type="checkbox"
                  checked={planForm.active}
                  onChange={(event) =>
                    setPlanForm((current) => ({
                      ...current,
                      active: event.target.checked,
                    }))
                  }
                />
                启用套餐
              </label>
            </div>
            <button
              type="submit"
              disabled={savingPlan || !planForm.code.trim() || !planForm.name.trim()}
              className="mt-4 inline-flex h-9 w-full items-center justify-center gap-2 rounded-md bg-zinc-950 px-3 text-sm font-medium text-white hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-400"
            >
              {savingPlan ? (
                <RefreshCw className="size-4 animate-spin" />
              ) : (
                <Save className="size-4" />
              )}
              保存套餐
            </button>
          </form>

          <form
            onSubmit={handleModuleSubmit}
            className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm"
          >
            <h2 className="text-base font-semibold text-zinc-950">套餐模块</h2>
            <div className="mt-4 grid gap-3">
              <label className="text-xs font-medium text-zinc-500">
                模块
                <select
                  value={moduleCode}
                  onChange={(event) => setModuleCode(event.target.value)}
                  className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
                >
                  <option value="">选择模块</option>
                  {(moduleQuery.data ?? []).map((module) => (
                    <option key={module.code} value={module.code}>
                      {module.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="text-xs font-medium text-zinc-500">
                模块排序
                <input
                  type="number"
                  value={moduleSortOrder}
                  onChange={(event) => setModuleSortOrder(event.target.value)}
                  className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
                />
              </label>
              <label className="text-xs font-medium text-zinc-500">
                模块额度 JSON
                <textarea
                  value={moduleLimits}
                  onChange={(event) => setModuleLimits(event.target.value)}
                  rows={3}
                  className="mt-1 w-full rounded-md border border-zinc-200 px-3 py-2 font-mono text-sm text-zinc-900"
                />
              </label>
            </div>
            <button
              type="submit"
              disabled={savingModule || !selectedCode || !moduleCode}
              className="mt-4 inline-flex h-9 w-full items-center justify-center gap-2 rounded-md bg-zinc-950 px-3 text-sm font-medium text-white hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-400"
            >
              {savingModule ? (
                <RefreshCw className="size-4 animate-spin" />
              ) : (
                <Save className="size-4" />
              )}
              保存模块
            </button>
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
                    <button
                      type="button"
                      aria-label={`移除 ${module.module_code}`}
                      onClick={() => handleModuleDelete(module.module_code)}
                      className="inline-flex size-8 items-center justify-center rounded-md border border-zinc-200 text-zinc-600 hover:border-red-200 hover:text-red-600"
                    >
                      <Trash2 className="size-4" />
                    </button>
                  </div>
                ))}
              </div>
            ) : null}
          </form>

          {selectedCode ? (
            <section className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
              <h2 className="text-base font-semibold text-zinc-950">
                使用租户
              </h2>
              <div className="mt-3 space-y-2">
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
              </div>
            </section>
          ) : null}

          {selectedCode ? (
            <section className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
              <h2 className="text-base font-semibold text-zinc-950">
                套餐审计
              </h2>
              <div className="mt-3 space-y-2">
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
              </div>
            </section>
          ) : null}
        </div>
      </section>
    </div>
  );
}

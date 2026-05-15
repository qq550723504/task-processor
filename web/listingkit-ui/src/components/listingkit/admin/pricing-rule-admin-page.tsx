"use client";

import { useQuery } from "@tanstack/react-query";
import { FormEvent, useMemo, useState } from "react";
import { Calculator, Plus, RefreshCw, Search, Trash2 } from "lucide-react";

import {
  createListingPricingRule,
  deleteListingPricingRule,
  getListingPricingRules,
  updateListingPricingRuleStatus,
  type ListingPricingRule,
  type ListingPricingRuleInput,
} from "@/lib/api/admin-pricing-rules";
import { getSimpleListingStores } from "@/lib/api/admin-stores";

const DEFAULT_FORM: ListingPricingRuleInput = {
  name: "",
  ruleCode: "",
  description: "",
  remark: "",
  storeId: undefined,
  categoryId: undefined,
  priceMin: 0,
  priceMax: 99999,
  ruleType: "multiple_fixed",
  ruleValue: 1,
  fixedValue: 0,
  acceptCondition: "",
  rejectCondition: "",
  status: 1,
};

const RULE_TYPE_LABEL: Record<string, string> = {
  fixed: "固定加价",
  percent: "加价百分比",
  percent_plus_fixed: "百分比加固定值",
  multiple_fixed: "倍数加固定值",
  multiple: "倍数",
  discount: "折扣率",
  fixed_price: "固定价格",
};

export function PricingRuleAdminPage() {
  const [ruleCode, setRuleCode] = useState("");
  const [ruleType, setRuleType] = useState("");
  const [form, setForm] = useState<ListingPricingRuleInput>(DEFAULT_FORM);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const query = useMemo(
    () => ({
      page: 1,
      page_size: 50,
      ruleCode: ruleCode || undefined,
      ruleType: ruleType || undefined,
    }),
    [ruleCode, ruleType],
  );

  const pricingRuleQuery = useQuery({
    queryKey: ["listingkit-admin-pricing-rules", query],
    queryFn: () => getListingPricingRules(query),
  });
  const storesQuery = useQuery({
    queryKey: ["listingkit-admin-simple-stores"],
    queryFn: getSimpleListingStores,
  });

  const rules: ListingPricingRule[] = pricingRuleQuery.data?.items ?? [];
  const stores = storesQuery.data ?? [];
  const total = pricingRuleQuery.data?.total ?? 0;
  const loading = pricingRuleQuery.isLoading || pricingRuleQuery.isFetching;
  const visibleError =
    error ||
    (pricingRuleQuery.error instanceof Error
      ? pricingRuleQuery.error.message
      : "") ||
    (storesQuery.error instanceof Error ? storesQuery.error.message : "");

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await createListingPricingRule(form);
      setForm(DEFAULT_FORM);
      await pricingRuleQuery.refetch();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleToggle(rule: ListingPricingRule) {
    setError("");
    try {
      await updateListingPricingRuleStatus(rule.id, rule.status === 1 ? 0 : 1);
      await pricingRuleQuery.refetch();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleDelete(id: number) {
    setError("");
    try {
      await deleteListingPricingRule(id);
      await pricingRuleQuery.refetch();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">核价规则</h1>
            <p className="mt-1 text-sm text-zinc-500">
              共 {total} 条规则，按当前 ZITADEL 租户隔离。
            </p>
          </div>
          <form
            className="flex flex-wrap gap-2"
            onSubmit={(event) => event.preventDefault()}
          >
            <RuleInput
              label="规则编码"
              value={ruleCode}
              onChange={setRuleCode}
              placeholder="AR"
            />
            <RuleSelect
              label="规则类型"
              value={ruleType}
              onChange={setRuleType}
              options={[
                ["", "全部"],
                ["fixed", "固定加价"],
                ["multiple_fixed", "倍数加固定值"],
                ["multiple", "倍数"],
                ["fixed_price", "固定价格"],
              ]}
            />
            <button
              type="button"
              onClick={() => void pricingRuleQuery.refetch()}
              className="mt-5 inline-flex h-9 items-center gap-2 rounded-md border border-zinc-200 px-3 text-sm font-medium text-zinc-700 hover:border-zinc-300"
            >
              {loading ? (
                <RefreshCw className="size-4 animate-spin" />
              ) : (
                <Search className="size-4" />
              )}
              查询
            </button>
          </form>
        </div>
        {visibleError ? (
          <div className="mt-4 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {visibleError}
          </div>
        ) : null}
      </section>

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_390px]">
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-zinc-200 text-sm">
              <thead className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
                <tr>
                  <th className="px-4 py-3">规则</th>
                  <th className="px-4 py-3">店铺</th>
                  <th className="px-4 py-3">价格区间</th>
                  <th className="px-4 py-3">类型</th>
                  <th className="px-4 py-3">规则值</th>
                  <th className="px-4 py-3">固定值</th>
                  <th className="px-4 py-3">状态</th>
                  <th className="px-4 py-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-100">
                {loading ? (
                  <tr>
                    <td className="px-4 py-6 text-zinc-500" colSpan={8}>
                      加载中...
                    </td>
                  </tr>
                ) : rules.length === 0 ? (
                  <tr>
                    <td className="px-4 py-6 text-zinc-500" colSpan={8}>
                      暂无核价规则
                    </td>
                  </tr>
                ) : (
                  rules.map((rule) => (
                    <tr key={rule.id} className="align-top">
                      <td className="px-4 py-3">
                        <div className="font-medium text-zinc-950">
                          {rule.name}
                        </div>
                        <div className="font-mono text-xs text-zinc-500">
                          {rule.ruleCode}
                        </div>
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {storeName(stores, rule.storeId)}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {rule.priceMin}-{rule.priceMax}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {RULE_TYPE_LABEL[rule.ruleType] ?? rule.ruleType}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {rule.ruleValue}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {rule.fixedValue ?? "-"}
                      </td>
                      <td className="px-4 py-3">
                        <button
                          type="button"
                          onClick={() => void handleToggle(rule)}
                          className="rounded-full bg-zinc-100 px-2 py-1 text-xs font-medium text-zinc-700 hover:bg-zinc-200"
                        >
                          {rule.status === 1 ? "启用" : "禁用"}
                        </button>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          type="button"
                          aria-label={`删除 ${rule.name}`}
                          onClick={() => void handleDelete(rule.id)}
                          className="inline-flex size-8 items-center justify-center rounded-md border border-zinc-200 text-zinc-500 hover:border-red-200 hover:text-red-600"
                        >
                          <Trash2 className="size-4" />
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
          onSubmit={handleCreate}
          className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm"
        >
          <div className="mb-4 flex items-center gap-2">
            <Calculator className="size-4 text-zinc-500" />
            <h2 className="text-base font-semibold text-zinc-950">新增规则</h2>
          </div>
          <RuleInput
            label="规则名称"
            value={form.name}
            onChange={(name) => setForm({ ...form, name })}
          />
          <RuleInput
            label="规则编码"
            value={form.ruleCode}
            onChange={(nextRuleCode) =>
              setForm({ ...form, ruleCode: nextRuleCode })
            }
          />
          <label className="mb-3 block text-xs font-medium text-zinc-500">
            店铺
            <select
              value={form.storeId ?? 0}
              onChange={(event) =>
                setForm({
                  ...form,
                  storeId: Number(event.target.value) || undefined,
                })
              }
              className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
            >
              <option value={0}>全部店铺</option>
              {stores.map((store) => (
                <option key={store.id} value={store.id}>
                  {store.name}
                </option>
              ))}
            </select>
          </label>
          <div className="grid grid-cols-2 gap-3">
            <RuleInput
              label="最低价格"
              type="number"
              value={String(form.priceMin ?? "")}
              onChange={(priceMin) =>
                setForm({ ...form, priceMin: Number(priceMin) || 0 })
              }
            />
            <RuleInput
              label="最高价格"
              type="number"
              value={String(form.priceMax ?? "")}
              onChange={(priceMax) =>
                setForm({ ...form, priceMax: Number(priceMax) || 0 })
              }
            />
          </div>
          <RuleSelect
            label="规则类型"
            value={form.ruleType}
            onChange={(nextRuleType) =>
              setForm({ ...form, ruleType: nextRuleType })
            }
            options={[
              ["fixed", "固定加价"],
              ["multiple_fixed", "倍数加固定值"],
              ["multiple", "倍数"],
              ["fixed_price", "固定价格"],
            ]}
          />
          <div className="grid grid-cols-2 gap-3">
            <RuleInput
              label="规则值"
              type="number"
              value={String(form.ruleValue ?? "")}
              onChange={(nextRuleValue) =>
                setForm({ ...form, ruleValue: Number(nextRuleValue) || 0 })
              }
            />
            <RuleInput
              label="固定值"
              type="number"
              value={String(form.fixedValue ?? "")}
              onChange={(nextFixedValue) =>
                setForm({ ...form, fixedValue: Number(nextFixedValue) || 0 })
              }
            />
          </div>
          <button
            type="submit"
            disabled={saving}
            className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-md bg-zinc-950 px-3 text-sm font-medium text-white hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-400"
          >
            {saving ? (
              <RefreshCw className="size-4 animate-spin" />
            ) : (
              <Plus className="size-4" />
            )}
            保存规则
          </button>
        </form>
      </section>
    </div>
  );
}

function storeName(
  stores: Array<{ id: number; name: string }>,
  storeId: number | undefined,
) {
  if (!storeId) {
    return "全部店铺";
  }
  return stores.find((store) => store.id === storeId)?.name ?? `#${storeId}`;
}

function RuleInput({
  label,
  type = "text",
  value,
  placeholder,
  onChange,
}: {
  label: string;
  type?: string;
  value: string;
  placeholder?: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <input
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
      />
    </label>
  );
}

function RuleSelect({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: Array<[string, string]>;
}) {
  return (
    <label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
      >
        {options.map(([optionValue, labelText]) => (
          <option key={optionValue} value={optionValue}>
            {labelText}
          </option>
        ))}
      </select>
    </label>
  );
}

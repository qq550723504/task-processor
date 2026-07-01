"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { useQuery } from "@tanstack/react-query";
import { FormEvent, useMemo, useState } from "react";
import { CircleDollarSign, Plus, RefreshCw, Search, Trash2 } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { formatSubscriptionApiError } from "@/lib/api/subscription";

import {
  createListingProfitRule,
  deleteListingProfitRule,
  getListingProfitRules,
  updateListingProfitRuleStatus,
  type ListingProfitRule,
  type ListingProfitRuleInput,
} from "@/lib/api/admin-profit-rules";
import {
  AdminStoreSelect,
  formatAdminStoreName,
  useAdminSimpleStores,
} from "@/components/listingkit/admin/admin-store-select";

const DEFAULT_FORM: ListingProfitRuleInput = {
  name: "",
  ruleCode: "",
  description: "",
  storeId: undefined,
  categoryId: undefined,
  salePriceMultiplier: 3,
  discountPriceMultiplier: 1,
  status: 1,
  remark: "",
};

export function ProfitRuleAdminPage() {
  const [ruleCode, setRuleCode] = useState("");
  const [status, setStatus] = useState("");
  const [form, setForm] = useState<ListingProfitRuleInput>(DEFAULT_FORM);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const query = useMemo(
    () => ({
      page: 1,
      page_size: 50,
      ruleCode: ruleCode || undefined,
      status: status || undefined,
    }),
    [ruleCode, status],
  );

  const profitRuleQuery = useQuery({
    queryKey: ["listingkit-admin-profit-rules", query],
    queryFn: () => getListingProfitRules(query),
  });
  const storesQuery = useAdminSimpleStores();

  const rules: ListingProfitRule[] = profitRuleQuery.data?.items ?? [];
  const stores = storesQuery.data ?? [];
  const total = profitRuleQuery.data?.total ?? 0;
  const loading = profitRuleQuery.isLoading || profitRuleQuery.isFetching;
  const visibleError =
    error ||
    (profitRuleQuery.error instanceof Error
      ? profitRuleQuery.error.message
      : "") ||
    (storesQuery.error instanceof Error ? storesQuery.error.message : "");

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await createListingProfitRule(form);
      setForm(DEFAULT_FORM);
      await profitRuleQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleToggle(rule: ListingProfitRule) {
    setError("");
    try {
      await updateListingProfitRuleStatus(rule.id, rule.status === 1 ? 0 : 1);
      await profitRuleQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  async function handleDelete(id: number) {
    setError("");
    try {
      await deleteListingProfitRule(id);
      await profitRuleQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">利润规则</h1>
            <p className="mt-1 text-sm text-zinc-500">
              共 {total} 条规则，按当前 ZITADEL 租户隔离。
            </p>
          </div>
          <form
            className="flex flex-col gap-2 sm:flex-row sm:flex-wrap"
            onSubmit={(event) => event.preventDefault()}
          >
            <RuleInput
              label="规则编码"
              value={ruleCode}
              onChange={setRuleCode}
              placeholder="PR"
            />
            <RuleSelect
              label="状态"
              value={status}
              onChange={setStatus}
              options={[
                ["", "全部"],
                ["1", "启用"],
                ["0", "禁用"],
              ]}
            />
            <Button
              type="button"
              onClick={() => void profitRuleQuery.refetch()}
              className="w-full sm:mt-5 sm:w-auto"
              variant="secondary"
            >
              {loading ? (
                <RefreshCw className="size-4 animate-spin" />
              ) : (
                <Search className="size-4" />
              )}
              查询
            </Button>
          </form>
        </div>
        {visibleError ? (
          <Alert className="mt-4" variant="destructive">
            <AlertDescription>{visibleError}</AlertDescription>
          </Alert>
        ) : null}
      </section>

      <section className="grid gap-4 2xl:grid-cols-[minmax(0,1fr)_370px]">
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
          <div className="overflow-x-auto">
            <Table className="min-w-[52rem] divide-y divide-zinc-200 text-sm">
              <TableHeader className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
                <TableRow>
                  <TableHead className="px-4 py-3">规则</TableHead>
                  <TableHead className="px-4 py-3">店铺</TableHead>
                  <TableHead className="px-4 py-3">售价系数</TableHead>
                  <TableHead className="px-4 py-3">折扣系数</TableHead>
                  <TableHead className="px-4 py-3">状态</TableHead>
                  <TableHead className="px-4 py-3 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className="divide-y divide-zinc-100">
                {loading ? (
                  <TableRow>
                    <TableCell className="px-4 py-6 text-zinc-500" colSpan={6}>
                      加载中...
                    </TableCell>
                  </TableRow>
                ) : rules.length === 0 ? (
                  <TableRow>
                    <TableCell className="px-4 py-6 text-zinc-500" colSpan={6}>
                      暂无利润规则
                    </TableCell>
                  </TableRow>
                ) : (
                  rules.map((rule) => (
                    <TableRow key={rule.id} className="align-top">
                      <TableCell className="px-4 py-3">
                        <div className="font-medium text-zinc-950">
                          {rule.name}
                        </div>
                        <div className="font-mono text-xs text-zinc-500">
                          {rule.ruleCode}
                        </div>
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {formatAdminStoreName(stores, rule.storeId, "全部店铺")}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {rule.salePriceMultiplier}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {rule.discountPriceMultiplier}
                      </TableCell>
                      <TableCell className="px-4 py-3">
                        <Button
                          type="button"
                          onClick={() => void handleToggle(rule)}
                          variant="ghost"
                          className="h-auto p-0 hover:bg-transparent"
                        >
                          <Badge variant={rule.status === 1 ? "success" : "neutral"}>
                            {rule.status === 1 ? "启用" : "禁用"}
                          </Badge>
                        </Button>
                      </TableCell>
                      <TableCell className="px-4 py-3 text-right">
                        <Button
                          type="button"
                          aria-label={`删除 ${rule.name}`}
                          onClick={() => void handleDelete(rule.id)}
                          size="icon"
                          variant="ghost"
                        >
                          <Trash2 className="size-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </div>

        <form
          onSubmit={handleCreate}
          className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm"
        >
          <div className="mb-4 flex items-center gap-2">
            <CircleDollarSign className="size-4 text-zinc-500" />
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
          <AdminStoreSelect
            value={form.storeId}
            onChange={(storeId) =>
              setForm({ ...form, storeId: storeId || undefined })
            }
            stores={stores}
            emptyLabel="全部店铺"
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <RuleInput
              label="售价系数"
              type="number"
              value={String(form.salePriceMultiplier ?? "")}
              onChange={(salePriceMultiplier) =>
                setForm({
                  ...form,
                  salePriceMultiplier: Number(salePriceMultiplier) || 0,
                })
              }
            />
            <RuleInput
              label="折扣系数"
              type="number"
              value={String(form.discountPriceMultiplier ?? "")}
              onChange={(discountPriceMultiplier) =>
                setForm({
                  ...form,
                  discountPriceMultiplier:
                    Number(discountPriceMultiplier) || 0,
                })
              }
            />
          </div>
          <Button
            type="submit"
            disabled={saving}
            className="w-full"
          >
            {saving ? (
              <RefreshCw className="size-4 animate-spin" />
            ) : (
              <Plus className="size-4" />
            )}
            保存规则
          </Button>
        </form>
      </section>
    </div>
  );
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
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <Input
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
      />
    </Label>
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
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <Select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
      >
        {options.map(([optionValue, labelText]) => (
          <option key={optionValue} value={optionValue}>
            {labelText}
          </option>
        ))}
      </Select>
    </Label>
  );
}

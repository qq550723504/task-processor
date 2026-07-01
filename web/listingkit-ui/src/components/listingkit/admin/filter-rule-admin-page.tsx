"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { useQuery } from "@tanstack/react-query";
import { FormEvent, useMemo, useState } from "react";
import { Filter, Plus, RefreshCw, Search, Trash2 } from "lucide-react";

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
  createListingFilterRule,
  deleteListingFilterRule,
  getListingFilterRules,
  updateListingFilterRuleStatus,
  type ListingFilterRule,
  type ListingFilterRuleInput,
} from "@/lib/api/admin-filter-rules";
import {
  AdminStoreSelect,
  formatAdminStoreName,
  useAdminSimpleStores,
} from "@/components/listingkit/admin/admin-store-select";

const DEFAULT_FORM: ListingFilterRuleInput = {
  name: "",
  ruleCode: "",
  description: "",
  storeId: undefined,
  categoryId: undefined,
  priceType: "special",
  priceMin: 0,
  priceMax: 99999,
  stockMin: 10,
  ratingMin: 0,
  reviewCountMin: 0,
  deliveryTimeMax: undefined,
  fulfillmentType: "ALL",
  status: 1,
  remark: "",
};

export function FilterRuleAdminPage() {
  const [ruleCode, setRuleCode] = useState("");
  const [status, setStatus] = useState("");
  const [form, setForm] = useState<ListingFilterRuleInput>(DEFAULT_FORM);
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

  const filterRuleQuery = useQuery({
    queryKey: ["listingkit-admin-filter-rules", query],
    queryFn: () => getListingFilterRules(query),
  });
  const storesQuery = useAdminSimpleStores();

  const rules: ListingFilterRule[] = filterRuleQuery.data?.items ?? [];
  const stores = storesQuery.data ?? [];
  const total = filterRuleQuery.data?.total ?? 0;
  const loading = filterRuleQuery.isLoading || filterRuleQuery.isFetching;
  const visibleError =
    error ||
    (filterRuleQuery.error instanceof Error
      ? filterRuleQuery.error.message
      : "") ||
    (storesQuery.error instanceof Error ? storesQuery.error.message : "");

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await createListingFilterRule(form);
      setForm(DEFAULT_FORM);
      await filterRuleQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleToggle(rule: ListingFilterRule) {
    setError("");
    try {
      await updateListingFilterRuleStatus(rule.id, rule.status === 1 ? 0 : 1);
      await filterRuleQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  async function handleDelete(id: number) {
    setError("");
    try {
      await deleteListingFilterRule(id);
      await filterRuleQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">筛选规则</h1>
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
              placeholder="FR"
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
              onClick={() => void filterRuleQuery.refetch()}
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

      <section className="grid gap-4 2xl:grid-cols-[minmax(0,1fr)_390px]">
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
          <div className="overflow-x-auto">
            <Table className="min-w-[60rem]">
              <TableHeader className="bg-zinc-50">
                <TableRow className="text-xs uppercase tracking-[0.2em] hover:bg-transparent">
                  <TableHead>规则</TableHead>
                  <TableHead>店铺</TableHead>
                  <TableHead>价格</TableHead>
                  <TableHead>库存</TableHead>
                  <TableHead>评分</TableHead>
                  <TableHead>配送</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  <TableRow>
                    <TableCell className="py-6 text-zinc-500" colSpan={8}>
                      加载中...
                    </TableCell>
                  </TableRow>
                ) : rules.length === 0 ? (
                  <TableRow>
                    <TableCell className="py-6 text-zinc-500" colSpan={8}>
                      暂无筛选规则
                    </TableCell>
                  </TableRow>
                ) : (
                  rules.map((rule) => (
                    <TableRow key={rule.id} className="align-top">
                      <TableCell>
                        <div className="font-medium text-zinc-950">
                          {rule.name}
                        </div>
                        <div className="font-mono text-xs text-zinc-500">
                          {rule.ruleCode}
                        </div>
                      </TableCell>
                      <TableCell className="text-zinc-700">
                        {formatAdminStoreName(stores, rule.storeId, "全部店铺")}
                      </TableCell>
                      <TableCell className="text-zinc-700">
                        {rule.priceMin}-{rule.priceMax}
                      </TableCell>
                      <TableCell className="text-zinc-700">
                        {`>= ${rule.stockMin}`}
                      </TableCell>
                      <TableCell className="text-zinc-700">
                        {`>= ${rule.ratingMin}`}
                      </TableCell>
                      <TableCell className="text-zinc-700">
                        {rule.fulfillmentType || "ALL"}
                      </TableCell>
                      <TableCell>
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
                      <TableCell className="text-right">
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
            <Filter className="size-4 text-zinc-500" />
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
          <div className="grid gap-3 sm:grid-cols-2">
            <RuleInput
              label="最低库存"
              type="number"
              value={String(form.stockMin ?? "")}
              onChange={(stockMin) =>
                setForm({ ...form, stockMin: Number(stockMin) || 0 })
              }
            />
            <RuleInput
              label="最低评分"
              type="number"
              value={String(form.ratingMin ?? "")}
              onChange={(ratingMin) =>
                setForm({ ...form, ratingMin: Number(ratingMin) || 0 })
              }
            />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <RuleInput
              label="最低评论"
              type="number"
              value={String(form.reviewCountMin ?? "")}
              onChange={(reviewCountMin) =>
                setForm({
                  ...form,
                  reviewCountMin: Number(reviewCountMin) || 0,
                })
              }
            />
            <RuleSelect
              label="配送方式"
              value={form.fulfillmentType ?? "ALL"}
              onChange={(fulfillmentType) =>
                setForm({ ...form, fulfillmentType })
              }
              options={[
                ["ALL", "ALL"],
                ["FBA", "FBA"],
                ["FBM", "FBM"],
                ["AMZ", "AMZ"],
              ]}
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
        className="mt-1 h-9"
        type={type}
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
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
        className="mt-1 h-9"
        value={value}
        onChange={(event) => onChange(event.target.value)}
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

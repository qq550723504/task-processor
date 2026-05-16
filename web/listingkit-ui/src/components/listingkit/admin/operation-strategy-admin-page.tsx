"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { useQuery } from "@tanstack/react-query";
import { FormEvent, useMemo, useState } from "react";
import { Plus, RefreshCw, Search, SlidersHorizontal, Trash2 } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
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
  createListingOperationStrategy,
  deleteListingOperationStrategy,
  getListingOperationStrategies,
  updateListingOperationStrategyStatus,
  type ListingOperationStrategy,
  type ListingOperationStrategyInput,
} from "@/lib/api/admin-operation-strategies";
import { getSimpleListingStores } from "@/lib/api/admin-stores";

const DEFAULT_FORM: ListingOperationStrategyInput = {
  storeId: 0,
  name: "",
  platform: "SHEIN",
  status: 0,
  stockChangeThreshold: 10,
  stockChangeAction: "按比例更新",
  outOfStockAction: "自动下架",
  minProfitRate: 0.2,
  lowProfitAction: "暂停上架",
  priceUpdateMultiplier: 1,
  fixedPriceAdjustment: 0,
  stockUpdateRatio: 1,
  remark: "",
};

export function OperationStrategyAdminPage() {
  const [platform, setPlatform] = useState("");
  const [status, setStatus] = useState("");
  const [form, setForm] = useState<ListingOperationStrategyInput>(DEFAULT_FORM);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const query = useMemo(
    () => ({
      page: 1,
      page_size: 50,
      platform: platform || undefined,
      status: status || undefined,
    }),
    [platform, status],
  );

  const strategyQuery = useQuery({
    queryKey: ["listingkit-admin-operation-strategies", query],
    queryFn: () => getListingOperationStrategies(query),
  });
  const storesQuery = useQuery({
    queryKey: ["listingkit-admin-simple-stores"],
    queryFn: getSimpleListingStores,
  });

  const strategies: ListingOperationStrategy[] = strategyQuery.data?.items ?? [];
  const stores = storesQuery.data ?? [];
  const total = strategyQuery.data?.total ?? 0;
  const loading = strategyQuery.isLoading || strategyQuery.isFetching;
  const visibleError =
    error ||
    (strategyQuery.error instanceof Error ? strategyQuery.error.message : "") ||
    (storesQuery.error instanceof Error ? storesQuery.error.message : "");

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await createListingOperationStrategy(form);
      setForm(DEFAULT_FORM);
      await strategyQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleToggle(strategy: ListingOperationStrategy) {
    setError("");
    try {
      await updateListingOperationStrategyStatus(
        strategy.id,
        strategy.status === 0 ? 1 : 0,
      );
      await strategyQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  async function handleDelete(id: number) {
    setError("");
    try {
      await deleteListingOperationStrategy(id);
      await strategyQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">运营策略</h1>
            <p className="mt-1 text-sm text-zinc-500">
              共 {total} 条策略，按当前 ZITADEL 租户隔离。
            </p>
          </div>
          <form
            className="flex flex-wrap gap-2"
            onSubmit={(event) => event.preventDefault()}
          >
            <StrategySelect
              label="平台"
              value={platform}
              onChange={setPlatform}
              options={[
                ["", "全部"],
                ["SHEIN", "SHEIN"],
                ["TEMU", "TEMU"],
              ]}
            />
            <StrategySelect
              label="状态"
              value={status}
              onChange={setStatus}
              options={[
                ["", "全部"],
                ["0", "启用"],
                ["1", "禁用"],
              ]}
            />
            <Button
              type="button"
              onClick={() => void strategyQuery.refetch()}
              className="mt-5"
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

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_390px]">
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
          <div className="overflow-x-auto">
            <Table className="min-w-full divide-y divide-zinc-200 text-sm">
              <TableHeader className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
                <TableRow>
                  <TableHead className="px-4 py-3">策略</TableHead>
                  <TableHead className="px-4 py-3">店铺</TableHead>
                  <TableHead className="px-4 py-3">平台</TableHead>
                  <TableHead className="px-4 py-3">库存变化</TableHead>
                  <TableHead className="px-4 py-3">缺货动作</TableHead>
                  <TableHead className="px-4 py-3">利润率下限</TableHead>
                  <TableHead className="px-4 py-3">状态</TableHead>
                  <TableHead className="px-4 py-3 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className="divide-y divide-zinc-100">
                {loading ? (
                  <TableRow>
                    <TableCell className="px-4 py-6 text-zinc-500" colSpan={8}>
                      加载中...
                    </TableCell>
                  </TableRow>
                ) : strategies.length === 0 ? (
                  <TableRow>
                    <TableCell className="px-4 py-6 text-zinc-500" colSpan={8}>
                      暂无运营策略
                    </TableCell>
                  </TableRow>
                ) : (
                  strategies.map((strategy) => (
                    <TableRow key={strategy.id} className="align-top">
                      <TableCell className="px-4 py-3">
                        <div className="font-medium text-zinc-950">
                          {strategy.name}
                        </div>
                        <div className="text-xs text-zinc-500">
                          {strategy.remark || "-"}
                        </div>
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {storeName(stores, strategy.storeId)}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {strategy.platform}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {strategy.stockChangeThreshold ?? "-"} /{" "}
                        {strategy.stockChangeAction || "-"}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {strategy.outOfStockAction || "-"}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {formatPercent(strategy.minProfitRate)}
                      </TableCell>
                      <TableCell className="px-4 py-3">
                        <Button
                          type="button"
                          onClick={() => void handleToggle(strategy)}
                          variant="ghost"
                          className="h-auto p-0 hover:bg-transparent"
                        >
                          <Badge variant={strategy.status === 0 ? "success" : "neutral"}>
                            {strategy.status === 0 ? "启用" : "禁用"}
                          </Badge>
                        </Button>
                      </TableCell>
                      <TableCell className="px-4 py-3 text-right">
                        <Button
                          type="button"
                          aria-label={`删除 ${strategy.name}`}
                          onClick={() => void handleDelete(strategy.id)}
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
            <SlidersHorizontal className="size-4 text-zinc-500" />
            <h2 className="text-base font-semibold text-zinc-950">新增策略</h2>
          </div>
          <StrategyInput
            label="策略名称"
            value={form.name}
            onChange={(name) => setForm({ ...form, name })}
          />
          <Label className="mb-3 block text-xs font-medium text-zinc-500">
            店铺
            <Select
              value={form.storeId}
              onChange={(event) =>
                setForm({ ...form, storeId: Number(event.target.value) || 0 })
              }
              className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
            >
              <option value={0}>请选择店铺</option>
              {stores.map((store) => (
                <option key={store.id} value={store.id}>
                  {store.name}
                </option>
              ))}
            </Select>
          </Label>
          <StrategySelect
            label="平台"
            value={form.platform}
            onChange={(nextPlatform) =>
              setForm({ ...form, platform: nextPlatform })
            }
            options={[
              ["SHEIN", "SHEIN"],
              ["TEMU", "TEMU"],
            ]}
          />
          <div className="grid grid-cols-2 gap-3">
            <StrategyInput
              label="库存变化阈值"
              type="number"
              value={String(form.stockChangeThreshold ?? "")}
              onChange={(value) =>
                setForm({
                  ...form,
                  stockChangeThreshold: Number(value) || 0,
                })
              }
            />
            <StrategyInput
              label="库存更新比例"
              type="number"
              value={String(form.stockUpdateRatio ?? "")}
              onChange={(value) =>
                setForm({ ...form, stockUpdateRatio: Number(value) || 0 })
              }
            />
          </div>
          <StrategyInput
            label="库存变化动作"
            value={form.stockChangeAction ?? ""}
            onChange={(stockChangeAction) =>
              setForm({ ...form, stockChangeAction })
            }
          />
          <StrategyInput
            label="缺货动作"
            value={form.outOfStockAction ?? ""}
            onChange={(outOfStockAction) =>
              setForm({ ...form, outOfStockAction })
            }
          />
          <div className="grid grid-cols-2 gap-3">
            <StrategyInput
              label="最低利润率"
              type="number"
              value={String(form.minProfitRate ?? "")}
              onChange={(value) =>
                setForm({ ...form, minProfitRate: Number(value) || 0 })
              }
            />
            <StrategyInput
              label="价格倍数"
              type="number"
              value={String(form.priceUpdateMultiplier ?? "")}
              onChange={(value) =>
                setForm({
                  ...form,
                  priceUpdateMultiplier: Number(value) || 0,
                })
              }
            />
          </div>
          <StrategyInput
            label="低利润动作"
            value={form.lowProfitAction ?? ""}
            onChange={(lowProfitAction) => setForm({ ...form, lowProfitAction })}
          />
          <StrategyInput
            label="备注"
            value={form.remark ?? ""}
            onChange={(remark) => setForm({ ...form, remark })}
          />
          <Button
            type="submit"
            disabled={saving || form.storeId <= 0}
            className="w-full"
          >
            {saving ? (
              <RefreshCw className="size-4 animate-spin" />
            ) : (
              <Plus className="size-4" />
            )}
            保存策略
          </Button>
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
    return "-";
  }
  return stores.find((store) => store.id === storeId)?.name ?? `#${storeId}`;
}

function formatPercent(value: number | undefined) {
  if (value === undefined) {
    return "-";
  }
  return `${Math.round(value * 10000) / 100}%`;
}

function StrategyInput({
  label,
  type = "text",
  value,
  onChange,
}: {
  label: string;
  type?: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <Input
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
      />
    </Label>
  );
}

function StrategySelect({
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

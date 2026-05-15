"use client";

import { useQuery } from "@tanstack/react-query";
import { FormEvent, useMemo, useState } from "react";
import { GitBranch, Plus, RefreshCw, Search, Trash2 } from "lucide-react";

import {
  createListingProductImportMapping,
  deleteListingProductImportMapping,
  getListingProductImportMappings,
  updateListingProductImportMappingStatus,
  type ListingProductImportMapping,
  type ListingProductImportMappingInput,
} from "@/lib/api/admin-product-import-mappings";

const DEFAULT_FORM: ListingProductImportMappingInput = {
  importTaskId: 0,
  storeId: 0,
  platform: "SHEIN",
  region: "US",
  productId: "",
  sku: "",
  salePriceMultiplier: 1,
  discountPriceMultiplier: 1,
  status: 0,
  remark: "",
};

const STATUS_LABEL: Record<number, string> = {
  0: "初始",
  1: "已生成 SKU",
  2: "已上架",
  3: "已下架",
};

export function ProductImportMappingAdminPage() {
  const [productId, setProductId] = useState("");
  const [platform, setPlatform] = useState("");
  const [sku, setSku] = useState("");
  const [form, setForm] =
    useState<ListingProductImportMappingInput>(DEFAULT_FORM);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const query = useMemo(
    () => ({
      page: 1,
      page_size: 50,
      productId: productId || undefined,
      platform: platform || undefined,
      sku: sku || undefined,
    }),
    [productId, platform, sku],
  );

  const mappingQuery = useQuery({
    queryKey: ["listingkit-admin-product-import-mappings", query],
    queryFn: () => getListingProductImportMappings(query),
  });

  const mappings: ListingProductImportMapping[] = mappingQuery.data?.items ?? [];
  const total = mappingQuery.data?.total ?? 0;
  const loading = mappingQuery.isLoading || mappingQuery.isFetching;
  const visibleError =
    error ||
    (mappingQuery.error instanceof Error ? mappingQuery.error.message : "");

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await createListingProductImportMapping(form);
      setForm(DEFAULT_FORM);
      await mappingQuery.refetch();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleAdvanceStatus(mapping: ListingProductImportMapping) {
    setError("");
    try {
      await updateListingProductImportMappingStatus(
        mapping.id,
        mapping.status >= 3 ? 0 : mapping.status + 1,
      );
      await mappingQuery.refetch();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleDelete(id: number) {
    setError("");
    try {
      await deleteListingProductImportMapping(id);
      await mappingQuery.refetch();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">导入映射</h1>
            <p className="mt-1 text-sm text-zinc-500">
              共 {total} 条映射，按当前 ZITADEL 租户隔离。
            </p>
          </div>
          <form
            className="flex flex-wrap gap-2"
            onSubmit={(event) => event.preventDefault()}
          >
            <MappingInput
              label="产品 ID"
              value={productId}
              onChange={setProductId}
              placeholder="B001"
            />
            <MappingInput
              label="SKU"
              value={sku}
              onChange={setSku}
              placeholder="SKU-001"
            />
            <MappingSelect
              label="平台"
              value={platform}
              onChange={setPlatform}
              options={[
                ["", "全部"],
                ["SHEIN", "SHEIN"],
                ["TEMU", "TEMU"],
                ["AMAZON", "AMAZON"],
              ]}
            />
            <button
              type="button"
              onClick={() => void mappingQuery.refetch()}
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
                  <th className="px-4 py-3">产品</th>
                  <th className="px-4 py-3">任务/店铺</th>
                  <th className="px-4 py-3">平台</th>
                  <th className="px-4 py-3">SKU</th>
                  <th className="px-4 py-3">平台产品</th>
                  <th className="px-4 py-3">倍数</th>
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
                ) : mappings.length === 0 ? (
                  <tr>
                    <td className="px-4 py-6 text-zinc-500" colSpan={8}>
                      暂无导入映射
                    </td>
                  </tr>
                ) : (
                  mappings.map((mapping) => (
                    <tr key={mapping.id} className="align-top">
                      <td className="px-4 py-3">
                        <div className="font-medium text-zinc-950">
                          {mapping.productId}
                        </div>
                        <div className="text-xs text-zinc-500">
                          {mapping.parentProductId || "-"}
                        </div>
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        #{mapping.importTaskId} / #{mapping.storeId}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {mapping.platform} {mapping.region}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {mapping.sku || "-"}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {mapping.platformProductId || "-"}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {mapping.salePriceMultiplier} /{" "}
                        {mapping.discountPriceMultiplier}
                      </td>
                      <td className="px-4 py-3">
                        <button
                          type="button"
                          onClick={() => void handleAdvanceStatus(mapping)}
                          className="rounded-full bg-zinc-100 px-2 py-1 text-xs font-medium text-zinc-700 hover:bg-zinc-200"
                        >
                          {STATUS_LABEL[mapping.status] ?? mapping.status}
                        </button>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          type="button"
                          aria-label={`删除 ${mapping.productId}`}
                          onClick={() => void handleDelete(mapping.id)}
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
            <GitBranch className="size-4 text-zinc-500" />
            <h2 className="text-base font-semibold text-zinc-950">新增映射</h2>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <MappingInput
              label="导入任务 ID"
              type="number"
              value={String(form.importTaskId || "")}
              onChange={(value) =>
                setForm({ ...form, importTaskId: Number(value) || 0 })
              }
            />
            <MappingInput
              label="店铺 ID"
              type="number"
              value={String(form.storeId || "")}
              onChange={(value) =>
                setForm({ ...form, storeId: Number(value) || 0 })
              }
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <MappingSelect
              label="平台"
              value={form.platform}
              onChange={(nextPlatform) =>
                setForm({ ...form, platform: nextPlatform })
              }
              options={[
                ["SHEIN", "SHEIN"],
                ["TEMU", "TEMU"],
                ["AMAZON", "AMAZON"],
              ]}
            />
            <MappingInput
              label="区域"
              value={form.region}
              onChange={(region) => setForm({ ...form, region })}
            />
          </div>
          <MappingInput
            label="产品 ID"
            value={form.productId}
            onChange={(nextProductId) =>
              setForm({ ...form, productId: nextProductId })
            }
          />
          <MappingInput
            label="SKU"
            value={form.sku ?? ""}
            onChange={(nextSku) => setForm({ ...form, sku: nextSku })}
          />
          <div className="grid grid-cols-2 gap-3">
            <MappingInput
              label="售价倍数"
              type="number"
              value={String(form.salePriceMultiplier ?? "")}
              onChange={(value) =>
                setForm({
                  ...form,
                  salePriceMultiplier: Number(value) || 1,
                })
              }
            />
            <MappingInput
              label="折扣倍数"
              type="number"
              value={String(form.discountPriceMultiplier ?? "")}
              onChange={(value) =>
                setForm({
                  ...form,
                  discountPriceMultiplier: Number(value) || 1,
                })
              }
            />
          </div>
          <MappingInput
            label="备注"
            value={form.remark ?? ""}
            onChange={(remark) => setForm({ ...form, remark })}
          />
          <button
            type="submit"
            disabled={
              saving ||
              form.importTaskId <= 0 ||
              form.storeId <= 0 ||
              !form.productId.trim()
            }
            className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-md bg-zinc-950 px-3 text-sm font-medium text-white hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-400"
          >
            {saving ? (
              <RefreshCw className="size-4 animate-spin" />
            ) : (
              <Plus className="size-4" />
            )}
            保存映射
          </button>
        </form>
      </section>
    </div>
  );
}

function MappingInput({
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

function MappingSelect({
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

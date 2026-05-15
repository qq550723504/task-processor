"use client";

import { useQuery } from "@tanstack/react-query";
import { FormEvent, useMemo, useState } from "react";
import { Plus, RefreshCw, Search, Trash2, Upload } from "lucide-react";

import { formatSubscriptionApiError } from "@/lib/api/subscription";

import {
  batchCreateListingImportTasks,
  deleteListingImportTask,
  getListingImportTasks,
  type BatchCreateListingImportTaskInput,
  type ListingImportTask,
} from "@/lib/api/admin-import-tasks";
import { getSimpleListingStores } from "@/lib/api/admin-stores";

const DEFAULT_FORM: BatchCreateListingImportTaskInput = {
  storeId: 0,
  categoryId: 0,
  platform: "Amazon",
  targetPlatform: "SHEIN",
  region: "US",
  priority: 5,
  productIds: [],
  remark: "",
};

const STATUS_TEXT: Record<number, string> = {
  0: "待处理",
  1: "处理中",
  2: "成功",
  3: "失败",
};

export function ImportTaskAdminPage() {
  const [platform, setPlatform] = useState("");
  const [productId, setProductId] = useState("");
  const [form, setForm] =
    useState<BatchCreateListingImportTaskInput>(DEFAULT_FORM);
  const [productText, setProductText] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const query = useMemo(
    () => ({
      page: 1,
      page_size: 50,
      platform: platform || undefined,
      productId: productId || undefined,
    }),
    [platform, productId],
  );

  const importTaskQuery = useQuery({
    queryKey: ["listingkit-admin-import-tasks", query],
    queryFn: () => getListingImportTasks(query),
  });
  const storesQuery = useQuery({
    queryKey: ["listingkit-admin-simple-stores"],
    queryFn: getSimpleListingStores,
  });

  const tasks: ListingImportTask[] = importTaskQuery.data?.items ?? [];
  const total = importTaskQuery.data?.total ?? 0;
  const stores = storesQuery.data ?? [];
  const loading = importTaskQuery.isLoading || importTaskQuery.isFetching;
  const visibleError =
    error ||
    (importTaskQuery.error instanceof Error
      ? importTaskQuery.error.message
      : "") ||
    (storesQuery.error instanceof Error ? storesQuery.error.message : "");

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      const productIds = productText
        .split(/[\n,，\s]+/)
        .map((value) => value.trim())
        .filter(Boolean);
      await batchCreateListingImportTasks({ ...form, productIds });
      setForm({ ...DEFAULT_FORM, storeId: form.storeId });
      setProductText("");
      await importTaskQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(id: number) {
    setError("");
    try {
      await deleteListingImportTask(id);
      await importTaskQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">任务导入</h1>
            <p className="mt-1 text-sm text-zinc-500">
              共 {total} 个导入任务，按当前 ZITADEL 租户隔离。
            </p>
          </div>
          <form
            className="flex flex-wrap gap-2"
            onSubmit={(event) => event.preventDefault()}
          >
            <ImportTaskSelect
              label="来源平台"
              value={platform}
              onChange={setPlatform}
              options={["", "Amazon", "SHEIN", "TEMU"]}
              labels={{ "": "全部" }}
            />
            <ImportTaskInput
              label="商品 ID"
              value={productId}
              onChange={setProductId}
              placeholder="搜索商品"
            />
            <button
              type="button"
              onClick={() => void importTaskQuery.refetch()}
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

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_380px]">
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-zinc-200 text-sm">
              <thead className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
                <tr>
                  <th className="px-4 py-3">商品</th>
                  <th className="px-4 py-3">店铺</th>
                  <th className="px-4 py-3">平台</th>
                  <th className="px-4 py-3">地区</th>
                  <th className="px-4 py-3">状态</th>
                  <th className="px-4 py-3">重试</th>
                  <th className="px-4 py-3">优先级</th>
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
                ) : tasks.length === 0 ? (
                  <tr>
                    <td className="px-4 py-6 text-zinc-500" colSpan={8}>
                      暂无导入任务
                    </td>
                  </tr>
                ) : (
                  tasks.map((task) => (
                    <tr key={task.id} className="align-top">
                      <td className="px-4 py-3">
                        <div className="font-medium text-zinc-950">
                          {task.productId}
                        </div>
                        <div className="font-mono text-xs text-zinc-500">
                          #{task.id}
                        </div>
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {storeName(stores, task.storeId)}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {task.platform}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {task.region || "-"}
                      </td>
                      <td className="px-4 py-3">
                        <span className="rounded-full bg-zinc-100 px-2 py-1 text-xs font-medium text-zinc-700">
                          {STATUS_TEXT[task.status] ?? `状态 ${task.status}`}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {task.retryCount ?? 0}/{task.maxRetryCount ?? 3}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {task.priority ?? 5}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          type="button"
                          aria-label={`删除 ${task.productId}`}
                          onClick={() => void handleDelete(task.id)}
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
            <Upload className="size-4 text-zinc-500" />
            <h2 className="text-base font-semibold text-zinc-950">批量导入</h2>
          </div>
          <label className="mb-3 block text-xs font-medium text-zinc-500">
            店铺
            <select
              value={form.storeId}
              onChange={(event) =>
                setForm({ ...form, storeId: Number(event.target.value) })
              }
              className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
            >
              <option value={0}>选择店铺</option>
              {stores.map((store) => (
                <option key={store.id} value={store.id}>
                  {store.name}
                </option>
              ))}
            </select>
          </label>
          <div className="grid grid-cols-2 gap-3">
            <ImportTaskInput
              label="类目 ID"
              type="number"
              value={String(form.categoryId || "")}
              onChange={(categoryId) =>
                setForm({ ...form, categoryId: Number(categoryId) || 0 })
              }
            />
            <ImportTaskInput
              label="优先级"
              type="number"
              value={String(form.priority ?? "")}
              onChange={(priority) =>
                setForm({ ...form, priority: Number(priority) || undefined })
              }
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <ImportTaskSelect
              label="来源平台"
              value={form.platform}
              onChange={(nextPlatform) =>
                setForm({ ...form, platform: nextPlatform })
              }
              options={["Amazon", "SHEIN", "TEMU"]}
            />
            <ImportTaskInput
              label="地区"
              value={form.region ?? ""}
              onChange={(region) => setForm({ ...form, region })}
            />
          </div>
          <label className="mb-3 block text-xs font-medium text-zinc-500">
            商品 ID
            <textarea
              value={productText}
              onChange={(event) => setProductText(event.target.value)}
              className="mt-1 min-h-32 w-full resize-y rounded-md border border-zinc-200 px-3 py-2 text-sm text-zinc-900"
              placeholder="每行一个商品 ID"
            />
          </label>
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
            创建任务
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
    return "-";
  }
  return stores.find((store) => store.id === storeId)?.name ?? `#${storeId}`;
}

function ImportTaskInput({
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

function ImportTaskSelect({
  label,
  value,
  onChange,
  options,
  labels = {},
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: string[];
  labels?: Record<string, string>;
}) {
  return (
    <label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
      >
        {options.map((option) => (
          <option key={option} value={option}>
            {labels[option] ?? option}
          </option>
        ))}
      </select>
    </label>
  );
}

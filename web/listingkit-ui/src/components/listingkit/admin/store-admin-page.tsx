"use client";

import { useQuery } from "@tanstack/react-query";
import { FormEvent, useMemo, useState } from "react";
import { Clock, Plus, RefreshCw, RotateCcw, Search, Trash2 } from "lucide-react";

import { formatSubscriptionApiError } from "@/lib/api/subscription";

import {
  createListingStore,
  deleteListingStore,
  extendListingStoreValidity,
  getDeletedListingStores,
  getListingStores,
  permanentlyDeleteListingStore,
  restoreListingStore,
  type ListingStore,
  type ListingStoreInput,
} from "@/lib/api/admin-stores";

const DEFAULT_FORM: ListingStoreInput = {
  name: "",
  username: "",
  password: "",
  platform: "SHEIN",
  shopType: "semi",
  region: "US",
  dailyLimit: 200,
  dailyLimitType: "SPU",
  fixedStockCount: 999,
  skuGenerateStrategy: "0",
  enableAutoListing: true,
  enableAutoLogin: true,
  enableDraft: false,
  enableAutoPrice: false,
  enableRebargain: false,
  status: 0,
};

export function StoreAdminPage() {
  const [platform, setPlatform] = useState("");
  const [keyword, setKeyword] = useState("");
  const [form, setForm] = useState<ListingStoreInput>(DEFAULT_FORM);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const query = useMemo(
    () => ({
      page: 1,
      page_size: 50,
      platform: platform || undefined,
      name: keyword || undefined,
    }),
    [keyword, platform],
  );

  const storeQuery = useQuery({
    queryKey: ["listingkit-admin-stores", query],
    queryFn: () => getListingStores(query),
  });
  const deletedStoreQuery = useQuery({
    queryKey: ["listingkit-admin-deleted-stores"],
    queryFn: getDeletedListingStores,
  });

  const stores: ListingStore[] = storeQuery.data?.items ?? [];
  const deletedStores = deletedStoreQuery.data ?? [];
  const total = storeQuery.data?.total ?? 0;
  const loading = storeQuery.isLoading || storeQuery.isFetching;
  const visibleError =
    error ||
    (storeQuery.error instanceof Error ? storeQuery.error.message : "") ||
    (deletedStoreQuery.error instanceof Error
      ? deletedStoreQuery.error.message
      : "");

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await createListingStore(form);
      setForm(DEFAULT_FORM);
      await storeQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(id: number) {
    setError("");
    try {
      await deleteListingStore(id);
      await storeQuery.refetch();
      await deletedStoreQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  async function handleRestore(id: number) {
    setError("");
    try {
      await restoreListingStore(id);
      await storeQuery.refetch();
      await deletedStoreQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  async function handlePermanentDelete(id: number) {
    setError("");
    try {
      await permanentlyDeleteListingStore(id);
      await deletedStoreQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  async function handleExtendValidity(id: number) {
    setError("");
    try {
      await extendListingStoreValidity(id, 30);
      await storeQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">店铺管理</h1>
            <p className="mt-1 text-sm text-zinc-500">
              共 {total} 个店铺，按当前 ZITADEL 租户隔离。
            </p>
          </div>
          <form className="flex flex-wrap gap-2" onSubmit={(event) => event.preventDefault()}>
            <button
              type="button"
              onClick={() => void deletedStoreQuery.refetch()}
              className="mt-5 inline-flex h-9 items-center gap-2 rounded-md border border-zinc-200 px-3 text-sm font-medium text-zinc-700 hover:border-zinc-300"
            >
              <RotateCcw className="size-4" />
              回收站
            </button>
            <label className="flex flex-col gap-1 text-xs font-medium text-zinc-500">
              平台
              <select
                value={platform}
                onChange={(event) => setPlatform(event.target.value)}
                className="h-9 rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
              >
                <option value="">全部</option>
                <option value="SHEIN">SHEIN</option>
                <option value="TEMU">TEMU</option>
              </select>
            </label>
            <label className="flex flex-col gap-1 text-xs font-medium text-zinc-500">
              店铺名称
              <input
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                className="h-9 w-52 rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
                placeholder="搜索店铺"
              />
            </label>
            <button
              type="button"
              onClick={() => void storeQuery.refetch()}
              className="mt-5 inline-flex h-9 items-center gap-2 rounded-md border border-zinc-200 px-3 text-sm font-medium text-zinc-700 hover:border-zinc-300"
            >
              {loading ? <RefreshCw className="size-4 animate-spin" /> : <Search className="size-4" />}
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

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-zinc-200 text-sm">
              <thead className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
                <tr>
                  <th className="px-4 py-3">店铺</th>
                  <th className="px-4 py-3">账号</th>
                  <th className="px-4 py-3">平台</th>
                  <th className="px-4 py-3">地区</th>
                  <th className="px-4 py-3">每日限制</th>
                  <th className="px-4 py-3">自动上架</th>
                  <th className="px-4 py-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-100">
                {loading ? (
                  <tr>
                    <td className="px-4 py-6 text-zinc-500" colSpan={7}>
                      加载中...
                    </td>
                  </tr>
                ) : stores.length === 0 ? (
                  <tr>
                    <td className="px-4 py-6 text-zinc-500" colSpan={7}>
                      暂无店铺
                    </td>
                  </tr>
                ) : (
                  stores.map((store) => (
                    <tr key={store.id} className="align-top">
                      <td className="px-4 py-3">
                        <div className="font-medium text-zinc-950">{store.name}</div>
                        <div className="font-mono text-xs text-zinc-500">#{store.id}</div>
                      </td>
                      <td className="px-4 py-3 text-zinc-700">{store.username}</td>
                      <td className="px-4 py-3 text-zinc-700">{store.platform}</td>
                      <td className="px-4 py-3 text-zinc-700">{store.region || "-"}</td>
                      <td className="px-4 py-3 text-zinc-700">
                        {store.dailyLimit ?? "-"} {store.dailyLimitType ?? ""}
                      </td>
                      <td className="px-4 py-3">
                        <span className="rounded-full bg-zinc-100 px-2 py-1 text-xs font-medium text-zinc-700">
                          {store.enableAutoListing ? "启用" : "关闭"}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          type="button"
                          aria-label={`延长 ${store.name} 有效期`}
                          onClick={() => void handleExtendValidity(store.id)}
                          className="mr-2 inline-flex size-8 items-center justify-center rounded-md border border-zinc-200 text-zinc-500 hover:border-zinc-300 hover:text-zinc-900"
                        >
                          <Clock className="size-4" />
                        </button>
                        <button
                          type="button"
                          aria-label={`删除 ${store.name}`}
                          onClick={() => void handleDelete(store.id)}
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
            <Plus className="size-4 text-zinc-500" />
            <h2 className="text-base font-semibold text-zinc-950">新增店铺</h2>
          </div>
          <StoreInput label="店铺名称" value={form.name} onChange={(name) => setForm({ ...form, name })} />
          <StoreInput label="登录用户名" value={form.username} onChange={(username) => setForm({ ...form, username })} />
          <StoreInput label="登录密码" type="password" value={form.password ?? ""} onChange={(password) => setForm({ ...form, password })} />
          <div className="grid grid-cols-2 gap-3">
            <StoreSelect label="平台" value={form.platform} onChange={(platformValue) => setForm({ ...form, platform: platformValue })} options={["SHEIN", "TEMU"]} />
            <StoreInput label="地区" value={form.region ?? ""} onChange={(region) => setForm({ ...form, region })} />
          </div>
          <StoreInput label="店铺类型" value={form.shopType} onChange={(shopType) => setForm({ ...form, shopType })} />
          <StoreInput label="每日上架限制" type="number" value={String(form.dailyLimit ?? "")} onChange={(dailyLimit) => setForm({ ...form, dailyLimit: Number(dailyLimit) || undefined })} />
          <label className="mb-3 flex items-center gap-2 text-sm text-zinc-700">
            <input
              type="checkbox"
              checked={Boolean(form.enableAutoListing)}
              onChange={(event) => setForm({ ...form, enableAutoListing: event.target.checked })}
            />
            启用自动上架
          </label>
          <button
            type="submit"
            disabled={saving}
            className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-md bg-zinc-950 px-3 text-sm font-medium text-white hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-400"
          >
            {saving ? <RefreshCw className="size-4 animate-spin" /> : <Plus className="size-4" />}
            保存店铺
          </button>
        </form>
      </section>

      <section className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
        <div className="flex items-center justify-between border-b border-zinc-200 px-4 py-3">
          <div>
            <h2 className="text-base font-semibold text-zinc-950">回收站</h2>
            <p className="text-sm text-zinc-500">
              共 {deletedStores.length} 个已删除店铺。
            </p>
          </div>
          <button
            type="button"
            onClick={() => void deletedStoreQuery.refetch()}
            className="inline-flex h-9 items-center gap-2 rounded-md border border-zinc-200 px-3 text-sm font-medium text-zinc-700 hover:border-zinc-300"
          >
            <RefreshCw
              className={`size-4 ${deletedStoreQuery.isFetching ? "animate-spin" : ""}`}
            />
            刷新
          </button>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-zinc-200 text-sm">
            <thead className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
              <tr>
                <th className="px-4 py-3">店铺</th>
                <th className="px-4 py-3">账号</th>
                <th className="px-4 py-3">平台</th>
                <th className="px-4 py-3">地区</th>
                <th className="px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {deletedStoreQuery.isLoading ? (
                <tr>
                  <td className="px-4 py-6 text-zinc-500" colSpan={5}>
                    加载中...
                  </td>
                </tr>
              ) : deletedStores.length === 0 ? (
                <tr>
                  <td className="px-4 py-6 text-zinc-500" colSpan={5}>
                    回收站为空
                  </td>
                </tr>
              ) : (
                deletedStores.map((store) => (
                  <tr key={store.id} className="align-top">
                    <td className="px-4 py-3">
                      <div className="font-medium text-zinc-950">{store.name}</div>
                      <div className="font-mono text-xs text-zinc-500">#{store.id}</div>
                    </td>
                    <td className="px-4 py-3 text-zinc-700">{store.username}</td>
                    <td className="px-4 py-3 text-zinc-700">{store.platform}</td>
                    <td className="px-4 py-3 text-zinc-700">{store.region || "-"}</td>
                    <td className="px-4 py-3 text-right">
                      <button
                        type="button"
                        aria-label={`恢复 ${store.name}`}
                        onClick={() => void handleRestore(store.id)}
                        className="mr-2 inline-flex size-8 items-center justify-center rounded-md border border-zinc-200 text-zinc-500 hover:border-zinc-300 hover:text-zinc-900"
                      >
                        <RotateCcw className="size-4" />
                      </button>
                      <button
                        type="button"
                        aria-label={`彻底删除 ${store.name}`}
                        onClick={() => void handlePermanentDelete(store.id)}
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
      </section>
    </div>
  );
}

function StoreInput({
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
    <label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <input
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
      />
    </label>
  );
}

function StoreSelect({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: string[];
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
            {option}
          </option>
        ))}
      </select>
    </label>
  );
}

"use client";

import Link from "next/link";

import { Alert, AlertDescription } from "@/components/ui/alert";
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
  createTenantListingStore,
  deleteTenantListingStore,
  getTenantListingStores,
  updateTenantListingStore,
} from "@/lib/api/tenant-stores";
import { useSheinLoginAccounts } from "@/lib/query/use-shein-login";
import type { ListingStore, ListingStoreInput } from "@/lib/api/admin-stores";
import { useQuery } from "@tanstack/react-query";
import { FormEvent, useMemo, useState } from "react";
import { Pencil, Plus, RefreshCw, Search, Trash2, X } from "lucide-react";
import {
  buildSheinLoginStatusMap,
  StoreLoginStatusBadge,
} from "@/components/listingkit/stores/store-login-status";

const STORE_TYPE_OPTIONS = [
  { value: "0", label: "半托" },
  { value: "2", label: "自营" },
  { value: "1", label: "全托" },
] as const;

const DEFAULT_FORM: ListingStoreInput = {
  name: "",
  username: "",
  password: "",
  platform: "SHEIN",
  shopType: "0",
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

export function TenantStoreDirectoryPanel() {
  const [platform, setPlatform] = useState("");
  const [keyword, setKeyword] = useState("");
  const [form, setForm] = useState<ListingStoreInput>(DEFAULT_FORM);
  const [editingStoreId, setEditingStoreId] = useState<number | undefined>();
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
    queryKey: ["listingkit-tenant-stores", query],
    queryFn: () => getTenantListingStores(query),
  });
  const sheinLoginQuery = useSheinLoginAccounts();

  const stores: ListingStore[] = storeQuery.data?.items ?? [];
  const total = storeQuery.data?.total ?? 0;
  const loading = storeQuery.isLoading || storeQuery.isFetching;
  const loginStatusMap = useMemo(
    () => buildSheinLoginStatusMap(sheinLoginQuery.data),
    [sheinLoginQuery.data],
  );
  const visibleError =
    error || (storeQuery.error instanceof Error ? storeQuery.error.message : "");

  async function handleSave(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      if (editingStoreId) {
        await updateTenantListingStore(editingStoreId, form);
      } else {
        await createTenantListingStore(form);
      }
      resetForm();
      await storeQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

  function resetForm() {
    setForm(DEFAULT_FORM);
    setEditingStoreId(undefined);
  }

  function handleEdit(store: ListingStore) {
    setError("");
    setEditingStoreId(store.id);
    setForm({
      storeId: store.storeId,
      name: store.name,
      username: store.username,
      password: store.password ?? "",
      loginUrl: store.loginUrl,
      shopType: normalizeStoreType(store.shopType),
      region: store.region,
      platform: store.platform,
      dailyLimit: store.dailyLimit,
      dailyLimitType: store.dailyLimitType,
      fixedStockCount: store.fixedStockCount,
      skuGenerateStrategy: store.skuGenerateStrategy,
      prefix: store.prefix,
      suffix: store.suffix,
      proxy: store.proxy,
      enableAutoListing: store.enableAutoListing,
      enableAutoLogin: store.enableAutoLogin,
      enableDraft: store.enableDraft,
      enableAutoPrice: store.enableAutoPrice,
      enableRebargain: store.enableRebargain,
      temuPriceRejectStrategy: store.temuPriceRejectStrategy,
      priceType: store.priceType,
      remark: store.remark,
      status: store.status,
    });
  }

  async function handleDelete(id: number) {
    setError("");
    try {
      await deleteTenantListingStore(id);
      await storeQuery.refetch();
      if (editingStoreId === id) {
        resetForm();
      }
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  return (
    <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
      <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
        <div className="flex flex-col gap-3 border-b border-zinc-200 px-4 py-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h2 className="text-base font-semibold text-zinc-950">店铺主数据</h2>
            <p className="text-sm text-zinc-500">
              共 {total} 个店铺。这里新增的是当前租户自己的店铺账号，后面的发布配置会直接复用这些店铺。
            </p>
          </div>
          <form className="flex flex-wrap gap-2" onSubmit={(event) => event.preventDefault()}>
            <Label className="flex flex-col gap-1 text-xs font-medium text-zinc-500">
              平台
              <Select value={platform} onChange={(event) => setPlatform(event.target.value)}>
                <option value="">全部</option>
                <option value="SHEIN">SHEIN</option>
                <option value="TEMU">TEMU</option>
              </Select>
            </Label>
            <Label className="flex flex-col gap-1 text-xs font-medium text-zinc-500">
              店铺名称
              <Input
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                className="h-9 w-52"
                placeholder="搜索店铺"
              />
            </Label>
            <Button type="button" onClick={() => void storeQuery.refetch()} className="mt-5" variant="secondary">
              {loading ? <RefreshCw className="size-4 animate-spin" /> : <Search className="size-4" />}
              查询
            </Button>
          </form>
        </div>
        {visibleError ? (
          <Alert className="m-4" variant="destructive">
            <AlertDescription>{visibleError}</AlertDescription>
          </Alert>
        ) : null}
        <div className="overflow-x-auto">
          <Table className="min-w-full divide-y divide-zinc-200 text-sm">
            <TableHeader className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
              <TableRow>
                <TableHead className="px-4 py-3">店铺</TableHead>
                <TableHead className="px-4 py-3">账号</TableHead>
                <TableHead className="px-4 py-3">平台</TableHead>
                <TableHead className="px-4 py-3">地区</TableHead>
                <TableHead className="px-4 py-3">登录状态</TableHead>
                <TableHead className="px-4 py-3">自动上架</TableHead>
                <TableHead className="px-4 py-3 text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="divide-y divide-zinc-100">
              {loading ? (
                <TableRow>
                  <TableCell className="px-4 py-6 text-zinc-500" colSpan={7}>
                    加载中...
                  </TableCell>
                </TableRow>
              ) : stores.length === 0 ? (
                <TableRow>
                  <TableCell className="px-4 py-6 text-zinc-500" colSpan={7}>
                    暂无店铺
                  </TableCell>
                </TableRow>
              ) : (
                stores.map((store) => (
                  <TableRow key={store.id} className="align-top">
                    <TableCell className="px-4 py-3">
                      <div className="font-medium text-zinc-950">{store.name}</div>
                      <div className="text-xs text-zinc-500">
                        {[store.storeId, `#${store.id}`].filter(Boolean).join(" / ")}
                      </div>
                    </TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">{store.username}</TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">{store.platform}</TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">{store.region || "-"}</TableCell>
                    <TableCell className="px-4 py-3">
                      <StoreLoginStatusBadge
                        store={store}
                        status={loginStatusMap.get(store.id)}
                        failed={sheinLoginQuery.isError}
                      />
                    </TableCell>
                    <TableCell className="px-4 py-3">
                      <Badge variant={store.enableAutoListing ? "success" : "neutral"}>
                        {store.enableAutoListing ? "启用" : "关闭"}
                      </Badge>
                    </TableCell>
                    <TableCell className="px-4 py-3 text-right">
                      {store.platform === "SHEIN" ? (
                        <Button asChild type="button" className="mr-2" size="sm" variant="outline">
                          <Link href={`/listing-kits/shein-login?store_id=${store.id}`}>去登录</Link>
                        </Button>
                      ) : null}
                      <Button
                        type="button"
                        aria-label={`编辑 ${store.name}`}
                        onClick={() => handleEdit(store)}
                        className="mr-2"
                        size="icon"
                        variant="ghost"
                      >
                        <Pencil className="size-4" />
                      </Button>
                      <Button
                        type="button"
                        aria-label={`删除 ${store.name}`}
                        onClick={() => void handleDelete(store.id)}
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
        aria-label="租户店铺表单"
        onSubmit={handleSave}
        className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm"
      >
        <div className="mb-4 flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            {editingStoreId ? <Pencil className="size-4 text-zinc-500" /> : <Plus className="size-4 text-zinc-500" />}
            <h2 className="text-base font-semibold text-zinc-950">
              {editingStoreId ? "编辑店铺" : "新增店铺"}
            </h2>
          </div>
          {editingStoreId ? (
            <Button type="button" aria-label="取消编辑" onClick={resetForm} size="icon" variant="ghost">
              <X className="size-4" />
            </Button>
          ) : null}
        </div>
        <StoreInput label="店铺名称" value={form.name} onChange={(name) => setForm({ ...form, name })} />
        <StoreInput label="店铺 ID" value={form.storeId ?? ""} onChange={(storeId) => setForm({ ...form, storeId })} />
        <StoreInput label="登录用户名" value={form.username} onChange={(username) => setForm({ ...form, username })} />
        <StoreInput
          label="登录密码"
          type="password"
          value={form.password ?? ""}
          onChange={(password) => setForm({ ...form, password })}
        />
        <div className="grid grid-cols-2 gap-3">
          <StoreSelect
            label="平台"
            value={form.platform}
            onChange={(platformValue) => setForm({ ...form, platform: platformValue })}
            options={["SHEIN", "TEMU"]}
          />
          <StoreInput label="地区" value={form.region ?? ""} onChange={(region) => setForm({ ...form, region })} />
        </div>
        <StoreSelect
          label="店铺类型"
          value={normalizeStoreType(form.shopType)}
          onChange={(shopType) => setForm({ ...form, shopType })}
          options={STORE_TYPE_OPTIONS}
        />
        <StoreInput
          label="每日上架限制"
          type="number"
          value={String(form.dailyLimit ?? "")}
          onChange={(dailyLimit) => setForm({ ...form, dailyLimit: Number(dailyLimit) || undefined })}
        />
        <Label className="mb-3 flex items-center gap-2 text-sm text-zinc-700">
          <Input
            type="checkbox"
            checked={Boolean(form.enableAutoListing)}
            onChange={(event) => setForm({ ...form, enableAutoListing: event.target.checked })}
          />
          启用自动上架
        </Label>
        <Button type="submit" disabled={saving} className="w-full">
          {saving ? <RefreshCw className="size-4 animate-spin" /> : editingStoreId ? <Pencil className="size-4" /> : <Plus className="size-4" />}
          {editingStoreId ? "保存修改" : "保存店铺"}
        </Button>
      </form>
    </section>
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

function StoreSelect({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: readonly string[] | readonly { value: string; label: string }[];
}) {
  return (
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <Select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
      >
        {options.map((option) => {
          const item = typeof option === "string" ? { value: option, label: option } : option;
          return (
            <option key={item.value} value={item.value}>
              {item.label}
            </option>
          );
        })}
      </Select>
    </Label>
  );
}

function normalizeStoreType(value?: string) {
  switch ((value ?? "").trim()) {
    case "semi":
    case "semi_managed":
    case "0":
      return "0";
    case "full":
    case "full_managed":
    case "1":
      return "1";
    case "self":
    case "self_operated":
    case "2":
      return "2";
    default:
      return value?.trim() || "0";
  }
}

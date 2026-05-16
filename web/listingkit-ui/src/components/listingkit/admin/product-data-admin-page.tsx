"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { useQuery } from "@tanstack/react-query";
import { FormEvent, useMemo, useState } from "react";
import { PackageSearch, Plus, RefreshCw, Search, Trash2 } from "lucide-react";

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
  createListingProductData,
  deleteListingProductData,
  getListingProductData,
  updateListingProductDataStatus,
  type ListingProductData,
  type ListingProductDataInput,
} from "@/lib/api/admin-product-data";

const DEFAULT_FORM: ListingProductDataInput = {
  productId: "",
  platform: "SHEIN",
  region: "US",
  title: "",
  priceCurrency: "USD",
  stock: "",
  status: 1,
  shelfStatus: 0,
};

const STATUS_TEXT: Record<number, string> = {
  0: "禁用",
  1: "正常",
};

const SHELF_STATUS_TEXT: Record<number, string> = {
  0: "未上架",
  1: "待发布",
  2: "已上架",
  3: "已下架",
};

export function ProductDataAdminPage() {
  const [platform, setPlatform] = useState("");
  const [productId, setProductId] = useState("");
  const [platformProductId, setPlatformProductId] = useState("");
  const [shelfStatus, setShelfStatus] = useState("");
  const [form, setForm] = useState<ListingProductDataInput>(DEFAULT_FORM);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const query = useMemo(
    () => ({
      page: 1,
      page_size: 50,
      platform: platform || undefined,
      productId: productId || undefined,
      platformProductId: platformProductId || undefined,
      shelfStatus: shelfStatus ? Number(shelfStatus) : undefined,
    }),
    [platform, productId, platformProductId, shelfStatus],
  );

  const productDataQuery = useQuery({
    queryKey: ["listingkit-admin-product-data", query],
    queryFn: () => getListingProductData(query),
  });

  const items: ListingProductData[] = productDataQuery.data?.items ?? [];
  const total = productDataQuery.data?.total ?? 0;
  const loading = productDataQuery.isLoading || productDataQuery.isFetching;
  const visibleError =
    error ||
    (productDataQuery.error instanceof Error
      ? productDataQuery.error.message
      : "");

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await createListingProductData(normalizeForm(form));
      setForm(DEFAULT_FORM);
      await productDataQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleToggle(item: ListingProductData) {
    setError("");
    try {
      await updateListingProductDataStatus(item.id, item.status === 1 ? 0 : 1);
      await productDataQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  async function handleDelete(id: number) {
    setError("");
    try {
      await deleteListingProductData(id);
      await productDataQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">商品数据</h1>
            <p className="mt-1 text-sm text-zinc-500">
              共 {total} 条商品数据，按当前 ZITADEL 租户隔离。
            </p>
          </div>
          <form
            className="flex flex-wrap gap-2"
            onSubmit={(event) => event.preventDefault()}
          >
            <ProductDataSelect
              label="平台"
              value={platform}
              onChange={setPlatform}
              options={[
                ["", "全部"],
                ["SHEIN", "SHEIN"],
                ["Amazon", "Amazon"],
                ["TEMU", "TEMU"],
              ]}
            />
            <ProductDataInput
              label="商品 ID"
              value={productId}
              onChange={setProductId}
              placeholder="B001"
            />
            <ProductDataInput
              label="平台商品 ID"
              value={platformProductId}
              onChange={setPlatformProductId}
              placeholder="SPU-001"
            />
            <ProductDataSelect
              label="上架状态"
              value={shelfStatus}
              onChange={setShelfStatus}
              options={[
                ["", "全部"],
                ["0", "未上架"],
                ["1", "待发布"],
                ["2", "已上架"],
                ["3", "已下架"],
              ]}
            />
            <Button
              type="button"
              onClick={() => void productDataQuery.refetch()}
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
                  <TableHead className="px-4 py-3">商品</TableHead>
                  <TableHead className="px-4 py-3">平台</TableHead>
                  <TableHead className="px-4 py-3">价格/库存</TableHead>
                  <TableHead className="px-4 py-3">状态</TableHead>
                  <TableHead className="px-4 py-3">同步时间</TableHead>
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
                ) : items.length === 0 ? (
                  <TableRow>
                    <TableCell className="px-4 py-6 text-zinc-500" colSpan={6}>
                      暂无商品数据
                    </TableCell>
                  </TableRow>
                ) : (
                  items.map((item) => (
                    <TableRow key={item.id} className="align-top">
                      <TableCell className="px-4 py-3">
                        <div className="font-medium text-zinc-950">
                          {item.title || item.productId}
                        </div>
                        <div className="font-mono text-xs text-zinc-500">
                          {item.productId}
                        </div>
                        <div className="text-xs text-zinc-500">
                          父级 {item.parentProductId || "-"}
                        </div>
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        <div>{item.platform || "-"}</div>
                        <div className="text-xs text-zinc-500">
                          地区 {item.region || "-"}
                        </div>
                        <div className="font-mono text-xs text-zinc-500">
                          {item.platformProductId || "-"}
                        </div>
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        <div>
                          {formatPrice(item.specialPrice, item.priceCurrency)}
                          <span className="ml-2 text-xs text-zinc-500">
                            原价 {formatPrice(item.originalPrice, item.priceCurrency)}
                          </span>
                        </div>
                        <div className="text-xs text-zinc-500">
                          库存 {item.stock || "-"}
                        </div>
                      </TableCell>
                      <TableCell className="px-4 py-3">
                        <Button
                          type="button"
                          onClick={() => void handleToggle(item)}
                          variant="ghost"
                          className="mb-1 h-auto p-0 hover:bg-transparent"
                        >
                          <Badge variant={item.status === 1 ? "success" : "neutral"}>
                            {STATUS_TEXT[item.status] ?? `状态 ${item.status}`}
                          </Badge>
                        </Button>
                        <div className="text-xs text-zinc-500">
                          {SHELF_STATUS_TEXT[item.shelfStatus ?? -1] ?? "未知"}
                        </div>
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {formatDate(item.lastSyncTime)}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-right">
                        <Button
                          type="button"
                          aria-label={`删除 ${item.productId}`}
                          onClick={() => void handleDelete(item.id)}
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
            <PackageSearch className="size-4 text-zinc-500" />
            <h2 className="text-base font-semibold text-zinc-950">新增商品数据</h2>
          </div>
          <ProductDataInput
            label="商品 ID"
            value={form.productId}
            onChange={(nextProductId) =>
              setForm({ ...form, productId: nextProductId })
            }
          />
          <ProductDataInput
            label="商品标题"
            value={form.title ?? ""}
            onChange={(title) => setForm({ ...form, title })}
          />
          <div className="grid grid-cols-2 gap-3">
            <ProductDataSelect
              label="平台"
              value={form.platform ?? ""}
              onChange={(nextPlatform) =>
                setForm({ ...form, platform: nextPlatform })
              }
              options={[
                ["SHEIN", "SHEIN"],
                ["Amazon", "Amazon"],
                ["TEMU", "TEMU"],
              ]}
            />
            <ProductDataInput
              label="地区"
              value={form.region ?? ""}
              onChange={(region) => setForm({ ...form, region })}
            />
          </div>
          <ProductDataInput
            label="平台商品 ID"
            value={form.platformProductId ?? ""}
            onChange={(nextPlatformProductId) =>
              setForm({ ...form, platformProductId: nextPlatformProductId })
            }
          />
          <div className="grid grid-cols-2 gap-3">
            <ProductDataInput
              label="售价"
              type="number"
              value={String(form.specialPrice ?? "")}
              onChange={(specialPrice) =>
                setForm({
                  ...form,
                  specialPrice: numberOrUndefined(specialPrice),
                })
              }
            />
            <ProductDataInput
              label="库存"
              value={form.stock ?? ""}
              onChange={(stock) => setForm({ ...form, stock })}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <ProductDataSelect
              label="数据状态"
              value={String(form.status ?? 1)}
              onChange={(nextStatus) =>
                setForm({ ...form, status: Number(nextStatus) })
              }
              options={[
                ["1", "正常"],
                ["0", "禁用"],
              ]}
            />
            <ProductDataSelect
              label="上架状态"
              value={String(form.shelfStatus ?? 0)}
              onChange={(nextShelfStatus) =>
                setForm({ ...form, shelfStatus: Number(nextShelfStatus) })
              }
              options={[
                ["0", "未上架"],
                ["1", "待发布"],
                ["2", "已上架"],
                ["3", "已下架"],
              ]}
            />
          </div>
          <Button
            type="submit"
            disabled={saving || !form.productId.trim()}
            className="w-full"
          >
            {saving ? (
              <RefreshCw className="size-4 animate-spin" />
            ) : (
              <Plus className="size-4" />
            )}
            保存商品
          </Button>
        </form>
      </section>
    </div>
  );
}

function normalizeForm(input: ListingProductDataInput): ListingProductDataInput {
  return {
    ...input,
    platform: input.platform || undefined,
    region: input.region || undefined,
    title: input.title || undefined,
    stock: input.stock || undefined,
    priceCurrency: input.priceCurrency || undefined,
    platformProductId: input.platformProductId || undefined,
  };
}

function numberOrUndefined(value: string) {
  if (value.trim() === "") {
    return undefined;
  }
  const numberValue = Number(value);
  return Number.isFinite(numberValue) ? numberValue : undefined;
}

function formatPrice(value: number | undefined, currency = "USD") {
  if (value === undefined) {
    return "-";
  }
  return `${currency} ${value.toFixed(2)}`;
}

function formatDate(value: string | undefined) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function ProductDataInput({
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

function ProductDataSelect({
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

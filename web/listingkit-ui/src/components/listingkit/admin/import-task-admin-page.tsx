"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { useQuery } from "@tanstack/react-query";
import { ChangeEvent, FormEvent, useMemo, useState } from "react";
import { Plus, RefreshCw, Search, Trash2, Upload } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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
  batchCreateListingImportTasks,
  deleteListingImportTask,
  getListingImportTasks,
  type BatchCreateListingImportTaskInput,
  type ListingImportTask,
} from "@/lib/api/admin-import-tasks";
import {
  AdminStoreSelect,
  formatAdminStoreName,
  useAdminSimpleStores,
} from "@/components/listingkit/admin/admin-store-select";
import { SHEIN_SITE_OPTIONS } from "@/components/listingkit/stores/shein-site-options";

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

const REGION_OPTIONS = [...SHEIN_SITE_OPTIONS];

type ProductIdImportSummary = {
  fileName: string;
  importedCount: number;
  duplicateCount: number;
};

export function ImportTaskAdminPage() {
  const [platform, setPlatform] = useState("");
  const [productId, setProductId] = useState("");
  const [form, setForm] =
    useState<BatchCreateListingImportTaskInput>(DEFAULT_FORM);
  const [productText, setProductText] = useState("");
  const [productImportSummary, setProductImportSummary] =
    useState<ProductIdImportSummary | null>(null);
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
  const storesQuery = useAdminSimpleStores();

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
      const productIds = uniqueProductIdsFromText(productText).values;
      await batchCreateListingImportTasks({ ...form, productIds });
      setForm({ ...DEFAULT_FORM, storeId: form.storeId });
      setProductText("");
      setProductImportSummary(null);
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

  async function handleProductFileChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }
    setError("");
    try {
      const text = await file.text();
      const parsed = parseProductIdImportText(text);
      if (parsed.values.length === 0) {
        setError("文件中没有可导入的商品 ID");
        setProductImportSummary(null);
        return;
      }
      setProductText(parsed.values.join("\n"));
      setProductImportSummary({
        fileName: file.name,
        importedCount: parsed.values.length,
        duplicateCount: parsed.duplicateCount,
      });
    } catch (err) {
      setError(formatSubscriptionApiError(err));
      setProductImportSummary(null);
    } finally {
      event.target.value = "";
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">任务导入</h1>
            <p className="mt-1 text-sm text-zinc-500">
              共 {total} 个导入任务，按当前 ZITADEL 租户隔离。
            </p>
          </div>
          <form
            className="flex flex-col gap-2 sm:flex-row sm:flex-wrap"
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
            <Button
              type="button"
              onClick={() => void importTaskQuery.refetch()}
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

      <section className="grid gap-4 2xl:grid-cols-[minmax(0,1fr)_380px]">
        <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
          <div className="overflow-x-auto">
            <Table className="min-w-[58rem] divide-y divide-zinc-200 text-sm">
              <TableHeader className="bg-zinc-50 text-left text-xs font-semibold uppercase text-zinc-500">
                <TableRow>
                  <TableHead className="px-4 py-3">商品</TableHead>
                  <TableHead className="px-4 py-3">店铺</TableHead>
                  <TableHead className="px-4 py-3">平台</TableHead>
                  <TableHead className="px-4 py-3">地区</TableHead>
                  <TableHead className="px-4 py-3">状态</TableHead>
                  <TableHead className="px-4 py-3">调度原因</TableHead>
                  <TableHead className="px-4 py-3">重试</TableHead>
                  <TableHead className="px-4 py-3">优先级</TableHead>
                  <TableHead className="px-4 py-3 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className="divide-y divide-zinc-100">
                {loading ? (
                  <TableRow>
                    <TableCell className="px-4 py-6 text-zinc-500" colSpan={9}>
                      加载中...
                    </TableCell>
                  </TableRow>
                ) : tasks.length === 0 ? (
                  <TableRow>
                    <TableCell className="px-4 py-6 text-zinc-500" colSpan={9}>
                      暂无导入任务
                    </TableCell>
                  </TableRow>
                ) : (
                  tasks.map((task) => (
                    <TableRow key={task.id} className="align-top">
                      <TableCell className="px-4 py-3">
                        <div className="font-medium text-zinc-950">
                          {task.productId}
                        </div>
                        <div className="font-mono text-xs text-zinc-500">
                          #{task.id}
                        </div>
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {formatAdminStoreName(stores, task.storeId)}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {task.platform}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {task.region || "-"}
                      </TableCell>
                      <TableCell className="px-4 py-3">
                        <Badge className="rounded-full px-2 py-1 text-xs" variant="neutral">
                          {STATUS_TEXT[task.status] ?? `状态 ${task.status}`}
                        </Badge>
                      </TableCell>
                      <TableCell className="max-w-72 px-4 py-3 text-zinc-700">
                        <ImportTaskDelayReason task={task} />
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {task.retryCount ?? 0}/{task.maxRetryCount ?? 3}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-zinc-700">
                        {task.priority ?? 5}
                      </TableCell>
                      <TableCell className="px-4 py-3 text-right">
                        <Button
                          type="button"
                          aria-label={`删除 ${task.productId}`}
                          onClick={() => void handleDelete(task.id)}
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
            <Upload className="size-4 text-zinc-500" />
            <h2 className="text-base font-semibold text-zinc-950">批量导入</h2>
          </div>
          <AdminStoreSelect
            value={form.storeId}
            onChange={(storeId) => setForm({ ...form, storeId })}
            stores={stores}
            emptyLabel="选择店铺"
          />
          <div className="grid gap-3 sm:grid-cols-2">
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
          <div className="grid gap-3 sm:grid-cols-2">
            <ImportTaskSelect
              label="来源平台"
              value={form.platform}
              onChange={(nextPlatform) =>
                setForm({ ...form, platform: nextPlatform })
              }
              options={["Amazon", "SHEIN", "TEMU"]}
            />
            <ImportTaskSelect
              label="地区"
              value={form.region ?? ""}
              onChange={(region) => setForm({ ...form, region })}
              options={REGION_OPTIONS}
            />
          </div>
          <Label className="mb-3 block text-xs font-medium text-zinc-500">
            商品 ID
            <Textarea
              value={productText}
              onChange={(event) => setProductText(event.target.value)}
              className="mt-1 min-h-32 w-full resize-y rounded-md border border-zinc-200 px-3 py-2 text-sm text-zinc-900"
              placeholder="每行一个商品 ID"
            />
          </Label>
          <Label className="mb-3 block text-xs font-medium text-zinc-500">
            批量导入文件
            <Input
              type="file"
              accept=".csv,.txt,text/csv,text/plain"
              onChange={(event) => void handleProductFileChange(event)}
              className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
            />
          </Label>
          {productImportSummary ? (
            <div className="mb-3 rounded-md border border-zinc-200 bg-zinc-50 px-3 py-2 text-xs text-zinc-600">
              <div className="font-medium text-zinc-800">
                {productImportSummary.fileName}
              </div>
              <div>已读取 {productImportSummary.importedCount} 个商品 ID</div>
              {productImportSummary.duplicateCount > 0 ? (
                <div>已去重 {productImportSummary.duplicateCount} 个重复 ID</div>
              ) : null}
            </div>
          ) : null}
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
            导入任务
          </Button>
        </form>
      </section>
    </div>
  );
}

function ImportTaskDelayReason({ task }: { task: ListingImportTask }) {
  const reasonCode = firstText(
    task.reasonCode,
    (task as { reason_code?: unknown }).reason_code,
  );
  const stage = firstText(task.stage);
  const message = firstText(
    task.errorMessage,
    (task as { error_message?: unknown }).error_message,
    task.remark,
  );

  if (!reasonCode && !stage && !message) {
    return <span className="text-zinc-400">-</span>;
  }

  return (
    <div className="space-y-1">
      <div className="flex flex-wrap items-center gap-1.5">
        {reasonCode ? (
          <Badge className="rounded-full px-2 py-1 text-xs" variant="secondary">
            {reasonCode}
          </Badge>
        ) : null}
        {stage ? (
          <span className="font-mono text-xs text-zinc-500">{stage}</span>
        ) : null}
      </div>
      {message ? (
        <div className="line-clamp-2 text-xs leading-5 text-zinc-500">
          {message}
        </div>
      ) : null}
    </div>
  );
}

function firstText(...values: unknown[]) {
  for (const value of values) {
    if (typeof value !== "string") {
      continue;
    }
    const trimmed = value.trim();
    if (trimmed) {
      return trimmed;
    }
  }
  return "";
}

function parseProductIdImportText(text: string) {
  const rows = text
    .split(/\r?\n/)
    .map((row) => row.trim())
    .filter(Boolean);
  if (rows.length === 0) {
    return { values: [] as string[], duplicateCount: 0 };
  }

  const firstColumns = splitDelimitedRow(rows[0]);
  const productIdColumnIndex = firstColumns.findIndex((column) =>
    isProductIdHeader(column),
  );
  const dataRows = productIdColumnIndex >= 0 ? rows.slice(1) : rows;
  const rawValues = dataRows.flatMap((row) => {
    if (productIdColumnIndex >= 0) {
      return splitDelimitedRow(row)[productIdColumnIndex] ?? "";
    }
    return row.split(/[\t,，\s]+/);
  });
  return uniqueProductIds(rawValues);
}

function uniqueProductIdsFromText(text: string) {
  return uniqueProductIds(text.split(/[\n,，\s]+/));
}

function uniqueProductIds(values: string[]) {
  const seen = new Set<string>();
  const out: string[] = [];
  let duplicateCount = 0;
  for (const value of values) {
    const productId = value.trim();
    if (!productId) {
      continue;
    }
    if (seen.has(productId)) {
      duplicateCount += 1;
      continue;
    }
    seen.add(productId);
    out.push(productId);
  }
  return { values: out, duplicateCount };
}

function splitDelimitedRow(row: string) {
  if (row.includes("\t")) {
    return row.split("\t").map((column) => column.trim());
  }
  return row.split(/[，,]/).map((column) => column.trim());
}

function isProductIdHeader(value: string) {
  return ["product_id", "productid", "product id", "商品id", "商品 ID"]
    .map((header) => header.toLowerCase())
    .includes(value.trim().toLowerCase());
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
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <Select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 bg-white px-3 text-sm text-zinc-900"
      >
        {options.map((option) => (
          <option key={option} value={option}>
            {labels[option] ?? option}
          </option>
        ))}
      </Select>
    </Label>
  );
}

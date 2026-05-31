"use client";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { useQuery } from "@tanstack/react-query";
import { FormEvent, useMemo, useState } from "react";
import { FolderTree, Plus, RefreshCw, Search, Trash2 } from "lucide-react";

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
  createListingCategory,
  deleteListingCategory,
  getListingCategories,
  updateListingCategoryStatus,
  type ListingCategory,
  type ListingCategoryInput,
} from "@/lib/api/admin-categories";

const DEFAULT_FORM: ListingCategoryInput = {
  name: "",
  code: "",
  parentId: 0,
  level: 1,
  sort: 0,
  icon: "",
  image: "",
  description: "",
  status: 1,
};

export function CategoryAdminPage() {
  const [name, setName] = useState("");
  const [status, setStatus] = useState("");
  const [form, setForm] = useState<ListingCategoryInput>(DEFAULT_FORM);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const query = useMemo(
    () => ({
      name: name || undefined,
      status: status || undefined,
    }),
    [name, status],
  );

  const categoryQuery = useQuery({
    queryKey: ["listingkit-admin-categories", query],
    queryFn: () => getListingCategories(query),
  });

  const categories = categoryQuery.data ?? [];
  const loading = categoryQuery.isLoading || categoryQuery.isFetching;
  const visibleError =
    error ||
    (categoryQuery.error instanceof Error ? categoryQuery.error.message : "");

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await createListingCategory(form);
      setForm(DEFAULT_FORM);
      await categoryQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleToggle(category: ListingCategory) {
    setError("");
    try {
      await updateListingCategoryStatus(
        category.id,
        category.status === 1 ? 0 : 1,
      );
      await categoryQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  async function handleDelete(id: number) {
    setError("");
    try {
      await deleteListingCategory(id);
      await categoryQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">分类</h1>
            <p className="mt-1 text-sm text-zinc-500">
              共 {categories.length} 个分类，按当前 ZITADEL 租户隔离。
            </p>
          </div>
          <form
            className="flex flex-col gap-2 sm:flex-row sm:flex-wrap"
            onSubmit={(event) => event.preventDefault()}
          >
            <CategoryInput
              label="分类名称"
              value={name}
              onChange={setName}
              placeholder="Apparel"
            />
            <CategorySelect
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
              onClick={() => void categoryQuery.refetch()}
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
            <Table className="min-w-[52rem]">
              <TableHeader className="bg-zinc-50">
                <TableRow className="text-xs uppercase tracking-[0.2em] hover:bg-transparent">
                  <TableHead>分类</TableHead>
                  <TableHead>编码</TableHead>
                  <TableHead>父级</TableHead>
                  <TableHead>层级</TableHead>
                  <TableHead>排序</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  <TableRow>
                    <TableCell className="py-6 text-zinc-500" colSpan={7}>
                      加载中...
                    </TableCell>
                  </TableRow>
                ) : categories.length === 0 ? (
                  <TableRow>
                    <TableCell className="py-6 text-zinc-500" colSpan={7}>
                      暂无分类
                    </TableCell>
                  </TableRow>
                ) : (
                  categories.map((category) => (
                    <TableRow key={category.id} className="align-top">
                      <TableCell>
                        <div className="font-medium text-zinc-950">
                          {category.name}
                        </div>
                        <div className="text-xs text-zinc-500">
                          {category.description || "-"}
                        </div>
                      </TableCell>
                      <TableCell className="font-mono text-xs text-zinc-700">
                        {category.code}
                      </TableCell>
                      <TableCell className="text-zinc-700">
                        {category.parentId > 0 ? `#${category.parentId}` : "顶级"}
                      </TableCell>
                      <TableCell className="text-zinc-700">
                        {category.level}
                      </TableCell>
                      <TableCell className="text-zinc-700">
                        {category.sort}
                      </TableCell>
                      <TableCell>
                        <Button
                          type="button"
                          onClick={() => void handleToggle(category)}
                          variant="ghost"
                          className="h-auto p-0 hover:bg-transparent"
                        >
                          <Badge variant={category.status === 1 ? "success" : "neutral"}>
                            {category.status === 1 ? "启用" : "禁用"}
                          </Badge>
                        </Button>
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          type="button"
                          aria-label={`删除 ${category.name}`}
                          onClick={() => void handleDelete(category.id)}
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
            <FolderTree className="size-4 text-zinc-500" />
            <h2 className="text-base font-semibold text-zinc-950">新增分类</h2>
          </div>
          <CategoryInput
            label="分类名称"
            value={form.name}
            onChange={(nextName) => setForm({ ...form, name: nextName })}
          />
          <CategoryInput
            label="分类编码"
            value={form.code}
            onChange={(code) => setForm({ ...form, code })}
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <CategoryInput
              label="父级 ID"
              type="number"
              value={String(form.parentId ?? 0)}
              onChange={(value) =>
                setForm({ ...form, parentId: Number(value) || 0 })
              }
            />
            <CategoryInput
              label="分类层级"
              type="number"
              value={String(form.level ?? 1)}
              onChange={(value) =>
                setForm({ ...form, level: Number(value) || 1 })
              }
            />
          </div>
          <CategoryInput
            label="显示顺序"
            type="number"
            value={String(form.sort ?? 0)}
            onChange={(value) => setForm({ ...form, sort: Number(value) || 0 })}
          />
          <CategoryInput
            label="分类描述"
            value={form.description ?? ""}
            onChange={(description) => setForm({ ...form, description })}
          />
          <Button
            type="submit"
            disabled={saving || !form.name.trim() || !form.code.trim()}
            className="w-full"
          >
            {saving ? (
              <RefreshCw className="size-4 animate-spin" />
            ) : (
              <Plus className="size-4" />
            )}
            保存分类
          </Button>
        </form>
      </section>
    </div>
  );
}

function CategoryInput({
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

function CategorySelect({
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

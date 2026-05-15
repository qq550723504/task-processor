"use client";

import { useQuery } from "@tanstack/react-query";
import { FormEvent, useMemo, useState } from "react";
import { FolderTree, Plus, RefreshCw, Search, Trash2 } from "lucide-react";

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
      setError(err instanceof Error ? err.message : String(err));
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
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleDelete(id: number) {
    setError("");
    try {
      await deleteListingCategory(id);
      await categoryQuery.refetch();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">分类</h1>
            <p className="mt-1 text-sm text-zinc-500">
              共 {categories.length} 个分类，按当前 ZITADEL 租户隔离。
            </p>
          </div>
          <form
            className="flex flex-wrap gap-2"
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
            <button
              type="button"
              onClick={() => void categoryQuery.refetch()}
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
                  <th className="px-4 py-3">分类</th>
                  <th className="px-4 py-3">编码</th>
                  <th className="px-4 py-3">父级</th>
                  <th className="px-4 py-3">层级</th>
                  <th className="px-4 py-3">排序</th>
                  <th className="px-4 py-3">状态</th>
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
                ) : categories.length === 0 ? (
                  <tr>
                    <td className="px-4 py-6 text-zinc-500" colSpan={7}>
                      暂无分类
                    </td>
                  </tr>
                ) : (
                  categories.map((category) => (
                    <tr key={category.id} className="align-top">
                      <td className="px-4 py-3">
                        <div className="font-medium text-zinc-950">
                          {category.name}
                        </div>
                        <div className="text-xs text-zinc-500">
                          {category.description || "-"}
                        </div>
                      </td>
                      <td className="px-4 py-3 font-mono text-xs text-zinc-700">
                        {category.code}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {category.parentId > 0 ? `#${category.parentId}` : "顶级"}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {category.level}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {category.sort}
                      </td>
                      <td className="px-4 py-3">
                        <button
                          type="button"
                          onClick={() => void handleToggle(category)}
                          className="rounded-full bg-zinc-100 px-2 py-1 text-xs font-medium text-zinc-700 hover:bg-zinc-200"
                        >
                          {category.status === 1 ? "启用" : "禁用"}
                        </button>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          type="button"
                          aria-label={`删除 ${category.name}`}
                          onClick={() => void handleDelete(category.id)}
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
          <div className="grid grid-cols-2 gap-3">
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
          <button
            type="submit"
            disabled={saving || !form.name.trim() || !form.code.trim()}
            className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-md bg-zinc-950 px-3 text-sm font-medium text-white hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-400"
          >
            {saving ? (
              <RefreshCw className="size-4 animate-spin" />
            ) : (
              <Plus className="size-4" />
            )}
            保存分类
          </button>
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

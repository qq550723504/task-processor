"use client";

import { useQuery } from "@tanstack/react-query";
import { FormEvent, useMemo, useState } from "react";
import { Plus, RefreshCw, Search, ShieldAlert, Trash2 } from "lucide-react";

import { formatSubscriptionApiError } from "@/lib/api/subscription";

import {
  createListingSensitiveWord,
  deleteListingSensitiveWord,
  getListingSensitiveWords,
  updateListingSensitiveWordStatus,
  type ListingSensitiveWord,
  type ListingSensitiveWordInput,
} from "@/lib/api/admin-sensitive-words";

const DEFAULT_FORM: ListingSensitiveWordInput = {
  word: "",
  language: "en",
  tags: "",
  level: 1,
  replaceWord: "",
  remark: "",
  status: 1,
};

const LEVEL_LABEL: Record<number, string> = {
  1: "低风险",
  2: "中风险",
  3: "高风险",
};

export function SensitiveWordAdminPage() {
  const [word, setWord] = useState("");
  const [language, setLanguage] = useState("");
  const [status, setStatus] = useState("");
  const [form, setForm] = useState<ListingSensitiveWordInput>(DEFAULT_FORM);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const query = useMemo(
    () => ({
      page: 1,
      page_size: 50,
      word: word || undefined,
      language: language || undefined,
      status: status || undefined,
    }),
    [word, language, status],
  );

  const sensitiveWordQuery = useQuery({
    queryKey: ["listingkit-admin-sensitive-words", query],
    queryFn: () => getListingSensitiveWords(query),
  });

  const words: ListingSensitiveWord[] = sensitiveWordQuery.data?.items ?? [];
  const total = sensitiveWordQuery.data?.total ?? 0;
  const loading =
    sensitiveWordQuery.isLoading || sensitiveWordQuery.isFetching;
  const visibleError =
    error ||
    (sensitiveWordQuery.error instanceof Error
      ? sensitiveWordQuery.error.message
      : "");

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await createListingSensitiveWord(form);
      setForm(DEFAULT_FORM);
      await sensitiveWordQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleToggle(item: ListingSensitiveWord) {
    setError("");
    try {
      await updateListingSensitiveWordStatus(item.id, item.status === 1 ? 0 : 1);
      await sensitiveWordQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  async function handleDelete(id: number) {
    setError("");
    try {
      await deleteListingSensitiveWord(id);
      await sensitiveWordQuery.refetch();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-zinc-950">敏感词</h1>
            <p className="mt-1 text-sm text-zinc-500">
              共 {total} 条敏感词，按当前 ZITADEL 租户隔离。
            </p>
          </div>
          <form
            className="flex flex-wrap gap-2"
            onSubmit={(event) => event.preventDefault()}
          >
            <SensitiveInput
              label="敏感词"
              value={word}
              onChange={setWord}
              placeholder="restricted"
            />
            <SensitiveInput
              label="语言"
              value={language}
              onChange={setLanguage}
              placeholder="en"
            />
            <SensitiveSelect
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
              onClick={() => void sensitiveWordQuery.refetch()}
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
                  <th className="px-4 py-3">敏感词</th>
                  <th className="px-4 py-3">语言</th>
                  <th className="px-4 py-3">级别</th>
                  <th className="px-4 py-3">标签</th>
                  <th className="px-4 py-3">替换词</th>
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
                ) : words.length === 0 ? (
                  <tr>
                    <td className="px-4 py-6 text-zinc-500" colSpan={7}>
                      暂无敏感词
                    </td>
                  </tr>
                ) : (
                  words.map((item) => (
                    <tr key={item.id} className="align-top">
                      <td className="px-4 py-3">
                        <div className="font-medium text-zinc-950">
                          {item.word}
                        </div>
                        <div className="text-xs text-zinc-500">
                          {item.remark || "-"}
                        </div>
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {item.language}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {LEVEL_LABEL[item.level] ?? item.level}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {item.tags || "-"}
                      </td>
                      <td className="px-4 py-3 text-zinc-700">
                        {item.replaceWord || "-"}
                      </td>
                      <td className="px-4 py-3">
                        <button
                          type="button"
                          onClick={() => void handleToggle(item)}
                          className="rounded-full bg-zinc-100 px-2 py-1 text-xs font-medium text-zinc-700 hover:bg-zinc-200"
                        >
                          {item.status === 1 ? "启用" : "禁用"}
                        </button>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          type="button"
                          aria-label={`删除 ${item.word}`}
                          onClick={() => void handleDelete(item.id)}
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
            <ShieldAlert className="size-4 text-zinc-500" />
            <h2 className="text-base font-semibold text-zinc-950">新增敏感词</h2>
          </div>
          <SensitiveInput
            label="敏感词内容"
            value={form.word}
            onChange={(nextWord) => setForm({ ...form, word: nextWord })}
          />
          <SensitiveInput
            label="语言类型"
            value={form.language}
            onChange={(nextLanguage) =>
              setForm({ ...form, language: nextLanguage })
            }
          />
          <div className="grid grid-cols-2 gap-3">
            <SensitiveSelect
              label="敏感级别"
              value={String(form.level ?? 1)}
              onChange={(level) => setForm({ ...form, level: Number(level) })}
              options={[
                ["1", "低风险"],
                ["2", "中风险"],
                ["3", "高风险"],
              ]}
            />
            <SensitiveSelect
              label="状态"
              value={String(form.status ?? 1)}
              onChange={(nextStatus) =>
                setForm({ ...form, status: Number(nextStatus) })
              }
              options={[
                ["1", "启用"],
                ["0", "禁用"],
              ]}
            />
          </div>
          <SensitiveInput
            label="敏感词标签"
            value={form.tags ?? ""}
            onChange={(tags) => setForm({ ...form, tags })}
          />
          <SensitiveInput
            label="替换词"
            value={form.replaceWord ?? ""}
            onChange={(replaceWord) => setForm({ ...form, replaceWord })}
          />
          <SensitiveInput
            label="备注"
            value={form.remark ?? ""}
            onChange={(remark) => setForm({ ...form, remark })}
          />
          <button
            type="submit"
            disabled={saving || !form.word.trim() || !form.language.trim()}
            className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-md bg-zinc-950 px-3 text-sm font-medium text-white hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-400"
          >
            {saving ? (
              <RefreshCw className="size-4 animate-spin" />
            ) : (
              <Plus className="size-4" />
            )}
            保存敏感词
          </button>
        </form>
      </section>
    </div>
  );
}

function SensitiveInput({
  label,
  value,
  placeholder,
  onChange,
}: {
  label: string;
  value: string;
  placeholder?: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <input
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
      />
    </label>
  );
}

function SensitiveSelect({
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

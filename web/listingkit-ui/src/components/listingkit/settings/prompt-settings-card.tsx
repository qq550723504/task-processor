"use client";

import { useMemo, useState } from "react";
import { Braces, FileText, Save, ToggleLeft, ToggleRight } from "lucide-react";

import { Button } from "@/components/shared/button";
import {
  usePromptSettings,
  useSetPromptSettingStatus,
  useUpsertPromptSetting,
} from "@/lib/query/use-prompt-settings";
import type { PromptSetting } from "@/lib/types/listingkit";

type PromptForm = {
  key: string;
  content: string;
  version: string;
  enabled: boolean;
};

const emptyForm: PromptForm = {
  key: "",
  content: "",
  version: "",
  enabled: true,
};

export function PromptSettingsCard() {
  const prompts = usePromptSettings();
  const upsert = useUpsertPromptSetting();
  const setStatus = useSetPromptSettingStatus();
  const items = useMemo(
    () => [...(prompts.data?.items ?? [])].sort((a, b) => a.key.localeCompare(b.key)),
    [prompts.data?.items],
  );
  const [selectedKey, setSelectedKey] = useState("");
  const [draft, setDraft] = useState<PromptForm>(emptyForm);

  const selectPrompt = (item: PromptSetting) => {
    setSelectedKey(item.key);
    setDraft({
      key: item.key,
      content: item.content ?? "",
      version: item.version ?? "",
      enabled: item.enabled ?? true,
    });
  };

  const updateDraft = <Key extends keyof PromptForm>(key: Key, value: PromptForm[Key]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const save = () => {
    upsert.mutate({
      key: draft.key.trim(),
      content: draft.content,
      version: draft.version.trim(),
      enabled: draft.enabled,
    });
  };

  const toggleEnabled = (item: PromptSetting) => {
    setStatus.mutate({ key: item.key, enabled: !(item.enabled ?? true) });
  };

  return (
    <section className="rounded-[1.5rem] border border-white/70 bg-white/86 p-4 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.24em] text-teal-700">
            <Braces className="h-4 w-4" />
            Prompt 管理
          </div>
          <h2 className="mt-1 text-lg font-semibold text-zinc-950">
            当前客户提示词模板
          </h2>
          <p className="mt-1 max-w-3xl text-sm leading-6 text-zinc-600">
            提示词按 ZITADEL 租户隔离保存。禁用或缺失的模板会被后端视为不可用，不会回落到代码模板。
          </p>
        </div>
        <Button
          onClick={() => {
            setSelectedKey("");
            setDraft(emptyForm);
          }}
        >
          新建提示词
        </Button>
      </div>

      <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(280px,0.9fr)_minmax(360px,1.1fr)]">
        <div className="min-h-[340px] overflow-hidden rounded-2xl border border-zinc-200 bg-zinc-50/70">
          <div className="flex items-center justify-between border-b border-zinc-200 bg-white px-3 py-2">
            <span className="text-sm font-semibold text-zinc-900">模板列表</span>
            <span className="text-xs text-zinc-500">{items.length} 个</span>
          </div>
          <div className="max-h-[520px] overflow-auto p-2">
            {prompts.isLoading ? (
              <div className="px-3 py-6 text-sm text-zinc-500">加载中...</div>
            ) : null}
            {!prompts.isLoading && items.length === 0 ? (
              <div className="px-3 py-6 text-sm text-zinc-500">暂无提示词模板</div>
            ) : null}
            {items.map((item) => {
              const enabled = item.enabled ?? true;
              const active = selectedKey === item.key;
              return (
                <div
                  key={item.key}
                  className={[
                    "mb-2 grid grid-cols-[1fr_auto] gap-2 rounded-xl border p-2",
                    active
                      ? "border-zinc-900 bg-white shadow-sm"
                      : "border-zinc-200 bg-white/70",
                  ].join(" ")}
                >
                  <button
                    className="min-w-0 text-left"
                    type="button"
                    onClick={() => selectPrompt(item)}
                  >
                    <span className="flex items-center gap-2 text-sm font-semibold text-zinc-900">
                      <FileText className="h-4 w-4 shrink-0 text-teal-700" />
                      <span className="truncate">{item.key}</span>
                    </span>
                    <span className="mt-1 block truncate text-xs text-zinc-500">
                      {item.version ? `版本 ${item.version}` : "未设置版本"}
                    </span>
                  </button>
                  <button
                    aria-label={enabled ? "禁用提示词" : "启用提示词"}
                    className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-zinc-200 bg-white text-zinc-600"
                    type="button"
                    onClick={() => toggleEnabled(item)}
                  >
                    {enabled ? <ToggleRight className="h-4 w-4" /> : <ToggleLeft className="h-4 w-4" />}
                  </button>
                </div>
              );
            })}
          </div>
        </div>

        <div className="rounded-2xl border border-zinc-200 bg-zinc-50/70 p-4">
          <div className="grid gap-3 md:grid-cols-[1fr_160px_auto]">
            <label className="space-y-1">
              <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
                Prompt Key
              </span>
              <input
                aria-label="Prompt Key"
                className="h-10 w-full rounded-xl border border-zinc-200 bg-white px-3 text-sm outline-none focus:border-zinc-400"
                value={draft.key}
                onChange={(event) => updateDraft("key", event.target.value)}
              />
            </label>
            <label className="space-y-1">
              <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
                版本
              </span>
              <input
                aria-label="版本"
                className="h-10 w-full rounded-xl border border-zinc-200 bg-white px-3 text-sm outline-none focus:border-zinc-400"
                value={draft.version}
                onChange={(event) => updateDraft("version", event.target.value)}
              />
            </label>
            <label className="mt-5 flex h-10 items-center gap-2 rounded-xl border border-zinc-200 bg-white px-3 text-sm text-zinc-700">
              <input
                checked={draft.enabled}
                className="h-4 w-4"
                type="checkbox"
                onChange={(event) => updateDraft("enabled", event.target.checked)}
              />
              启用
            </label>
          </div>
          <label className="mt-3 block space-y-1">
            <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
              Prompt 内容
            </span>
            <textarea
              aria-label="Prompt 内容"
              className="min-h-[300px] w-full resize-y rounded-xl border border-zinc-200 bg-white px-3 py-3 font-mono text-sm leading-6 outline-none focus:border-zinc-400"
              value={draft.content}
              onChange={(event) => updateDraft("content", event.target.value)}
            />
          </label>
          <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
            <div className="text-xs text-zinc-500">
              {draft.key ? "保存后当前租户立即使用该模板" : "填写 key 后保存新模板"}
            </div>
            <Button className="gap-2" disabled={upsert.isPending || !draft.key.trim()} onClick={save}>
              <Save className="h-4 w-4" />
              {upsert.isPending ? "保存中..." : "保存提示词"}
            </Button>
          </div>
          {prompts.isError ? (
            <p className="mt-3 text-sm text-rose-600">提示词列表读取失败。</p>
          ) : null}
          {upsert.error || setStatus.error ? (
            <p className="mt-3 text-sm text-rose-600">提示词保存失败。</p>
          ) : null}
        </div>
      </div>
    </section>
  );
}

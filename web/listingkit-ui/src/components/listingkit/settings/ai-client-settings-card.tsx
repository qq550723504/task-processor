"use client";

import { useMemo, useState } from "react";
import { KeyRound, ServerCog } from "lucide-react";

import { Button } from "@/components/shared/button";
import { useAIClientSettings, useUpdateAIClientSettings } from "@/lib/query/use-ai-client-settings";

type AISettingsForm = {
  scope: "tenant" | "user";
  user_id: string;
  client_name: string;
  api_key: string;
  base_url: string;
  model: string;
  timeout_second: string;
  enabled: boolean;
};

export function AIClientSettingsCard() {
  const [scope, setScope] = useState<"tenant" | "user">("tenant");
  const [userId, setUserId] = useState("");
  const settings = useAIClientSettings(scope, "default", userId);
  const update = useUpdateAIClientSettings();
  const [draft, setDraft] = useState<AISettingsForm | null>(null);

  const loadedForm = useMemo<AISettingsForm>(() => {
    const data = settings.data;
    return {
      scope,
      user_id: userId,
      client_name: data?.client_name ?? "default",
      api_key: "",
      base_url: data?.base_url ?? "",
      model: data?.model ?? "",
      timeout_second: String(data?.timeout_second ?? 60),
      enabled: data?.enabled ?? true,
    };
  }, [scope, settings.data, userId]);
  const form = draft ?? loadedForm;

  const set = <Key extends keyof AISettingsForm>(
    key: Key,
    value: AISettingsForm[Key],
  ) => setDraft((current) => ({ ...(current ?? loadedForm), [key]: value }));

  const submit = () => {
    update.mutate({
      scope: form.scope,
      user_id: form.scope === "user" ? form.user_id.trim() : undefined,
      client_name: form.client_name || "default",
      api_key: form.api_key.trim() || undefined,
      base_url: form.base_url.trim(),
      model: form.model.trim(),
      timeout_second: Number(form.timeout_second) || 0,
      enabled: form.enabled,
    });
  };

  return (
    <section className="rounded-[1.5rem] border border-white/70 bg-white/86 p-4 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.24em] text-teal-700">
            <ServerCog className="h-4 w-4" />
            AI 配置
          </div>
          <h2 className="mt-1 text-lg font-semibold text-zinc-950">
            客户自有模型接口
          </h2>
          <p className="mt-1 max-w-3xl text-sm leading-6 text-zinc-600">
            每个租户或用户可以配置自己的 OpenAI 兼容 endpoint、API Key 和默认模型。任务运行时会优先使用用户配置，再回退到租户配置。
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span className="inline-flex h-9 items-center gap-2 rounded-xl border border-zinc-200 bg-zinc-50 px-3 text-xs font-medium text-zinc-600">
            <KeyRound className="h-4 w-4" />
            {settings.data?.api_key_set ? "密钥已配置" : "密钥未配置"}
          </span>
          <Button disabled={update.isPending} onClick={submit}>
            {update.isPending ? "保存中..." : "保存 AI 配置"}
          </Button>
        </div>
      </div>

      <div className="mt-4 grid gap-3 md:grid-cols-[180px_1fr_1fr_140px]">
        <label className="space-y-1">
          <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
            配置范围
          </span>
          <select
            aria-label="配置范围"
            className="h-10 w-full rounded-xl border border-zinc-200 bg-white px-3 text-sm"
            value={form.scope}
            onChange={(event) => {
              const nextScope = event.target.value as "tenant" | "user";
              setScope(nextScope);
              setDraft(null);
            }}
          >
            <option value="tenant">当前客户</option>
            <option value="user">当前用户</option>
          </select>
          <span className="block text-[11px] leading-4 text-zinc-500">
            用户配置优先于客户配置
          </span>
        </label>
        {form.scope === "user" ? (
          <Input
            label="User ID"
            hint="用于给指定用户覆盖客户默认配置"
            value={form.user_id}
            onChange={(value) => {
              setUserId(value);
              set("user_id", value);
            }}
          />
        ) : null}
        <Input
          label="Endpoint"
          hint="OpenAI 兼容接口地址，例如 https://api.openai.com/v1"
          value={form.base_url}
          onChange={(value) => set("base_url", value)}
        />
        <Input
          label="Model"
          hint="用于文案、属性映射和提示词生成的默认模型"
          value={form.model}
          onChange={(value) => set("model", value)}
        />
        <Input
          label="超时秒数"
          hint="留空时使用系统默认"
          value={form.timeout_second}
          onChange={(value) => set("timeout_second", value)}
        />
      </div>
      <div className="mt-3 grid gap-3 md:grid-cols-[1fr_auto] md:items-end">
        <Input
          label="API Key"
          hint={settings.data?.api_key_set ? "留空表示继续使用已保存密钥" : "保存后后端不会回显明文密钥"}
          type="password"
          value={form.api_key}
          onChange={(value) => set("api_key", value)}
        />
        <label className="flex h-10 items-center gap-2 rounded-xl border border-zinc-200 bg-white px-3 text-sm text-zinc-700">
          <input
            checked={form.enabled}
            className="h-4 w-4"
            type="checkbox"
            onChange={(event) => set("enabled", event.target.checked)}
          />
          启用配置
        </label>
      </div>
      {settings.isError ? (
        <p className="mt-3 text-sm text-rose-600">AI 配置读取失败，请检查后端配置接口。</p>
      ) : null}
      {update.error ? (
        <p className="mt-3 text-sm text-rose-600">AI 配置保存失败，请检查 endpoint、model 和 API Key。</p>
      ) : null}
    </section>
  );
}

function Input({
  label,
  hint,
  type = "text",
  value,
  onChange,
}: {
  label: string;
  hint?: string;
  type?: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="space-y-1">
      <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
        {label}
      </span>
      <input
        aria-label={label}
        className="h-10 w-full rounded-xl border border-zinc-200 bg-white px-3 text-sm outline-none focus:border-zinc-400"
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
      {hint ? (
        <span className="block text-[11px] leading-4 text-zinc-500">{hint}</span>
      ) : null}
    </label>
  );
}

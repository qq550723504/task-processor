"use client";

import { useMemo, useState } from "react";
import { Image, KeyRound, ServerCog, Sparkles, Tags } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input as TextInput } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { ListingKitSettingsSection } from "@/components/listingkit/settings/listingkit-settings-section";
import { useAIClientSettings, useUpdateAIClientSettings } from "@/lib/query/use-ai-client-settings";

const aiClientOptions = [
  {
    name: "default",
    label: "通用文案",
    description: "SHEIN 文案优化、标题与通用提示词生成",
    modelHint: "用于文案、分类和通用提示词的默认模型",
    icon: Sparkles,
  },
  {
    name: "scorer",
    label: "属性映射",
    description: "SHEIN 属性、销售属性和打分映射",
    modelHint: "用于属性判断和映射的专用模型",
    icon: Tags,
  },
  {
    name: "image_nanobanana",
    label: "Nano Banana",
    description: "SHEIN Studio 默认生图链路，单独配置 Nano Banana 的地址和密钥",
    modelHint: "例如 nano-banana-fast",
    icon: Image,
  },
  {
    name: "image_gpt_image_2",
    label: "GPT Image 2",
    description: "透明背景和 GPT Image 2 生图链路，单独配置它的地址和密钥",
    modelHint: "例如 gpt-image-2",
    icon: Image,
  },
] as const;

type AIClientName = (typeof aiClientOptions)[number]["name"];

type AISettingsForm = {
  scope: "tenant" | "user";
  user_id: string;
  client_name: AIClientName;
  api_key: string;
  base_url: string;
  model: string;
  timeout_second: string;
  enabled: boolean;
};

export function AIClientSettingsCard() {
  const [scope, setScope] = useState<"tenant" | "user">("tenant");
  const [userId, setUserId] = useState("");
  const [clientName, setClientName] = useState<AIClientName>("default");
  const settings = useAIClientSettings(scope, clientName, userId);
  const update = useUpdateAIClientSettings();
  const [draft, setDraft] = useState<AISettingsForm | null>(null);
  const selectedClient =
    aiClientOptions.find((option) => option.name === clientName) ?? aiClientOptions[0];

  const loadedForm = useMemo<AISettingsForm>(() => {
    const data = settings.data;
    return {
      scope,
      user_id: userId,
      client_name: (data?.client_name as AIClientName | undefined) ?? clientName,
      api_key: "",
      base_url: data?.base_url ?? "",
      model: data?.model ?? "",
      timeout_second: String(data?.timeout_second ?? 60),
      enabled: data?.enabled ?? true,
    };
  }, [clientName, scope, settings.data, userId]);
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
    <ListingKitSettingsSection
      id="ai"
      eyebrow="AI 配置"
      title="客户自有模型接口"
      description="每个租户或用户可以配置自己的 OpenAI 兼容 endpoint、API Key 和默认模型。任务运行时会优先使用用户配置，再回退到租户配置。"
      actions={
        <>
          <span className="inline-flex h-9 items-center gap-2 rounded-xl border border-zinc-200 bg-zinc-50 px-3 text-xs font-medium text-zinc-600">
            <KeyRound className="h-4 w-4" />
            {settings.data?.api_key_set ? "密钥已配置" : "密钥未配置"}
          </span>
          <Button disabled={update.isPending} onClick={submit}>
            {update.isPending ? "保存中..." : `保存 ${selectedClient.label} 配置`}
          </Button>
        </>
      }
    >

      <div className="mt-4 flex flex-wrap gap-2">
        {aiClientOptions.map((option) => {
          const Icon = option.icon;
          const active = option.name === clientName;
          return (
            <Button
              key={option.name}
              variant={active ? "default" : "outline"}
              className="h-auto min-w-[160px] justify-start rounded-xl px-3 py-3 text-left"
              type="button"
              onClick={() => {
                setClientName(option.name);
                setDraft(null);
              }}
            >
              <div className="flex items-center gap-2 text-sm font-semibold">
                <Icon className="h-4 w-4" />
                {option.label}
              </div>
              <div
                className={[
                  "mt-1 text-xs leading-5",
                  active ? "text-white/78" : "text-zinc-500",
                ].join(" ")}
              >
                {option.description}
              </div>
            </Button>
          );
        })}
      </div>

      <div className="mt-4 rounded-2xl border border-zinc-200 bg-zinc-50/70 p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="text-sm font-semibold text-zinc-950">{selectedClient.label}</div>
            <div className="mt-1 text-sm text-zinc-600">{selectedClient.description}</div>
          </div>
          <div className="rounded-xl border border-zinc-200 bg-white px-3 py-2 text-xs text-zinc-500">
            client_name: <span className="font-semibold text-zinc-800">{selectedClient.name}</span>
          </div>
        </div>

        <div className="mt-4 grid gap-3 md:grid-cols-[180px_1fr_1fr_140px]">
          <Label className="space-y-1">
            <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
              配置范围
            </span>
            <Select
              aria-label="配置范围"
              value={form.scope}
              onChange={(event) => {
                const nextScope = event.target.value as "tenant" | "user";
                setScope(nextScope);
                setDraft(null);
              }}
            >
              <option value="tenant">当前客户</option>
              <option value="user">当前用户</option>
            </Select>
            <span className="block text-[11px] leading-4 text-zinc-500">
              用户配置优先于客户配置
            </span>
          </Label>
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
            hint={selectedClient.modelHint}
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
          <Label className="flex h-10 items-center gap-2 rounded-md border border-input bg-background px-3 text-sm text-foreground">
            <Checkbox
              checked={form.enabled}
              onChange={(event) => set("enabled", event.target.checked)}
            />
            启用配置
          </Label>
        </div>
      </div>
      {settings.isError ? (
        <p className="mt-3 text-sm text-rose-600">AI 配置读取失败，请检查后端配置接口。</p>
      ) : null}
      {update.error ? (
        <p className="mt-3 text-sm text-rose-600">AI 配置保存失败，请检查 endpoint、model 和 API Key。</p>
      ) : null}
    </ListingKitSettingsSection>
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
    <Label className="space-y-1">
      <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
        {label}
      </span>
      <TextInput
        aria-label={label}
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
      {hint ? (
        <span className="block text-[11px] leading-4 text-zinc-500">{hint}</span>
      ) : null}
    </Label>
  );
}

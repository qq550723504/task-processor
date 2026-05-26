"use client";

import { useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { ListingKitSettingsSection } from "@/components/listingkit/settings/listingkit-settings-section";
import { useStoreProfiles } from "@/lib/query/use-store-profiles";
import { useStoreRouting, useUpdateStoreRouting } from "@/lib/query/use-store-routing";
import { formatSheinStoreOptionLabel } from "@/lib/shein-studio/store-option-label";

type RoutingForm = {
  selection_strategy: string;
  fallback_store_id: string;
  allow_manual_override: boolean;
  allow_fallback: boolean;
};

export function StoreRoutingSettingsCard() {
  const routing = useStoreRouting();
  const profiles = useStoreProfiles();
  const update = useUpdateStoreRouting();
  const [draft, setDraft] = useState<RoutingForm | null>(null);

  const loadedForm = useMemo<RoutingForm>(() => ({
    selection_strategy: routing.data?.selection_strategy ?? "manual",
    fallback_store_id: routing.data?.fallback_store_id
      ? String(routing.data.fallback_store_id)
      : "",
    allow_manual_override: routing.data?.allow_manual_override ?? true,
    allow_fallback: routing.data?.allow_fallback ?? true,
  }), [routing.data]);
  const form = draft ?? loadedForm;

  const fallbackOptions = useMemo(
    () => (profiles.data ?? []).filter((item) => item.enabled !== false),
    [profiles.data],
  );

  const set = <K extends keyof RoutingForm>(key: K, value: RoutingForm[K]) =>
    setDraft((current) => ({ ...(current ?? loadedForm), [key]: value }));

  return (
    <ListingKitSettingsSection
      id="store-routing"
      eyebrow="店铺路由"
      title="多店铺默认选店策略"
      description="控制 ListingKit 在没有显式指定店铺时，如何从已启用的 SHEIN 店铺配置中选择执行店铺。"
      actions={
        <Button
          disabled={update.isPending}
          onClick={() =>
            update.mutate({
              selection_strategy: form.selection_strategy,
              fallback_store_id: form.fallback_store_id
                ? Number(form.fallback_store_id)
                : undefined,
              allow_manual_override: form.allow_manual_override,
              allow_fallback: form.allow_fallback,
            })
          }
        >
          {update.isPending ? "保存中..." : "保存路由策略"}
        </Button>
      }
    >
      <div className="grid gap-3 md:grid-cols-[220px_1fr]">
        <Label className="space-y-1">
          <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
            默认选店策略
          </span>
          <Select
            aria-label="默认选店策略"
            value={form.selection_strategy}
            onChange={(event) => set("selection_strategy", event.target.value)}
          >
            <option value="manual">手工优先</option>
            <option value="priority">按优先级</option>
            <option value="country">按国家匹配</option>
          </Select>
          <span className="block text-[11px] leading-4 text-zinc-500">
            `manual` 先尊重任务里显式指定的店铺；`priority` 按启用 profile 的优先级选；`country` 会优先匹配 profile 里的国家规则。
          </span>
        </Label>

        <Label className="space-y-1">
          <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
            fallback 店铺
          </span>
          <Select
            aria-label="fallback 店铺"
            value={form.fallback_store_id}
            onChange={(event) => set("fallback_store_id", event.target.value)}
          >
            <option value="">不指定</option>
            {fallbackOptions.map((item) => (
              <option key={item.id ?? item.store_id} value={String(item.store_id)}>
                {formatSheinStoreOptionLabel(item)}
              </option>
            ))}
          </Select>
          <span className="block text-[11px] leading-4 text-zinc-500">
            当默认策略没有命中结果时，用这个店铺兜底。
          </span>
        </Label>
      </div>

      <div className="mt-3 grid gap-3 md:grid-cols-2">
        <Label className="flex items-center gap-2 rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground">
          <Checkbox
            checked={form.allow_manual_override}
            onChange={(event) => set("allow_manual_override", event.target.checked)}
          />
          允许任务手工指定店铺覆盖默认策略
        </Label>
        <Label className="flex items-center gap-2 rounded-md border border-input bg-background px-3 py-2 text-sm text-foreground">
          <Checkbox
            checked={form.allow_fallback}
            onChange={(event) => set("allow_fallback", event.target.checked)}
          />
          未命中时允许 fallback 到兜底店铺
        </Label>
      </div>

      {routing.isError ? (
        <p className="mt-3 text-sm text-rose-600">店铺路由配置读取失败。</p>
      ) : null}
      {update.error ? (
        <p className="mt-3 text-sm text-rose-600">店铺路由配置保存失败。</p>
      ) : null}
      {profiles.isError ? (
        <p className="mt-3 text-sm text-rose-600">店铺 profile 列表读取失败，fallback 选项可能不完整。</p>
      ) : null}
    </ListingKitSettingsSection>
  );
}

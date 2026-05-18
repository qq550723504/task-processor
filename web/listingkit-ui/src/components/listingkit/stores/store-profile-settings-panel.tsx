"use client";

import { useMemo, useState } from "react";
import { Pencil, Plus, RefreshCw, Trash2, X } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
import { getTenantListingStores } from "@/lib/api/tenant-stores";
import { useDeleteStoreProfile, useStoreProfiles, useUpsertStoreProfile } from "@/lib/query/use-store-profiles";
import type { ListingKitStoreProfile } from "@/lib/types/listingkit";

type StoreProfileForm = {
  id?: number;
  store_id: string;
  enabled: boolean;
  priority: string;
  is_fallback: boolean;
  country_rules: string;
  category_rules: string;
  site: string;
  warehouse_code: string;
  default_stock: string;
  default_submit_mode: "publish" | "save_draft";
  exchange_rate: string;
  markup_multiplier: string;
  minimum_price: string;
  round_to: string;
  price_ending: string;
};

type StoreOption = {
  id: number;
  storeId?: string;
  name?: string;
  platform?: string;
  region?: string;
};

const DEFAULT_FORM: StoreProfileForm = {
  store_id: "",
  enabled: true,
  priority: "100",
  is_fallback: false,
  country_rules: "",
  category_rules: "",
  site: "US",
  warehouse_code: "",
  default_stock: "100",
  default_submit_mode: "publish",
  exchange_rate: "7.2",
  markup_multiplier: "2",
  minimum_price: "9.99",
  round_to: "0.01",
  price_ending: "",
};

export function StoreProfileSettingsPanel() {
  const profiles = useStoreProfiles();
  const upsert = useUpsertStoreProfile();
  const remove = useDeleteStoreProfile();
  const [draft, setDraft] = useState<StoreProfileForm>(DEFAULT_FORM);
  const [error, setError] = useState("");

  const storeOptionsQuery = useQuery({
    queryKey: ["listingkit-tenant-store-options"],
    queryFn: () => getTenantListingStores({ page: 1, page_size: 200, platform: "SHEIN" }),
  });

  const items = profiles.data ?? [];
  const sheinStores = useMemo(() => {
    const byID = new Map<number, StoreOption>();
    for (const item of storeOptionsQuery.data?.items ?? []) {
      if (item?.id) {
        byID.set(item.id, {
          id: item.id,
          storeId: item.storeId,
          name: item.name,
          platform: item.platform,
          region: item.region,
        });
      }
    }
    for (const item of items) {
      if (item.store?.id) {
        byID.set(item.store.id, item.store);
      } else if (item.store_id > 0) {
        byID.set(item.store_id, {
          id: item.store_id,
          store_id: item.store?.store_id,
          name: item.store?.name,
          platform: item.store?.platform,
          region: item.store?.region,
        });
      }
    }
    return Array.from(byID.values()).sort((left, right) => left.id - right.id);
  }, [items, storeOptionsQuery.data?.items]);

  function resetForm() {
    setDraft(DEFAULT_FORM);
  }

  function startEdit(profile: ListingKitStoreProfile) {
    setDraft({
      id: profile.id,
      store_id: String(profile.store_id),
      enabled: profile.enabled ?? true,
      priority: String(profile.priority ?? 100),
      is_fallback: profile.is_fallback ?? false,
      country_rules: formatMatchRuleValues(profile, "country"),
      category_rules: formatMatchRuleValues(profile, "category"),
      site: profile.site ?? "US",
      warehouse_code: profile.warehouse_code ?? "",
      default_stock: String(profile.default_stock ?? 100),
      default_submit_mode: profile.default_submit_mode ?? "publish",
      exchange_rate: String(profile.pricing?.exchange_rate ?? 7.2),
      markup_multiplier: String(profile.pricing?.markup_multiplier ?? 2),
      minimum_price: String(profile.pricing?.minimum_price ?? 9.99),
      round_to: String(profile.pricing?.round_to ?? 0.01),
      price_ending:
        profile.pricing?.price_ending === undefined
          ? ""
          : String(profile.pricing.price_ending),
    });
  }

  async function saveProfile() {
    setError("");
    try {
      await upsert.mutateAsync({
        id: draft.id,
        store_id: Number(draft.store_id),
        enabled: draft.enabled,
        priority: Number(draft.priority) || 0,
        is_fallback: draft.is_fallback,
        match_rules: buildMatchRules(draft),
        site: draft.site.trim().toUpperCase(),
        warehouse_code: draft.warehouse_code.trim(),
        default_stock: Number(draft.default_stock) || 0,
        default_submit_mode: draft.default_submit_mode,
        pricing: {
          source_currency: "CNY",
          target_currency: "USD",
          exchange_rate: Number(draft.exchange_rate) || 0,
          markup_multiplier: Number(draft.markup_multiplier) || 0,
          minimum_price: Number(draft.minimum_price) || 0,
          round_to: Number(draft.round_to) || 0,
          price_ending: draft.price_ending ? Number(draft.price_ending) : undefined,
        },
      });
      resetForm();
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  async function deleteProfile(id?: number) {
    if (!id) {
      return;
    }
    setError("");
    try {
      await remove.mutateAsync(id);
      if (draft.id === id) {
        resetForm();
      }
    } catch (err) {
      setError(formatSubscriptionApiError(err));
    }
  }

  const visibleError =
    error ||
    (storeOptionsQuery.error instanceof Error ? storeOptionsQuery.error.message : "");

  return (
    <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
      <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm">
        <div className="flex items-center justify-between border-b border-zinc-200 px-4 py-3">
          <div>
            <h2 className="text-base font-semibold text-zinc-950">我的店铺配置</h2>
            <p className="text-sm text-zinc-500">
              为当前租户可用的 SHEIN 店铺单独配置站点、仓库、提交方式和价格规则。
            </p>
          </div>
          <Button
            type="button"
            variant="secondary"
            onClick={() => {
              void profiles.refetch();
              void storeOptionsQuery.refetch();
            }}
          >
            <RefreshCw
              className={`size-4 ${
                profiles.isFetching || storeOptionsQuery.isFetching ? "animate-spin" : ""
              }`}
            />
            刷新
          </Button>
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
                <TableHead className="px-4 py-3">站点 / 仓库</TableHead>
                <TableHead className="px-4 py-3">匹配规则</TableHead>
                <TableHead className="px-4 py-3">优先级</TableHead>
                <TableHead className="px-4 py-3">状态</TableHead>
                <TableHead className="px-4 py-3 text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="divide-y divide-zinc-100">
              {profiles.isLoading ? (
                <TableRow>
                  <TableCell className="px-4 py-6 text-zinc-500" colSpan={6}>
                    加载中...
                  </TableCell>
                </TableRow>
              ) : items.length === 0 ? (
                <TableRow>
                  <TableCell className="px-4 py-6 text-zinc-500" colSpan={6}>
                    当前还没有店铺发布配置
                  </TableCell>
                </TableRow>
              ) : (
                items.map((item) => (
                  <TableRow key={item.id} className="align-top">
                    <TableCell className="px-4 py-3">
                      <div className="font-medium text-zinc-950">
                        {item.store?.name?.trim() || item.store?.store_id?.trim() || `店铺 ${item.store_id}`}
                      </div>
                      <div className="text-xs text-zinc-500">
                        {[item.store?.store_id?.trim(), item.store?.region?.trim()].filter(Boolean).join(" / ") ||
                          `store_id=${item.store_id}`}
                      </div>
                    </TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">
                      <div>{item.site || "-"}</div>
                      <div className="text-xs text-zinc-500">{item.warehouse_code || "-"}</div>
                    </TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">
                      <div className="text-xs text-zinc-600">{summarizeMatchRules(item)}</div>
                    </TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">{item.priority ?? "-"}</TableCell>
                    <TableCell className="px-4 py-3 text-zinc-700">
                      <div>{item.enabled === false ? "已禁用" : "已启用"}</div>
                      {item.is_fallback ? <div className="text-xs text-zinc-500">fallback</div> : null}
                    </TableCell>
                    <TableCell className="px-4 py-3 text-right">
                      <Button
                        type="button"
                        aria-label={`编辑 ${item.store?.name ?? item.store_id}`}
                        onClick={() => startEdit(item)}
                        size="icon"
                        variant="ghost"
                      >
                        <Pencil className="size-4" />
                      </Button>
                      <Button
                        type="button"
                        aria-label={`删除 ${item.store?.name ?? item.store_id}`}
                        onClick={() => void deleteProfile(item.id)}
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

      <section className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm">
        <div className="mb-4 flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            {draft.id ? <Pencil className="size-4 text-zinc-500" /> : <Plus className="size-4 text-zinc-500" />}
            <h2 className="text-base font-semibold text-zinc-950">
              {draft.id ? "编辑店铺发布配置" : "新增店铺发布配置"}
            </h2>
          </div>
          {draft.id ? (
            <Button type="button" aria-label="取消编辑" onClick={resetForm} size="icon" variant="ghost">
              <X className="size-4" />
            </Button>
          ) : null}
        </div>

        <Field label="SHEIN 店铺">
          <Select
            aria-label="SHEIN 店铺"
            value={draft.store_id}
            onChange={(event) => setDraft((current) => ({ ...current, store_id: event.target.value }))}
          >
            <option value="">请选择店铺</option>
            {sheinStores.map((store) => (
              <option key={store.id} value={String(store.id)}>
                {formatStoreOptionLabel(store)}
              </option>
            ))}
          </Select>
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <TextField label="站点" value={draft.site} onChange={(site) => setDraft((current) => ({ ...current, site }))} />
          <TextField
            label="仓库编码"
            value={draft.warehouse_code}
            onChange={(warehouse_code) => setDraft((current) => ({ ...current, warehouse_code }))}
          />
        </div>
        <div className="grid grid-cols-2 gap-3">
          <TextField
            label="国家规则"
            value={draft.country_rules}
            onChange={(country_rules) => setDraft((current) => ({ ...current, country_rules }))}
            placeholder="US, CA, GB"
          />
          <TextField
            label="类目规则"
            value={draft.category_rules}
            onChange={(category_rules) => setDraft((current) => ({ ...current, category_rules }))}
            placeholder="shoes, jewelry"
          />
        </div>
        <p className="-mt-1 mb-3 text-xs leading-5 text-zinc-500">
          `国家规则` 会匹配任务里的 `country`；`类目规则` 会匹配类目 hint 或 SDS 类目路径。多个值用逗号分隔。
        </p>
        <div className="grid grid-cols-2 gap-3">
          <TextField
            label="优先级"
            type="number"
            value={draft.priority}
            onChange={(priority) => setDraft((current) => ({ ...current, priority }))}
          />
          <TextField
            label="默认库存"
            type="number"
            value={draft.default_stock}
            onChange={(default_stock) => setDraft((current) => ({ ...current, default_stock }))}
          />
        </div>
        <Field label="默认提交方式">
          <Select
            value={draft.default_submit_mode}
            onChange={(event) =>
              setDraft((current) => ({
                ...current,
                default_submit_mode: event.target.value as "publish" | "save_draft",
              }))
            }
          >
            <option value="publish">直接发布</option>
            <option value="save_draft">保存草稿</option>
          </Select>
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <TextField
            label="汇率"
            type="number"
            step="0.01"
            value={draft.exchange_rate}
            onChange={(exchange_rate) => setDraft((current) => ({ ...current, exchange_rate }))}
          />
          <TextField
            label="加价倍数"
            type="number"
            step="0.01"
            value={draft.markup_multiplier}
            onChange={(markup_multiplier) => setDraft((current) => ({ ...current, markup_multiplier }))}
          />
        </div>
        <div className="grid grid-cols-3 gap-3">
          <TextField
            label="最低售价"
            type="number"
            step="0.01"
            value={draft.minimum_price}
            onChange={(minimum_price) => setDraft((current) => ({ ...current, minimum_price }))}
          />
          <TextField
            label="取整精度"
            type="number"
            step="0.01"
            value={draft.round_to}
            onChange={(round_to) => setDraft((current) => ({ ...current, round_to }))}
          />
          <TextField
            label="尾数"
            type="number"
            step="0.01"
            value={draft.price_ending}
            onChange={(price_ending) => setDraft((current) => ({ ...current, price_ending }))}
            placeholder="可选"
          />
        </div>
        <div className="mt-3 space-y-2">
          <Label className="flex items-center gap-2 text-sm text-zinc-700">
            <Checkbox
              checked={draft.enabled}
              onChange={(event) => setDraft((current) => ({ ...current, enabled: event.target.checked }))}
            />
            启用这个店铺配置
          </Label>
          <Label className="flex items-center gap-2 text-sm text-zinc-700">
            <Checkbox
              checked={draft.is_fallback}
              onChange={(event) => setDraft((current) => ({ ...current, is_fallback: event.target.checked }))}
            />
            作为 fallback 店铺
          </Label>
        </div>

        {sheinStores.length === 0 && !storeOptionsQuery.isLoading ? (
          <p className="mt-3 text-sm text-amber-700">
            当前租户还没有可用的 SHEIN 店铺，请先在上面的“店铺主数据”里新增店铺。
          </p>
        ) : null}

        <div className="mt-4 flex gap-2">
          <Button
            type="button"
            disabled={upsert.isPending || !draft.store_id}
            onClick={() => void saveProfile()}
          >
            {upsert.isPending ? "保存中..." : draft.id ? "保存配置" : "新增配置"}
          </Button>
          {draft.id ? (
            <Button type="button" variant="secondary" onClick={resetForm}>
              取消
            </Button>
          ) : null}
        </div>
      </section>
    </section>
  );
}

function formatStoreOptionLabel(store: StoreOption) {
  const primary = store.name?.trim() || store.storeId?.trim() || `店铺 ${store.id}`;
  const meta = [store.storeId?.trim(), store.region?.trim()].filter(Boolean).join(" / ");
  return meta ? `${primary} (${meta})` : primary;
}

function formatMatchRuleValues(profile: ListingKitStoreProfile, kind: string) {
  const match = profile.match_rules?.find((item) => item.kind === kind);
  return match?.values?.join(", ") ?? "";
}

function buildMatchRules(form: StoreProfileForm) {
  const items = [
    { kind: "country", values: splitRuleInput(form.country_rules) },
    { kind: "category", values: splitRuleInput(form.category_rules) },
  ];
  return items.filter((item) => item.values.length > 0);
}

function splitRuleInput(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function summarizeMatchRules(profile: ListingKitStoreProfile) {
  const parts: string[] = [];
  for (const rule of profile.match_rules ?? []) {
    if (!rule.kind || !rule.values?.length) {
      continue;
    }
    const label = rule.kind === "country" ? "国家" : rule.kind === "category" ? "类目" : rule.kind;
    parts.push(`${label}: ${rule.values.join(", ")}`);
  }
  return parts.length > 0 ? parts.join(" · ") : "未配置";
}

function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <div className="mt-1">{children}</div>
    </Label>
  );
}

function TextField({
  label,
  value,
  onChange,
  type = "text",
  placeholder,
  step,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: string;
  placeholder?: string;
  step?: string;
}) {
  return (
    <Label className="mb-3 block text-xs font-medium text-zinc-500">
      {label}
      <Input
        className="mt-1 h-9 w-full rounded-md border border-zinc-200 px-3 text-sm text-zinc-900"
        placeholder={placeholder}
        step={step}
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </Label>
  );
}

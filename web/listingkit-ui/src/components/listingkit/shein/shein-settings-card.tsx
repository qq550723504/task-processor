"use client";

import { useMemo, useState } from "react";

import { Button } from "@/components/shared/button";
import { useSheinSettings, useUpdateSheinSettings } from "@/lib/query/use-shein-settings";

export function SheinSettingsCard() {
  const settings = useSheinSettings();
  const update = useUpdateSheinSettings();
  const availableStores = useMemo(
    () => settings.data?.available_stores ?? [],
    [settings.data?.available_stores],
  );
  const [draft, setDraft] = useState<Record<string, string> | null>(null);
  const loadedForm = useMemo(() => {
    const data = settings.data;
    return {
      default_store_id: String(data?.default_store_id ?? ""),
      site: data?.site ?? "US",
      warehouse_code: data?.warehouse_code ?? "DEFAULT",
      default_stock: String(data?.default_stock ?? 100),
      default_submit_mode: data?.default_submit_mode ?? "publish",
      exchange_rate: String(data?.pricing?.exchange_rate ?? 7.2),
      markup_multiplier: String(data?.pricing?.markup_multiplier ?? 2),
      minimum_price: String(data?.pricing?.minimum_price ?? 9.99),
      round_to: String(data?.pricing?.round_to ?? 0.01),
      price_ending: String(data?.pricing?.price_ending ?? ""),
    };
  }, [settings.data]);
  const form = draft ?? loadedForm;
  const storeOptions = useMemo(() => {
    if (!form.default_store_id) {
      return availableStores;
    }
    if (availableStores.some((store) => String(store.id) === form.default_store_id)) {
      return availableStores;
    }
    return [
      ...availableStores,
      {
        id: Number(form.default_store_id),
        store_id: form.default_store_id,
        name: "当前已保存店铺",
      },
    ];
  }, [availableStores, form.default_store_id]);

  const set = (key: keyof typeof form, value: string) =>
    setDraft((current) => ({ ...(current ?? loadedForm), [key]: value }));

  return (
    <section className="rounded-[1.5rem] border border-white/70 bg-white/86 p-4 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-teal-700">
            SHEIN 配置
          </p>
          <h2 className="mt-1 text-lg font-semibold text-zinc-950">
            店铺与价格规则
          </h2>
          <p className="mt-1 text-sm text-zinc-600">
            单客户默认配置，新任务价格预览会使用这里的汇率和倍率。
          </p>
        </div>
        <Button
          disabled={update.isPending}
          onClick={() =>
            update.mutate({
              default_store_id: Number(form.default_store_id),
              site: form.site,
              warehouse_code: form.warehouse_code,
              default_stock: Number(form.default_stock),
              default_submit_mode: form.default_submit_mode as "publish" | "save_draft",
              pricing: {
                source_currency: "CNY",
                target_currency: "USD",
                exchange_rate: Number(form.exchange_rate),
                markup_multiplier: Number(form.markup_multiplier),
                minimum_price: Number(form.minimum_price),
                round_to: Number(form.round_to),
                price_ending: Number(form.price_ending),
              },
            })
          }
        >
          {update.isPending ? "保存中..." : "保存配置"}
        </Button>
      </div>
      <div className="mt-4 grid gap-3 md:grid-cols-5">
        <label className="space-y-1">
          <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
            默认店铺
          </span>
          <select
            aria-label="默认店铺"
            className="h-10 w-full rounded-xl border border-zinc-200 bg-white px-3 text-sm"
            value={form.default_store_id}
            onChange={(event) => set("default_store_id", event.target.value)}
          >
            <option value="">
              {storeOptions.length > 0 ? "请选择当前租户店铺" : "当前租户下暂无 SHEIN 店铺"}
            </option>
            {storeOptions.map((store) => (
              <option key={store.id} value={String(store.id)}>
                {formatStoreOptionLabel(store)}
              </option>
            ))}
          </select>
          <span className="block text-[11px] leading-4 text-zinc-500">
            店铺列表从当前登录租户的 `listing_store` 数据读取
          </span>
        </label>
        <Input
          label="站点"
          hint="例如 US，美国站"
          value={form.site}
          onChange={(value) => set("site", value)}
        />
        <Input
          label="仓库编码"
          hint="SHEIN 后台分配的发货仓"
          value={form.warehouse_code}
          onChange={(value) => set("warehouse_code", value)}
        />
        <Input
          label="默认库存"
          hint="每个 SKU 默认填入的库存"
          value={form.default_stock}
          onChange={(value) => set("default_stock", value)}
        />
        <label className="space-y-1">
          <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
            默认提交方式
          </span>
          <select className="h-10 w-full rounded-xl border border-zinc-200 bg-white px-3 text-sm" value={form.default_submit_mode} onChange={(event) => set("default_submit_mode", event.target.value)}>
            <option value="publish">直接发布</option>
            <option value="save_draft">保存草稿</option>
          </select>
          <span className="block text-[11px] leading-4 text-zinc-500">
            客户仍可在最终确认页临时切换
          </span>
        </label>
        <Input
          label="人民币转美元汇率"
          hint="例：7.2 表示 1 USD = 7.2 CNY"
          value={form.exchange_rate}
          onChange={(value) => set("exchange_rate", value)}
        />
        <Input
          label="售价倍率"
          hint="成本换算美元后乘以该倍率"
          value={form.markup_multiplier}
          onChange={(value) => set("markup_multiplier", value)}
        />
        <Input
          label="最低售价（美元）"
          hint="低于该值时按最低售价"
          value={form.minimum_price}
          onChange={(value) => set("minimum_price", value)}
        />
        <Input
          label="价格步进"
          hint="例：0.01 表示保留到美分"
          value={form.round_to}
          onChange={(value) => set("round_to", value)}
        />
        <Input
          label="价格尾数"
          hint="例：0.99 可生成 12.99 这种价格；留空则不固定尾数"
          value={form.price_ending}
          onChange={(value) => set("price_ending", value)}
        />
      </div>
      {update.error ? (
        <p className="mt-3 text-sm text-rose-600">配置保存失败，请检查字段后重试。</p>
      ) : null}
    </section>
  );
}

function formatStoreOptionLabel(store: {
  id: number;
  store_id?: string;
  name?: string;
  region?: string;
}) {
  const primary = store.name?.trim() || store.store_id?.trim() || `店铺 ${store.id}`;
  const meta = [store.store_id?.trim(), store.region?.trim()]
    .filter(Boolean)
    .join(" / ");
  return meta ? `${primary} (${meta})` : primary;
}

function Input({
  label,
  hint,
  value,
  onChange,
}: {
  label: string;
  hint?: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="space-y-1">
      <span className="text-[10px] font-semibold tracking-[0.12em] text-zinc-500">
        {label}
      </span>
      <input
        className="h-10 w-full rounded-xl border border-zinc-200 bg-white px-3 text-sm outline-none focus:border-zinc-400"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
      {hint ? (
        <span className="block text-[11px] leading-4 text-zinc-500">{hint}</span>
      ) : null}
    </label>
  );
}

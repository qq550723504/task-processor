"use client";

import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type {
  SheinActivityPartakeType,
  SheinActivityPriceMode,
  SheinActivityStrategyRecord,
  SheinUpdateActivityStrategyInput,
} from "@/lib/types/listingkit/shein-enrollment";

type ActivityStrategyForm = {
  activity_price_mode: SheinActivityPriceMode;
  activity_partake_type: SheinActivityPartakeType;
  activity_discount_rate: string;
  activity_limited_discount_rate: string;
  activity_stock_ratio: string;
  activity_min_profit_rate: string;
  fixed_price_adjustment: string;
};

const DEFAULT_FORM: ActivityStrategyForm = {
  activity_price_mode: "DISCOUNT",
  activity_partake_type: "REGULAR",
  activity_discount_rate: "0.2",
  activity_limited_discount_rate: "0.25",
  activity_stock_ratio: "0.5",
  activity_min_profit_rate: "0.15",
  fixed_price_adjustment: "0",
};

export function SheinActivityStrategyCard({
  configured,
  onSave,
  saving,
  strategy,
}: {
  configured?: boolean;
  onSave: (input: SheinUpdateActivityStrategyInput) => Promise<void>;
  saving?: boolean;
  strategy?: SheinActivityStrategyRecord | null;
}) {
  return (
    <SheinActivityStrategyForm
      configured={configured}
      initialForm={strategyToForm(strategy)}
      key={activityStrategyFormKey(strategy)}
      onSave={onSave}
      saving={saving}
    />
  );
}

function SheinActivityStrategyForm({
  configured,
  initialForm,
  onSave,
  saving,
}: {
  configured?: boolean;
  initialForm: ActivityStrategyForm;
  onSave: (input: SheinUpdateActivityStrategyInput) => Promise<void>;
  saving?: boolean;
}) {
  const [form, setForm] = useState<ActivityStrategyForm>(initialForm);

  const input = formToInput(form);
  const valid = isActivityStrategyInputValid(input);

  return (
    <section className="rounded-2xl border border-zinc-200 bg-white p-4">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between">
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-sm font-semibold text-zinc-950">活动设置</h2>
            <span
              className={
                configured
                  ? "rounded-full bg-emerald-50 px-2 py-1 text-xs text-emerald-700"
                  : "rounded-full bg-amber-50 px-2 py-1 text-xs text-amber-700"
              }
            >
              {configured ? "已配置" : "未配置"}
            </span>
          </div>
          <div
            className="flex flex-wrap gap-2"
            role="group"
            aria-label="价格模式"
          >
            {(["DISCOUNT", "PROFIT"] as SheinActivityPriceMode[]).map(
              (mode) => (
                <button
                  className={
                    form.activity_price_mode === mode
                      ? "h-9 rounded-lg bg-zinc-950 px-3 text-sm text-white"
                      : "h-9 rounded-lg border border-zinc-200 bg-white px-3 text-sm text-zinc-700"
                  }
                  key={mode}
                  onClick={() =>
                    setForm((current) => ({
                      ...current,
                      activity_price_mode: mode,
                    }))
                  }
                  type="button"
                >
                  {mode === "DISCOUNT" ? "按折扣" : "按利润"}
                </button>
              ),
            )}
          </div>
          <div
            className="flex flex-wrap gap-2"
            role="group"
            aria-label="活动类型"
          >
            {(["REGULAR", "LIMITED", "BOTH"] as SheinActivityPartakeType[]).map(
              (type) => (
                <button
                  className={
                    form.activity_partake_type === type
                      ? "h-9 rounded-lg bg-zinc-950 px-3 text-sm text-white"
                      : "h-9 rounded-lg border border-zinc-200 bg-white px-3 text-sm text-zinc-700"
                  }
                  key={type}
                  onClick={() =>
                    setForm((current) => ({
                      ...current,
                      activity_partake_type: type,
                    }))
                  }
                  type="button"
                >
                  {partakeTypeLabel(type)}
                </button>
              ),
            )}
          </div>
        </div>

        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          {form.activity_price_mode === "DISCOUNT" ? (
            <label className="text-xs font-medium text-zinc-600">
              折扣率
              <Input
                aria-label="折扣率"
                className="mt-1"
                max="0.99"
                min="0.01"
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    activity_discount_rate: event.target.value,
                  }))
                }
                step="0.01"
                type="number"
                value={form.activity_discount_rate}
              />
            </label>
          ) : null}
          {form.activity_price_mode === "DISCOUNT" &&
          form.activity_partake_type === "BOTH" ? (
            <label className="text-xs font-medium text-zinc-600">
              限量折扣率
              <Input
                aria-label="限量折扣率"
                className="mt-1"
                max="0.99"
                min="0.01"
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    activity_limited_discount_rate: event.target.value,
                  }))
                }
                step="0.01"
                type="number"
                value={form.activity_limited_discount_rate}
              />
            </label>
          ) : null}
          {form.activity_price_mode === "PROFIT" ? (
            <label className="text-xs font-medium text-zinc-600">
              最低利润率
              <Input
                aria-label="最低利润率"
                className="mt-1"
                max="0.99"
                min="0"
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    activity_min_profit_rate: event.target.value,
                  }))
                }
                step="0.01"
                type="number"
                value={form.activity_min_profit_rate}
              />
            </label>
          ) : null}
          {partakeTypeNeedsStockRatio(form.activity_partake_type) ? (
            <label className="text-xs font-medium text-zinc-600">
              活动库存比例
              <Input
                aria-label="活动库存比例"
                className="mt-1"
                max="1"
                min="0.01"
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    activity_stock_ratio: event.target.value,
                  }))
                }
                step="0.01"
                type="number"
                value={form.activity_stock_ratio}
              />
            </label>
          ) : null}
          <label className="text-xs font-medium text-zinc-600">
            固定调价
            <Input
              aria-label="固定调价"
              className="mt-1"
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  fixed_price_adjustment: event.target.value,
                }))
              }
              step="0.01"
              type="number"
              value={form.fixed_price_adjustment}
            />
          </label>
          <Button
            className="mt-auto"
            disabled={!valid || saving}
            onClick={() => void onSave(input)}
            type="button"
          >
            {saving ? "保存中..." : "保存活动设置"}
          </Button>
        </div>
      </div>
    </section>
  );
}

export function isSheinActivityStrategyReady(response?: {
  configured?: boolean;
  strategy?: SheinActivityStrategyRecord | null;
}) {
  if (!response?.configured || !response.strategy) {
    return false;
  }
  return isActivityStrategyInputValid({
    activity_price_mode:
      response.strategy.activity_price_mode === "PROFIT"
        ? "PROFIT"
        : "DISCOUNT",
    activity_partake_type: normalizeActivityPartakeType(
      response.strategy.activity_partake_type,
    ),
    activity_discount_rate: response.strategy.activity_discount_rate,
    activity_limited_discount_rate:
      response.strategy.activity_limited_discount_rate,
    activity_stock_ratio: response.strategy.activity_stock_ratio,
    activity_min_profit_rate: response.strategy.activity_min_profit_rate,
    fixed_price_adjustment: response.strategy.fixed_price_adjustment ?? 0,
  });
}

function strategyToForm(
  strategy?: SheinActivityStrategyRecord | null,
): ActivityStrategyForm {
  if (!strategy) {
    return DEFAULT_FORM;
  }
  return {
    activity_price_mode:
      strategy.activity_price_mode === "PROFIT" ? "PROFIT" : "DISCOUNT",
    activity_partake_type: normalizeActivityPartakeType(
      strategy.activity_partake_type,
    ),
    activity_discount_rate: String(
      strategy.activity_discount_rate ?? DEFAULT_FORM.activity_discount_rate,
    ),
    activity_limited_discount_rate: String(
      strategy.activity_limited_discount_rate ??
        DEFAULT_FORM.activity_limited_discount_rate,
    ),
    activity_stock_ratio: String(
      strategy.activity_stock_ratio ?? DEFAULT_FORM.activity_stock_ratio,
    ),
    activity_min_profit_rate: String(
      strategy.activity_min_profit_rate ??
        DEFAULT_FORM.activity_min_profit_rate,
    ),
    fixed_price_adjustment: String(
      strategy.fixed_price_adjustment ?? DEFAULT_FORM.fixed_price_adjustment,
    ),
  };
}

function activityStrategyFormKey(
  strategy?: SheinActivityStrategyRecord | null,
) {
  if (!strategy) {
    return "empty";
  }
  return [
    strategy.activity_price_mode,
    strategy.activity_partake_type,
    strategy.activity_discount_rate,
    strategy.activity_limited_discount_rate,
    strategy.activity_stock_ratio,
    strategy.activity_min_profit_rate,
    strategy.fixed_price_adjustment,
  ].join(":");
}

function formToInput(
  form: ActivityStrategyForm,
): SheinUpdateActivityStrategyInput {
  const input: SheinUpdateActivityStrategyInput = {
    activity_price_mode: form.activity_price_mode,
    activity_partake_type: form.activity_partake_type,
    fixed_price_adjustment: parseDecimal(form.fixed_price_adjustment) ?? 0,
  };
  if (partakeTypeNeedsStockRatio(form.activity_partake_type)) {
    input.activity_stock_ratio = parseDecimal(form.activity_stock_ratio) ?? 0;
  }
  if (form.activity_price_mode === "DISCOUNT") {
    input.activity_discount_rate = parseDecimal(form.activity_discount_rate);
    if (form.activity_partake_type === "BOTH") {
      input.activity_limited_discount_rate =
        parseDecimal(form.activity_limited_discount_rate) ?? 0;
    }
  } else {
    input.activity_min_profit_rate = parseDecimal(
      form.activity_min_profit_rate,
    );
  }
  return input;
}

function isActivityStrategyInputValid(input: SheinUpdateActivityStrategyInput) {
  if (
    partakeTypeNeedsStockRatio(input.activity_partake_type) &&
    !withinRatio(input.activity_stock_ratio, true)
  ) {
    return false;
  }
  if (input.activity_price_mode === "DISCOUNT") {
    if (!withinRatio(input.activity_discount_rate, false)) {
      return false;
    }
    if (input.activity_partake_type === "BOTH") {
      return (
        withinRatio(input.activity_limited_discount_rate, false) &&
        input.activity_limited_discount_rate! > input.activity_discount_rate!
      );
    }
    return true;
  }
  return withinProfitFloor(input.activity_min_profit_rate);
}

function withinRatio(value: number | undefined, allowOne: boolean) {
  if (typeof value !== "number" || Number.isNaN(value) || value <= 0) {
    return false;
  }
  return allowOne ? value <= 1 : value < 1;
}

function withinProfitFloor(value: number | undefined) {
  return (
    typeof value === "number" && !Number.isNaN(value) && value >= 0 && value < 1
  );
}

function parseDecimal(value: string) {
  if (value.trim() === "") {
    return undefined;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function normalizeActivityPartakeType(
  value?: SheinActivityPartakeType | string,
): SheinActivityPartakeType {
  if (value === "LIMITED" || value === "BOTH") {
    return value;
  }
  return "REGULAR";
}

function partakeTypeNeedsStockRatio(type: SheinActivityPartakeType) {
  return type === "LIMITED" || type === "BOTH";
}

function partakeTypeLabel(type: SheinActivityPartakeType) {
  switch (type) {
    case "LIMITED":
      return "限量活动";
    case "BOTH":
      return "常规+限量";
    default:
      return "常规活动";
  }
}

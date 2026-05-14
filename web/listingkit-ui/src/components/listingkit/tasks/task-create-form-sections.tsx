import type { ReactNode } from "react";
import { Button } from "@/components/shared/button";
import { platformOptions } from "@/components/listingkit/tasks/task-create-form-model";
import type { UseFormRegister, FieldErrors } from "react-hook-form";
import type { FormValues } from "@/components/listingkit/tasks/task-create-form-model";

export function TaskPlatformFieldset({
  errors,
  register,
  selectedPlatforms,
}: {
  errors: FieldErrors<FormValues>;
  register: UseFormRegister<FormValues>;
  selectedPlatforms?: string[];
}) {
  return (
    <fieldset className="space-y-3">
      <legend className="text-sm font-medium text-zinc-700">目标平台</legend>
      <div className="grid gap-2">
        {platformOptions.map((platform) => (
          <label
            className="flex items-center justify-between gap-3 rounded-xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-900 transition hover:border-zinc-300"
            key={platform.value}
          >
            <span className="flex items-center gap-3">
              <input
                aria-label={platform.label}
                className="h-4 w-4 rounded border-zinc-300 accent-zinc-950"
                defaultChecked={selectedPlatforms?.includes(platform.value)}
                type="checkbox"
                value={platform.value}
                {...register("platforms")}
              />
              <span className="font-medium">{platform.label}</span>
            </span>
            <span className="text-xs text-zinc-500">
              {selectedPlatforms?.includes(platform.value) ? "已选择" : "可启用"}
            </span>
          </label>
        ))}
      </div>
      <p className="text-sm text-zinc-500">
        已选择 {selectedPlatforms?.length ?? 0} 个平台
      </p>
      {errors.platforms ? (
        <p className="text-sm text-red-600">{errors.platforms.message}</p>
      ) : null}
    </fieldset>
  );
}

export function TaskSheinStoreField({
  register,
}: {
  register: UseFormRegister<FormValues>;
}) {
  return (
    <label className="block space-y-2">
      <span className="text-sm font-medium text-zinc-700">SHEIN 店铺 ID</span>
      <input
        aria-label="SHEIN 店铺 ID"
        className="w-full rounded-xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
        inputMode="numeric"
        placeholder="869"
        {...register("sheinStoreId")}
      />
      <p className="text-sm leading-6 text-zinc-500">
        如果当前环境只对应一个店铺，可以先留空；多个 SHEIN 店铺共用时再填写。
      </p>
    </label>
  );
}

export function TaskAdvancedSettingsToggle({
  children,
  showAdvancedSettings,
  setShowAdvancedSettings,
  variant = "default",
}: {
  children?: ReactNode;
  showAdvancedSettings: boolean;
  setShowAdvancedSettings: (value: boolean | ((current: boolean) => boolean)) => void;
  variant?: "default" | "sds";
}) {
  return (
    <section className="space-y-3 rounded-2xl border border-zinc-200 bg-zinc-50/70 px-4 py-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1">
          <h2 className="text-sm font-medium text-zinc-900">高级设置</h2>
          <p className="text-sm leading-6 text-zinc-500">
            {variant === "sds"
              ? "先填写基础信息；SDS 和场景等高级配置可以稍后再补充。"
              : "先填写基础信息；场景等高级配置可以稍后再补充。"}
          </p>
        </div>
        <Button
          className="min-w-[112px]"
          onClick={() => setShowAdvancedSettings((current) => !current)}
          tone="secondary"
          type="button"
        >
          {showAdvancedSettings ? "收起高级设置" : "显示高级设置"}
        </Button>
      </div>
      {showAdvancedSettings && children ? (
        <div className="border-t border-zinc-200 pt-4">{children}</div>
      ) : null}
    </section>
  );
}

export function TaskCreateFormActions({
  isCreating,
  onBack,
  submitLabel,
}: {
  isCreating: boolean;
  onBack: () => void;
  submitLabel: string;
}) {
  return (
    <div className="flex flex-wrap gap-3">
      <Button className="min-w-[140px]" disabled={isCreating} type="submit">
        {isCreating ? "创建中..." : submitLabel}
      </Button>
      <Button className="min-w-[112px]" tone="secondary" onClick={onBack} type="button">
        返回首页
      </Button>
    </div>
  );
}

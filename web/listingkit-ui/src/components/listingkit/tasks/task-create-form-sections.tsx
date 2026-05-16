import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
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
          <Label
            className="flex items-center justify-between gap-3 rounded-xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-900 transition hover:border-zinc-300"
            key={platform.value}
          >
            <span className="flex items-center gap-3">
              <Checkbox
                aria-label={platform.label}
                defaultChecked={selectedPlatforms?.includes(platform.value)}
                value={platform.value}
                {...register("platforms")}
              />
              <span className="font-medium">{platform.label}</span>
            </span>
            <span className="text-xs text-zinc-500">
              {selectedPlatforms?.includes(platform.value) ? "已选择" : "可启用"}
            </span>
          </Label>
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
    <Label className="block space-y-2">
      <span className="text-sm font-medium text-zinc-700">SHEIN 店铺 ID</span>
      <Input
        aria-label="SHEIN 店铺 ID"
        className="rounded-xl px-4 py-3"
        inputMode="numeric"
        placeholder="869"
        {...register("sheinStoreId")}
      />
      <p className="text-sm leading-6 text-zinc-500">
        如果当前环境只对应一个店铺，可以先留空；多个 SHEIN 店铺共用时再填写。
      </p>
    </Label>
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
          variant="secondary"
          type="button"
        >
          {showAdvancedSettings ? "收起高级设置" : "显示高级设置"}
        </Button>
      </div>
      {showAdvancedSettings && children ? (
        <>
          <Separator />
          <div>{children}</div>
        </>
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
      <Button className="min-w-[112px]" variant="secondary" onClick={onBack} type="button">
        返回首页
      </Button>
    </div>
  );
}

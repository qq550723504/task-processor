import type { ReactNode } from "react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { platformOptions } from "@/components/listingkit/tasks/task-create-form-model";
import type { UseFormRegister, FieldErrors } from "react-hook-form";
import type { FormValues } from "@/components/listingkit/tasks/task-create-form-model";
import { useSheinStoreSelector } from "@/lib/query/use-shein-store-selector";

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
  selectedPlatforms,
  selectedStoreId,
  register,
}: {
  selectedPlatforms?: string[];
  selectedStoreId?: string;
  register: UseFormRegister<FormValues>;
}) {
  const {
    anyLoggedInStore,
    enabledProfiles,
    profiles,
    recommendedReason,
    recommendedStoreId,
    routing,
    selectedStoreLoginStatus,
  } = useSheinStoreSelector(selectedStoreId);
  const sheinSelected = selectedPlatforms?.includes("shein");
  const selectedStoreUnavailable = Boolean(
    sheinSelected &&
      selectedStoreLoginStatus &&
      !selectedStoreLoginStatus.login_in_progress &&
      !selectedStoreLoginStatus.waiting_for_verify_code &&
      !selectedStoreLoginStatus.has_cookie &&
      (selectedStoreLoginStatus.cookie_ttl ?? 0) <= 0,
  );
  const noLoggedInStore = Boolean(
    sheinSelected &&
      !selectedStoreLoginStatus &&
      !anyLoggedInStore &&
      enabledProfiles.length > 0,
  );

  return (
    <Label className="block space-y-2">
      <span className="text-sm font-medium text-zinc-700">SHEIN 店铺</span>
      <Select
        aria-label="SHEIN 店铺"
        className="rounded-xl px-4 py-3"
        defaultValue={recommendedStoreId}
        {...register("sheinStoreId")}
      >
        <option value="">
          {enabledProfiles.length > 0 ? "按默认路由自动选择" : "当前没有已启用店铺配置"}
        </option>
        {enabledProfiles.map((item) => (
          <option key={item.id ?? item.store_id} value={String(item.store_id)}>
            {formatStoreProfileOption(item)}
          </option>
        ))}
      </Select>
      <p className="text-sm leading-6 text-zinc-500">
        不选时交给后端路由自动选店；只在需要固定到某个店铺时手工指定。
      </p>
      {routing.data?.selection_strategy ? (
        <p className="text-sm leading-6 text-zinc-500">
          当前默认策略：
          {routeStrategyLabel(routing.data.selection_strategy)}
          {recommendedReason ? `。${recommendedReason}` : ""}
        </p>
      ) : null}
      {profiles.isError ? (
        <p className="text-sm text-rose-600">店铺配置读取失败，当前只能依赖后端默认路由。</p>
      ) : null}
      {selectedStoreUnavailable ? (
        <Alert className="border-amber-200 bg-amber-50/80">
          <AlertDescription className="text-sm leading-6 text-amber-900">
            当前选中的 SHEIN 店铺还未登录。你仍然可以先创建标准商品和 SDS 图片；但后续 SHEIN
            在线类目、属性解析和提交会在平台阶段受阻，需要先重新登录店铺。
          </AlertDescription>
        </Alert>
      ) : null}
      {noLoggedInStore ? (
        <Alert className="border-amber-200 bg-amber-50/80">
          <AlertDescription className="text-sm leading-6 text-amber-900">
            当前没有可用的 SHEIN 店铺登录态。你仍然可以先创建标准商品和 SDS 图片；但后续 SHEIN
            在线解析和提交会在平台阶段受阻，建议先完成店铺登录。
          </AlertDescription>
        </Alert>
      ) : null}
    </Label>
  );
}

function formatStoreProfileOption(item: {
  store_id: number;
  store?: { name?: string; store_id?: string; region?: string };
  site?: string;
  priority?: number;
  is_fallback?: boolean;
  match_rules?: Array<{ kind?: string }>;
}) {
  const primary =
    item.store?.name?.trim() || item.store?.store_id?.trim() || `店铺 ${item.store_id}`;
  const meta = [
    item.store?.region?.trim(),
    item.site?.trim(),
    item.priority?.toString(),
    summarizeRuleKinds(item),
  ]
    .filter(Boolean)
    .join(" / ");
  return meta ? `${primary} (${meta})` : primary;
}

function summarizeRuleKinds(item: {
  is_fallback?: boolean;
  match_rules?: Array<{ kind?: string }>;
}) {
  const labels: string[] = (item.match_rules ?? [])
    .map((rule) => {
      switch (rule.kind) {
        case "country":
          return "国家规则";
        case "category":
          return "类目规则";
        default:
          return "";
      }
    })
    .filter(Boolean);
  if (item.is_fallback) {
    labels.push("fallback");
  }
  return labels.join(" + ");
}

function routeStrategyLabel(strategy: string) {
  switch (strategy) {
    case "priority":
      return "按优先级";
    case "country":
      return "按国家匹配";
    case "manual":
    default:
      return "手工优先";
  }
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
          className="w-full sm:min-w-[112px] sm:w-auto"
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
    <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap">
      <Button className="w-full sm:min-w-[140px] sm:w-auto" disabled={isCreating} type="submit">
        {isCreating ? "创建中..." : submitLabel}
      </Button>
      <Button
        className="w-full sm:min-w-[112px] sm:w-auto"
        variant="secondary"
        onClick={onBack}
        type="button"
      >
        返回首页
      </Button>
    </div>
  );
}

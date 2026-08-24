import { Button } from "@/components/ui/button";

type FinalReviewSubmitAction = "publish" | "save_draft";

export function FinalReviewHeader({ confirmed }: { confirmed: boolean }) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.26em] text-zinc-500">
          SHEIN 最终确认
        </p>
        <h2 className="mt-1 text-lg font-semibold text-zinc-950">
          确认即将提交的资料
        </h2>
        <p className="mt-1 max-w-2xl text-sm text-zinc-600">
          发布前核对价格、SKU、属性和最终图片。提交时会自动保存当前修改。
        </p>
      </div>
      <span
        className={`rounded-full px-3 py-1 text-xs font-semibold ${
          confirmed
            ? "bg-emerald-100 text-emerald-700"
            : "bg-amber-100 text-amber-700"
        }`}
      >
        {confirmed ? "已确认" : "待确认"}
      </span>
    </div>
  );
}

export function FinalReviewReadinessBanner({
  blockingCount,
  confirmed,
  ready,
  saveDraftReady,
}: {
  blockingCount: number;
  confirmed: boolean;
  ready: boolean;
  saveDraftReady?: boolean;
}) {
  return (
    <div
      className={`rounded-2xl border p-4 ${
        ready
          ? "border-emerald-200 bg-emerald-50"
          : "border-amber-200 bg-amber-50"
      }`}
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div
            className={`text-xs font-semibold uppercase tracking-[0.18em] ${
              ready ? "text-emerald-700" : "text-amber-700"
            }`}
          >
            提交前检查
          </div>
          <div className="mt-1 text-sm font-semibold text-zinc-950">
            {ready
              ? confirmed
                ? "可以提交"
                : "资料已就绪，还需要最终确认"
              : saveDraftReady
                ? "可以先保存草稿"
              : "暂时不能提交"}
          </div>
          <p className="mt-1 text-sm leading-6 text-zinc-700">
            {ready
              ? "后端 readiness 已通过。提交前请确认价格、图片和 SKU。"
              : saveDraftReady
                ? "当前阻断项只影响正式发布，仍可先保存到 SHEIN 草稿箱。"
              : "需要先修复阻断项，提交按钮会保持不可用。"}
          </p>
        </div>
        <span
          className={`rounded-full px-3 py-1 text-xs font-semibold ${
            ready ? "bg-emerald-100 text-emerald-700" : "bg-amber-100 text-amber-700"
          }`}
        >
          {ready ? "已就绪" : `${blockingCount} 个阻断项`}
        </span>
      </div>
    </div>
  );
}

export function FinalReviewSubmitActions({
  confirmed,
  isSaving,
  isPublished,
  isSubmitting,
  manualOverrides,
  onSubmit,
  ready,
  saveDraftReady,
  submitAction,
  submitHint,
}: {
  confirmed: boolean;
  isSaving?: boolean;
  isPublished?: boolean;
  isSubmitting?: boolean;
  manualOverrides: Record<string, number>;
  onSubmit?: (
    action: FinalReviewSubmitAction,
    payload?: {
      confirmed?: boolean;
      submit_mode?: FinalReviewSubmitAction;
      manual_price_overrides?: Record<string, number>;
    },
  ) => void;
  ready: boolean;
  saveDraftReady?: boolean;
  submitAction?: FinalReviewSubmitAction | null;
  submitHint: string;
}) {
  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap">
      <div className="basis-full rounded-2xl border border-zinc-200 bg-zinc-50 p-3 text-sm leading-6 text-zinc-700">
        <p className="font-semibold text-zinc-950">{submitHint}</p>
        <p className="mt-1">
          保存草稿：上传图片并保存到 SHEIN 草稿箱，不直接上架。正式发布：上传图片并提交 SHEIN 发布接口。
        </p>
      </div>
      <Button
        className="w-full sm:w-auto"
        variant="secondary"
        disabled={isSaving || !(saveDraftReady ?? ready) || isSubmitting}
        onClick={() =>
          onSubmit?.("save_draft", {
            confirmed: true,
            submit_mode: "save_draft",
            manual_price_overrides: manualOverrides,
          })
        }
      >
        {submitAction === "save_draft" ? "保存中..." : "保存到 SHEIN 草稿箱"}
      </Button>
      <Button
        className="w-full sm:w-auto"
        disabled={!ready || isSubmitting || isPublished}
        onClick={() =>
          onSubmit?.("publish", {
            confirmed: true,
            submit_mode: "publish",
            manual_price_overrides: manualOverrides,
          })
        }
      >
        {isPublished
          ? "已发布到 SHEIN"
          : submitAction === "publish"
            ? "发布中..."
            : "发布到 SHEIN"}
      </Button>
    </div>
  );
}

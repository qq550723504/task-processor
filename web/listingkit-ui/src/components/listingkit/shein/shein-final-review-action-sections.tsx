import { Button } from "@/components/shared/button";

type FinalReviewSubmitAction = "publish" | "save_draft";

export function FinalReviewHeader({ confirmed }: { confirmed: boolean }) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.26em] text-zinc-500">
          SHEIN 最终确认
        </p>
        <h2 className="mt-1 text-lg font-semibold text-zinc-950">
          确认即将提交的资料
        </h2>
        <p className="mt-1 max-w-2xl text-sm text-zinc-600">
          发布前核对价格、SKU、属性和最终图片。保存最终草稿后才能从这里提交。
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
}: {
  blockingCount: number;
  confirmed: boolean;
  ready: boolean;
}) {
  return (
    <div
      className={`rounded-2xl border p-4 ${
        ready
          ? "border-emerald-200 bg-emerald-50"
          : "border-amber-200 bg-amber-50"
      }`}
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
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
              : "暂时不能提交"}
          </div>
          <p className="mt-1 text-sm leading-6 text-zinc-700">
            {ready
              ? "后端 readiness 已通过。提交前请确认价格、图片和 SKU。"
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

export function FinalReviewPublishConfirmCard({
  categoryId,
  finalImageCount,
  isSubmitting,
  onCancel,
  onConfirm,
  skuCount,
}: {
  categoryId?: number | null;
  finalImageCount: number;
  isSubmitting?: boolean;
  onCancel: () => void;
  onConfirm: () => void;
  skuCount: number;
}) {
  return (
    <div className="rounded-2xl border border-zinc-200 bg-zinc-50 p-4">
      <div className="space-y-2">
        <h3 className="text-base font-semibold text-zinc-950">确认发布到 SHEIN</h3>
        <p className="text-sm leading-6 text-zinc-600">
          这会把当前已确认资料正式提交到 SHEIN，请先核对类目、图片和 SKU。
        </p>
        <div className="grid gap-2 text-sm text-zinc-700 sm:grid-cols-3">
          <div className="rounded-xl border border-zinc-200 bg-white px-3 py-2">
            类目：{categoryId ?? "未确认"}
          </div>
          <div className="rounded-xl border border-zinc-200 bg-white px-3 py-2">
            图片：{finalImageCount} 张
          </div>
          <div className="rounded-xl border border-zinc-200 bg-white px-3 py-2">
            SKU：{skuCount} 个
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button tone="secondary" onClick={onCancel} type="button">
            取消
          </Button>
          <Button disabled={isSubmitting} onClick={onConfirm} type="button">
            确认发布
          </Button>
        </div>
      </div>
    </div>
  );
}

export function FinalReviewSubmitActions({
  confirmed,
  isSaving,
  isSubmitting,
  manualOverrides,
  onSaveFinalDraft,
  onStartPublishConfirm,
  onSubmit,
  ready,
  submitAction,
  submitHint,
}: {
  confirmed: boolean;
  isSaving?: boolean;
  isSubmitting?: boolean;
  manualOverrides: Record<string, number>;
  onSaveFinalDraft?: (payload: {
    confirmed?: boolean;
    submit_mode?: FinalReviewSubmitAction;
    manual_price_overrides?: Record<string, number>;
  }) => void;
  onStartPublishConfirm: () => void;
  onSubmit?: (action: FinalReviewSubmitAction) => void;
  ready: boolean;
  submitAction?: FinalReviewSubmitAction | null;
  submitHint: string;
}) {
  return (
    <div className="flex flex-wrap gap-2">
      <div className="basis-full rounded-2xl border border-zinc-200 bg-zinc-50 p-3 text-sm leading-6 text-zinc-700">
        <p className="font-semibold text-zinc-950">{submitHint}</p>
        <p className="mt-1">
          保存草稿：上传图片并保存到 SHEIN 草稿箱，不直接上架。正式发布：上传图片并提交 SHEIN 发布接口。
        </p>
      </div>
      <Button
        tone="secondary"
        disabled={isSaving}
        onClick={() =>
          onSaveFinalDraft?.({
            confirmed: true,
            submit_mode: "save_draft",
            manual_price_overrides: manualOverrides,
          })
        }
      >
        确认最终草稿
      </Button>
      <Button
        tone="secondary"
        disabled={!confirmed || !ready || isSubmitting}
        onClick={() => onSubmit?.("save_draft")}
      >
        {submitAction === "save_draft" ? "保存中..." : "保存到 SHEIN 草稿箱"}
      </Button>
      <Button
        disabled={!confirmed || !ready || isSubmitting}
        onClick={onStartPublishConfirm}
      >
        {submitAction === "publish" ? "发布中..." : "发布到 SHEIN"}
      </Button>
    </div>
  );
}

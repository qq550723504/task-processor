import { Button } from "@/components/ui/button";
import { SubmitFailureGuidance } from "@/components/listingkit/shein/shein-submit-readiness-sections";
import type {
  SheinImageUploadPreflight,
  SheinSubmissionReport,
} from "@/lib/types/listingkit";

export function SubmitActionCard({
  canRunSubmitActions,
  isPublishing,
  isSavingDraft,
  isSubmitting,
  onSaveDraft,
  onSubmit,
}: {
  canRunSubmitActions: boolean;
  isPublishing: boolean;
  isSavingDraft: boolean;
  isSubmitting?: boolean;
  onSaveDraft?: (() => void) | null;
  onSubmit?: (() => void) | null;
}) {
  return (
    <div className="rounded-2xl border border-emerald-200 bg-emerald-50/70 p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-emerald-700">
            发布操作
          </p>
          <p className="text-sm font-semibold text-zinc-950">
            保存草稿或提交到 SHEIN
          </p>
          <p className="text-sm leading-6 text-zinc-700">
            当前资料包已通过提交前检查。建议先保存到 SHEIN 草稿箱，确认后再正式发布。
          </p>
        </div>
        {canRunSubmitActions && (onSaveDraft || onSubmit) ? (
          <div className="flex flex-wrap gap-2">
            {onSaveDraft ? (
              <Button
                className="h-8 px-3 text-xs"
                disabled={isSubmitting}
                variant="secondary"
                onClick={onSaveDraft}
              >
                {isSavingDraft ? "保存草稿中..." : "保存到 SHEIN 草稿箱"}
              </Button>
            ) : null}
            {onSubmit ? (
              <Button
                className="h-8 px-3 text-xs"
                disabled={isSubmitting}
                onClick={onSubmit}
              >
                {isPublishing ? "提交中..." : "发布到 SHEIN"}
              </Button>
            ) : null}
          </div>
        ) : null}
      </div>
    </div>
  );
}

export function PrimaryActionCard({
  canRunPrimary,
  onRunPrimaryAction,
  primaryAction,
  primaryActionKey,
}: {
  canRunPrimary: boolean;
  onRunPrimaryAction?: ((key?: string | null) => void) | null;
  primaryAction?: string | null;
  primaryActionKey?: string | null;
}) {
  if (!primaryAction) {
    return null;
  }

  return (
    <div className="rounded-2xl border border-zinc-200 bg-white/80 p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
            下一步处理
          </p>
          <p className="text-sm font-semibold text-zinc-950">
            {primaryAction}
          </p>
        </div>
        {canRunPrimary && onRunPrimaryAction ? (
          <Button
            className="h-8 px-3 text-xs"
            variant="secondary"
            onClick={() => onRunPrimaryAction(primaryActionKey)}
          >
            去处理
          </Button>
        ) : null}
      </div>
    </div>
  );
}

export function CurrentSubmitStatusCard({
  activeSubmitAction,
  backendSubmitPhase,
  hasBackendSubmitAttempt,
  isSubmitting,
  leaseExpiresAt,
  submission,
  submitErrorMessage,
}: {
  activeSubmitAction?: "publish" | "save_draft" | null;
  backendSubmitPhase?: string | null;
  hasBackendSubmitAttempt: boolean;
  isSubmitting?: boolean;
  leaseExpiresAt?: string | null;
  submission?: SheinSubmissionReport | null;
  submitErrorMessage?: string | null;
}) {
  if (!isSubmitting && !submitErrorMessage && !hasBackendSubmitAttempt) {
    return null;
  }

  const active = Boolean(isSubmitting || hasBackendSubmitAttempt);

  return (
    <div
      className={`rounded-2xl border p-4 ${
        active
          ? "border-sky-200 bg-sky-50/70"
          : "border-rose-200 bg-rose-50/70"
      }`}
    >
      <div className="space-y-1">
        <p
          className={`text-xs font-semibold uppercase tracking-[0.18em] ${
            active ? "text-sky-700" : "text-rose-700"
          }`}
        >
          当前提交
        </p>
        <p className="text-sm font-semibold text-zinc-950">
          {active
            ? activeSubmitAction === "save_draft"
              ? "正在保存到 SHEIN 草稿箱..."
              : hasBackendSubmitAttempt
                ? "正在发布到 SHEIN"
                : "正在提交到 SHEIN..."
            : activeSubmitAction === "save_draft"
              ? "保存草稿失败"
              : "提交失败"}
        </p>
        {backendSubmitPhase ? (
          <p className="text-sm leading-6 text-zinc-700">
            当前阶段：{backendSubmitPhase}
          </p>
        ) : null}
        {submission?.current_request_id ? (
          <p className="break-words text-xs leading-5 text-zinc-600">
            Request ID: {submission.current_request_id}
          </p>
        ) : null}
        {leaseExpiresAt ? (
          <p className="text-xs leading-5 text-zinc-600">
            Lease 到期：{leaseExpiresAt}
          </p>
        ) : null}
        {submission?.current_phase === "confirm_remote" ? (
          <p className="text-sm leading-6 text-zinc-700">
            远端可能已收到，正在按供方货号确认。
          </p>
        ) : null}
        {submitErrorMessage ? (
          <SubmitFailureGuidance
            detail={submitErrorMessage}
            impact={
              activeSubmitAction === "save_draft"
                ? "本次不会把资料保存到 SHEIN 草稿箱，请先处理图片上传或阻断项后再重试。"
                : "本次不会把资料正式提交到 SHEIN，当前任务会停留在可修复状态。"
            }
            nextStep={
              activeSubmitAction === "save_draft"
                ? "先检查图片上传、最终资料和阻断项，再重新保存到 SHEIN 草稿箱。"
                : "先回到工作台处理阻断项或图片上传问题，确认后再重新发布。"
            }
          />
        ) : (
          <p className="text-sm leading-6 text-zinc-700">
            正在上传图片并{activeSubmitAction === "save_draft" ? "保存到 SHEIN 草稿箱" : "提交 SHEIN"}。
            完成或失败后会刷新最新提交记录。
          </p>
        )}
      </div>
    </div>
  );
}

export function ImageUploadPreflightCard({
  compact,
  imageUpload,
}: {
  compact?: boolean;
  imageUpload?: SheinImageUploadPreflight | null;
}) {
  if (!imageUpload) {
    return null;
  }

  return (
    <div className="rounded-2xl border border-sky-200 bg-sky-50/70 p-4">
      <div className="space-y-3">
        <div className="space-y-1">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-sky-700">
            图片上传检查
          </p>
          <p className="text-sm font-semibold text-zinc-950">
            提交前将上传 {imageUpload.pending_upload_urls ?? 0} 张唯一图片到 SHEIN
          </p>
        </div>
        <div className="grid gap-2 text-xs text-zinc-700 sm:grid-cols-2">
          <span>图片引用：{imageUpload.total_image_references ?? 0}</span>
          <span>唯一图片：{imageUpload.unique_image_urls ?? 0}</span>
          <span>SDS 图：{imageUpload.sds_mockup_urls ?? 0}</span>
          <span>SHEIN 已上传：{imageUpload.shein_uploaded_urls ?? 0}</span>
        </div>
        {!compact && imageUpload.summary?.length ? (
          <div className="space-y-1">
            {imageUpload.summary.map((line) => (
              <p className="text-xs leading-5 text-zinc-700" key={line}>
                {line}
              </p>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}

export function LatestSubmissionCard({
  latestSubmissionMessage,
  latestSubmissionStatus,
  latestSubmissionSummary,
  latestSubmissionTitle,
  latestValidationNotes,
  remoteStatus,
  submission,
}: {
  latestSubmissionMessage?: string | null;
  latestSubmissionStatus?: string | null;
  latestSubmissionSummary?: string | null;
  latestSubmissionTitle?: string;
  latestValidationNotes: string[];
  remoteStatus?: string | null;
  submission?: SheinSubmissionReport | null;
}) {
  if (!latestSubmissionStatus) {
    return null;
  }

  return (
    <div className="rounded-2xl border border-zinc-200 bg-white/80 p-4">
      <div className="space-y-3">
        <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
          最新提交记录
        </p>
        <div className="space-y-1">
          <p className="text-sm font-semibold text-zinc-950">
            {latestSubmissionTitle}
          </p>
          {latestSubmissionSummary ? (
            <p className="text-sm leading-6 text-zinc-700">
              {latestSubmissionSummary}
            </p>
          ) : null}
          {remoteStatus ? (
            <p className="text-xs leading-5 text-zinc-600">
              远端状态：{remoteStatus}
              {submission?.remote_checked_at
                ? ` · ${submission.remote_checked_at.replace("T", " ").replace(/\.\d+Z?$/, "").replace(/Z$/, "")}`
                : ""}
            </p>
          ) : null}
          {latestSubmissionMessage ? (
            <details className="rounded-2xl border border-zinc-200 bg-zinc-50 p-3">
              <summary className="cursor-pointer text-xs font-semibold text-zinc-700">
                查看原始接口返回
              </summary>
              <p className="mt-2 break-words text-xs leading-5 text-zinc-600">
                {latestSubmissionMessage}
              </p>
            </details>
          ) : null}
        </div>
        {latestValidationNotes.length ? (
          <details className="rounded-2xl border border-rose-100 bg-rose-50/70 p-3">
            <summary className="cursor-pointer text-[11px] font-semibold uppercase tracking-[0.18em] text-rose-700">
              查看原始 SHEIN 校验提示
            </summary>
            <ul className="space-y-1">
              {latestValidationNotes.map((note, index) => (
                <li
                  className="break-words text-xs leading-5 text-rose-700"
                  key={`${index}-${note}`}
                >
                  {note}
                </li>
              ))}
            </ul>
          </details>
        ) : null}
      </div>
    </div>
  );
}

import { Card } from "@/components/shared/card";
import { Button } from "@/components/shared/button";
import { SheinCustomerIssueSummary } from "@/components/listingkit/shein/shein-customer-issue-summary";
import {
  buildSheinCustomerIssues,
  type CustomerIssue,
} from "@/lib/shein-studio/shein-customer-issues";
import {
  sheinLatestSubmissionSummary,
  sheinLatestSubmissionTitle,
} from "@/lib/shein-studio/shein-submission-display";
import type {
  SheinChecklistGroupItem,
  SheinImageUploadPreflight,
  SheinReadinessItem,
  SheinResolutionCacheInfo,
  SheinResolutionCacheSummary,
  SheinSubmissionReport,
  SheinSubmitChecklist,
  SheinSubmitReadiness,
  SheinWorkspaceOverview,
} from "@/lib/types/listingkit";

function statusLabel(status?: string) {
  switch (status) {
    case "blocked":
      return "有阻断";
    case "ready_with_warnings":
      return "可提交但有提醒";
    case "ready":
      return "可提交";
    default:
      return "未知";
  }
}

function statusTone(status?: string) {
  switch (status) {
    case "blocked":
      return "border-rose-200 bg-rose-50 text-rose-700";
    case "ready_with_warnings":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "ready":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    default:
      return "border-zinc-200 bg-zinc-50 text-zinc-700";
  }
}

function checklistLabel(items?: SheinChecklistGroupItem[] | null) {
  if (!items?.length) {
    return null;
  }
  return items;
}

function fieldPathsLabel(paths?: string[] | null) {
  if (!paths?.length) {
    return null;
  }
  return paths.join(" · ");
}

function compactSubmissionMessage(message?: string | null) {
  if (!message) {
    return null;
  }

  const status = message.match(/STATUS:\s*([0-9]+)/i)?.[1];
  const eventId = message.match(/EVENT ID:\s*([0-9]+)/i)?.[1];
  const url = message.match(/\(URL:\s*([^)]+)\)/i)?.[1];
  if (status) {
    return [
      `SHEIN endpoint returned ${status}.`,
      eventId ? `Event ID: ${eventId}.` : null,
      url ? `URL: ${url}` : null,
    ]
      .filter(Boolean)
      .join(" ");
  }

  const text = message
    .replace(/<style[\s\S]*?<\/style>/gi, " ")
    .replace(/<script[\s\S]*?<\/script>/gi, " ")
    .replace(/<[^>]*>/g, " ")
    .replace(/\s+/g, " ")
    .trim();

  if (text.length <= 320) {
    return text;
  }
  return `${text.slice(0, 320)}...`;
}

function normalizedSubmissionStatus(submission?: SheinSubmissionReport | null) {
  const status = submission?.last_status;
  const result = submission?.last_result;
  if (
    status === "unknown" &&
    (result?.success === false || result?.validation_notes?.length)
  ) {
    return "failed";
  }
  return status;
}

function cacheSourceLabel(source?: string) {
  switch (source) {
    case "manual_cache":
      return "Manual";
    case "history_cache":
      return "DB";
    case "memory_cache":
      return "Memory";
    case "live_resolver":
      return "Live";
    case "static_fallback":
      return "Static";
    case "llm":
      return "LLM";
    default:
      return source ?? "未知";
  }
}

function cacheUpdatedLabel(value?: string) {
  if (!value) {
    return "暂无时间";
  }
  return value.replace("T", " ").replace(/\.\d+Z?$/, "").replace(/Z$/, "");
}

function hasResolutionCache(cache?: SheinResolutionCacheSummary | null) {
  return Boolean(cache?.category || cache?.attributes || cache?.sale_attributes);
}

function ResolutionCacheRow({
  title,
  item,
  kind,
  onClear,
  isClearing,
}: {
  title: string;
  item?: SheinResolutionCacheInfo | null;
  kind: "category" | "attribute" | "sale_attribute";
  onClear?: ((kind: "category" | "attribute" | "sale_attribute") => void) | null;
  isClearing?: boolean;
}) {
  return (
    <div className="rounded-2xl border border-zinc-200 bg-white/80 p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <p className="text-sm font-semibold text-zinc-950">{title}</p>
          <p className="text-xs leading-5 text-zinc-600">
            {item
            ? `${cacheSourceLabel(item.source)} · ${item.short_key ?? "无 key"}`
              : "暂无缓存信息"}
          </p>
          {item ? (
            <p className="text-[11px] leading-5 text-zinc-500">
              {item.status ?? "未知"} · 命中 {item.hit_count ?? 0} ·{" "}
              {cacheUpdatedLabel(item.updated_at)}
              {item.manual ? " · 人工确认" : ""}
            </p>
          ) : null}
        </div>
        {item?.clearable && onClear ? (
          <Button
            className="h-8 shrink-0 px-3 text-xs"
            disabled={isClearing}
            tone="secondary"
            onClick={() => onClear(kind)}
          >
            清除
          </Button>
        ) : null}
      </div>
    </div>
  );
}

function ReadinessItems({
  title,
  items,
  actionLabel = "去处理",
  canSelectItem,
  onSelectItem,
}: {
  title: string;
  items?: SheinReadinessItem[] | null;
  actionLabel?: string;
  canSelectItem?: ((item: SheinReadinessItem) => boolean) | null;
  onSelectItem?: ((item: SheinReadinessItem) => void) | null;
}) {
  if (!items?.length) {
    return null;
  }

  return (
    <div className="space-y-3">
      <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
        {title}
      </p>
      <div className="space-y-3">
        {items.map((item) => {
          const canAct = canSelectItem ? canSelectItem(item) : false;
          return (
            <div
              className="space-y-2 rounded-2xl border border-zinc-200 bg-white/80 p-4"
              key={`${title}-${item.key}-${item.label}`}
            >
              <div className="space-y-1">
                <p className="text-sm font-semibold text-zinc-950">
                  {item.label ?? item.key ?? "未命名问题"}
                </p>
                {item.message ? (
                  <p className="text-sm leading-6 text-zinc-700">{item.message}</p>
                ) : null}
              </div>
              {item.reason?.summary ? (
                <p className="text-xs leading-5 text-zinc-600">
                  {item.reason.summary}
                </p>
              ) : null}
              {fieldPathsLabel(item.field_paths) ? (
                <p className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">
                  {fieldPathsLabel(item.field_paths)}
                </p>
              ) : null}
              <div className="flex flex-wrap items-center gap-2">
                {item.suggested_action ? (
                  <span className="rounded-full border border-zinc-200 bg-zinc-100 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-700">
                    {item.suggested_action}
                  </span>
                ) : null}
                {canAct && onSelectItem ? (
                  <Button
                    className="h-8 px-3 text-xs"
                    tone="secondary"
                    onClick={() => onSelectItem(item)}
                  >
                    {actionLabel}
                  </Button>
                ) : null}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function ChecklistSection({
  title,
  items,
}: {
  title: string;
  items?: SheinChecklistGroupItem[] | null;
}) {
  if (!items?.length) {
    return null;
  }

  return (
    <div className="space-y-2">
      <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
        {title}
      </p>
      <div className="space-y-2">
        {items.map((item) => (
          <div
            className="flex items-start justify-between gap-3 rounded-xl border border-zinc-200/80 bg-white/70 px-3 py-2"
            key={`${title}-${item.key}-${item.label}`}
          >
            <div className="space-y-1">
              <p className="text-sm font-medium text-zinc-900">
                {item.label ?? item.key ?? "未命名检查项"}
              </p>
              {item.message ? (
                <p className="text-xs leading-5 text-zinc-600">{item.message}</p>
              ) : null}
            </div>
            <span className="rounded-full border border-zinc-200 bg-zinc-100 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-700">
              {item.status ?? "未知"}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

export function SheinSubmitReadinessPanel({
  readiness,
  checklist,
  submission,
  imageUpload,
  resolutionCache,
  workspaceOverview,
  canSelectBlockingItem,
  onSelectBlockingItem,
  canRunPrimaryAction,
  onRunPrimaryAction,
  canSubmit,
  onSubmit,
  onSaveDraft,
  isSubmitting,
  submitAction,
  submitErrorMessage,
  onClearResolutionCache,
  clearingResolutionCacheKind,
  compact = false,
}: {
  readiness?: SheinSubmitReadiness | null;
  checklist?: SheinSubmitChecklist | null;
  submission?: SheinSubmissionReport | null;
  imageUpload?: SheinImageUploadPreflight | null;
  resolutionCache?: SheinResolutionCacheSummary | null;
  workspaceOverview?: SheinWorkspaceOverview | null;
  canSelectBlockingItem?: ((item: SheinReadinessItem) => boolean) | null;
  onSelectBlockingItem?: ((item: SheinReadinessItem) => void) | null;
  canRunPrimaryAction?: ((key?: string | null) => boolean) | null;
  onRunPrimaryAction?: ((key?: string | null) => void) | null;
  canSubmit?: boolean;
  onSubmit?: (() => void) | null;
  onSaveDraft?: (() => void) | null;
  isSubmitting?: boolean;
  submitAction?: "publish" | "save_draft" | null;
  submitErrorMessage?: string | null;
  onClearResolutionCache?:
    | ((kind: "category" | "attribute" | "sale_attribute") => void)
    | null;
  clearingResolutionCacheKind?: string | null;
  compact?: boolean;
}) {
  if (!readiness && !checklist && !workspaceOverview) {
    return null;
  }

  const required = checklistLabel(checklist?.required);
  const recommended = checklistLabel(checklist?.recommended);
  const primaryActionKey = workspaceOverview?.primary_action_key;
  const canRunPrimary =
    readiness?.status !== "ready" &&
    Boolean(primaryActionKey) &&
    (canRunPrimaryAction ? canRunPrimaryAction(primaryActionKey) : false);
  const submitReady = readiness?.ready === true || readiness?.status === "ready";
  const latestValidationNotes = submission?.last_result?.validation_notes ?? [];
  const latestSubmissionStatus = normalizedSubmissionStatus(submission);
  const latestSubmissionMessage = compactSubmissionMessage(
    submission?.last_error ??
      (latestValidationNotes.length ? null : submission?.last_result?.message),
  );
  const latestSubmissionTitle = sheinLatestSubmissionTitle(submission);
  const latestSubmissionSummary = sheinLatestSubmissionSummary(submission);
  const isSavingDraft = isSubmitting && submitAction === "save_draft";
  const isPublishing = isSubmitting && submitAction !== "save_draft";
  const canRunSubmitActions = canSubmit === true && submitReady;
  const customerIssues = buildSheinCustomerIssues({
    submit_readiness: readiness ?? undefined,
    submission: submission ?? undefined,
  });
  const canSelectIssue = (issue: CustomerIssue) =>
    Boolean(
      issue.actionKey &&
        canSelectBlockingItem?.({
          key: issue.actionKey,
          label: issue.title,
          message: issue.message,
        }),
    );
  const handleSelectIssue = (issue: CustomerIssue) => {
    if (!issue.actionKey) {
      return;
    }
    onSelectBlockingItem?.({
      key: issue.actionKey,
      label: issue.title,
      message: issue.message,
    });
  };

  return (
    <Card className="border-zinc-300 bg-zinc-50/80 p-5">
      <div className="space-y-4">
        <div className="space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-600">
              SHEIN 发布检查
            </p>
            <span
              className={`rounded-full border px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] ${statusTone(
                readiness?.status ?? workspaceOverview?.submit_state?.status,
              )}`}
            >
              {statusLabel(readiness?.status ?? workspaceOverview?.submit_state?.status)}
            </span>
          </div>
          {workspaceOverview?.headline ? (
            <p className="text-sm font-semibold text-zinc-950">
              {workspaceOverview.headline}
            </p>
          ) : null}
          {workspaceOverview?.subheadline ? (
            <p className="text-sm leading-6 text-zinc-700">
              {workspaceOverview.subheadline}
            </p>
          ) : null}
          <div className="flex flex-wrap gap-2 text-xs uppercase tracking-[0.16em] text-zinc-500">
            <span>
              阻断 {readiness?.blocking_items?.length ?? workspaceOverview?.submit_state?.blocking_count ?? 0}
            </span>
            <span>
              提醒 {readiness?.warning_items?.length ?? workspaceOverview?.submit_state?.warning_count ?? 0}
            </span>
          </div>
        </div>

        <SheinCustomerIssueSummary
          compact={compact}
          issues={customerIssues}
          canSelectIssue={canSelectIssue}
          onSelectIssue={handleSelectIssue}
        />

        {submitReady ? (
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
                      tone="secondary"
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
        ) : workspaceOverview?.primary_action ? (
          <div className="rounded-2xl border border-zinc-200 bg-white/80 p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="space-y-1">
                <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  下一步处理
                </p>
                <p className="text-sm font-semibold text-zinc-950">
                  {workspaceOverview.primary_action}
                </p>
              </div>
              {canRunPrimary && onRunPrimaryAction ? (
                <Button
                  className="h-8 px-3 text-xs"
                  tone="secondary"
                  onClick={() => onRunPrimaryAction(primaryActionKey)}
                >
                  去处理
                </Button>
              ) : null}
            </div>
          </div>
        ) : null}

        {isSubmitting || submitErrorMessage ? (
          <div
            className={`rounded-2xl border p-4 ${
              isSubmitting
                ? "border-sky-200 bg-sky-50/70"
                : "border-rose-200 bg-rose-50/70"
            }`}
          >
            <div className="space-y-1">
              <p
                className={`text-xs font-semibold uppercase tracking-[0.18em] ${
                  isSubmitting ? "text-sky-700" : "text-rose-700"
                }`}
              >
                当前提交
              </p>
              <p className="text-sm font-semibold text-zinc-950">
                {isSubmitting
                  ? isSavingDraft
                    ? "正在保存到 SHEIN 草稿箱..."
                    : "正在提交到 SHEIN..."
                  : submitAction === "save_draft"
                    ? "保存草稿失败"
                    : "提交失败"}
              </p>
              {submitErrorMessage ? (
                <p className="break-words text-sm leading-6 text-rose-700">
                  {submitErrorMessage}
                </p>
              ) : (
                <p className="text-sm leading-6 text-zinc-700">
                  正在上传图片并{submitAction === "save_draft" ? "保存到 SHEIN 草稿箱" : "提交 SHEIN"}。
                  完成或失败后会刷新最新提交记录。
                </p>
              )}
            </div>
          </div>
        ) : null}

        {imageUpload ? (
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
        ) : null}

        {hasResolutionCache(resolutionCache) ? (
          <div className="rounded-2xl border border-zinc-200 bg-white/70 p-4">
            <div className="space-y-3">
              <div className="space-y-1">
                <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  解析缓存
                </p>
                <p className="text-sm leading-6 text-zinc-700">
                  分类、普通属性、销售属性的解析来源和缓存状态。
                </p>
              </div>
              <ResolutionCacheRow
                title="类目"
                item={resolutionCache?.category}
                kind="category"
                isClearing={clearingResolutionCacheKind === "category"}
                onClear={onClearResolutionCache}
              />
              <ResolutionCacheRow
                title="普通属性"
                item={resolutionCache?.attributes}
                kind="attribute"
                isClearing={clearingResolutionCacheKind === "attribute"}
                onClear={onClearResolutionCache}
              />
              <ResolutionCacheRow
                title="销售属性"
                item={resolutionCache?.sale_attributes}
                kind="sale_attribute"
                isClearing={clearingResolutionCacheKind === "sale_attribute"}
                onClear={onClearResolutionCache}
              />
            </div>
          </div>
        ) : null}

        {latestSubmissionStatus ? (
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
        ) : null}

        {!compact && readiness?.summary?.length ? (
          <div className="space-y-2">
            {readiness.summary.map((line) => (
              <p className="text-sm leading-6 text-zinc-700" key={line}>
                {line}
              </p>
            ))}
          </div>
        ) : null}

        {!compact ? (
          <ReadinessItems
            title="阻断项"
            items={readiness?.blocking_items}
            canSelectItem={canSelectBlockingItem}
            onSelectItem={onSelectBlockingItem}
          />
        ) : null}

        {!compact ? (
          <ReadinessItems
            title="提醒项"
            items={readiness?.warning_items}
          />
        ) : null}

        {!compact && workspaceOverview?.highlights?.length ? (
          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              重点信息
            </p>
            <div className="space-y-2">
              {workspaceOverview.highlights.map((line) => (
                <p className="text-sm leading-6 text-zinc-700" key={line}>
                  {line}
                </p>
              ))}
            </div>
          </div>
        ) : null}

        {!compact && workspaceOverview?.next_actions?.length ? (
          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              下一步
            </p>
            <div className="flex flex-wrap gap-2">
              {workspaceOverview.next_actions.map((line) => (
                <span
                  className="rounded-full border border-zinc-200 bg-white/80 px-3 py-1 text-[11px] font-medium tracking-[0.02em] text-zinc-700"
                  key={line}
                >
                  {line}
                </span>
              ))}
            </div>
          </div>
        ) : null}

        {!compact && (required || recommended) ? (
          <div className="space-y-3">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              检查清单
            </p>
            <ChecklistSection title="必须完成" items={required} />
            <ChecklistSection title="建议确认" items={recommended} />
          </div>
        ) : null}
      </div>
    </Card>
  );
}

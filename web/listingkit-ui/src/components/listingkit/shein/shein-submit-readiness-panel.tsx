import { Card } from "@/components/ui/card";
import { SheinCustomerIssueSummary } from "@/components/listingkit/shein/shein-customer-issue-summary";
import {
  checklistLabel,
  compactSubmissionMessage,
  hasResolutionCache,
  normalizedSubmissionStatus,
  statusLabel,
  statusTone,
} from "@/components/listingkit/shein/shein-submit-readiness-helpers";
import {
  ChecklistSection,
  ReadinessItems,
  ResolutionCacheSummaryCard,
} from "@/components/listingkit/shein/shein-submit-readiness-sections";
import {
  CurrentSubmitStatusCard,
  ImageUploadPreflightCard,
  LatestSubmissionCard,
  PrimaryActionCard,
  SubmitActionCard,
} from "@/components/listingkit/shein/shein-submit-readiness-submission-sections";
import {
  buildSheinCustomerIssues,
  type CustomerIssue,
} from "@/lib/shein-studio/shein-customer-issues";
import {
  sheinLatestSubmissionSummary,
  sheinLatestSubmissionTitle,
  sheinPublishInFlight,
  sheinPublishSucceeded,
  sheinSubmitPhaseLabel,
} from "@/lib/shein-studio/shein-submission-display";
import type {
  SheinImageUploadPreflight,
  SheinReadinessItem,
  SheinResolutionCacheSummary,
  SheinSubmissionReport,
  SheinSubmitChecklist,
  SheinSubmitReadiness,
  SheinWorkspaceOverview,
} from "@/lib/types/listingkit";

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
  showSubmitActions = true,
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
    | ((kind: "category" | "attribute" | "sale_attribute" | "pricing") => void)
    | null;
  clearingResolutionCacheKind?: string | null;
  compact?: boolean;
  showSubmitActions?: boolean;
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
  const isSavingDraft = Boolean(isSubmitting && submitAction === "save_draft");
  const publishInFlight = Boolean(isSubmitting && submitAction !== "save_draft") || sheinPublishInFlight(submission);
  const publishSucceeded = sheinPublishSucceeded(submission);
  const isPublishing = publishInFlight;
  const backendSubmitPhase = sheinSubmitPhaseLabel(submission?.current_phase);
  const backendSubmitAction = submission?.current_action as
    | "publish"
    | "save_draft"
    | undefined;
  const hasBackendSubmitAttempt = Boolean(submission?.current_phase);
  const leaseExpiresAt = submission?.lease_expires_at
    ? submission.lease_expires_at.replace("T", " ").replace(/\.\d+Z?$/, "").replace(/Z$/, "")
    : null;
  const remoteStatus = submission?.remote_status;
  const activeSubmitAction = submitAction ?? backendSubmitAction ?? null;
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

        {!compact ? (
          <SheinCustomerIssueSummary
            compact={compact}
            issues={customerIssues}
            canSelectIssue={canSelectIssue}
            onSelectIssue={handleSelectIssue}
          />
        ) : null}

        {showSubmitActions && submitReady ? (
          <SubmitActionCard
            canRunSubmitActions={canRunSubmitActions}
            isPublished={publishSucceeded}
            isPublishing={isPublishing}
            isSavingDraft={isSavingDraft}
            isSubmitting={isSubmitting || publishInFlight}
            onSaveDraft={onSaveDraft}
            onSubmit={onSubmit}
          />
        ) : showSubmitActions && workspaceOverview?.primary_action ? (
          <PrimaryActionCard
            canRunPrimary={canRunPrimary}
            onRunPrimaryAction={onRunPrimaryAction}
            primaryAction={workspaceOverview.primary_action}
            primaryActionKey={primaryActionKey}
          />
        ) : null}

        <CurrentSubmitStatusCard
          activeSubmitAction={activeSubmitAction}
          backendSubmitPhase={backendSubmitPhase}
          hasBackendSubmitAttempt={hasBackendSubmitAttempt}
          isSubmitting={isSubmitting}
          leaseExpiresAt={leaseExpiresAt}
          submission={submission}
          submitErrorMessage={submitErrorMessage}
        />

        <ImageUploadPreflightCard compact={compact} imageUpload={imageUpload} />

        {hasResolutionCache(resolutionCache) ? (
          <ResolutionCacheSummaryCard
            clearingResolutionCacheKind={clearingResolutionCacheKind}
            onClearResolutionCache={onClearResolutionCache}
            resolutionCache={resolutionCache}
          />
        ) : null}

        <LatestSubmissionCard
          latestSubmissionMessage={latestSubmissionMessage}
          latestSubmissionStatus={latestSubmissionStatus}
          latestSubmissionSummary={latestSubmissionSummary}
          latestSubmissionTitle={latestSubmissionTitle}
          latestValidationNotes={latestValidationNotes}
          remoteStatus={remoteStatus}
          submission={submission}
        />

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

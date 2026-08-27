"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";

import { RecoverySummaryCard } from "@/components/listingkit/review/recovery-summary-card";
import { ResolvedActionCard } from "@/components/listingkit/review/resolved-action-card";
import { SheinFlowNav } from "@/components/listingkit/shein/shein-flow-nav";
import { TaskRevisionHistoryPanel } from "@/components/listingkit/tasks/task-revision-history-panel";
import { TaskStatusPanel } from "@/components/listingkit/tasks/task-status-panel";
import { TaskProgressNotice } from "@/components/listingkit/tasks/task-progress-notice";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ProductWorkspaceAIReview } from "@/components/listingkit/workspace/product-workspace-ai-review";
import {
  buildProductWorkspaceAttentionSummary,
  buildProductWorkspaceCanonicalNavigation,
  buildProductWorkspacePlatformNavigation,
  type ProductWorkspaceNavItem,
  type ProductWorkspaceSectionKey,
} from "@/components/listingkit/workspace/product-workspace-model";
import { ProductWorkspaceNavigation } from "@/components/listingkit/workspace/product-workspace-navigation";
import {
  buildProductWorkspaceReviewIssues,
  type ProductWorkspaceReviewIssue,
} from "@/components/listingkit/workspace/product-workspace-review-model";
import { ProductWorkspaceShell } from "@/components/listingkit/workspace/product-workspace-shell";
import { SDSRepairPanel } from "@/components/listingkit/workspace/sds-repair-panel";
import { SheinAdvancedReviewDetails } from "@/components/listingkit/workspace/shein-advanced-review-details";
import {
  SheinFinalReviewWorkspaceView,
  WorkspaceReviewView,
} from "@/components/listingkit/workspace/workspace-screen-views";
import {
  buildSheinAdvancedReviewDetailsProps,
  buildSheinWorkspaceViewProps,
} from "@/components/listingkit/workspace/shein-workspace-view-props";
import { submitErrorMessage } from "@/components/listingkit/workspace/workspace-screen-helpers";
import { WorkspaceOverviewPanel } from "@/components/listingkit/workspace/workspace-overview-panel";
import { useSheinWorkspaceActions } from "@/components/listingkit/workspace/use-shein-workspace-actions";
import { useWorkspaceData } from "@/components/listingkit/workspace/use-workspace-data";
import { useWorkspaceNavigationActions } from "@/components/listingkit/workspace/use-workspace-navigation-actions";
import {
  WorkspaceLoadingState,
  WorkspaceLoadErrorState,
  WorkspacePendingDataState,
} from "@/components/listingkit/workspace/workspace-screen-states";
import { buildWorkspaceReviewViewProps } from "@/components/listingkit/workspace/workspace-review-view-props";
import { useApplyRevision } from "@/lib/query/use-apply-revision";
import { useExecuteAction } from "@/lib/query/use-action";
import {
  getTaskRetryVersion,
  useRetryChildTask,
} from "@/lib/query/use-child-task-retry";
import { useClearSheinResolutionCache } from "@/lib/query/use-shein-resolution-cache";
import { useUpdateSheinFinalDraft } from "@/lib/query/use-shein-final-draft";
import {
  useRefreshSubmissionStatus,
  useSubmitTask,
} from "@/lib/query/use-submit-task";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

export function WorkspaceScreen({ taskId }: { taskId: string }) {
  const [sdsRepairOpen, setSDSRepairOpen] = useState(false);
  const [selectedProductSection, setSelectedProductSection] =
    useState<ProductWorkspaceSectionKey>("overview");
  const [historySelected, setHistorySelected] = useState(false);
  const searchParams = useSearchParams();
  const workspaceData = useWorkspaceData({ taskId, searchParams });
  const {
    baseQuery,
    preview,
    taskResult,
    session,
    reviewPreview,
    sessionData,
    platformCards,
    focusedPreview,
    selectedPlatform,
    focusedScenePreset,
    suppressResolvedActionSummary,
    resolvedActionSummary,
    previewSuggestion,
    sheinImages,
    sheinAvailableImages,
    sheinMockupImages,
    sheinVariantCount,
    sheinPreviewPayload,
    showSheinCategoryReview,
    showSheinAttributeReview,
    showSheinSaleAttributeReview,
    showSheinReviewDetails,
    shouldOpenSheinAdvancedDetails,
    isSheinFinalReviewMode,
    sheinFlowSteps,
    workspaceTitle,
    workspaceStatusLabel,
    workspaceUpdatedAt,
    workspaceSubtitle,
  } = workspaceData;
  const applyRevision = useApplyRevision(taskId);
  const submitTask = useSubmitTask(taskId);
  const refreshSubmissionStatus = useRefreshSubmissionStatus(taskId);
  const updateSheinFinalDraft = useUpdateSheinFinalDraft(taskId);
  const clearSheinResolutionCache = useClearSheinResolutionCache(taskId);
  const layerAction = useExecuteAction(taskId, baseQuery);
  const sheinActions = useSheinWorkspaceActions({
    taskId,
    sheinPreview: sheinPreviewPayload,
    preview,
    taskResult,
    applyRevision,
    submitTask,
    updateSheinFinalDraft,
  });
  const workspaceActions = useWorkspaceNavigationActions({
    taskId,
    baseQuery,
    searchParams,
    focusedTarget: session.data?.session?.focused_target,
    sheinStoreID:
      preview.data?.shein?.store_resolution?.store_id ??
      taskResult.data?.result?.shein_store_resolution?.store_id,
    sheinFreshnessActions: {
      shein_category_template_freshness: sheinActions.handleRefreshSheinCategory,
      shein_attribute_template_freshness:
        sheinActions.handleRegenerateSheinAttributes,
      shein_sale_attribute_template_freshness:
        sheinActions.handleRegenerateSheinSaleAttributes,
      shein_sale_attribute_freshness:
        sheinActions.handleRegenerateSheinSaleAttributes,
    },
  });
  const childTaskRetry = useRetryChildTask(
    taskId,
    getTaskRetryVersion(taskResult.data),
    taskResult.data?.child_retries,
  );
  const sheinViewProps = buildSheinWorkspaceViewProps({
    shein: preview.data?.shein,
    selectedPlatform,
    focusedPreview,
    sheinImages,
    sheinAvailableImages,
    sheinMockupImages,
    sheinVariantCount,
    sheinActions,
    isSavingFinalDraft: updateSheinFinalDraft.isPending,
    isSubmitting: submitTask.isPending,
    submitError: submitTask.error,
    clearingResolutionCacheKind: clearSheinResolutionCache.isPending
      ? clearSheinResolutionCache.variables
      : null,
    isRefreshingSubmissionStatus: refreshSubmissionStatus.isPending,
    onSelectBlockingItem: workspaceActions.handleSelectSheinBlockingItem,
    onRunPrimaryAction: workspaceActions.handleRunSheinPrimaryAction,
    onClearResolutionCache: (kind) => clearSheinResolutionCache.mutate(kind),
    onRefreshSubmissionStatus: () => refreshSubmissionStatus.mutate(),
  });
  const sheinAdvancedReviewDetailsProps = buildSheinAdvancedReviewDetailsProps({
    applyErrorMessage: submitErrorMessage(applyRevision.error),
    taskId,
    shein: preview.data?.shein,
    selectedPlatform,
    showReviewDetails: showSheinReviewDetails,
    showCategoryReview: showSheinCategoryReview,
    showAttributeReview: showSheinAttributeReview,
    showSaleAttributeReview: showSheinSaleAttributeReview,
    isFinalReviewMode: isSheinFinalReviewMode,
    open: shouldOpenSheinAdvancedDetails,
    isApplying: applyRevision.isPending,
    sheinActions,
  });
  const refetchWorkspace = () =>
    Promise.all([preview.refetch(), session.refetch(), taskResult.refetch()]);

  const handleRunStandardProductTemporal = () => {
    layerAction.mutate({
      action_key: "run_standard_product_temporal",
    });
  };

  const handleRunPlatformAdaptTemporal = () => {
    layerAction.mutate({
      action_key: "run_platform_adapt_temporal",
      target: {
        action_key: "run_platform_adapt_temporal",
        queue_query: {
          platform: "all",
        },
      },
    });
  };

  if (preview.isLoading || session.isLoading) {
    return <WorkspaceLoadingState />;
  }

  if (preview.isError || session.isError || taskResult.isError) {
    return <WorkspaceLoadErrorState onRetry={refetchWorkspace} />;
  }

  if (!preview.data || !sessionData) {
    return <WorkspacePendingDataState onRetry={refetchWorkspace} />;
  }

  const sheinAdvancedReviewDetails = sheinAdvancedReviewDetailsProps ? (
    <SheinAdvancedReviewDetails {...sheinAdvancedReviewDetailsProps} />
  ) : null;
  const workspaceReviewViewProps = buildWorkspaceReviewViewProps({
    selectedPlatform,
    previewSuggestion,
    sessionData,
    reviewPreviewData: reviewPreview.data,
    taskResult: taskResult.data,
    focusedPreview,
    sheinViewProps,
    focusedScenePreset,
    recoveryDescriptors:
      session.data?.recovery_summary?.recommended_descriptors ??
      sessionData.overview?.recovery_summary?.recommended_descriptors,
    onDispatch: workspaceActions.dispatchTarget,
    onToolbarAction: workspaceActions.handleToolbarAction,
    onRecovery: workspaceActions.handleRecovery,
  });

  const canonicalNavigation = buildProductWorkspaceCanonicalNavigation(
    selectedProductSection,
  );
  const platformNavigation = buildProductWorkspacePlatformNavigation(
    platformCards.map((card) => ({
      platform: card.platform,
      label: card.platform.toUpperCase(),
      status: platformStatusForTask(card.platform, selectedPlatform, taskResult.data),
    })),
    selectedPlatform,
  );
  const reviewIssues = buildProductWorkspaceReviewIssues(
    taskResult.data,
    selectedPlatform,
  );
  const issueSummary = buildProductWorkspaceAttentionSummary({
    blockingCount: countIssueSeverity(reviewIssues, "blocking"),
    warningCount: countIssueSeverity(reviewIssues, "warning"),
    passedCount: 0,
  });
  const recoverySummary =
    sessionData?.overview?.recovery_summary ??
    session.data?.recovery_summary ??
    preview.data.asset_generation_overview?.recovery_summary;
  const canRepairSDS =
    taskResult.data?.status === "needs_review" &&
    taskResult.data?.result?.child_tasks?.some(
      (child) => child.kind === "sds_design_sync" && child.status === "failed",
    );

  const handleNavigationSelect = (item: ProductWorkspaceNavItem) => {
    setHistorySelected(false);
    if (item.platform) {
      const platformCard = platformCards.find(
        (card) => card.platform === item.platform,
      );
      workspaceActions.dispatchTarget(
        platformCard?.primary_navigation_target ??
          platformCard?.resolved_action_summary?.navigation_target,
      );
      workspaceActions.handlePlatformSelect(item.platform);
      return;
    }
    if (isProductSectionKey(item.key)) {
      setSelectedProductSection(item.key);
    }
  };

  const centralWork = (
    <div className="min-w-0 space-y-4">
      <div className="grid min-w-0 gap-3 2xl:grid-cols-2">
        <ResolvedActionCard
          summary={suppressResolvedActionSummary ? undefined : resolvedActionSummary}
          onSelect={(summary) => workspaceActions.handleAction(summary)}
        />
        <RecoverySummaryCard
          summary={recoverySummary}
          onSelect={workspaceActions.handleRecovery}
        />
      </div>

      {selectedPlatform === "shein" ? (
        <SheinFlowNav
          eyebrow="SHEIN 流程状态"
          steps={sheinFlowSteps}
          title="准备资料 → 校验 → 待提交 → 已发布"
        />
      ) : null}

      {shouldOpenSheinAdvancedDetails ? sheinAdvancedReviewDetails : null}

      {isSheinFinalReviewMode ? (
        <SheinFinalReviewWorkspaceView
          taskId={taskId}
          imageGalleryProps={sheinViewProps.imageGalleryProps}
          finalReviewProps={sheinViewProps.finalReviewProps}
          readinessProps={sheinViewProps.finalModeReadinessProps}
          timelineProps={sheinViewProps.timelineProps}
        />
      ) : (
        <WorkspaceReviewView {...workspaceReviewViewProps} />
      )}

      {!shouldOpenSheinAdvancedDetails ? sheinAdvancedReviewDetails : null}

      {selectedProductSection === "overview" ? (
        <WorkspaceOverviewPanel
          overview={sessionData.overview}
          reviewSummary={sessionData.review_summary}
        />
      ) : null}
    </div>
  );

  return (
    <div className="min-w-0 space-y-5 overflow-x-hidden">
      <ProductWorkspaceHeader
        layerActionsPending={layerAction.isPending}
        onGeneratePlatformData={handleRunPlatformAdaptTemporal}
        onGenerateProduct={handleRunStandardProductTemporal}
        statusLabel={workspaceStatusLabel}
        subtitle={workspaceSubtitle}
        taskId={taskId}
        title={workspaceTitle}
        updatedAtLabel={workspaceUpdatedAt}
      />

      <ProductWorkspaceShell
        navigation={
          <ProductWorkspaceNavigation
            canonicalItems={canonicalNavigation}
            historySelected={historySelected}
            onSelect={handleNavigationSelect}
            onSelectHistory={() => setHistorySelected(true)}
            platformItems={platformNavigation}
          />
        }
        work={centralWork}
        aiReview={
          <ProductWorkspaceAIReview
            issues={reviewIssues}
            onSelectIssue={(issue) => {
              if (issue.actionKey) {
                workspaceActions.handleRunSheinPrimaryAction(issue.actionKey);
              }
            }}
            summary={issueSummary}
          />
        }
        actions={
          <div className="space-y-4">
            <TaskProgressNotice task={taskResult.data} />
            <Card className="p-4">
              <details>
                <summary className="cursor-pointer text-sm font-semibold text-foreground">
                  执行与修复
                </summary>
                <div className="mt-4 space-y-4">
                  {canRepairSDS ? (
                    <Button
                      onClick={() => setSDSRepairOpen(true)}
                      type="button"
                      variant="secondary"
                    >
                      修复 SDS
                    </Button>
                  ) : null}
                  <TaskStatusPanel
                    task={taskResult.data}
                    onRetryChildTask={(kind) => childTaskRetry.mutate({ kind })}
                    retryingChildTaskKind={
                      childTaskRetry.isPending
                        ? childTaskRetry.variables?.kind ?? null
                        : null
                    }
                    retryQueued={childTaskRetry.retryQueued}
                    retryError={childTaskRetry.error}
                  />
                </div>
              </details>
            </Card>
            <TaskRevisionHistoryPanel taskId={taskId} defaultCollapsed />
            <SDSRepairPanel
              taskId={taskId}
              open={sdsRepairOpen}
              onClose={() => setSDSRepairOpen(false)}
            />
          </div>
        }
      />
    </div>
  );
}

function ProductWorkspaceHeader({
  title,
  subtitle,
  statusLabel,
  updatedAtLabel,
  taskId,
  layerActionsPending,
  onGenerateProduct,
  onGeneratePlatformData,
}: {
  title: string;
  subtitle?: string;
  statusLabel?: string;
  updatedAtLabel?: string;
  taskId: string;
  layerActionsPending: boolean;
  onGenerateProduct: () => void;
  onGeneratePlatformData: () => void;
}) {
  return (
    <section className="min-w-0 border-b border-border pb-5">
      <div className="flex min-w-0 flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <Link
              className="text-sm font-medium text-muted-foreground transition hover:text-foreground"
              href="/listing-kits/canonical-products"
            >
              返回商品中心
            </Link>
            {statusLabel ? <Badge variant="neutral">{statusLabel}</Badge> : null}
            {updatedAtLabel ? (
              <span className="text-xs text-muted-foreground">最近更新 {updatedAtLabel}</span>
            ) : null}
          </div>
          <h1 className="mt-3 break-words text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
            {title}
          </h1>
          {subtitle ? (
            <p className="mt-2 break-words text-sm text-muted-foreground">{subtitle}</p>
          ) : null}
        </div>

        <div className="flex flex-wrap gap-2">
          <Button asChild variant="outline">
            <Link href={`/listing-kits/${taskId}/status`}>查看执行记录</Link>
          </Button>
          <Button
            disabled={layerActionsPending}
            onClick={onGenerateProduct}
            type="button"
            variant="secondary"
          >
            AI 生成商品
          </Button>
          <Button
            disabled={layerActionsPending}
            onClick={onGeneratePlatformData}
            type="button"
          >
            生成平台资料
          </Button>
        </div>
      </div>
    </section>
  );
}

function countIssueSeverity(
  issues: readonly ProductWorkspaceReviewIssue[],
  severity: ProductWorkspaceReviewIssue["severity"],
) {
  return issues.filter((issue) => issue.severity === severity).length;
}

function platformStatusForTask(
  platform: string,
  selectedPlatform: string | undefined,
  task?: ListingKitTaskResult | null,
): "ready" | "processing" | "attention" | "failed" | "idle" {
  if (platform !== selectedPlatform) {
    return "idle";
  }
  if (task?.status === "failed") {
    return "failed";
  }
  if ((task?.result?.summary?.blocking_count ?? 0) > 0 || task?.status === "needs_review") {
    return "attention";
  }
  if (task?.status === "processing") {
    return "processing";
  }
  if (task?.status === "completed") {
    return "ready";
  }
  return "idle";
}

function isProductSectionKey(value: string): value is ProductWorkspaceSectionKey {
  return [
    "overview",
    "images",
    "basic",
    "sku",
    "specs",
    "attributes",
    "description",
  ].includes(value);
}

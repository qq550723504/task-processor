"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { LoaderCircle } from "lucide-react";

import { PlatformCardRail } from "@/components/listingkit/shared/platform-card-rail";
import { PreviewCanvas } from "@/components/listingkit/shared/preview-canvas";
import { RecoveryActionList } from "@/components/listingkit/review/recovery-action-list";
import { ReviewSectionTabs } from "@/components/listingkit/review/review-section-tabs";
import { ReviewToolbar } from "@/components/listingkit/review/review-toolbar";
import { ScenePresetPanel } from "@/components/listingkit/review/scene-preset-panel";
import { SheinCategoryReviewCard } from "@/components/listingkit/shein/shein-category-review-card";
import { SheinAttributeReviewCard } from "@/components/listingkit/shein/shein-attribute-review-card";
import { SheinDataImageGallery } from "@/components/listingkit/shein/shein-data-image-gallery";
import {
  SheinFlowNav,
  type SheinFlowStep,
} from "@/components/listingkit/shein/shein-flow-nav";
import { SheinSaleAttributeReviewCard } from "@/components/listingkit/shein/shein-sale-attribute-review-card";
import { SheinSubmitReadinessPanel } from "@/components/listingkit/shein/shein-submit-readiness-panel";
import { SheinSourceProductPanel } from "@/components/listingkit/shein/shein-source-product-panel";
import {
  collectSheinPreviewImages,
  type SheinPreviewImage,
} from "@/components/listingkit/shein/shein-preview-image";
import {
  canSelectSheinReadinessItem,
  isSheinWorkspaceActionKey,
  sheinWorkspaceTargetIdForKey,
} from "@/components/listingkit/shein/shein-workspace-actions";
import { SlotNavigationList } from "@/components/listingkit/review/slot-navigation-list";
import { ReviewReasonsCard } from "@/components/listingkit/review/review-reasons-card";
import { TaskStatusPanel } from "@/components/listingkit/tasks/task-status-panel";
import { TaskProgressNotice } from "@/components/listingkit/tasks/task-progress-notice";
import { WorkspacePreviewSuggestionCard } from "@/components/listingkit/workspace/workspace-preview-suggestion";
import { deriveWorkspacePreviewSuggestion } from "@/components/listingkit/workspace/workspace-preview-routing";
import { resolveWorkspaceScenePreset } from "@/components/listingkit/workspace/workspace-scene-preset";
import {
  deriveTaskPreviewEmptyState,
  shouldSuppressResolvedActionSummary,
} from "@/components/listingkit/tasks/task-status-display";
import { WorkspaceHeader } from "@/components/listingkit/workspace/workspace-header";
import { WorkspaceOverviewPanel } from "@/components/listingkit/workspace/workspace-overview-panel";
import { regenerateSheinDataImage } from "@/lib/api/shein-image-regeneration";
import {
  deriveRecoveryNavigationTarget,
  pickWorkspaceResolvedActionSummary,
} from "@/components/listingkit/workspace/workspace-action-routing";
import { shouldSyncPlatformOnRecovery } from "@/components/listingkit/workspace/workspace-recovery-routing";
import { buildWorkspaceSearch } from "@/components/listingkit/workspace/workspace-routing";
import { EmptyState } from "@/components/shared/empty-state";
import { ApiError } from "@/lib/api/client";
import { useExecuteAction } from "@/lib/query/use-action";
import { useDispatchNavigation } from "@/lib/query/use-dispatch";
import { useListingKitPreview } from "@/lib/query/use-preview";
import { useReviewPreview } from "@/lib/query/use-review-preview";
import { useReviewSession } from "@/lib/query/use-review-session";
import { useListingKitTaskResult } from "@/lib/query/use-task-result";
import { useApplyRevision } from "@/lib/query/use-apply-revision";
import { useSubmitTask } from "@/lib/query/use-submit-task";
import { useClearSheinResolutionCache } from "@/lib/query/use-shein-resolution-cache";
import type {
  ActionExecutionRequest,
  NavigationTarget,
  QueueQuery,
  RecoveryDescriptor,
  ResolvedActionSummary,
  ReviewSection,
  ReviewSlot,
  SheinReadinessItem,
  SheinResolvedAttribute,
  ToolbarAction,
} from "@/lib/types/listingkit";
import { toImageProxyUrl } from "@/lib/utils/image-proxy-url";

function queryFromSearchParams(searchParams: URLSearchParams): QueueQuery {
  return {
    platform: searchParams.get("platform") ?? undefined,
    slot: searchParams.get("slot") ?? undefined,
    preview_capability: searchParams.get("preview_capability") ?? undefined,
    response_mode: searchParams.get("response_mode") ?? undefined,
  };
}

function submitErrorMessage(error: unknown) {
  if (!error) {
    return null;
  }
  if (error instanceof ApiError) {
    const payload = error.payload;
    if (payload && typeof payload === "object" && "message" in payload) {
      const message = (payload as { message?: unknown }).message;
      if (typeof message === "string" && message.trim()) {
        return message;
      }
    }
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

export function WorkspaceScreen({ taskId }: { taskId: string }) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [selectedSheinImageUrl, setSelectedSheinImageUrl] = useState<string>();
  const [regeneratingSheinImage, setRegeneratingSheinImage] = useState(false);
  const [sheinImageRegenerationError, setSheinImageRegenerationError] =
    useState<string | null>(null);
  const [sheinSubmitAction, setSheinSubmitAction] = useState<
    "publish" | "save_draft" | null
  >(null);
  const baseQuery = useMemo(
    () => queryFromSearchParams(searchParams),
    [searchParams],
  );

  const preview = useListingKitPreview(taskId);
  const taskResult = useListingKitTaskResult(taskId);
  const session = useReviewSession(taskId, baseQuery);
  const focusedPreviewQuery =
    session.data?.session?.focused_target?.navigation_target?.preview_query;
  const reviewPreview = useReviewPreview(
    taskId,
    focusedPreviewQuery ?? baseQuery,
    Boolean(focusedPreviewQuery ?? baseQuery.slot ?? baseQuery.platform),
  );
  const dispatch = useDispatchNavigation(taskId, baseQuery);
  const action = useExecuteAction(taskId, baseQuery);
  const applyRevision = useApplyRevision(taskId);
  const submitTask = useSubmitTask(taskId);
  const clearSheinResolutionCache = useClearSheinResolutionCache(taskId);

  const sessionData = session.data?.session;
  const platformCards =
    sessionData?.platform_cards ?? preview.data?.overview?.platform_cards ?? [];
  const focusedPreview =
    reviewPreview.data?.preview ?? sessionData?.focused_render_preview;
  const selectedPlatform = sessionData?.selected_platform ?? preview.data?.selected_platform;
  const focusedScenePreset = resolveWorkspaceScenePreset({
    reviewPreviewPreset: reviewPreview.data?.scene_preset,
    focusedScenePreset: sessionData?.focused_scene_preset,
    previewScenePresets: {
      amazon: preview.data?.amazon?.scene_presets,
      shein: preview.data?.shein?.scene_presets,
      temu: preview.data?.temu?.scene_presets,
      walmart: preview.data?.walmart?.scene_presets,
    },
    queueItems: sessionData?.queue?.items,
    selectedPlatform,
    selectedSlot: sessionData?.selected_slot,
    focusedAssetId: focusedPreview?.asset_id,
  });
  const suppressResolvedActionSummary = shouldSuppressResolvedActionSummary(
    taskResult.data,
    {
      hasPreviewSvg: Boolean(focusedPreview?.preview_svg),
      queueTotal: sessionData?.queue?.summary?.total_items ?? 0,
    },
  );
  const resolvedActionSummary = pickWorkspaceResolvedActionSummary(
    sessionData?.overview?.resolved_action_summary ??
      session.data?.resolved_action_summary,
    preview.data?.asset_generation_overview?.resolved_action_summary,
  );
  const previewSuggestion = deriveWorkspacePreviewSuggestion({
    slots: sessionData?.slot_navigation,
    selectedSlot: sessionData?.selected_slot,
    focusedPreview,
  });
  const sheinImages = useMemo(
    () =>
      collectSheinPreviewImages(
        preview.data?.shein,
        taskResult.data?.result?.sds_sync,
      ),
    [preview.data?.shein, taskResult.data?.result?.sds_sync],
  );
  const selectedSheinImage =
    sheinImages.find((image) => image.url === selectedSheinImageUrl) ??
    sheinImages[0];
  const sheinFallbackPreview =
    selectedPlatform === "shein" && !focusedPreview?.asset_url && !focusedPreview?.preview_svg
      ? {
          asset_url: toImageProxyUrl(selectedSheinImage?.url),
          template_label: selectedSheinImage?.label ?? "SHEIN product image",
          asset_id: selectedSheinImage?.id ?? "shein-product-image",
        }
      : undefined;
  const sheinPreviewPayload = preview.data?.shein;
  const sheinBlockingKeys = new Set(
    sheinPreviewPayload?.submit_readiness?.blocking_items?.map((item) => item.key) ?? [],
  );
  const sheinCategoryBlocked =
    sheinBlockingKeys.has("category") || sheinBlockingKeys.has("category_review");
  const sheinAttributeBlocked =
    sheinBlockingKeys.has("attributes") || sheinBlockingKeys.has("attribute_review");
  const sheinSaleAttributeBlocked =
    sheinBlockingKeys.has("sale_attributes") || sheinBlockingKeys.has("variants");
  const sheinPreviewBlocked =
    sheinBlockingKeys.has("images") || sheinBlockingKeys.has("preview_product");
  const sheinReadyStatus = sheinPreviewPayload?.submit_readiness?.status;
  const sheinFlowSteps: SheinFlowStep[] = [
    {
      key: "preview",
      label: "Preview images",
      description: sheinImages.length
        ? `${sheinImages.length} SHEIN data images ready for review.`
        : "检查 SDS 官方渲染图是否已经进入 SHEIN 资料。",
      href: "#shein-preview-images",
      state: sheinPreviewBlocked || !sheinImages.length ? "blocked" : "done",
      actionLabel: "Open images",
    },
    {
      key: "category",
      label: "Confirm category",
      description: "确认 SHEIN 类目和 category path，不使用静态兜底。",
      href: "#shein-category-review-card",
      state: sheinCategoryBlocked ? "blocked" : "done",
      actionLabel: "Review category",
    },
    {
      key: "attributes",
      label: "Confirm attributes",
      description: "补齐普通属性候选值，人工确认后才缓存。",
      href: "#shein-attribute-review-card",
      state: sheinAttributeBlocked ? "blocked" : "done",
      actionLabel: "Review attributes",
    },
    {
      key: "sale-attributes",
      label: "Confirm sale attributes",
      description: "检查颜色、尺寸等销售属性映射。",
      href: "#shein-sale-attribute-review-card",
      state: sheinSaleAttributeBlocked ? "blocked" : "done",
      actionLabel: "Review sale attrs",
    },
    {
      key: "submit",
      label: "Submit",
      description: "先上传 SHEIN 图片，再保存草稿或发布。",
      href: "#shein-submit-readiness",
      state:
        sheinReadyStatus === "ready"
          ? "active"
          : sheinReadyStatus === "blocked"
            ? "blocked"
            : "pending",
      actionLabel: "Open submit panel",
    },
  ];

  useEffect(() => {
    const focusedTarget = session.data?.session?.focused_target;
    if (!focusedTarget) {
      return;
    }

    const nextSearch = buildWorkspaceSearch(searchParams.toString(), focusedTarget);
    const currentSearch = searchParams.toString();
    if (nextSearch === currentSearch) {
      return;
    }

    router.replace(
      `/listing-kits/${taskId}/workspace${nextSearch ? `?${nextSearch}` : ""}`,
    );
  }, [router, searchParams, session.data?.session?.focused_target, taskId]);

  const handleDispatch = (target?: NavigationTarget | null) => {
    if (!target) return;
    dispatch.mutate(target);
  };

  const handleAction = (
    actionSummary?: ResolvedActionSummary | null,
    request?: ActionExecutionRequest,
  ) => {
    if (request) {
      action.mutate(request);
      return;
    }

    if (actionSummary?.action_target || actionSummary?.action_key) {
      action.mutate({
        action_key: actionSummary.action_key,
        response_mode: "patch_only",
        target: actionSummary.action_target,
      });
      return;
    }

    handleDispatch(actionSummary?.navigation_target);
  };

  const handleToolbarAction = (toolbarAction: ToolbarAction) => {
    if (toolbarAction.action_target || toolbarAction.kind === "workflow") {
      action.mutate({
        action_key: toolbarAction.action_target?.action_key,
        response_mode: "patch_only",
        target: toolbarAction.action_target,
      });
      return;
    }

    handleDispatch(
      toolbarAction.navigation_target ?? toolbarAction.target?.navigation_target,
    );
  };

  const handleRecovery = (descriptor: RecoveryDescriptor) => {
    const target = deriveRecoveryNavigationTarget(descriptor);
    if (target) {
      handleDispatch(target);
    }
  };

  const handleApplySuggestedSheinCategory = () => {
    const sheinPreview = preview.data?.shein;
    const current = sheinPreview?.editor_context?.category?.current;
    const suggested = current?.suggested_category;
    const saleCurrent = sheinPreview?.editor_context?.sale_attributes?.current;

    if (!suggested?.category_id) {
      return;
    }

    applyRevision.mutate({
      platform: "shein",
      actor: "workspace",
      reason: "Apply suggested SHEIN category",
      shein: {
        category_resolution: {
          category_id: suggested.category_id,
          category_id_list: suggested.category_id_list,
          product_type_id: suggested.product_type_id,
          top_category_id: suggested.top_category_id,
          matched_path: suggested.matched_path,
          source: suggested.source,
          status: "resolved",
        },
        sale_attribute_resolution: {
          recommend_category_review: false,
          category_review_reason:
            saleCurrent?.category_review_reason,
        },
      },
    });
  };

  const handleConfirmSheinAttributes = (attributes: SheinResolvedAttribute[]) => {
    const current = preview.data?.shein?.editor_context?.attributes?.current;
    if (!current || attributes.length === 0) {
      return;
    }
    const resolvedAttributes = [
      ...(current.resolved_attributes ?? []),
      ...attributes,
    ];
    const selectedIDs = new Set(
      attributes
        .map((attribute) => attribute.attribute_id)
        .filter((attributeID): attributeID is number => Boolean(attributeID)),
    );
    const pendingAttributeCandidates =
      current.pending_attribute_candidates?.filter(
        (candidate) => !selectedIDs.has(candidate.attribute_id ?? 0),
      ) ?? [];
    const recommendedAttributeCandidates =
      current.recommended_attribute_candidates?.filter(
        (candidate) => !selectedIDs.has(candidate.attribute_id ?? 0),
      ) ?? [];
    const pendingAttributes =
      current.pending_attributes?.filter((attribute) => {
        const matchingCandidate = current.pending_attribute_candidates?.find(
          (candidate) => candidate.name === attribute.name,
        );
        return !matchingCandidate?.attribute_id || !selectedIDs.has(matchingCandidate.attribute_id);
      }) ?? [];

    applyRevision.mutate({
      platform: "shein",
      actor: "workspace",
      reason: "Apply SHEIN attribute candidate selections",
      shein: {
        attribute_resolution: {
          status: pendingAttributeCandidates.length === 0 ? "resolved" : "partial",
          source: "manual_review",
          category_id: preview.data?.shein?.category_id,
          template_count: current.template_count,
          resolved_count: resolvedAttributes.length,
          unresolved_count: pendingAttributeCandidates.length,
          resolved_attributes: resolvedAttributes,
          pending_attributes: pendingAttributes,
          pending_attribute_candidates: pendingAttributeCandidates,
          recommended_attribute_candidates: recommendedAttributeCandidates,
          review_notes:
            pendingAttributeCandidates.length === 0
              ? ["SHEIN 普通属性已人工确认"]
              : current.review_notes,
        },
      },
    });
  };

  const canSelectSheinBlockingItem = (item: SheinReadinessItem) =>
    canSelectSheinReadinessItem(item);

  const handleSelectSheinBlockingItem = (item: SheinReadinessItem) => {
    if (!isSheinWorkspaceActionKey(item.key)) {
      return;
    }
    const targetId = sheinWorkspaceTargetIdForKey(item.key);
    const card = document.getElementById(targetId);
    if (!card) {
      return;
    }
    card.scrollIntoView({ behavior: "smooth", block: "start" });
  };

  const handleRunSheinPrimaryAction = (key?: string | null) => {
    if (!isSheinWorkspaceActionKey(key)) {
      return;
    }
    const card = document.getElementById(sheinWorkspaceTargetIdForKey(key));
    if (!card) {
      return;
    }
    card.scrollIntoView({ behavior: "smooth", block: "start" });
  };

  const handleSubmitShein = () => {
    setSheinSubmitAction("publish");
    submitTask.mutate(
      {
        platform: "shein",
        action: "publish",
      },
      {
        onSettled: () => setSheinSubmitAction(null),
      },
    );
  };

  const handleSaveSheinDraft = () => {
    setSheinSubmitAction("save_draft");
    submitTask.mutate(
      {
        platform: "shein",
        action: "save_draft",
      },
      {
        onSettled: () => setSheinSubmitAction(null),
      },
    );
  };

  const handleRegenerateSheinImage = async (
    image: SheinPreviewImage,
    prompt: string,
  ) => {
    setRegeneratingSheinImage(true);
    setSheinImageRegenerationError(null);
    try {
      const response = await regenerateSheinDataImage(taskId, {
        image_url: image.url,
        label: image.label,
        prompt,
      });
      setSelectedSheinImageUrl(response.image?.image_url ?? undefined);
      await Promise.all([preview.refetch(), taskResult.refetch()]);
    } catch (error) {
      setSheinImageRegenerationError(submitErrorMessage(error));
      throw error;
    } finally {
      setRegeneratingSheinImage(false);
    }
  };

  const handlePlatformSelect = (platform: string) => {
    const params = new URLSearchParams(searchParams.toString());
    params.set("platform", platform);
    router.replace(`/listing-kits/${taskId}/workspace?${params.toString()}`);
  };

  if (preview.isLoading || session.isLoading) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <LoaderCircle className="h-8 w-8 animate-spin text-zinc-400" />
      </div>
    );
  }

  if (!preview.data || !sessionData) {
    return (
      <EmptyState
        title="Workspace unavailable"
        description="The task did not return preview and review session data."
      />
    );
  }

  return (
    <div className="min-w-0 space-y-6 overflow-x-hidden">
      <WorkspaceHeader
        title={`Task ${taskId.slice(0, 8)}`}
        subtitle={taskId}
        summary={
          suppressResolvedActionSummary
            ? undefined
            : resolvedActionSummary
        }
        recoverySummary={
          sessionData?.overview?.recovery_summary ??
          session.data?.recovery_summary ??
          preview.data.asset_generation_overview?.recovery_summary
        }
        onSelectAction={(summary) => handleAction(summary)}
        onSelectRecovery={handleRecovery}
      />
      <TaskStatusPanel task={taskResult.data} />
      <ReviewReasonsCard task={taskResult.data} />
      <TaskProgressNotice task={taskResult.data} />
      <WorkspaceOverviewPanel
        overview={sessionData.overview}
        reviewSummary={sessionData.review_summary}
      />

      <PlatformCardRail
        cards={platformCards}
        selectedPlatform={sessionData.selected_platform}
        onSelect={(card) => {
          handleDispatch(
            card.primary_navigation_target ??
              card.resolved_action_summary?.navigation_target,
          );
          handlePlatformSelect(card.platform);
        }}
        onSelectRecovery={(descriptor, card) => {
          handleRecovery(descriptor);
          if (shouldSyncPlatformOnRecovery(descriptor)) {
            handlePlatformSelect(card.platform);
          }
        }}
      />

      {selectedPlatform === "shein" ? (
        <SheinFlowNav
          eyebrow="SHEIN review flow"
          steps={sheinFlowSteps}
          title="Review, repair, then submit"
        />
      ) : null}

      <div className="grid min-w-0 gap-6 md:grid-cols-[minmax(0,1fr)_20rem] 2xl:grid-cols-[minmax(0,1fr)_24rem]">
        <main className="min-w-0 space-y-4">
          <WorkspacePreviewSuggestionCard
            suggestion={previewSuggestion}
            onSelect={(slot) => handleDispatch(slot.review_target?.navigation_target)}
          />
          <ReviewSectionTabs
            sections={sessionData.sections}
            selectedKey={sessionData.focused_section_key}
            onSelect={(section: ReviewSection) =>
              handleDispatch(section.review_target?.navigation_target)
            }
          />
          {selectedPlatform === "shein" ? (
            <div id="shein-source-product" className="scroll-mt-6">
              <SheinSourceProductPanel shein={preview.data?.shein} />
            </div>
          ) : null}
          {selectedPlatform === "shein" ? (
            <div id="shein-preview-images" className="scroll-mt-6">
                <SheinDataImageGallery
                  images={sheinImages}
                  selectedUrl={selectedSheinImage?.url}
                  onSelect={(image) => setSelectedSheinImageUrl(image.url)}
                  onRegenerate={handleRegenerateSheinImage}
                  isRegenerating={regeneratingSheinImage}
                  regenerationError={sheinImageRegenerationError}
                />
            </div>
          ) : null}
          <div id="shein-preview-canvas" className="scroll-mt-6">
            <PreviewCanvas
              preview={sheinFallbackPreview ?? focusedPreview}
              response={reviewPreview.data}
              emptyState={deriveTaskPreviewEmptyState(taskResult.data)}
            />
          </div>
          <SlotNavigationList
            slots={sessionData.slot_navigation}
            selectedSlot={sessionData.selected_slot}
            selectedAssetId={focusedPreview?.asset_id}
            onSelect={(slot: ReviewSlot) =>
              handleDispatch(slot.review_target?.navigation_target)
            }
          />
        </main>

        <aside className="min-w-0 space-y-4 md:sticky md:top-6 md:self-start">
          <ReviewToolbar
            toolbar={reviewPreview.data?.toolbar ?? sessionData.focused_toolbar}
            onAction={handleToolbarAction}
          />
          {selectedPlatform === "shein" ? (
            <div id="shein-submit-readiness" className="scroll-mt-6">
              <SheinSubmitReadinessPanel
                readiness={preview.data?.shein?.submit_readiness}
                checklist={preview.data?.shein?.submit_checklist}
                submission={preview.data?.shein?.submission}
                imageUpload={preview.data?.shein?.image_upload}
                resolutionCache={preview.data?.shein?.resolution_cache}
                workspaceOverview={preview.data?.shein?.workspace_overview}
                canSelectBlockingItem={canSelectSheinBlockingItem}
                onSelectBlockingItem={handleSelectSheinBlockingItem}
                canRunPrimaryAction={isSheinWorkspaceActionKey}
                onRunPrimaryAction={handleRunSheinPrimaryAction}
                canSubmit={preview.data?.shein?.submit_readiness?.ready === true}
                isSubmitting={submitTask.isPending}
                submitAction={sheinSubmitAction}
                submitErrorMessage={submitErrorMessage(submitTask.error)}
                onSubmit={handleSubmitShein}
                onSaveDraft={handleSaveSheinDraft}
                clearingResolutionCacheKind={
                  clearSheinResolutionCache.isPending
                    ? clearSheinResolutionCache.variables
                    : null
                }
                onClearResolutionCache={(kind) =>
                  clearSheinResolutionCache.mutate(kind)
                }
                compact
              />
            </div>
          ) : null}
          <ScenePresetPanel summary={focusedScenePreset} />
          <RecoveryActionList
            descriptors={
              session.data?.recovery_summary?.recommended_descriptors ??
              sessionData.overview?.recovery_summary?.recommended_descriptors
            }
            onSelect={handleRecovery}
          />
        </aside>
      </div>

      {selectedPlatform === "shein" ? (
        <section className="space-y-3">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.24em] text-zinc-500">
              SHEIN review details
            </p>
            <h2 className="mt-1 text-xl font-semibold tracking-tight text-zinc-950">
              Category and attribute mapping
            </h2>
          </div>
          <div className="grid min-w-0 gap-4 xl:grid-cols-3">
            <div id="shein-category-review-card" className="min-w-0">
              <SheinCategoryReviewCard
                editorContext={preview.data?.shein?.editor_context}
                isApplying={applyRevision.isPending}
                onApplySuggestedCategory={handleApplySuggestedSheinCategory}
              />
            </div>
            <div id="shein-attribute-review-card" className="min-w-0">
              <SheinAttributeReviewCard
                editorContext={preview.data?.shein?.editor_context}
                isApplying={applyRevision.isPending}
                onConfirmAttributes={handleConfirmSheinAttributes}
              />
            </div>
            <div id="shein-sale-attribute-review-card" className="min-w-0">
              <SheinSaleAttributeReviewCard
                editorContext={preview.data?.shein?.editor_context}
              />
            </div>
          </div>
        </section>
      ) : null}
    </div>
  );
}

"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
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
import { SheinFinalReviewPanel } from "@/components/listingkit/shein/shein-final-review-panel";
import { SheinSaleAttributeReviewCard } from "@/components/listingkit/shein/shein-sale-attribute-review-card";
import { SheinSubmitReadinessPanel } from "@/components/listingkit/shein/shein-submit-readiness-panel";
import { SheinSubmissionTimeline } from "@/components/listingkit/shein/shein-submission-timeline";
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
import type { UpdateSheinFinalDraftRequest } from "@/lib/api/shein-final-draft";
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
import { useUpdateSheinFinalDraft } from "@/lib/query/use-shein-final-draft";
import { useClearSheinResolutionCache } from "@/lib/query/use-shein-resolution-cache";
import type {
  ActionExecutionRequest,
  NavigationTarget,
  QueueQuery,
  RecoveryDescriptor,
  ResolvedActionSummary,
  ReviewSection,
  ReviewSlot,
  SheinEditorContext,
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

function hasSheinCategoryReviewSignal(editorContext?: SheinEditorContext | null) {
  const currentCategory = editorContext?.category?.current;
  const currentSale = editorContext?.sale_attributes?.current;
  const revisionSale =
    editorContext?.revision_skeleton?.shein?.sale_attribute_resolution;

  return Boolean(
    currentCategory?.suggested_category?.category_id ||
      currentSale?.recommend_category_review ||
      revisionSale?.recommend_category_review,
  );
}

function hasSheinAttributeReviewSignal(editorContext?: SheinEditorContext | null) {
  const current = editorContext?.attributes?.current;
  return Boolean(
    current?.status ||
      current?.review_notes?.length ||
      current?.resolved_attributes?.length ||
      current?.pending_attribute_candidates?.length ||
      current?.recommended_attribute_candidates?.length,
  );
}

function hasSheinSaleAttributeReviewSignal(
  editorContext?: SheinEditorContext | null,
) {
  const current = editorContext?.sale_attributes?.current;
  return Boolean(
    current?.status ||
      current?.review_notes?.length ||
      current?.skc_attributes?.length ||
      current?.sku_attributes?.length ||
      current?.candidates?.length,
  );
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
  const [sheinFinalDraftMessage, setSheinFinalDraftMessage] = useState<
    string | null
  >(null);
  const [sheinFinalDraftError, setSheinFinalDraftError] = useState<
    string | null
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
  const updateSheinFinalDraft = useUpdateSheinFinalDraft(taskId);
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
  const sheinEditorContext = sheinPreviewPayload?.editor_context;
  const showSheinCategoryReview =
    hasSheinCategoryReviewSignal(sheinEditorContext);
  const showSheinAttributeReview =
    hasSheinAttributeReviewSignal(sheinEditorContext);
  const showSheinSaleAttributeReview =
    hasSheinSaleAttributeReviewSignal(sheinEditorContext);
  const showSheinReviewDetails =
    showSheinCategoryReview ||
    showSheinAttributeReview ||
    showSheinSaleAttributeReview;
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
  const isSheinFinalReviewMode =
    selectedPlatform === "shein" &&
    searchParams.get("section_key") === "final_review";
  const sheinFlowSteps: SheinFlowStep[] = [
    {
      key: "preview",
      label: "检查图片",
      description: sheinImages.length
        ? `${sheinImages.length} SHEIN data images ready for review.`
        : "检查 SDS 官方渲染图是否已经进入 SHEIN 资料。",
      href: "#shein-preview-images",
      state: sheinPreviewBlocked || !sheinImages.length ? "blocked" : "done",
      actionLabel: "查看图片",
    },
    {
      key: "category",
      label: "确认类目",
      description: "确认 SHEIN 类目和 category path，不使用静态兜底。",
      href: "#shein-category-review-card",
      state: sheinCategoryBlocked ? "blocked" : "done",
      actionLabel: "确认类目",
    },
    {
      key: "attributes",
      label: "确认普通属性",
      description: "补齐普通属性候选值，人工确认后才缓存。",
      href: "#shein-attribute-review-card",
      state: sheinAttributeBlocked ? "blocked" : "done",
      actionLabel: "确认属性",
    },
    {
      key: "sale-attributes",
      label: "确认销售属性",
      description: "检查颜色、尺寸等销售属性映射。",
      href: "#shein-sale-attribute-review-card",
      state: sheinSaleAttributeBlocked ? "blocked" : "done",
      actionLabel: "确认销售属性",
    },
    {
      key: "submit",
      label: "提交",
      description: "先上传 SHEIN 图片，再保存草稿或发布。",
      href: `/listing-kits/${taskId}/workspace?platform=shein&section_key=final_review`,
      state:
        sheinReadyStatus === "ready"
          ? "active"
          : sheinReadyStatus === "blocked"
            ? "blocked"
            : "pending",
      actionLabel: "打开最终确认",
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
    openSheinAdvancedDetailsForTarget(targetId);
    const card =
      item.key === "attributes" || item.key === "attribute_review"
        ? document.getElementById("shein-attribute-required-group") ??
          document.getElementById(targetId)
        : item.key === "sale_attributes" || item.key === "variants"
          ? document.getElementById("shein-sale-attribute-unresolved-group") ??
            document.getElementById(targetId)
        : document.getElementById(targetId);
    if (!card) {
      return;
    }
    card.scrollIntoView({ behavior: "smooth", block: "start" });
  };

  const handleRunSheinPrimaryAction = (key?: string | null) => {
    if (!isSheinWorkspaceActionKey(key)) {
      return;
    }
    const targetId = sheinWorkspaceTargetIdForKey(key);
    openSheinAdvancedDetailsForTarget(targetId);
    const card =
      key === "attributes" || key === "attribute_review"
        ? document.getElementById("shein-attribute-required-group") ??
          document.getElementById(targetId)
        : key === "sale_attributes" || key === "variants"
          ? document.getElementById("shein-sale-attribute-unresolved-group") ??
            document.getElementById(targetId)
        : document.getElementById(targetId);
    if (!card) {
      return;
    }
    card.scrollIntoView({ behavior: "smooth", block: "start" });
  };

  const openSheinAdvancedDetailsForTarget = (targetId: string) => {
    if (
      targetId !== "shein-category-review-card" &&
      targetId !== "shein-attribute-review-card" &&
      targetId !== "shein-sale-attribute-review-card"
    ) {
      return;
    }
    const details = document.getElementById("shein-advanced-review-details");
    if (details instanceof HTMLDetailsElement) {
      details.open = true;
    }
  };

  const handleSaveSheinFinalDraft = (
    payload: UpdateSheinFinalDraftRequest,
    successMessage = "Final SHEIN draft saved.",
  ) => {
    setSheinFinalDraftMessage(null);
    setSheinFinalDraftError(null);
    updateSheinFinalDraft.mutate(payload, {
      onSuccess: () => setSheinFinalDraftMessage(successMessage),
      onError: (error) => setSheinFinalDraftError(submitErrorMessage(error)),
    });
  };

  const handleSubmitShein = (actionType: "publish" | "save_draft" = "publish") => {
    const confirmed = window.confirm(
      actionType === "publish"
        ? "确认要直接发布到 SHEIN 吗？系统会先上传最终图片，然后提交商品资料。"
        : "确认要保存到 SHEIN 草稿箱吗？系统会先上传最终图片。",
    );
    if (!confirmed) {
      return;
    }
    setSheinSubmitAction(actionType);
    submitTask.mutate(
      {
        platform: "shein",
        action: actionType,
        confirmed_final: true,
      },
      {
        onSettled: () => setSheinSubmitAction(null),
      },
    );
  };

  const handleSaveSheinDraft = () => {
    handleSubmitShein("save_draft");
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
          title="先审核修复，再提交"
        />
      ) : null}

      {isSheinFinalReviewMode ? (
        <section className="grid min-w-0 items-start gap-6 lg:grid-cols-[minmax(0,1fr)_24rem] 2xl:grid-cols-[minmax(0,1fr)_26rem]">
          <main className="min-w-0 space-y-4">
            <div className="rounded-[1.75rem] border border-zinc-200 bg-white p-5 shadow-sm">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p className="text-[11px] font-semibold uppercase tracking-[0.26em] text-zinc-500">
                    Final review mode
                  </p>
                  <h2 className="mt-1 text-2xl font-semibold tracking-tight text-zinc-950">
                    确认图片、价格和 SKU 后再提交
                  </h2>
                  <p className="mt-2 max-w-3xl text-sm leading-6 text-zinc-600">
                    这里是客户提交前的主视图。只保留最终会影响 SHEIN 提交的数据；如果有阻断项，点击“去处理”回到对应修复卡片。
                  </p>
                </div>
                <Link
                  className="inline-flex h-10 items-center justify-center rounded-xl bg-white px-4 text-sm font-medium text-zinc-900 ring-1 ring-zinc-200 transition hover:bg-zinc-100"
                  href={`/listing-kits/${taskId}/workspace?platform=shein&section_key=general_review`}
                >
                  打开完整审核
                </Link>
              </div>
            </div>

            <div id="shein-preview-images" className="scroll-mt-6">
              <SheinDataImageGallery
                images={sheinImages}
                finalImages={preview.data?.shein?.final_review?.images}
                isSavingControls={updateSheinFinalDraft.isPending}
                saveErrorMessage={sheinFinalDraftError}
                saveMessage={sheinFinalDraftMessage}
                selectedUrl={selectedSheinImage?.url}
                onSelect={(image) => setSelectedSheinImageUrl(image.url)}
                onSaveImageControls={(payload) =>
                  handleSaveSheinFinalDraft(
                    payload,
                    "图片设置已保存，最终提交会使用当前排序和角色。",
                  )
                }
                onRegenerate={handleRegenerateSheinImage}
                isRegenerating={regeneratingSheinImage}
                regenerationError={sheinImageRegenerationError}
              />
            </div>

            <div id="shein-final-review" className="scroll-mt-6">
              <SheinFinalReviewPanel
                shein={preview.data?.shein}
                isSaving={updateSheinFinalDraft.isPending}
                isSubmitting={submitTask.isPending}
                saveErrorMessage={sheinFinalDraftError}
                saveMessage={sheinFinalDraftMessage}
                submitAction={sheinSubmitAction}
                submitErrorMessage={submitErrorMessage(submitTask.error)}
                canSelectBlockingItem={canSelectSheinBlockingItem}
                onSaveFinalDraft={(payload) =>
                  handleSaveSheinFinalDraft(
                    payload,
                    "最终草稿已确认。资料就绪后可以保存草稿或发布。",
                  )
                }
                onSelectBlockingItem={handleSelectSheinBlockingItem}
                onSubmit={handleSubmitShein}
              />
            </div>
          </main>

          <aside className="min-w-0 space-y-4 md:sticky md:top-6 md:self-start">
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
                canSubmit={
                  preview.data?.shein?.submit_readiness?.ready === true &&
                  preview.data?.shein?.final_review?.confirmed === true
                }
                isSubmitting={submitTask.isPending}
                submitAction={sheinSubmitAction}
                submitErrorMessage={submitErrorMessage(submitTask.error)}
                onSubmit={() => handleSubmitShein("publish")}
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
            <SheinSubmissionTimeline
              events={preview.data?.shein?.submission_events}
            />
          </aside>
        </section>
      ) : (
      <div className="grid min-w-0 items-start gap-6 lg:grid-cols-[minmax(0,1fr)_21rem] 2xl:grid-cols-[minmax(0,1fr)_24rem]">
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
                  finalImages={preview.data?.shein?.final_review?.images}
                  isSavingControls={updateSheinFinalDraft.isPending}
                  saveErrorMessage={sheinFinalDraftError}
                  saveMessage={sheinFinalDraftMessage}
                  selectedUrl={selectedSheinImage?.url}
                  onSelect={(image) => setSelectedSheinImageUrl(image.url)}
                  onSaveImageControls={(payload) =>
                    handleSaveSheinFinalDraft(
                      payload,
                      "图片设置已保存，最终提交会使用当前排序和角色。",
                    )
                  }
                  onRegenerate={handleRegenerateSheinImage}
                  isRegenerating={regeneratingSheinImage}
                  regenerationError={sheinImageRegenerationError}
                />
            </div>
          ) : null}
          {selectedPlatform === "shein" ? (
            <div id="shein-final-review" className="scroll-mt-6">
              <SheinFinalReviewPanel
                shein={preview.data?.shein}
                isSaving={updateSheinFinalDraft.isPending}
                isSubmitting={submitTask.isPending}
                saveErrorMessage={sheinFinalDraftError}
                saveMessage={sheinFinalDraftMessage}
                submitAction={sheinSubmitAction}
                submitErrorMessage={submitErrorMessage(submitTask.error)}
                canSelectBlockingItem={canSelectSheinBlockingItem}
                onSaveFinalDraft={(payload) =>
                  handleSaveSheinFinalDraft(
                    payload,
                    "最终草稿已确认。资料就绪后可以保存草稿或发布。",
                  )
                }
                onSelectBlockingItem={handleSelectSheinBlockingItem}
                onSubmit={handleSubmitShein}
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
                canSubmit={
                  preview.data?.shein?.submit_readiness?.ready === true &&
                  preview.data?.shein?.final_review?.confirmed === true
                }
                isSubmitting={submitTask.isPending}
                submitAction={sheinSubmitAction}
                submitErrorMessage={submitErrorMessage(submitTask.error)}
                onSubmit={() => handleSubmitShein("publish")}
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
          {selectedPlatform === "shein" ? (
            <SheinSubmissionTimeline
              events={preview.data?.shein?.submission_events}
            />
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
      )}

      {selectedPlatform === "shein" && showSheinReviewDetails && !isSheinFinalReviewMode ? (
        <details
          className="group rounded-[1.75rem] border border-zinc-200 bg-white p-5 shadow-sm"
          id="shein-advanced-review-details"
        >
          <summary className="flex cursor-pointer list-none flex-wrap items-start justify-between gap-3">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.24em] text-zinc-500">
                高级详情
              </p>
              <h2 className="mt-1 text-xl font-semibold tracking-tight text-zinc-950">
                类目和属性映射诊断
              </h2>
              <p className="mt-1 text-sm leading-6 text-zinc-600">
                这里是内部排查信息，默认收起。需要处理类目、普通属性或销售属性时再展开。
              </p>
            </div>
            <span className="rounded-full border border-zinc-200 bg-zinc-50 px-3 py-1 text-xs font-semibold text-zinc-600">
              点击展开
            </span>
          </summary>
          <div className="mt-5 grid min-w-0 items-start gap-4 xl:grid-cols-2">
            {showSheinCategoryReview ? (
              <div id="shein-category-review-card" className="min-w-0">
                <SheinCategoryReviewCard
                  editorContext={preview.data?.shein?.editor_context}
                  isApplying={applyRevision.isPending}
                  onApplySuggestedCategory={handleApplySuggestedSheinCategory}
                />
              </div>
            ) : null}
            {showSheinAttributeReview ? (
              <div id="shein-attribute-review-card" className="min-w-0">
                <SheinAttributeReviewCard
                  editorContext={preview.data?.shein?.editor_context}
                  isApplying={applyRevision.isPending}
                  onConfirmAttributes={handleConfirmSheinAttributes}
                />
              </div>
            ) : null}
            {showSheinSaleAttributeReview ? (
              <div id="shein-sale-attribute-review-card" className="min-w-0">
                <SheinSaleAttributeReviewCard
                  editorContext={preview.data?.shein?.editor_context}
                />
              </div>
            ) : null}
          </div>
        </details>
      ) : null}
    </div>
  );
}

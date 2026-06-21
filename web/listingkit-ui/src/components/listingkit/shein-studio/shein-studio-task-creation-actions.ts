import type { MutableRefObject } from "react";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import {
  evaluateImportedGalleryDesigns,
  flattenItemizedBatchDesigns,
  getApprovedItemizedBatchDesignIDs,
  hasInFlightItemizedBatchGeneration,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import {
  createSheinStudioBatchTasks,
  type SheinStudioBatchTaskCreationResult,
} from "@/lib/api/shein-studio-batches";
import { formatSubscriptionApiError } from "@/lib/api/subscription";
import {
  createGroupedSheinReviewTasks,
  type GroupedSheinTaskCreationWarning,
  createSheinReviewTasks,
} from "@/lib/shein-studio/create-review-tasks";
import { useToast } from "@/components/providers/toast-provider";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  GroupedSDSSelectionEligibility,
  SDSBaselineStatus,
} from "@/lib/types/sds-baseline";
import type {
  SheinStudioBatchDetail,
  SheinStudioCreatedTask,
  SheinStudioFailedTask,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioRejectedTask,
  SheinStudioSelectedSDSImage,
} from "@/lib/types/shein-studio";

type PersistDraft = (
  overrides?: Partial<{
    designs: SheinStudioGeneratedDesign[];
    selectedIds: string[];
    createdTasks: SheinStudioCreatedTask[];
  }>,
  options?: {
    navigationTriggered?: boolean;
    source?: string;
    signal?: AbortSignal;
  },
) => Promise<unknown>;

export function useSheinStudioTaskCreationAction({
  activeSelection,
  designs,
  groupedImageMode,
  imageStrategy,
  navigateToStep,
  persistDraft,
  productImageCount,
  productImagePrompt,
  productImagePrompts,
  prompt,
  renderSizeImagesWithSds,
  selectedIds,
  selectedSdsImages,
  groupedSelections,
  activeSelectionBaselineStatus,
  activeSelectionBaselineReason,
  setCreatedTasks,
  setCreatingError,
  setCreatingMessage,
  setCreatingWarning,
  setGalleryRatioCheck,
  setIsCreatingTasks,
  sheinStoreId,
  hasLocalWorkflowStateRef,
  itemizedBatchContext,
}: {
  activeSelection?: SDSProductVariantSelection;
  designs: SheinStudioGeneratedDesign[];
  groupedImageMode: SheinStudioGroupedImageMode;
  imageStrategy: SheinStudioImageStrategy;
  navigateToStep: (step: SheinStudioStepKey) => void;
  persistDraft: PersistDraft;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  prompt: string;
  renderSizeImagesWithSds: boolean;
  selectedIds: string[];
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  groupedSelections: GroupedSDSSelectionEligibility[];
  activeSelectionBaselineStatus: SDSBaselineStatus;
  activeSelectionBaselineReason: string;
  setCreatedTasks: (value: SheinStudioCreatedTask[]) => void;
  setCreatingError: (value: string) => void;
  setCreatingMessage: (value: string) => void;
  setCreatingWarning: (value: string) => void;
  setGalleryRatioCheck: (
    value: ReturnType<typeof evaluateImportedGalleryDesigns>,
  ) => void;
  setIsCreatingTasks: (value: boolean) => void;
  sheinStoreId: string;
  hasLocalWorkflowStateRef: MutableRefObject<boolean>;
  itemizedBatchContext?: {
    batchId: string;
    tenantId?: string;
    detail: SheinStudioBatchDetail;
    onCreated: (result: SheinStudioBatchTaskCreationResult) => void;
  };
}) {
  const toast = useToast();

  async function handleCreateTasks() {
    if (!activeSelection?.variantId) {
      setCreatingError("请先选择 SDS 变体。");
      setCreatingWarning("");
      toast.error("无法创建 SHEIN 资料", "请先选择 SDS 变体。");
      return;
    }
    if (!sheinStoreId.trim()) {
      setCreatingError("请先选择批次店铺。");
      setCreatingWarning("");
      toast.error("无法创建 SHEIN 资料", "请先选择批次店铺。");
      return;
    }
    const approvedDesignIdsForTaskCreation = itemizedBatchContext
      ? getApprovedItemizedBatchDesignIDs(itemizedBatchContext.detail)
      : selectedIds;
    const approvedDesignIDSet = new Set(approvedDesignIdsForTaskCreation);
    const candidateDesigns = itemizedBatchContext
      ? flattenItemizedBatchDesigns(itemizedBatchContext.detail)
      : designs;
    const approved = candidateDesigns.filter((design) =>
      approvedDesignIDSet.has(design.id),
    );
    if (approved.length === 0) {
      setCreatingError("请至少批准 1 个款式后再创建 SHEIN 任务。");
      setCreatingWarning("");
      toast.error("无法创建 SHEIN 资料", "请至少批准 1 个款式后再创建 SHEIN 任务。");
      return;
    }
    let allowPartialWhileGenerating = false;
    if (itemizedBatchContext) {
      allowPartialWhileGenerating = hasInFlightItemizedBatchGeneration(
        itemizedBatchContext.detail,
      );
      if (
        allowPartialWhileGenerating &&
        !window.confirm(buildInFlightTaskCreationConfirmation(approved.length))
      ) {
        return;
      }
    }
    const latestRatioCheck = evaluateImportedGalleryDesigns(
      approved,
      activeSelection,
    );
    setGalleryRatioCheck(latestRatioCheck);
    if (latestRatioCheck?.status === "blocking") {
      setCreatingError(latestRatioCheck.message);
      setCreatingWarning("");
      toast.error("无法创建 SHEIN 资料", latestRatioCheck.message);
      return;
    }

    setCreatingError("");
    setCreatingMessage("正在开始生成 SHEIN 资料...");
    setCreatingWarning("");
    setIsCreatingTasks(true);
    let keepCreatingState = false;

    try {
      let created: SheinStudioCreatedTask[] = [];
      let reused: SheinStudioCreatedTask[] = [];
      let creationWarnings: GroupedSheinTaskCreationWarning[] = [];
      let batchTaskFailures: SheinStudioFailedTask[] = [];
      let batchTaskRejections: SheinStudioRejectedTask[] = [];

      if (itemizedBatchContext) {
        const batchTenantId = itemizedBatchContext.tenantId?.trim();
        const approvedDesignIds = approvedDesignIdsForTaskCreation;
        const requestOptions = {
          ...(batchTenantId ? { tenantId: batchTenantId } : {}),
          ...(allowPartialWhileGenerating
            ? { allowPartialWhileGenerating: true }
            : {}),
        };
        const hasRequestOptions = Object.keys(requestOptions).length > 0;
        const result = hasRequestOptions
          ? await createSheinStudioBatchTasks(
              itemizedBatchContext.batchId,
              approvedDesignIds,
              requestOptions,
            )
          : await createSheinStudioBatchTasks(
              itemizedBatchContext.batchId,
              approvedDesignIds,
            );
        created = result.createdTasks;
        reused = result.reusedTasks ?? [];
        batchTaskRejections = result.rejectedTasks ?? [];
        batchTaskFailures = result.failedTasks ?? [];
        itemizedBatchContext.onCreated(result);
        keepCreatingState = result.batch.status === "tasks_creating";
        if (keepCreatingState) {
          setCreatingMessage("已开始在后台创建 SHEIN 资料，可离开当前页面，结果会自动刷新。");
          setCreatingWarning("");
          toast.info(
            "已开始后台创建 SHEIN 资料",
            "可离开当前页面，结果会自动刷新。",
            7000,
          );
          return;
        }
      } else if (groupedSelections.length > 0) {
        const result = await createGroupedSheinReviewTasks({
          prompt,
          groupedImageMode,
          imageStrategy,
          selectedSdsImages,
          productImageCount,
          productImagePrompt,
          productImagePrompts,
          renderSizeImagesWithSds,
          onProgress: setCreatingMessage,
          groups: [
            {
              sheinStoreId,
              selections: [
                {
                  selection: activeSelection,
                  baselineStatus: activeSelectionBaselineStatus,
                  baselineReason: activeSelectionBaselineReason,
                  eligible: true,
                },
              ],
              approvedDesigns: approved,
            },
            ...groupSelectionsByStore(groupedSelections).map((group) => ({
              sheinStoreId: group.sheinStoreId,
              selections: group.items.map((item) => ({
                selection: item.selection,
                baselineStatus: item.baselineStatus,
                baselineReason: item.baselineReason,
                eligible: item.eligible,
                eligibilityReason: item.eligibilityReason,
              })),
              approvedDesigns: approved,
            })),
          ],
        });
        created = result.created;
        creationWarnings = result.warnings;
      } else {
        created = await createSheinReviewTasks({
          prompt,
          sheinStoreId,
          imageStrategy,
          selectedSdsImages,
          productImageCount,
          productImagePrompt,
          productImagePrompts,
          renderSizeImagesWithSds,
          selection: activeSelection,
          approvedDesigns: approved,
          onProgress: setCreatingMessage,
        });
      }

      hasLocalWorkflowStateRef.current = true;
      const availableTasks = [...created, ...reused];
      setCreatedTasks(availableTasks);

      if (batchTaskFailures.length > 0 || batchTaskRejections.length > 0) {
        const failedSummary = buildBatchTaskCreationFailureSummary(
          batchTaskFailures,
          batchTaskRejections,
        );
        setCreatingWarning(availableTasks.length > 0 ? failedSummary : "");
        if (availableTasks.length > 0) {
          setCreatingMessage(
            `已可处理 ${availableTasks.length} 个 SHEIN 资料任务，另有 ${batchTaskRejections.length} 个被拒绝、${batchTaskFailures.length} 个创建失败。`,
          );
          toast.warning(
            "SHEIN 资料已部分创建",
            `可处理 ${availableTasks.length} 个，拒绝 ${batchTaskRejections.length} 个，失败 ${batchTaskFailures.length} 个。`,
            8000,
          );
        } else {
          setCreatingError(failedSummary);
          setCreatingMessage("");
          toast.error("SHEIN 资料创建失败", failedSummary, 8000);
          return;
        }
      } else {
        setCreatingMessage(
          groupedSelections.length > 0
            ? `已为 ${availableTasks.length} 个 SDS 商品生成或复用 SHEIN 资料任务。请在下方打开并审核。`
            : `已生成或复用 ${availableTasks.length} 个 SHEIN 资料任务。请在下方打开并审核。`,
        );
        setCreatingWarning(buildGroupedTaskCreationWarningSummary(creationWarnings));
        toast.success(
          "SHEIN 资料创建完成",
          `共生成或复用 ${availableTasks.length} 个任务。`,
          7000,
        );
      }

      if (availableTasks.length === 0) {
        return;
      }

      navigateToStep("tasks");
      if (!itemizedBatchContext) {
        void persistDraft(
          { createdTasks: availableTasks },
          {
            navigationTriggered: true,
            source: "task_creation_success",
          },
        ).catch(() => undefined);
      }
    } catch (error) {
      const message = formatSubscriptionApiError(error);
      setCreatingError(message);
      setCreatingMessage("");
      setCreatingWarning("");
      toast.error("SHEIN 资料创建失败", message, 8000);
    } finally {
      if (!keepCreatingState) {
        setIsCreatingTasks(false);
      }
    }
  }

  return { handleCreateTasks };
}

function buildInFlightTaskCreationConfirmation(approvedCount: number) {
  return `当前批次仍有图片正在生成。本次只会为当前已批准的 ${approvedCount} 个款式创建 SHEIN 资料，剩余图片生成完成并批准后需要再次创建。是否继续？`;
}

function buildBatchTaskCreationFailureSummary(
  failedTasks: SheinStudioFailedTask[],
  rejectedTasks: SheinStudioRejectedTask[] = [],
) {
  if (failedTasks.length === 0 && rejectedTasks.length === 0) {
    return "";
  }
  const rejectedPreview = rejectedTasks
    .slice(0, 3)
    .map((task) => `${task.title?.trim() || task.designId}: ${task.reasonCode ? `${task.reasonCode} · ` : ""}${task.message ?? "候选不满足创建条件"}`)
    .join("；");
  const failedPreview = failedTasks
    .slice(0, Math.max(0, 3 - rejectedTasks.length))
    .map((task) => `${task.title}: ${task.reasonCode ? `${task.reasonCode} · ` : ""}${task.message}`)
    .join("；");
  const preview = [rejectedPreview, failedPreview].filter(Boolean).join("；");
  const total = rejectedTasks.length + failedTasks.length;
  const suffix = total > 3 ? ` 等 ${total} 个任务` : "";
  if (failedTasks.length === 0) {
    return `部分任务被拒绝：${preview}${suffix}`;
  }
  if (rejectedTasks.length === 0) {
    return `部分任务创建失败：${preview}${suffix}`;
  }
  return `部分任务被拒绝或创建失败：${preview}${suffix}`;
}

function buildGroupedTaskCreationWarningSummary(
  warnings: GroupedSheinTaskCreationWarning[],
) {
  if (warnings.length === 0) {
    return "";
  }
  const labels = warnings.map((warning) => warning.label.trim()).filter(Boolean);
  const preview = labels.slice(0, 5).join("、");
  const suffix =
    labels.length > 5
      ? ` 等 ${labels.length} 款商品`
      : labels.length > 1
        ? ` 共 ${labels.length} 款商品`
        : "";
  return `有 ${warnings.length} 款商品因为没有匹配到自己的款式图而被跳过：${preview}${suffix}。这些商品不会创建错误任务，你可以回到生成区补图后再重试。`;
}

function groupSelectionsByStore(items: GroupedSDSSelectionEligibility[]) {
  const byStore = new Map<
    string,
    { sheinStoreId: string; items: GroupedSDSSelectionEligibility[] }
  >();
  for (const item of items) {
    const key = item.sheinStoreId.trim();
    const existing = byStore.get(key);
    if (existing) {
      existing.items.push(item);
      continue;
    }
    byStore.set(key, { sheinStoreId: key, items: [item] });
  }
  return [...byStore.values()];
}

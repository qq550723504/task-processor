import type { MutableRefObject } from "react";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import { evaluateImportedGalleryDesigns } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
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
import type {
  GroupedSDSSelectionEligibility,
  SDSBaselineStatus,
} from "@/lib/types/sds-baseline";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
  SheinStudioBatchDetail,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
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
    detail: SheinStudioBatchDetail;
    onCreated: (result: SheinStudioBatchTaskCreationResult) => void;
  };
}) {
  async function handleCreateTasks() {
    if (!activeSelection?.variantId) {
      setCreatingError("请先选择 SDS 变体。");
      setCreatingWarning("");
      return;
    }
    if (!sheinStoreId.trim()) {
      setCreatingError("请先选择批次店铺。");
      setCreatingWarning("");
      return;
    }
    const approved = designs.filter((design) => selectedIds.includes(design.id));
    if (approved.length === 0) {
      setCreatingError("请至少批准 1 个款式后再创建 SHEIN 任务。");
      setCreatingWarning("");
      return;
    }
    const latestRatioCheck = evaluateImportedGalleryDesigns(
      approved,
      activeSelection,
    );
    setGalleryRatioCheck(latestRatioCheck);
    if (latestRatioCheck?.status === "blocking") {
      setCreatingError(latestRatioCheck.message);
      setCreatingWarning("");
      return;
    }

    setCreatingError("");
    setCreatingMessage("正在开始生成 SHEIN 资料...");
    setCreatingWarning("");
    setIsCreatingTasks(true);

    try {
      let created: SheinStudioCreatedTask[] = [];
      let creationWarnings: GroupedSheinTaskCreationWarning[] = [];
      if (itemizedBatchContext) {
        const result = await createSheinStudioBatchTasks(
          itemizedBatchContext.batchId,
          approved.map((design) => design.id),
        );
        created = result.createdTasks;
        itemizedBatchContext.onCreated(result);
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
      setCreatedTasks(created);
      setCreatingMessage(
        groupedSelections.length > 0
          ? `已为 ${created.length} 个 SDS 商品生成 SHEIN 资料任务。请在下方打开并审核。`
          : `已生成 ${created.length} 个 SHEIN 资料任务。请在下方打开并审核。`,
      );
      setCreatingWarning(buildGroupedTaskCreationWarningSummary(creationWarnings));
      navigateToStep("tasks");
      void persistDraft(
        { createdTasks: created },
        {
          navigationTriggered: true,
          source: "task_creation_success",
        },
      ).catch(() => undefined);
    } catch (error) {
      setCreatingError(formatSubscriptionApiError(error));
      setCreatingMessage("");
      setCreatingWarning("");
    } finally {
      setIsCreatingTasks(false);
    }
  }

  return { handleCreateTasks };
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
    labels.length > 5 ? ` 等 ${labels.length} 款商品` : labels.length > 1 ? ` 共 ${labels.length} 款商品` : "";
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

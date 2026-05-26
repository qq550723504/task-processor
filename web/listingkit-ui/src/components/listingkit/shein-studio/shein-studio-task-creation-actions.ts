import type { MutableRefObject } from "react";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import { evaluateImportedGalleryDesigns } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import { formatSubscriptionApiError } from "@/lib/api/subscription";
import {
  createGroupedSheinReviewTasks,
  createSheinReviewTasks,
} from "@/lib/shein-studio/create-review-tasks";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
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
  setGalleryRatioCheck,
  setIsCreatingTasks,
  sheinStoreId,
  hasLocalWorkflowStateRef,
}: {
  activeSelection?: SDSProductVariantSelection;
  designs: SheinStudioGeneratedDesign[];
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
  activeSelectionBaselineStatus: "ready" | "missing" | "failed";
  activeSelectionBaselineReason: string;
  setCreatedTasks: (value: SheinStudioCreatedTask[]) => void;
  setCreatingError: (value: string) => void;
  setCreatingMessage: (value: string) => void;
  setGalleryRatioCheck: (
    value: ReturnType<typeof evaluateImportedGalleryDesigns>,
  ) => void;
  setIsCreatingTasks: (value: boolean) => void;
  sheinStoreId: string;
  hasLocalWorkflowStateRef: MutableRefObject<boolean>;
}) {
  async function handleCreateTasks() {
    if (!activeSelection?.variantId) {
      setCreatingError("请先选择 SDS 变体。");
      return;
    }
    const approved = designs.filter((design) => selectedIds.includes(design.id));
    if (approved.length === 0) {
      setCreatingError("请至少批准 1 个款式后再创建 SHEIN 任务。");
      return;
    }
    const latestRatioCheck = evaluateImportedGalleryDesigns(
      approved,
      activeSelection,
    );
    setGalleryRatioCheck(latestRatioCheck);
    if (latestRatioCheck?.status === "blocking") {
      setCreatingError(latestRatioCheck.message);
      return;
    }

    setCreatingError("");
    setCreatingMessage("正在开始生成 SHEIN 资料...");
    setIsCreatingTasks(true);

    try {
      const created =
        groupedSelections.length > 0
          ? await createGroupedSheinReviewTasks({
              prompt,
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
                    },
                  ],
                  designs: approved,
                  selectedIds: approved.map((design) => design.id),
                },
                ...groupSelectionsByStore(groupedSelections).map((group) => ({
                  sheinStoreId: group.sheinStoreId,
                  selections: group.items.map((item) => ({
                    selection: item.selection,
                    baselineStatus: item.baselineStatus,
                    baselineReason: item.baselineReason,
                  })),
                  designs: approved,
                  selectedIds: approved.map((design) => design.id),
                })),
              ],
            })
          : await createSheinReviewTasks({
              prompt,
              sheinStoreId,
              imageStrategy,
              selectedSdsImages,
              productImageCount,
              productImagePrompt,
              productImagePrompts,
              renderSizeImagesWithSds,
              selection: activeSelection,
              designs: approved,
              selectedIds: approved.map((design) => design.id),
              onProgress: setCreatingMessage,
            });
      hasLocalWorkflowStateRef.current = true;
      setCreatedTasks(created);
      setCreatingMessage(
        groupedSelections.length > 0
          ? `已为 ${created.length} 个 SDS 商品生成 SHEIN 资料任务。请在下方打开并审核。`
          : `已生成 ${created.length} 个 SHEIN 资料任务。请在下方打开并审核。`,
      );
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
    } finally {
      setIsCreatingTasks(false);
    }
  }

  return { handleCreateTasks };
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

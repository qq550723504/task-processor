import type { MutableRefObject } from "react";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import { evaluateImportedGalleryDesigns } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import { createSheinReviewTasks } from "@/lib/shein-studio/create-review-tasks";
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
      const created = await createSheinReviewTasks({
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
        `已生成 ${created.length} 个 SHEIN 资料任务。请在下方打开并审核。`,
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
      setCreatingError(
        error instanceof Error ? error.message : "SHEIN 任务创建失败。",
      );
      setCreatingMessage("");
    } finally {
      setIsCreatingTasks(false);
    }
  }

  return { handleCreateTasks };
}

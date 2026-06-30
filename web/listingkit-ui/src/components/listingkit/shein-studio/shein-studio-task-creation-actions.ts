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
  createSheinReviewTasks,
} from "@/lib/shein-studio/create-review-tasks";
import { hydrateSDSVariantSelection } from "@/lib/shein-studio/hydrate-sds-selection";
import {
  buildBatchTaskCreationFailureSummary,
  executeItemizedBatchTaskCreation,
  executeStandaloneTaskCreation,
  resolveTaskCreationStartValidation,
} from "@/lib/shein-studio/task-creation-controller";
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
    selection: SDSProductVariantSelection;
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
  promptMode,
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
  promptMode?: "managed" | "raw";
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
    const startValidation = resolveTaskCreationStartValidation({
      activeSelection,
      activeSelectionBaselineReason,
      activeSelectionBaselineStatus,
      approvedCount: approved.length,
      sheinStoreId,
    });
    if (startValidation) {
      setCreatingError(startValidation.error);
      setCreatingWarning("");
      toast.error("无法创建 SHEIN 资料", startValidation.error);
      return;
    }
    const taskCreationSelection = activeSelection;
    if (!taskCreationSelection) {
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
      setCreatingMessage("正在刷新 SDS 商品资料...");
      const hydratedTaskCreationSelection =
        (await hydrateSDSVariantSelection(taskCreationSelection)) ??
        taskCreationSelection;
      if (itemizedBatchContext) {
        await persistDraft(
          { selection: hydratedTaskCreationSelection },
          {
            source: "task_creation_preflight",
          },
        );
      }
      setCreatingMessage("正在开始生成 SHEIN 资料...");

      let created: SheinStudioCreatedTask[] = [];
      let reused: SheinStudioCreatedTask[] = [];
      let batchTaskFailures: SheinStudioFailedTask[] = [];
      let batchTaskRejections: SheinStudioRejectedTask[] = [];

      if (itemizedBatchContext) {
        const result = await executeItemizedBatchTaskCreation({
          allowPartialWhileGenerating,
          approvedDesignIds: approvedDesignIdsForTaskCreation,
          batchId: itemizedBatchContext.batchId,
          createBatchTasks: createSheinStudioBatchTasks,
          onCreated: itemizedBatchContext.onCreated,
          tenantId: itemizedBatchContext.tenantId,
        });
        created = result.created;
        reused = result.reused;
        batchTaskRejections = result.rejected;
        batchTaskFailures = result.failed;
        keepCreatingState = result.keepCreatingState;
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
      } else {
        const result = await executeStandaloneTaskCreation({
          activeSelection: hydratedTaskCreationSelection,
          activeSelectionBaselineReason,
          activeSelectionBaselineStatus,
          approvedDesigns: approved,
          createGroupedTasks: createGroupedSheinReviewTasks,
          createTasks: createSheinReviewTasks,
          groupedImageMode,
          groupedSelections,
          hasLocalWorkflowStateRef,
          imageStrategy,
          navigateToStep,
          persistDraft,
          productImageCount,
          productImagePrompt,
          productImagePrompts,
          prompt,
          promptMode,
          renderSizeImagesWithSds,
          selectedSdsImages,
          setCreatedTasks,
          setCreatingMessage,
          setCreatingWarning,
          sheinStoreId,
        });
        toast.success(
          "SHEIN 资料创建完成",
          `共生成或复用 ${result.availableTasks.length} 个任务。`,
          7000,
        );
        return;
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
        setCreatingWarning("");
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

import type { MutableRefObject, RefObject } from "react";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import { useSheinStudioTaskCreationAction } from "@/components/listingkit/shein-studio/shein-studio-task-creation-actions";
import type { SheinStudioWorkbenchController } from "@/components/listingkit/shein-studio/shein-studio-workbench-state";
import {
  buildSheinStudioGenerateRequest,
  STUDIO_SESSION_SYNC_TIMEOUT_MS,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import { generateSheinStudioDesigns } from "@/lib/api/shein-studio";
import {
  ensureSheinStudioSession,
  updateSheinStudioSession,
} from "@/lib/api/shein-studio-sessions";
import { formatSubscriptionApiError } from "@/lib/api/subscription";
import {
  buildGroupedGenerationTargets,
  resolveDesignTargetKey,
} from "@/lib/shein-studio/grouped-image-mode";
import { parsePositiveInt } from "@/lib/shein-studio/create-review-tasks";
import { buildSDSProductReferenceImageUrls } from "@/lib/shein-studio/sds-reference-images";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SDSGroupedPromptHistoryEntry,
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
  SheinStudioGroupedWorkspace,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";

type PersistDraft = (
  overrides?: Partial<{
    designs: SheinStudioGeneratedDesign[];
    groups: SheinStudioGroupedWorkspace[];
    selectedIds: string[];
    createdTasks: SheinStudioCreatedTask[];
  }>,
  options?: {
    navigationTriggered?: boolean;
    source?: string;
    signal?: AbortSignal;
  },
) => Promise<unknown>;

type UseSheinStudioDesignActionsParams = {
  activeGroupId: string;
  activeSelection?: SDSProductVariantSelection;
  artworkModel: SheinStudioArtworkModel;
  designs: SheinStudioGeneratedDesign[];
  groups: SheinStudioGroupedWorkspace[];
  groupedImageMode: SheinStudioGroupedImageMode;
  imageStrategy: SheinStudioImageStrategy;
  navigateToStep: (step: SheinStudioStepKey) => void;
  persistDraft: PersistDraft;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  prompt: string;
  promptInputRef: RefObject<HTMLTextAreaElement | null>;
  renderSizeImagesWithSds: boolean;
  selectedIds: string[];
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  groupedSelections: GroupedSDSSelectionEligibility[];
  activeSelectionBaselineStatus: "ready" | "missing" | "failed";
  activeSelectionBaselineReason: string;
  workbench: Pick<SheinStudioWorkbenchController, "setField">;
  sheinStoreId: string;
  styleCount: string;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
  hasLocalWorkflowStateRef: MutableRefObject<boolean>;
};

export function useSheinStudioDesignActions({
  activeGroupId,
  activeSelection,
  artworkModel,
  designs,
  groups,
  groupedImageMode,
  imageStrategy,
  navigateToStep,
  persistDraft,
  productImageCount,
  productImagePrompt,
  productImagePrompts,
  prompt,
  promptInputRef,
  renderSizeImagesWithSds,
  selectedIds,
  selectedSdsImages,
  groupedSelections,
  activeSelectionBaselineStatus,
  activeSelectionBaselineReason,
  workbench,
  sheinStoreId,
  styleCount,
  transparentBackground,
  variationIntensity,
  hasLocalWorkflowStateRef,
}: UseSheinStudioDesignActionsParams) {
  const { handleCreateTasks } = useSheinStudioTaskCreationAction({
    activeSelection,
    designs,
    imageStrategy,
    groupedImageMode,
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
    setCreatedTasks: (value) => workbench.setField("createdTasks", value),
    setCreatingError: (value) => workbench.setField("creatingError", value),
    setCreatingMessage: (value) => workbench.setField("creatingMessage", value),
    setGalleryRatioCheck: (value) =>
      workbench.setField("galleryRatioCheck", value),
    setIsCreatingTasks: (value) => workbench.setField("isCreatingTasks", value),
    sheinStoreId,
    hasLocalWorkflowStateRef,
  });

  function buildNextPromptHistoryGroups() {
    const trimmedPrompt = prompt.trim();
    if (!trimmedPrompt || !activeGroupId) {
      return groups;
    }
    const historyEntry: SDSGroupedPromptHistoryEntry = {
      prompt: trimmedPrompt,
      groupedImageMode,
      createdAt: new Date().toISOString(),
    };
    return groups.map((group) => {
      if (group.id !== activeGroupId) {
        return group;
      }
      const newest = group.promptHistory[0];
      const promptHistory =
        newest?.prompt === historyEntry.prompt &&
        newest?.groupedImageMode === historyEntry.groupedImageMode
          ? group.promptHistory
          : [historyEntry, ...group.promptHistory].slice(0, 5);
      return {
        ...group,
        currentPrompt: trimmedPrompt,
        promptHistory,
        updatedAt: historyEntry.createdAt,
      };
    });
  }

  async function handleGenerate() {
    if (!activeSelection?.variantId) {
      workbench.setField("generationError", "请先选择 SDS 变体。");
      return;
    }
    if (!prompt.trim()) {
      workbench.setField("generationError", "请先填写主题提示词。");
      promptInputRef.current?.scrollIntoView({
        behavior: "smooth",
        block: "center",
      });
      promptInputRef.current?.focus();
      return;
    }

    workbench.setField("generationError", "");
    workbench.setField("generationWarning", "");
    workbench.setField("creatingError", "");
    workbench.setField("creatingMessage", "");
    workbench.setField("createdTasks", []);
    workbench.setField("draftWarning", "");
    workbench.setField("isGenerating", true);
    const nextGroups = buildNextPromptHistoryGroups();
    if (nextGroups !== groups) {
      workbench.setField("groups", nextGroups);
    }

    const sessionSyncPromise = (async () => {
      try {
        const sessionDetail = await ensureSheinStudioSession(activeSelection, {
          timeoutMs: STUDIO_SESSION_SYNC_TIMEOUT_MS,
        });
        const sessionId = sessionDetail?.session?.id ?? "";
        if (!sessionId) {
          return "";
        }
        await updateSheinStudioSession(
          sessionId,
          {
            status: "generating",
            prompt: prompt.trim(),
            styleCount,
            variationIntensity,
            productImageCount,
            productImagePrompt,
            productImagePrompts,
            artworkModel,
            imageStrategy,
            groupedImageMode,
            selectedSdsImages,
            groups: nextGroups,
            transparentBackground,
            renderSizeImagesWithSds,
            sheinStoreId,
            generationError: "",
            approvedDesignIds: [],
            createdTasks: [],
          },
          {
            timeoutMs: STUDIO_SESSION_SYNC_TIMEOUT_MS,
          },
        );
        return sessionId;
      } catch (error) {
        console.warn(
          "shein studio generation session sync failed",
          error instanceof Error ? error.message : error,
        );
        return "";
      }
    })();

    try {
      const sessionId = await sessionSyncPromise;
      const targets = buildGroupedGenerationTargets({
        activeSelection,
        groupedSelections: groupedSelections
          .filter((item) => item.eligible)
          .map((item) => item.selection),
        groupedImageMode,
      });
      const responses = await Promise.all(
        targets.map(async (target, index) => {
          const response = await generateSheinStudioDesigns(
            buildSheinStudioGenerateRequest({
              prompt: prompt.trim(),
              variationIntensity,
              printableWidth: target.selection.printableWidth,
              printableHeight: target.selection.printableHeight,
              productReferenceImageUrls:
                buildSDSProductReferenceImageUrls(target.selection),
              styleCount: parsePositiveInt(styleCount) ?? 1,
              artworkModel,
              transparentBackground,
            }),
            {
              sessionId,
              onJobStarted: (jobId) => {
                if (!sessionId || index > 0) {
                  return;
                }
                void updateSheinStudioSession(
                  sessionId,
                  {
                    status: "generating",
                    generationJobId: jobId,
                    generationError: "",
                  },
                  {
                    timeoutMs: STUDIO_SESSION_SYNC_TIMEOUT_MS,
                  },
                ).catch(() => undefined);
              },
            },
          );
          return {
            ...response,
            images: response.images.map((image) => ({
              ...image,
              targetGroupKey: target.key,
              targetGroupLabel: target.label,
            })),
          };
        }),
      );
      const allImages = responses.flatMap((response) => response.images);
      if (!allImages.length) {
        throw new Error(
          "款式图生成完成，但没有返回任何图片。请重试一次；如果持续出现，说明上游生成链路返回了空结果。",
        );
      }
      const nextSelectedIds = allImages.map((item) => item.id);
      console.info("[shein-studio] generation succeeded", {
        designCount: allImages.length,
        draftSaveStatus: "pending",
        selectionVariantId: activeSelection.variantId,
      });
      const warnings = responses.flatMap((response) => response.warnings ?? []);
      if (warnings.length > 0) {
        workbench.setField("generationWarning", warnings.join(" "));
      }
      hasLocalWorkflowStateRef.current = true;
      workbench.setField("designs", allImages);
      workbench.setField("selectedIds", nextSelectedIds);
      navigateToStep("review");
      void persistDraft(
        {
          designs: allImages,
          groups: nextGroups,
          selectedIds: nextSelectedIds,
          createdTasks: [],
        },
        {
          navigationTriggered: true,
          source: "generate_success",
        },
      ).catch(() => undefined);
    } catch (error) {
      const message = formatSubscriptionApiError(error);
      workbench.setField("designs", []);
      workbench.setField("selectedIds", []);
      workbench.setField("generationWarning", "");
      void sessionSyncPromise
        .then((sessionId) => {
          if (!sessionId) {
            return;
          }
          return updateSheinStudioSession(
            sessionId,
            {
              status: "failed",
              generationError: message,
            },
            {
              timeoutMs: STUDIO_SESSION_SYNC_TIMEOUT_MS,
            },
          );
        })
        .catch(() => undefined);
      workbench.setField("generationError", message);
    } finally {
      workbench.setField("isGenerating", false);
    }
  }

  async function handleRegenerate(designId: string) {
    if (!activeSelection?.variantId) {
      workbench.setField("generationError", "请先选择 SDS 变体。");
      return;
    }
    if (!prompt.trim()) {
      workbench.setField("generationError", "请先填写主题提示词。");
      promptInputRef.current?.scrollIntoView({
        behavior: "smooth",
        block: "center",
      });
      promptInputRef.current?.focus();
      return;
    }

    workbench.setField("generationError", "");
    workbench.setField("regeneratingId", designId);
    const nextGroups = buildNextPromptHistoryGroups();
    if (nextGroups !== groups) {
      workbench.setField("groups", nextGroups);
    }

    try {
      const targets = buildGroupedGenerationTargets({
        activeSelection,
        groupedSelections: groupedSelections
          .filter((item) => item.eligible)
          .map((item) => item.selection),
        groupedImageMode,
      });
      const currentDesign = designs.find((design) => design.id === designId);
      const target = targets.find(
        (item) =>
          currentDesign &&
          resolveDesignTargetKey(currentDesign, item.selection, groupedImageMode) ===
            item.key,
      );
      const targetSelection = target?.selection ?? activeSelection;
      const response = await generateSheinStudioDesigns(
        buildSheinStudioGenerateRequest({
          prompt: prompt.trim(),
          variationIntensity,
          printableWidth: targetSelection?.printableWidth,
          printableHeight: targetSelection?.printableHeight,
          productReferenceImageUrls:
            buildSDSProductReferenceImageUrls(targetSelection),
          styleCount: 1,
          artworkModel,
          transparentBackground,
        }),
      );
      const replacement = response.images[0];
      if (!replacement) {
        throw new Error("重新生成已完成，但没有返回任何图片。");
      }

      hasLocalWorkflowStateRef.current = true;
      const nextDesigns = designs.map((design) =>
        design.id === designId
          ? {
              ...replacement,
              id: designId,
              targetGroupKey: design.targetGroupKey,
              targetGroupLabel: design.targetGroupLabel,
            }
          : design,
      );
      workbench.setField("designs", (current) =>
        current.map((design) =>
          design.id === designId
            ? {
                ...replacement,
                id: designId,
                targetGroupKey: design.targetGroupKey,
                targetGroupLabel: design.targetGroupLabel,
              }
            : design,
        ),
      );
      const nextSelectedIds = selectedIds.includes(designId)
        ? selectedIds
        : [...selectedIds, designId];
      workbench.setField("selectedIds", nextSelectedIds);
      void persistDraft(
        {
          designs: nextDesigns,
          groups: nextGroups,
          selectedIds: nextSelectedIds,
        },
        {
          source: "regenerate_success",
        },
      ).catch(() => undefined);
    } catch (error) {
      workbench.setField("generationError", formatSubscriptionApiError(error));
    } finally {
      workbench.setField("regeneratingId", "");
    }
  }

  return {
    handleCreateTasks,
    handleGenerate,
    handleRegenerate,
  };
}

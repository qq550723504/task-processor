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
  appendSheinStudioSessionDesigns,
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
import type {
  GroupedSDSSelectionEligibility,
  SDSBaselineStatus,
} from "@/lib/types/sds-baseline";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SDSGroupedPromptHistoryEntry,
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioGenerationJob,
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
    generationJobs: SheinStudioGenerationJob[];
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
  persistedUpdatedAt: string;
  prompt: string;
  promptInputRef: RefObject<HTMLTextAreaElement | null>;
  renderSizeImagesWithSds: boolean;
  selectedIds: string[];
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  groupedSelections: GroupedSDSSelectionEligibility[];
  generationJobs: SheinStudioGenerationJob[];
  activeSelectionBaselineStatus: SDSBaselineStatus;
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
  persistedUpdatedAt,
  prompt,
  promptInputRef,
  renderSizeImagesWithSds,
  selectedIds,
  selectedSdsImages,
  groupedSelections,
  generationJobs,
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

  function withTargetMetadata(
    images: SheinStudioGeneratedDesign[],
    target: { key: string; label?: string },
  ) {
    return images.map((image) => ({
      ...image,
      targetGroupKey: target.key,
      targetGroupLabel: target.label,
    }));
  }

  function mergeDesignCollections(
    currentDesigns: SheinStudioGeneratedDesign[],
    incomingDesigns: SheinStudioGeneratedDesign[],
  ) {
    const nextByID = new Map(currentDesigns.map((design) => [design.id, design]));
    for (const design of incomingDesigns) {
      nextByID.set(design.id, design);
    }
    return Array.from(nextByID.values());
  }

  function mergeSelectedIDs(currentIDs: string[], designs: SheinStudioGeneratedDesign[]) {
    const next = new Set(currentIDs);
    for (const design of designs) {
      next.add(design.id);
    }
    return Array.from(next);
  }

  async function handleGenerate() {
    if (!activeSelection?.variantId) {
      workbench.setField("generationError", "请先选择 SDS 变体。");
      return;
    }
    if (!sheinStoreId.trim()) {
      workbench.setField("generationError", "请先选择批次店铺。");
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
    workbench.setField("generationJobs", []);
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
      let latestPersistedUpdatedAt = persistedUpdatedAt;
      const targets = buildGroupedGenerationTargets({
        activeSelection,
        groupedSelections: groupedSelections
          .filter((item) => item.eligible)
          .map((item) => item.selection),
        groupedImageMode,
      });
      let nextGenerationJobs = generationJobs.filter(
        (job) => job.status === "running" && job.jobId.trim(),
      );
      let accumulatedDesigns: SheinStudioGeneratedDesign[] = [];
      let accumulatedSelectedIDs: string[] = [];
      const aggregatedWarnings: string[] = [];
      const targetErrors: string[] = [];

      const syncGenerationJobs = (jobs: SheinStudioGenerationJob[]) => {
        nextGenerationJobs = jobs;
        workbench.setField("generationJobs", jobs);
        if (!sessionId) {
          return;
        }
        void updateSheinStudioSession(
          sessionId,
          {
            status: "generating",
            generationJobId: jobs[0]?.jobId ?? "",
            generationJobs: jobs,
            generationError: "",
          },
          {
            timeoutMs: STUDIO_SESSION_SYNC_TIMEOUT_MS,
          },
        ).catch(() => undefined);
      };

      const persistProgress = async (
        incomingDesigns: SheinStudioGeneratedDesign[],
        nextSelectedIds: string[],
        jobs: SheinStudioGenerationJob[],
      ) => {
        if (!sessionId) {
          await persistDraft(
            {
              designs: accumulatedDesigns,
              groups: nextGroups,
              selectedIds: nextSelectedIds,
              createdTasks: [],
              generationJobs: jobs,
            },
            {
              source: "generate_progress",
            },
          ).catch(() => undefined);
          return;
        }
        const nextStatus = jobs.length > 0 ? "generating" : "reviewing";
        await appendSheinStudioSessionDesigns(
          sessionId,
          {
            expectedUpdatedAt: latestPersistedUpdatedAt,
            status: nextStatus,
            approvedDesignIds: nextSelectedIds,
            generationJobs: jobs,
            designs: incomingDesigns,
          },
          {
            timeoutMs: STUDIO_SESSION_SYNC_TIMEOUT_MS,
          },
        )
          .then((detail) => {
            const updatedAt = detail?.session?.updated_at?.trim();
            if (updatedAt) {
              latestPersistedUpdatedAt = updatedAt;
              workbench.setField("persistedUpdatedAt", updatedAt);
            }
          })
          .catch(() => undefined);
      };

      syncGenerationJobs([]);

      const settled = await Promise.allSettled(
        targets.map(async (target) => {
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
                const existingIndex = nextGenerationJobs.findIndex(
                  (job) => job.jobId === jobId,
                );
                const nextJob: SheinStudioGenerationJob = {
                  jobId,
                  targetGroupKey: target.key,
                  targetGroupLabel: target.label,
                  status: "running",
                };
                const jobs =
                  existingIndex >= 0
                    ? nextGenerationJobs.map((job, index) =>
                        index === existingIndex ? nextJob : job,
                      )
                    : [...nextGenerationJobs, nextJob];
                syncGenerationJobs(jobs);
              },
            },
          );
          const nextImages = withTargetMetadata(response.images, target);
          const targetJobIndex = nextGenerationJobs.findIndex(
            (job) => job.targetGroupKey === target.key,
          );
          if (targetJobIndex >= 0) {
            const jobs: SheinStudioGenerationJob[] = nextGenerationJobs.map(
              (job, index) =>
                index === targetJobIndex
                  ? { ...job, status: "succeeded" as const }
                  : job,
            );
            syncGenerationJobs(jobs);
          }
          if (response.warnings?.length) {
            aggregatedWarnings.push(...response.warnings);
          }
          if (nextImages.length > 0) {
            accumulatedDesigns = mergeDesignCollections(accumulatedDesigns, nextImages);
            accumulatedSelectedIDs = mergeSelectedIDs(accumulatedSelectedIDs, nextImages);
            hasLocalWorkflowStateRef.current = true;
            workbench.setField("designs", accumulatedDesigns);
            workbench.setField("selectedIds", accumulatedSelectedIDs);
            workbench.setField("createdTasks", []);
            navigateToStep("review");
            await persistProgress(
              nextImages,
              accumulatedSelectedIDs,
              nextGenerationJobs.filter((job) => job.status === "running"),
            );
          }
          return nextImages;
        }),
      );

      for (let index = 0; index < settled.length; index += 1) {
        const result = settled[index];
        if (result.status === "fulfilled") {
          continue;
        }
        const target = targets[index];
        const message = formatSubscriptionApiError(result.reason);
        targetErrors.push(target.label ? `${target.label}: ${message}` : message);
        const targetJobIndex = nextGenerationJobs.findIndex(
          (job) => job.targetGroupKey === target.key,
        );
        if (targetJobIndex >= 0) {
          const jobs: SheinStudioGenerationJob[] = nextGenerationJobs.map(
            (job, jobIndex) =>
              jobIndex === targetJobIndex
                ? { ...job, status: "failed" as const }
                : job,
          );
          syncGenerationJobs(jobs);
        }
      }

      if (!accumulatedDesigns.length) {
        throw new Error(
          "款式图生成完成，但没有返回任何图片。请重试一次；如果持续出现，说明上游生成链路返回了空结果。",
        );
      }
      console.info("[shein-studio] generation succeeded", {
        designCount: accumulatedDesigns.length,
        draftSaveStatus: "pending",
        selectionVariantId: activeSelection.variantId,
      });
      if (aggregatedWarnings.length > 0 || targetErrors.length > 0) {
        workbench.setField(
          "generationWarning",
          [...aggregatedWarnings, ...targetErrors].join(" "),
        );
      }
      hasLocalWorkflowStateRef.current = true;
      workbench.setField("designs", accumulatedDesigns);
      workbench.setField("selectedIds", accumulatedSelectedIDs);
      workbench.setField("generationJobs", []);
      navigateToStep("review");
      if (!sessionId) {
        void persistDraft(
          {
            designs: accumulatedDesigns,
            groups: nextGroups,
            selectedIds: accumulatedSelectedIDs,
            createdTasks: [],
            generationJobs: [],
          },
          {
            navigationTriggered: true,
            source: "generate_success",
          },
        ).catch(() => undefined);
      }
    } catch (error) {
      const message = formatSubscriptionApiError(error);
      workbench.setField("designs", []);
      workbench.setField("selectedIds", []);
      workbench.setField("generationJobs", []);
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

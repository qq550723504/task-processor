import type { MutableRefObject, RefObject } from "react";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import { useSheinStudioTaskCreationAction } from "@/components/listingkit/shein-studio/shein-studio-task-creation-actions";
import {
  buildSheinStudioGenerateRequest,
  evaluateImportedGalleryDesigns,
  STUDIO_SESSION_SYNC_TIMEOUT_MS,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import { generateSheinStudioDesigns } from "@/lib/api/shein-studio";
import {
  ensureSheinStudioSession,
  updateSheinStudioSession,
} from "@/lib/api/shein-studio-sessions";
import { parsePositiveInt } from "@/lib/shein-studio/create-review-tasks";
import { buildSDSProductReferenceImageUrls } from "@/lib/shein-studio/sds-reference-images";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
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

type UseSheinStudioDesignActionsParams = {
  activeSelection?: SDSProductVariantSelection;
  artworkModel: SheinStudioArtworkModel;
  designs: SheinStudioGeneratedDesign[];
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
  setCreatedTasks: (value: SheinStudioCreatedTask[]) => void;
  setCreatingError: (value: string) => void;
  setCreatingMessage: (value: string) => void;
  setDesigns: (
    value:
      | SheinStudioGeneratedDesign[]
      | ((
          current: SheinStudioGeneratedDesign[],
        ) => SheinStudioGeneratedDesign[]),
  ) => void;
  setDraftWarning: (value: string) => void;
  setGalleryRatioCheck: (
    value: ReturnType<typeof evaluateImportedGalleryDesigns>,
  ) => void;
  setGenerationError: (value: string) => void;
  setIsCreatingTasks: (value: boolean) => void;
  setIsGenerating: (value: boolean) => void;
  setRegeneratingId: (value: string) => void;
  setSelectedIds: (value: string[] | ((current: string[]) => string[])) => void;
  sheinStoreId: string;
  styleCount: string;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
  hasLocalWorkflowStateRef: MutableRefObject<boolean>;
};

export function useSheinStudioDesignActions({
  activeSelection,
  artworkModel,
  designs,
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
  setCreatedTasks,
  setCreatingError,
  setCreatingMessage,
  setDesigns,
  setDraftWarning,
  setGalleryRatioCheck,
  setGenerationError,
  setIsCreatingTasks,
  setIsGenerating,
  setRegeneratingId,
  setSelectedIds,
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
  });

  async function handleGenerate() {
    if (!activeSelection?.variantId) {
      setGenerationError("请先选择 SDS 变体。");
      return;
    }
    if (!prompt.trim()) {
      setGenerationError("请先填写主题提示词。");
      promptInputRef.current?.scrollIntoView({
        behavior: "smooth",
        block: "center",
      });
      promptInputRef.current?.focus();
      return;
    }

    setGenerationError("");
    setCreatingError("");
    setCreatingMessage("");
    setCreatedTasks([]);
    setDraftWarning("");
    setIsGenerating(true);

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
            selectedSdsImages,
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
      const response = await generateSheinStudioDesigns(
        buildSheinStudioGenerateRequest({
          prompt: prompt.trim(),
          variationIntensity,
          printableWidth: activeSelection.printableWidth,
          printableHeight: activeSelection.printableHeight,
          productReferenceImageUrls:
            buildSDSProductReferenceImageUrls(activeSelection),
          styleCount: parsePositiveInt(styleCount) ?? 1,
          artworkModel,
          transparentBackground,
        }),
        {
          onJobStarted: (jobId) => {
            void sessionSyncPromise
              .then((sessionId) => {
                if (!sessionId) {
                  return;
                }
                return updateSheinStudioSession(
                  sessionId,
                  {
                    status: "generating",
                    generationJobId: jobId,
                    generationError: "",
                  },
                  {
                    timeoutMs: STUDIO_SESSION_SYNC_TIMEOUT_MS,
                  },
                );
              })
              .catch(() => undefined);
          },
        },
      );
      if (!response.images.length) {
        throw new Error(
          "款式图生成完成，但没有返回任何图片。请重试一次；如果持续出现，说明上游生成链路返回了空结果。",
        );
      }
      const nextSelectedIds = response.images.map((item) => item.id);
      console.info("[shein-studio] generation succeeded", {
        designCount: response.images.length,
        draftSaveStatus: "pending",
        selectionVariantId: activeSelection.variantId,
      });
      hasLocalWorkflowStateRef.current = true;
      setDesigns(response.images);
      setSelectedIds(nextSelectedIds);
      navigateToStep("review");
      void persistDraft(
        {
          designs: response.images,
          selectedIds: nextSelectedIds,
          createdTasks: [],
        },
        {
          navigationTriggered: true,
          source: "generate_success",
        },
      ).catch(() => undefined);
    } catch (error) {
      setDesigns([]);
      setSelectedIds([]);
      void sessionSyncPromise
        .then((sessionId) => {
          if (!sessionId) {
            return;
          }
          return updateSheinStudioSession(
            sessionId,
            {
              status: "failed",
              generationError:
                error instanceof Error ? error.message : "款式图生成失败。",
            },
            {
              timeoutMs: STUDIO_SESSION_SYNC_TIMEOUT_MS,
            },
          );
        })
        .catch(() => undefined);
      setGenerationError(
        error instanceof Error ? error.message : "款式图生成失败。",
      );
    } finally {
      setIsGenerating(false);
    }
  }

  async function handleRegenerate(designId: string) {
    if (!activeSelection?.variantId) {
      setGenerationError("请先选择 SDS 变体。");
      return;
    }
    if (!prompt.trim()) {
      setGenerationError("请先填写主题提示词。");
      promptInputRef.current?.scrollIntoView({
        behavior: "smooth",
        block: "center",
      });
      promptInputRef.current?.focus();
      return;
    }

    setGenerationError("");
    setRegeneratingId(designId);

    try {
      const response = await generateSheinStudioDesigns(
        buildSheinStudioGenerateRequest({
          prompt: prompt.trim(),
          variationIntensity,
          printableWidth: activeSelection.printableWidth,
          printableHeight: activeSelection.printableHeight,
          productReferenceImageUrls:
            buildSDSProductReferenceImageUrls(activeSelection),
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
      setDesigns((current) =>
        current.map((design) =>
          design.id === designId ? { ...replacement, id: designId } : design,
        ),
      );
      const nextSelectedIds = selectedIds.includes(designId)
        ? selectedIds
        : [...selectedIds, designId];
      setSelectedIds(nextSelectedIds);
      void persistDraft(
        {
          designs: designs.map((design) =>
            design.id === designId ? { ...replacement, id: designId } : design,
          ),
          selectedIds: nextSelectedIds,
        },
        {
          source: "regenerate_success",
        },
      ).catch(() => undefined);
    } catch (error) {
      setGenerationError(
        error instanceof Error ? error.message : "重新生成款式失败。",
      );
    } finally {
      setRegeneratingId("");
    }
  }

  return {
    handleCreateTasks,
    handleGenerate,
    handleRegenerate,
  };
}

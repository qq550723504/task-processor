import type { SheinStudioWorkbenchHydratedBatch } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import {
  pickLocalArrayValue,
  pickLocalStringValue,
  shouldUseLocalDraftOverRemote,
} from "@/lib/shein-studio/local-remote-conflict-policy";
import type { SheinStudioDraft } from "@/lib/types/shein-studio";

export type DedicatedBatchLocalSnapshot = {
  batchId?: string;
  draft: SheinStudioDraft;
};

type ResolveDedicatedBatchHydrationInput = {
  batchId: string;
  hydratedBatch: SheinStudioWorkbenchHydratedBatch;
  localSnapshot?: DedicatedBatchLocalSnapshot | null;
  promptOverride?: string;
};

export function resolveDedicatedBatchHydration({
  batchId,
  hydratedBatch,
  localSnapshot,
  promptOverride,
}: ResolveDedicatedBatchHydrationInput): SheinStudioWorkbenchHydratedBatch {
  if (
    localSnapshot?.batchId === batchId &&
    shouldUseLocalDraftOverRemote({
      localUpdatedAt: localSnapshot.draft.updatedAt,
      remoteUpdatedAt:
        hydratedBatch.savedBatch.draftUpdatedAt ??
        hydratedBatch.savedBatch.updatedAt,
    })
  ) {
    return mergeDedicatedBatchWithLocalSnapshot(
      hydratedBatch,
      localSnapshot,
      promptOverride,
    );
  }

  if (!promptOverride) {
    return hydratedBatch;
  }

  return {
    savedBatch: {
      ...hydratedBatch.savedBatch,
      prompt: promptOverride,
    },
    detail: hydratedBatch.detail,
  };
}

function mergeDedicatedBatchWithLocalSnapshot(
  hydratedBatch: SheinStudioWorkbenchHydratedBatch,
  localSnapshot: DedicatedBatchLocalSnapshot,
  promptOverride?: string,
): SheinStudioWorkbenchHydratedBatch {
  const savedBatch = hydratedBatch.savedBatch;
  const localDraft = localSnapshot.draft;
  return {
    savedBatch: {
      ...savedBatch,
      prompt:
        promptOverride ?? pickLocalStringValue(localDraft.prompt, savedBatch.prompt),
      promptMode: localDraft.promptMode ?? savedBatch.promptMode,
      styleCount: pickLocalStringValue(localDraft.styleCount, savedBatch.styleCount),
      variationIntensity:
        localDraft.variationIntensity ?? savedBatch.variationIntensity,
      productImageCount: pickLocalStringValue(
        localDraft.productImageCount,
        savedBatch.productImageCount,
      ),
      productImagePrompt: pickLocalStringValue(
        localDraft.productImagePrompt,
        savedBatch.productImagePrompt,
      ),
      productImagePrompts: pickLocalArrayValue(
        localDraft.productImagePrompts,
        savedBatch.productImagePrompts,
      ),
      artworkModel: pickLocalStringValue(
        localDraft.artworkModel,
        savedBatch.artworkModel,
      ),
      transparentBackground:
        localDraft.transparentBackground ?? savedBatch.transparentBackground,
      sheinStoreId: pickLocalStringValue(
        localDraft.sheinStoreId,
        savedBatch.sheinStoreId,
      ),
      imageStrategy: localDraft.imageStrategy ?? savedBatch.imageStrategy,
      groupedImageMode:
        localDraft.groupedImageMode ?? savedBatch.groupedImageMode,
      selectedSdsImages: pickLocalArrayValue(
        localDraft.selectedSdsImages,
        savedBatch.selectedSdsImages,
      ),
      renderSizeImagesWithSds:
        localDraft.renderSizeImagesWithSds ?? savedBatch.renderSizeImagesWithSds,
      hotStyleReferenceImageUrls: pickLocalPresentArrayValue(
        localDraft,
        "hotStyleReferenceImageUrls",
        localDraft.hotStyleReferenceImageUrls,
        savedBatch.hotStyleReferenceImageUrls,
      ),
      hotStyleReferenceBrief: pickLocalPresentStringValue(
        localDraft,
        "hotStyleReferenceBrief",
        localDraft.hotStyleReferenceBrief,
        savedBatch.hotStyleReferenceBrief,
      ),
      hotStyleReferencePrompt: pickLocalPresentStringValue(
        localDraft,
        "hotStyleReferencePrompt",
        localDraft.hotStyleReferencePrompt,
        savedBatch.hotStyleReferencePrompt,
      ),
      selection: localDraft.selection ?? savedBatch.selection,
      groupedSelections: pickLocalArrayValue(
        localDraft.groupedSelections,
        savedBatch.groupedSelections,
      ),
      groups: pickLocalArrayValue(localDraft.groups, savedBatch.groups),
    },
    detail: hydratedBatch.detail,
  };
}

function hasOwnDraftProperty(value: object | undefined, key: PropertyKey) {
  return !!value && Object.prototype.hasOwnProperty.call(value, key);
}

function pickLocalPresentStringValue(
  source: object,
  key: PropertyKey,
  localValue: string | undefined,
  remoteValue: string | undefined,
) {
  return hasOwnDraftProperty(source, key)
    ? (localValue?.trim() ?? "")
    : (remoteValue ?? "");
}

function pickLocalPresentArrayValue<T>(
  source: object,
  key: PropertyKey,
  localValue: T[] | undefined,
  remoteValue: T[] | undefined,
) {
  return (hasOwnDraftProperty(source, key)
    ? (localValue ?? [])
    : (remoteValue ?? [])) as T[];
}

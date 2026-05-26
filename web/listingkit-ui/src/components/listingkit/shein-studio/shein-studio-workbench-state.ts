import {
  DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
  DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
  DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
  DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
} from "@/lib/shein-studio/storage-shared";
import { DEFAULT_SHEIN_STORE_ID } from "@/lib/shein-studio/create-review-tasks";
import type { SDSRatioMatch } from "@/lib/shein-studio/gallery-handoff";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSavedBatch,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";

export type SheinStudioWorkbenchState = {
  prompt: string;
  styleCount: string;
  variationIntensity: SheinStudioVariationIntensity;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  artworkModel: SheinStudioArtworkModel;
  transparentBackground: boolean;
  sheinStoreId: string;
  imageStrategy: SheinStudioImageStrategy;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  groupedSelections: GroupedSDSSelectionEligibility[];
  renderSizeImagesWithSds: boolean;
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  generationError: string;
  generationWarning: string;
  creatingError: string;
  creatingMessage: string;
  isGenerating: boolean;
  isCreatingTasks: boolean;
  regeneratingId: string;
  createdTasks: SheinStudioCreatedTask[];
  galleryRatioCheck: SDSRatioMatch | null;
  savedBatches: SheinStudioSavedBatch[];
  isLoadingWorkspace: boolean;
  saveMessage: string;
  draftWarning: string;
};

export type SheinStudioWorkbenchStateUpdater<
  K extends keyof SheinStudioWorkbenchState,
> =
  | SheinStudioWorkbenchState[K]
  | ((current: SheinStudioWorkbenchState[K]) => SheinStudioWorkbenchState[K]);

export type SheinStudioWorkbenchDraftPatch = Pick<
  SheinStudioWorkbenchState,
  | "prompt"
  | "styleCount"
  | "variationIntensity"
  | "productImageCount"
  | "productImagePrompt"
  | "productImagePrompts"
  | "artworkModel"
  | "transparentBackground"
  | "sheinStoreId"
  | "imageStrategy"
  | "selectedSdsImages"
  | "renderSizeImagesWithSds"
  | "designs"
  | "selectedIds"
  | "createdTasks"
  | "galleryRatioCheck"
>;

export type SheinStudioWorkbenchController = {
  applyBatch: (batch: SheinStudioSavedBatch) => void;
  applyDraft: (draft: SheinStudioWorkbenchDraftPatch) => void;
  setField: <K extends keyof SheinStudioWorkbenchState>(
    field: K,
    value: SheinStudioWorkbenchStateUpdater<K>,
  ) => void;
};

export type SheinStudioWorkbenchAction =
  | {
      [K in keyof SheinStudioWorkbenchState]: {
        type: "set-field";
        field: K;
        value: SheinStudioWorkbenchStateUpdater<K>;
      };
    }[keyof SheinStudioWorkbenchState]
  | {
      type: "apply-draft";
      draft: SheinStudioWorkbenchDraftPatch;
    }
  | {
      type: "apply-batch";
      batch: SheinStudioSavedBatch;
    };

export function buildInitialSheinStudioWorkbenchState(): SheinStudioWorkbenchState {
  return {
    prompt: "",
    styleCount: "1",
    variationIntensity: DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
    productImageCount: DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    productImagePrompt: "",
    productImagePrompts: [],
    artworkModel: DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
    transparentBackground: false,
    sheinStoreId: DEFAULT_SHEIN_STORE_ID,
    imageStrategy: DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
    selectedSdsImages: [],
    groupedSelections: [],
    renderSizeImagesWithSds: true,
    designs: [],
    selectedIds: [],
    generationError: "",
    generationWarning: "",
    creatingError: "",
    creatingMessage: "",
    isGenerating: false,
    isCreatingTasks: false,
    regeneratingId: "",
    createdTasks: [],
    galleryRatioCheck: null,
    savedBatches: [],
    isLoadingWorkspace: true,
    saveMessage: "",
    draftWarning: "",
  };
}

export function setSheinStudioWorkbenchField<
  K extends keyof SheinStudioWorkbenchState,
>(
  field: K,
  value: SheinStudioWorkbenchStateUpdater<K>,
): Extract<SheinStudioWorkbenchAction, { field: K }> {
  return {
    type: "set-field",
    field,
    value,
  } as Extract<SheinStudioWorkbenchAction, { field: K }>;
}

export function applySheinStudioWorkbenchDraft(
  draft: SheinStudioWorkbenchDraftPatch,
): SheinStudioWorkbenchAction {
  return {
    type: "apply-draft",
    draft,
  };
}

export function applySheinStudioWorkbenchBatch(
  batch: SheinStudioSavedBatch,
): SheinStudioWorkbenchAction {
  return {
    type: "apply-batch",
    batch,
  };
}

export function sheinStudioWorkbenchReducer(
  state: SheinStudioWorkbenchState,
  action: SheinStudioWorkbenchAction,
): SheinStudioWorkbenchState {
  switch (action.type) {
    case "set-field": {
      const current = state[action.field];
      const next =
        typeof action.value === "function"
          ? (action.value as (value: typeof current) => typeof current)(current)
          : action.value;
      if (Object.is(current, next)) {
        return state;
      }
      return {
        ...state,
        [action.field]: next,
      };
    }
    case "apply-draft":
      return {
        ...state,
        ...action.draft,
      };
    case "apply-batch":
      return {
        ...state,
        prompt: action.batch.prompt,
        styleCount: action.batch.styleCount,
        variationIntensity:
          action.batch.variationIntensity ??
          DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
        productImageCount:
          action.batch.productImageCount ??
          DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
        productImagePrompt: action.batch.productImagePrompt ?? "",
        productImagePrompts: action.batch.productImagePrompts ?? [],
        artworkModel:
          action.batch.artworkModel ?? DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
        transparentBackground: action.batch.transparentBackground ?? false,
        sheinStoreId: action.batch.sheinStoreId || DEFAULT_SHEIN_STORE_ID,
        imageStrategy:
          action.batch.imageStrategy ?? DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
        selectedSdsImages: action.batch.selectedSdsImages ?? [],
        renderSizeImagesWithSds: action.batch.renderSizeImagesWithSds ?? true,
        designs: action.batch.designs,
        selectedIds: action.batch.selectedIds,
        createdTasks: action.batch.createdTasks,
      };
  }
}

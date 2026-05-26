import {
  DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
  DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
  DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
  DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
  DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
} from "@/lib/shein-studio/storage-shared";
import { DEFAULT_SHEIN_STORE_ID } from "@/lib/shein-studio/create-review-tasks";
import type { SDSRatioMatch } from "@/lib/shein-studio/gallery-handoff";
import { pickActiveSheinStudioGroup, projectGroupToWorkbench } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedWorkspace,
  SheinStudioGroupedImageMode,
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
  groupedImageMode: SheinStudioGroupedImageMode;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  groups: SheinStudioGroupedWorkspace[];
  activeGroupId: string;
  groupedSelections: GroupedSDSSelectionEligibility[];
  renderSizeImagesWithSds: boolean;
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  generationError: string;
  generationWarning: string;
  generationWarningAction:
    | {
        intent: "focus_generate" | "warm_baseline";
        label: string;
      }
    | null;
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

export type SheinStudioWorkbenchDraftPatch = Partial<Pick<
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
  | "groupedImageMode"
  | "selectedSdsImages"
  | "groups"
  | "activeGroupId"
  | "groupedSelections"
  | "renderSizeImagesWithSds"
  | "designs"
  | "selectedIds"
  | "createdTasks"
  | "galleryRatioCheck"
>>;

export type SheinStudioWorkbenchController = {
  applyBatch: (batch: SheinStudioSavedBatch) => void;
  applyDraft: (draft: SheinStudioWorkbenchDraftPatch) => void;
  selectGroup: (groupId: string) => void;
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
    }
  | {
      type: "select-group";
      groupId: string;
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
    groupedImageMode: DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
    selectedSdsImages: [],
    groups: [],
    activeGroupId: "",
    groupedSelections: [],
    renderSizeImagesWithSds: true,
    designs: [],
    selectedIds: [],
    generationError: "",
    generationWarning: "",
    generationWarningAction: null,
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

export function selectSheinStudioWorkbenchGroup(
  groupId: string,
): SheinStudioWorkbenchAction {
  return {
    type: "select-group",
    groupId,
  };
}

const ACTIVE_GROUP_SYNC_FIELDS = new Set<keyof SheinStudioWorkbenchState>([
  "prompt",
  "styleCount",
  "variationIntensity",
  "productImageCount",
  "productImagePrompt",
  "productImagePrompts",
  "artworkModel",
  "transparentBackground",
  "sheinStoreId",
  "imageStrategy",
  "groupedImageMode",
  "selectedSdsImages",
  "groupedSelections",
  "renderSizeImagesWithSds",
  "designs",
  "selectedIds",
  "createdTasks",
]);

function syncActiveGroupFromState(
  state: SheinStudioWorkbenchState,
  patch: Partial<SheinStudioWorkbenchState>,
) {
  if (!state.activeGroupId || state.groups.length === 0) {
    return state.groups;
  }
  return state.groups.map((group) =>
    group.id === state.activeGroupId
      ? {
          ...group,
          currentPrompt:
            typeof patch.prompt === "string" ? patch.prompt : state.prompt,
          styleCount:
            typeof patch.styleCount === "string" ? patch.styleCount : state.styleCount,
          variationIntensity:
            patch.variationIntensity ?? state.variationIntensity,
          productImageCount:
            typeof patch.productImageCount === "string"
              ? patch.productImageCount
              : state.productImageCount,
          productImagePrompt:
            typeof patch.productImagePrompt === "string"
              ? patch.productImagePrompt
              : state.productImagePrompt,
          productImagePrompts:
            patch.productImagePrompts ?? state.productImagePrompts,
          artworkModel: patch.artworkModel ?? state.artworkModel,
          transparentBackground:
            patch.transparentBackground ?? state.transparentBackground,
          sheinStoreId:
            typeof patch.sheinStoreId === "string"
              ? patch.sheinStoreId
              : state.sheinStoreId,
          imageStrategy: patch.imageStrategy ?? state.imageStrategy,
          groupedImageMode:
            patch.groupedImageMode ?? state.groupedImageMode,
          selectedSdsImages:
            patch.selectedSdsImages ?? state.selectedSdsImages,
          groupedSelections:
            patch.groupedSelections ?? state.groupedSelections,
          renderSizeImagesWithSds:
            patch.renderSizeImagesWithSds ?? state.renderSizeImagesWithSds,
          designs: patch.designs ?? state.designs,
          selectedIds: patch.selectedIds ?? state.selectedIds,
          createdTasks: patch.createdTasks ?? state.createdTasks,
          updatedAt: new Date().toISOString(),
        }
      : group,
  );
}

function projectActiveGroupIntoState(
  state: SheinStudioWorkbenchState,
  groups: SheinStudioGroupedWorkspace[],
  activeGroupId?: string,
) {
  const activeGroup = pickActiveSheinStudioGroup(groups, activeGroupId);
  if (!activeGroup) {
    return {
      ...state,
      groups,
      activeGroupId: "",
    };
  }
  return {
    ...state,
    groups,
    activeGroupId: activeGroup.id,
    ...projectGroupToWorkbench(activeGroup),
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
        groups: ACTIVE_GROUP_SYNC_FIELDS.has(action.field)
          ? syncActiveGroupFromState(state, {
              [action.field]: next,
            } as Partial<SheinStudioWorkbenchState>)
          : state.groups,
        [action.field]: next,
      };
    }
    case "apply-draft":
      return projectActiveGroupIntoState(
        {
          ...state,
          ...action.draft,
        },
        action.draft.groups ?? state.groups,
        action.draft.activeGroupId ?? state.activeGroupId,
      );
    case "apply-batch":
      return projectActiveGroupIntoState(
        {
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
          groupedImageMode:
            action.batch.groupedImageMode ?? DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
          selectedSdsImages: action.batch.selectedSdsImages ?? [],
          groupedSelections: action.batch.groupedSelections ?? [],
          renderSizeImagesWithSds: action.batch.renderSizeImagesWithSds ?? true,
          designs: action.batch.designs,
          selectedIds: action.batch.selectedIds,
          createdTasks: action.batch.createdTasks,
        },
        action.batch.groups ?? state.groups,
      );
    case "select-group":
      return projectActiveGroupIntoState(
        state,
        state.groups,
        action.groupId,
      );
  }
}

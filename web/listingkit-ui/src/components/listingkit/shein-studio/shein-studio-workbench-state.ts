import {
  DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
  DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
  DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
  DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
  DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
} from "@/lib/shein-studio/storage-shared";
import { DEFAULT_SHEIN_STORE_ID } from "@/lib/shein-studio/create-review-tasks";
import type { SDSRatioMatch } from "@/lib/shein-studio/gallery-handoff";
import {
  pickActiveSheinStudioGroup,
  projectGroupToWorkbench,
  projectHydratedBatchToWorkbench,
  type SheinStudioWorkbenchHydratedBatch,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioArtworkModel,
  SheinStudioBatchDetail,
  SheinStudioBatchQueueMode,
  SheinStudioCreatedTask,
  SheinStudioGenerationJob,
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
  selection?: SDSProductVariantSelection;
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
  generationJobs: SheinStudioGenerationJob[];
  generationError: string;
  generationWarning: string;
  generationWarningAction:
    | {
        intent: "focus_generate" | "warm_baseline" | "open_sds_login";
        label: string;
      }
    | null;
  creatingError: string;
  creatingMessage: string;
  creatingWarning: string;
  isGenerating: boolean;
  isCreatingTasks: boolean;
  regeneratingId: string;
  createdTasks: SheinStudioCreatedTask[];
  itemizedBatchDetail: SheinStudioBatchDetail | null;
  persistedUpdatedAt: string;
  galleryRatioCheck: SDSRatioMatch | null;
  savedBatches: SheinStudioSavedBatch[];
  batchQueueMode: SheinStudioBatchQueueMode | null;
  queuedBatchIds: string[];
  queuedBatchIndex: number;
  queueMessage: string;
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
  | "selection"
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
  | "generationJobs"
  | "createdTasks"
  | "persistedUpdatedAt"
  | "galleryRatioCheck"
>>;

export type SheinStudioWorkbenchController = {
  applyBatch: (batch: SheinStudioSavedBatch) => void;
  applyHydratedBatch: (batch: SheinStudioWorkbenchHydratedBatch) => void;
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
      type: "apply-hydrated-batch";
      batch: SheinStudioWorkbenchHydratedBatch;
    }
  | {
      type: "select-group";
      groupId: string;
    };

export function buildInitialSheinStudioWorkbenchState(): SheinStudioWorkbenchState {
  return {
    prompt: "",
    selection: undefined,
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
    generationJobs: [],
    generationError: "",
    generationWarning: "",
    generationWarningAction: null,
    creatingError: "",
    creatingMessage: "",
    creatingWarning: "",
    isGenerating: false,
    isCreatingTasks: false,
    regeneratingId: "",
    createdTasks: [],
    itemizedBatchDetail: null,
    persistedUpdatedAt: "",
    galleryRatioCheck: null,
    savedBatches: [],
    batchQueueMode: null,
    queuedBatchIds: [],
    queuedBatchIndex: 0,
    queueMessage: "",
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

export function applySheinStudioWorkbenchHydratedBatch(
  batch: SheinStudioWorkbenchHydratedBatch,
): SheinStudioWorkbenchAction {
  return {
    type: "apply-hydrated-batch",
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

const ITEMIZED_OVERRIDE_FIELDS = new Set<keyof SheinStudioWorkbenchState>([
  "designs",
  "selectedIds",
  "generationJobs",
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
          generationJobs: patch.generationJobs ?? state.generationJobs,
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
  if (!action) {
    return state;
  }
  switch (action.type) {
    case "set-field": {
      const setFieldAction = action as Extract<
        SheinStudioWorkbenchAction,
        { type: "set-field" }
      >;
      const field = setFieldAction.field;
      const value = setFieldAction.value;
      const current = state[field];
      const next =
        typeof value === "function"
          ? (value as (value: typeof current) => typeof current)(current)
          : value;
      if (Object.is(current, next)) {
        return state;
      }
      return {
        ...state,
        groups: ACTIVE_GROUP_SYNC_FIELDS.has(field)
          ? syncActiveGroupFromState(state, {
              [field]: next,
            } as Partial<SheinStudioWorkbenchState>)
          : state.groups,
        itemizedBatchDetail:
          ITEMIZED_OVERRIDE_FIELDS.has(field) ? null : state.itemizedBatchDetail,
        [field]: next,
      };
    }
    case "apply-draft":
      return projectActiveGroupIntoState(
        {
          ...state,
          ...action.draft,
          itemizedBatchDetail:
            "designs" in action.draft ||
            "selectedIds" in action.draft ||
            "createdTasks" in action.draft ||
            "generationJobs" in action.draft
              ? null
              : state.itemizedBatchDetail,
          persistedUpdatedAt:
            "updatedAt" in action.draft && typeof action.draft.updatedAt === "string"
              ? action.draft.updatedAt
              : state.persistedUpdatedAt,
        },
        action.draft.groups ?? state.groups,
        action.draft.activeGroupId ?? state.activeGroupId,
      );
    case "apply-batch":
      return projectActiveGroupIntoState(
        {
          ...state,
          selection: action.batch.selection,
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
          generationJobs: action.batch.generationJobs ?? [],
          createdTasks: action.batch.createdTasks,
          itemizedBatchDetail: null,
          persistedUpdatedAt: action.batch.updatedAt,
        },
        action.batch.groups ?? state.groups,
      );
    case "apply-hydrated-batch":
      return projectActiveGroupIntoState(
        {
          ...state,
          ...projectHydratedBatchToWorkbench(action.batch),
        },
        action.batch.savedBatch.groups ?? state.groups,
      );
    case "select-group":
      return projectActiveGroupIntoState(
        {
          ...state,
          itemizedBatchDetail: null,
        },
        state.groups,
        action.groupId,
      );
  }
}

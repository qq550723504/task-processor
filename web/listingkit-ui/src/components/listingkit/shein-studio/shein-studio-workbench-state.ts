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
  projectSavedBatchToWorkbench,
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
  SheinStudioPromptMode,
  SheinStudioSavedBatch,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";

export type SheinStudioWorkbenchState = {
  selection?: SDSProductVariantSelection;
  prompt: string;
  promptMode: SheinStudioPromptMode;
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
  | "promptMode"
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
    promptMode: "managed",
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

type SheinStudioWorkbenchSetField = SheinStudioWorkbenchController["setField"];

export function buildSheinStudioWorkbenchSetters(
  setField: SheinStudioWorkbenchSetField,
) {
  return {
    setArtworkModel: (value: SheinStudioWorkbenchStateUpdater<"artworkModel">) =>
      setField("artworkModel", value),
    setCreatedTasks: (value: SheinStudioWorkbenchStateUpdater<"createdTasks">) =>
      setField("createdTasks", value),
    setCreatingError: (value: SheinStudioWorkbenchStateUpdater<"creatingError">) =>
      setField("creatingError", value),
    setCreatingWarning: (
      value: SheinStudioWorkbenchStateUpdater<"creatingWarning">,
    ) => setField("creatingWarning", value),
    setCreatingMessage: (
      value: SheinStudioWorkbenchStateUpdater<"creatingMessage">,
    ) => setField("creatingMessage", value),
    setDesigns: (value: SheinStudioWorkbenchStateUpdater<"designs">) =>
      setField("designs", value),
    setDraftWarning: (value: SheinStudioWorkbenchStateUpdater<"draftWarning">) =>
      setField("draftWarning", value),
    setGalleryRatioCheck: (
      value: SheinStudioWorkbenchStateUpdater<"galleryRatioCheck">,
    ) => setField("galleryRatioCheck", value),
    setGenerationError: (
      value: SheinStudioWorkbenchStateUpdater<"generationError">,
    ) => setField("generationError", value),
    setGenerationWarning: (
      value: SheinStudioWorkbenchStateUpdater<"generationWarning">,
    ) => setField("generationWarning", value),
    setGenerationWarningAction: (
      value: SheinStudioWorkbenchStateUpdater<"generationWarningAction">,
    ) => setField("generationWarningAction", value),
    setGroupedImageMode: (
      value: SheinStudioWorkbenchStateUpdater<"groupedImageMode">,
    ) => setField("groupedImageMode", value),
    setImageStrategy: (value: SheinStudioWorkbenchStateUpdater<"imageStrategy">) =>
      setField("imageStrategy", value),
    setIsCreatingTasks: (
      value: SheinStudioWorkbenchStateUpdater<"isCreatingTasks">,
    ) => setField("isCreatingTasks", value),
    setIsGenerating: (value: SheinStudioWorkbenchStateUpdater<"isGenerating">) =>
      setField("isGenerating", value),
    setIsLoadingWorkspace: (
      value: SheinStudioWorkbenchStateUpdater<"isLoadingWorkspace">,
    ) => setField("isLoadingWorkspace", value),
    setProductImageCount: (
      value: SheinStudioWorkbenchStateUpdater<"productImageCount">,
    ) => setField("productImageCount", value),
    setProductImagePrompt: (
      value: SheinStudioWorkbenchStateUpdater<"productImagePrompt">,
    ) => setField("productImagePrompt", value),
    setProductImagePrompts: (
      value: SheinStudioWorkbenchStateUpdater<"productImagePrompts">,
    ) => setField("productImagePrompts", value),
    setPersistedUpdatedAt: (
      value: SheinStudioWorkbenchStateUpdater<"persistedUpdatedAt">,
    ) => setField("persistedUpdatedAt", value),
    setBatchQueueMode: (
      value: SheinStudioWorkbenchStateUpdater<"batchQueueMode">,
    ) => setField("batchQueueMode", value),
    setGroupedSelections: (
      value: SheinStudioWorkbenchStateUpdater<"groupedSelections">,
    ) => setField("groupedSelections", value),
    setPrompt: (value: SheinStudioWorkbenchStateUpdater<"prompt">) =>
      setField("prompt", value),
    setPromptMode: (value: SheinStudioWorkbenchStateUpdater<"promptMode">) =>
      setField("promptMode", value),
    setQueueMessage: (value: SheinStudioWorkbenchStateUpdater<"queueMessage">) =>
      setField("queueMessage", value),
    setQueuedBatchIds: (
      value: SheinStudioWorkbenchStateUpdater<"queuedBatchIds">,
    ) => setField("queuedBatchIds", value),
    setQueuedBatchIndex: (
      value: SheinStudioWorkbenchStateUpdater<"queuedBatchIndex">,
    ) => setField("queuedBatchIndex", value),
    setRegeneratingId: (
      value: SheinStudioWorkbenchStateUpdater<"regeneratingId">,
    ) => setField("regeneratingId", value),
    setRenderSizeImagesWithSds: (
      value: SheinStudioWorkbenchStateUpdater<"renderSizeImagesWithSds">,
    ) => setField("renderSizeImagesWithSds", value),
    setSavedBatches: (value: SheinStudioWorkbenchStateUpdater<"savedBatches">) =>
      setField("savedBatches", value),
    setSaveMessage: (value: SheinStudioWorkbenchStateUpdater<"saveMessage">) =>
      setField("saveMessage", value),
    setSelectedIds: (value: SheinStudioWorkbenchStateUpdater<"selectedIds">) =>
      setField("selectedIds", value),
    setSelectedSdsImages: (
      value: SheinStudioWorkbenchStateUpdater<"selectedSdsImages">,
    ) => setField("selectedSdsImages", value),
    setSheinStoreId: (value: SheinStudioWorkbenchStateUpdater<"sheinStoreId">) =>
      setField("sheinStoreId", value),
    setStyleCount: (value: SheinStudioWorkbenchStateUpdater<"styleCount">) =>
      setField("styleCount", value),
    setTransparentBackground: (
      value: SheinStudioWorkbenchStateUpdater<"transparentBackground">,
    ) => setField("transparentBackground", value),
    setVariationIntensity: (
      value: SheinStudioWorkbenchStateUpdater<"variationIntensity">,
    ) => setField("variationIntensity", value),
  };
}

export type SheinStudioWorkbenchSetters = ReturnType<
  typeof buildSheinStudioWorkbenchSetters
>;

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

function parseWorkbenchTimestamp(value?: string) {
  const parsed = Date.parse(value ?? "");
  return Number.isFinite(parsed) ? parsed : null;
}

function isOlderWorkbenchTimestamp(
  candidate?: string,
  current?: string,
) {
  const candidateTime = parseWorkbenchTimestamp(candidate);
  const currentTime = parseWorkbenchTimestamp(current);
  if (candidateTime == null || currentTime == null) {
    return false;
  }
  return candidateTime < currentTime;
}

function upsertSavedBatchSnapshot(
  batches: SheinStudioSavedBatch[],
  nextBatch: SheinStudioSavedBatch,
) {
  return [nextBatch, ...batches.filter((batch) => batch.id !== nextBatch.id)].sort(
    (left, right) => right.updatedAt.localeCompare(left.updatedAt),
  );
}

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
          itemizedBatchDetail: state.itemizedBatchDetail,
          persistedUpdatedAt:
            "updatedAt" in action.draft && typeof action.draft.updatedAt === "string"
              ? action.draft.updatedAt
              : state.persistedUpdatedAt,
        },
        action.draft.groups ?? state.groups,
        action.draft.activeGroupId ?? state.activeGroupId,
      );
    case "apply-batch":
      {
        const batchPatch = projectSavedBatchToWorkbench(action.batch);
        return projectActiveGroupIntoState(
          {
            ...state,
            ...batchPatch,
          },
          action.batch.groups ?? state.groups,
        );
      }
    case "apply-hydrated-batch":
      {
        if (
          state.itemizedBatchDetail?.batch.id === action.batch.detail.batch.id &&
          isOlderWorkbenchTimestamp(
            action.batch.detail.batch.updatedAt,
            state.itemizedBatchDetail.batch.updatedAt || state.persistedUpdatedAt,
          )
        ) {
          return state;
        }
        const hydratedPatch = projectHydratedBatchToWorkbench(action.batch);
        const hydratedGroups = action.batch.savedBatch.groups ?? state.groups;
        const hydratedCompatibilitySnapshot = {
          ...(action.batch.savedBatch.legacyCompatibilitySnapshot ?? {}),
          designs: hydratedPatch.designs,
          selectedIds: hydratedPatch.selectedIds,
          createdTasks: hydratedPatch.createdTasks,
          generationJobs: hydratedPatch.generationJobs,
          generationError: hydratedPatch.generationError,
          generationJobId:
            hydratedPatch.generationJobs.length > 0
              ? (action.batch.savedBatch.generationJobId ??
                action.batch.savedBatch.legacyCompatibilitySnapshot?.generationJobId)
              : "",
        };
        const hydratedSavedBatch: SheinStudioSavedBatch = {
          ...action.batch.savedBatch,
          designs: hydratedPatch.designs,
          selectedIds: hydratedPatch.selectedIds,
          createdTasks: hydratedPatch.createdTasks,
          generationJobs: hydratedPatch.generationJobs,
          legacyCompatibilitySnapshot: hydratedCompatibilitySnapshot,
          generationError: hydratedPatch.generationError,
          generationJobId: hydratedCompatibilitySnapshot.generationJobId,
          batchStatus: action.batch.detail.batch.status,
          draftUpdatedAt:
            action.batch.detail.batch.draftUpdatedAt ??
            action.batch.savedBatch.draftUpdatedAt ??
            action.batch.savedBatch.updatedAt,
          updatedAt: action.batch.savedBatch.updatedAt,
        };
        const baseState = {
          ...state,
          ...hydratedPatch,
          groups: hydratedGroups,
          savedBatches: upsertSavedBatchSnapshot(
            state.savedBatches,
            hydratedSavedBatch,
          ),
        };
        const syncedGroups = syncActiveGroupFromState(baseState, hydratedPatch);
        const projected = projectActiveGroupIntoState(
          {
            ...baseState,
            groups: syncedGroups,
          },
          syncedGroups,
        );
        return {
          ...projected,
          itemizedBatchDetail: hydratedPatch.itemizedBatchDetail,
          designs: hydratedPatch.designs,
          selectedIds: hydratedPatch.selectedIds,
          createdTasks: hydratedPatch.createdTasks,
          persistedUpdatedAt: hydratedPatch.persistedUpdatedAt,
        };
      }
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

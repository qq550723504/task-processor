import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioCreatedTask,
  SheinStudioFailedTask,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioRejectedTask,
  SheinStudioSelectedSDSImage,
} from "@/lib/types/shein-studio";
import type {
  GroupedSheinTaskCreationWarning,
} from "@/lib/shein-studio/create-review-tasks";
import type { SheinStudioBatchTaskCreationResult } from "@/lib/api/shein-studio-batches";

type TaskCreationGroupSelection = {
  selection: SDSProductVariantSelection;
  baselineStatus: GroupedSDSSelectionEligibility["baselineStatus"];
  baselineReason: string;
  eligible: boolean;
  eligibilityReason?: string;
};

type StandaloneCreateTasksInput = {
  prompt: string;
  sheinStoreId: string;
  imageStrategy?: SheinStudioImageStrategy;
  selectedSdsImages?: SheinStudioSelectedSDSImage[];
  productImageCount?: string;
  productImagePrompt?: string;
  productImagePrompts?: SheinStudioProductImagePrompt[];
  renderSizeImagesWithSds?: boolean;
  selection?: SDSProductVariantSelection;
  approvedDesigns: SheinStudioGeneratedDesign[];
  onProgress?: (message: string) => void;
};

type StandaloneCreateGroupedTasksInput = {
  prompt: string;
  imageStrategy?: SheinStudioImageStrategy;
  groupedImageMode?: SheinStudioGroupedImageMode;
  selectedSdsImages?: SheinStudioSelectedSDSImage[];
  productImageCount?: string;
  productImagePrompt?: string;
  productImagePrompts?: SheinStudioProductImagePrompt[];
  renderSizeImagesWithSds?: boolean;
  onProgress?: (message: string) => void;
  groups: Array<{
    sheinStoreId: string;
    selections: TaskCreationGroupSelection[];
    approvedDesigns: SheinStudioGeneratedDesign[];
  }>;
};

type StandaloneTaskCreationPersistDraft = (
  overrides?: Partial<{ createdTasks: SheinStudioCreatedTask[] }>,
  options?: {
    navigationTriggered?: boolean;
    source?: string;
  },
) => Promise<unknown>;

type ExecuteStandaloneTaskCreationInput = {
  activeSelection: SDSProductVariantSelection;
  activeSelectionBaselineReason: string;
  activeSelectionBaselineStatus: GroupedSDSSelectionEligibility["baselineStatus"];
  approvedDesigns: SheinStudioGeneratedDesign[];
  createGroupedTasks: (
    input: StandaloneCreateGroupedTasksInput,
  ) => Promise<{
    created: SheinStudioCreatedTask[];
    warnings: GroupedSheinTaskCreationWarning[];
  }>;
  createTasks: (
    input: StandaloneCreateTasksInput,
  ) => Promise<SheinStudioCreatedTask[]>;
  groupedImageMode: SheinStudioGroupedImageMode;
  groupedSelections: GroupedSDSSelectionEligibility[];
  hasLocalWorkflowStateRef: { current: boolean };
  imageStrategy: SheinStudioImageStrategy;
  navigateToStep: (step: "tasks") => void;
  persistDraft: StandaloneTaskCreationPersistDraft;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  prompt: string;
  renderSizeImagesWithSds: boolean;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  setCreatedTasks: (value: SheinStudioCreatedTask[]) => void;
  setCreatingMessage: (value: string) => void;
  setCreatingWarning: (value: string) => void;
  sheinStoreId: string;
};

type ExecuteStandaloneTaskCreationResult = {
  availableTasks: SheinStudioCreatedTask[];
  warnings: GroupedSheinTaskCreationWarning[];
};

type CreateBatchTasksOptions = {
  tenantId?: string;
  allowPartialWhileGenerating?: boolean;
};

type ExecuteItemizedBatchTaskCreationInput = {
  allowPartialWhileGenerating: boolean;
  approvedDesignIds: string[];
  batchId: string;
  createBatchTasks: (
    batchId: string,
    approvedDesignIds: string[],
    options?: CreateBatchTasksOptions,
  ) => Promise<SheinStudioBatchTaskCreationResult>;
  onCreated: (result: SheinStudioBatchTaskCreationResult) => void;
  tenantId?: string;
};

type ExecuteItemizedBatchTaskCreationResult = {
  created: SheinStudioCreatedTask[];
  reused: SheinStudioCreatedTask[];
  rejected: SheinStudioRejectedTask[];
  failed: SheinStudioFailedTask[];
  keepCreatingState: boolean;
  rawResult: SheinStudioBatchTaskCreationResult;
};

export function resolveTaskCreationStartValidation({
  activeSelection,
  approvedCount,
  sheinStoreId,
}: {
  activeSelection?: SDSProductVariantSelection;
  approvedCount: number;
  sheinStoreId: string;
}) {
  if (!activeSelection?.variantId) {
    return { error: "请先选择 SDS 变体。" };
  }
  if (!sheinStoreId.trim()) {
    return { error: "请先选择批次店铺。" };
  }
  if (approvedCount === 0) {
    return { error: "请至少批准 1 个款式后再创建 SHEIN 任务。" };
  }
  return null;
}

export function buildBatchTaskCreationFailureSummary(
  failedTasks: SheinStudioFailedTask[],
  rejectedTasks: SheinStudioRejectedTask[] = [],
) {
  if (failedTasks.length === 0 && rejectedTasks.length === 0) {
    return "";
  }
  const rejectedPreview = rejectedTasks
    .slice(0, 3)
    .map(
      (task) =>
        `${task.title?.trim() || task.designId}: ${
          task.reasonCode ? `${task.reasonCode} · ` : ""
        }${task.message ?? "候选不满足创建条件"}`,
    )
    .join("；");
  const failedPreview = failedTasks
    .slice(0, Math.max(0, 3 - rejectedTasks.length))
    .map(
      (task) =>
        `${task.title}: ${task.reasonCode ? `${task.reasonCode} · ` : ""}${
          task.message
        }`,
    )
    .join("；");
  const preview = [rejectedPreview, failedPreview].filter(Boolean).join("；");
  const total = rejectedTasks.length + failedTasks.length;
  const suffix = total > 3 ? ` 等 ${total} 个任务` : "";
  if (failedTasks.length === 0) {
    return `部分任务被拒绝：${preview}${suffix}`;
  }
  if (rejectedTasks.length === 0) {
    return `部分任务创建失败：${preview}${suffix}`;
  }
  return `部分任务被拒绝或创建失败：${preview}${suffix}`;
}

export function buildGroupedTaskCreationWarningSummary(
  warnings: GroupedSheinTaskCreationWarning[],
) {
  if (warnings.length === 0) {
    return "";
  }
  const labels = warnings.map((warning) => warning.label.trim()).filter(Boolean);
  const preview = labels.slice(0, 5).join("、");
  const suffix =
    labels.length > 5
      ? ` 等 ${labels.length} 款商品`
      : labels.length > 1
        ? ` 共 ${labels.length} 款商品`
        : "";
  return `有 ${warnings.length} 款商品因为没有匹配到自己的款式图而被跳过：${preview}${suffix}。这些商品不会创建错误任务，你可以回到生成区补图后再重试。`;
}

export function groupTaskCreationSelectionsByStore(
  items: GroupedSDSSelectionEligibility[],
) {
  const byStore = new Map<
    string,
    { sheinStoreId: string; items: GroupedSDSSelectionEligibility[] }
  >();
  for (const item of items) {
    const key = item.sheinStoreId.trim();
    const existing = byStore.get(key);
    if (existing) {
      existing.items.push(item);
      continue;
    }
    byStore.set(key, { sheinStoreId: key, items: [item] });
  }
  return [...byStore.values()];
}

export async function executeStandaloneTaskCreation({
  activeSelection,
  activeSelectionBaselineReason,
  activeSelectionBaselineStatus,
  approvedDesigns,
  createGroupedTasks,
  createTasks,
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
  renderSizeImagesWithSds,
  selectedSdsImages,
  setCreatedTasks,
  setCreatingMessage,
  setCreatingWarning,
  sheinStoreId,
}: ExecuteStandaloneTaskCreationInput): Promise<ExecuteStandaloneTaskCreationResult> {
  let created: SheinStudioCreatedTask[] = [];
  let warnings: GroupedSheinTaskCreationWarning[] = [];

  if (groupedSelections.length > 0) {
    const result = await createGroupedTasks({
      prompt,
      groupedImageMode,
      imageStrategy,
      selectedSdsImages,
      productImageCount,
      productImagePrompt,
      productImagePrompts,
      renderSizeImagesWithSds,
      onProgress: setCreatingMessage,
      groups: [
        {
          sheinStoreId,
          selections: [
            {
              selection: activeSelection,
              baselineStatus: activeSelectionBaselineStatus,
              baselineReason: activeSelectionBaselineReason,
              eligible: true,
            },
          ],
          approvedDesigns,
        },
        ...groupTaskCreationSelectionsByStore(groupedSelections).map((group) => ({
          sheinStoreId: group.sheinStoreId,
          selections: group.items.map((item) => ({
            selection: item.selection,
            baselineStatus: item.baselineStatus,
            baselineReason: item.baselineReason,
            eligible: item.eligible,
            eligibilityReason: item.eligibilityReason,
          })),
          approvedDesigns,
        })),
      ],
    });
    created = result.created;
    warnings = result.warnings;
  } else {
    created = await createTasks({
      prompt,
      sheinStoreId,
      imageStrategy,
      selectedSdsImages,
      productImageCount,
      productImagePrompt,
      productImagePrompts,
      renderSizeImagesWithSds,
      selection: activeSelection,
      approvedDesigns,
      onProgress: setCreatingMessage,
    });
  }

  hasLocalWorkflowStateRef.current = true;
  setCreatedTasks(created);
  setCreatingMessage(
    groupedSelections.length > 0
      ? `已为 ${created.length} 个 SDS 商品生成或复用 SHEIN 资料任务。请在下方打开并审核。`
      : `已生成或复用 ${created.length} 个 SHEIN 资料任务。请在下方打开并审核。`,
  );
  setCreatingWarning(buildGroupedTaskCreationWarningSummary(warnings));

  if (created.length > 0) {
    navigateToStep("tasks");
    await persistDraft(
      { createdTasks: created },
      {
        navigationTriggered: true,
        source: "task_creation_success",
      },
    ).catch(() => undefined);
  }

  return { availableTasks: created, warnings };
}

export async function executeItemizedBatchTaskCreation({
  allowPartialWhileGenerating,
  approvedDesignIds,
  batchId,
  createBatchTasks,
  onCreated,
  tenantId,
}: ExecuteItemizedBatchTaskCreationInput): Promise<ExecuteItemizedBatchTaskCreationResult> {
  const trimmedTenantId = tenantId?.trim();
  const requestOptions = {
    ...(trimmedTenantId ? { tenantId: trimmedTenantId } : {}),
    ...(allowPartialWhileGenerating
      ? { allowPartialWhileGenerating: true }
      : {}),
  };
  const result =
    Object.keys(requestOptions).length > 0
      ? await createBatchTasks(batchId, approvedDesignIds, requestOptions)
      : await createBatchTasks(batchId, approvedDesignIds);
  onCreated(result);
  return {
    created: result.createdTasks,
    reused: result.reusedTasks ?? [],
    rejected: result.rejectedTasks ?? [],
    failed: result.failedTasks ?? [],
    keepCreatingState: result.batch.status === "tasks_creating",
    rawResult: result,
  };
}

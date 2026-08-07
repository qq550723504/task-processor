import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { SheinStudioGenerationFormModel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import {
  applyBaselineWarmupResult,
  buildSheinStudioGenerationPanelProps,
  projectActiveSelectionBaselineState,
  projectBaselineWarmupFeedback,
  resolveBaselineReadinessEntries,
  runBaselineWarmup,
  type SheinStudioGenerationPanelActionProjection,
  type SheinStudioGenerationPanelProjectionInput,
  type SheinStudioGenerationPanelStatusProjection,
  useActiveSelectionBaselineStatuses,
  useBaselineWarmupAction,
  useSheinStudioBatchGenerationContext,
} from "@/components/listingkit/shein-studio/shein-studio-generation-controller";
import type { SheinStudioSavedBatch } from "@/lib/types/shein-studio";
import type { SheinStudioSaveInput } from "@/lib/utils/shein-studio-batches";

function buildSavedBatch(id = "batch-1"): SheinStudioSavedBatch {
  return {
    id,
    name: "Batch 1",
    prompt: "prompt",
    styleCount: "1",
    sheinStoreId: "869",
    designs: [],
    selectedIds: [],
    createdTasks: [],
    updatedAt: "2026-06-22T00:00:00.000Z",
  };
}

const selection = {
  layerId: "layer-1",
  parentProductId: 1,
  productId: 10,
  productName: "Tee",
  prototypeGroupId: 20,
  selectedVariantIds: [100, 101],
  variantId: 100,
  variantLabel: "Black / M",
};

function buildGenerationPanelForm(): SheinStudioGenerationFormModel {
  return {
    artworkGenerationMode: "theme_prompt",
    artworkModel: "nanobanana",
    availableSdsImages: [],
    groupedImageMode: "shared_by_size",
    hotStyleReferenceBrief: "",
    hotStyleReferenceImageUrls: [],
    hotStyleReferencePrompt: "",
    imageStrategy: "ai_generated",
    productImageCount: "5",
    productImagePrompt: "",
    productImagePrompts: [],
    prompt: "",
    promptHistory: [],
    promptInputRef: { current: null },
    promptMode: "managed",
    renderSizeImagesWithSds: true,
    selectedSdsImages: [],
    styleCount: "2",
    transparentBackground: false,
    variationIntensity: "medium",
  };
}

function buildGenerationPanelActions(
  overrides: Partial<SheinStudioGenerationPanelActionProjection> = {},
): SheinStudioGenerationPanelActionProjection {
  const noop = () => undefined;
  return {
    analyzeReferenceStyle: async () => ({
      referenceStyleBrief: "",
      sanitizedPrompt: "",
      warnings: [],
    }),
    generate: noop,
    onCreateTasks: noop,
    onDeleteBatch: noop,
    onLoadBatch: noop,
    onRestorePrompt: noop,
    onSaveBatch: noop,
    retryFailedItem: async () => undefined,
    retryFailedItems: async () => undefined,
    setArtworkGenerationMode: noop,
    setArtworkModel: noop,
    setGroupedImageMode: noop,
    setHotStyleReferenceBrief: noop,
    setHotStyleReferenceImageUrls: noop,
    setHotStyleReferencePrompt: noop,
    setImageStrategy: noop,
    setProductImageCount: noop,
    setProductImagePrompt: noop,
    setProductImagePrompts: noop,
    setPrompt: noop,
    setPromptMode: noop,
    setRenderSizeImagesWithSds: noop,
    setSelectedSdsImages: noop,
    setStyleCount: noop,
    setTransparentBackground: noop,
    setVariationIntensity: noop,
    uploadHotStyleReferenceImages: async () => [],
    ...overrides,
  };
}

function buildGenerationPanelStatus(
  overrides: Partial<SheinStudioGenerationPanelStatusProjection> = {},
): SheinStudioGenerationPanelStatusProjection {
  return {
    activeSelection: selection,
    createdTasks: [],
    creatingError: "",
    creatingMessage: "",
    currentStoreLabel: "Store 869",
    generationError: "",
    groupedSelections: [],
    hasRetryableFailedItems: false,
    initialBatchId: undefined,
    isCreatingTasks: false,
    isGenerating: false,
    itemizedBatchDetail: undefined,
    retryableFailedItemCount: 0,
    retryingFailedItemId: "",
    savedBatches: [],
    saveMessage: "",
    selectedStyleCount: 2,
    storeRequiredMessage: "",
    subscriptionBlockedMessage: "",
    ...overrides,
  };
}

function buildGenerationPanelProjectionInput(
  overrides: {
    actions?: Partial<SheinStudioGenerationPanelActionProjection>;
    form?: SheinStudioGenerationFormModel;
    status?: Partial<SheinStudioGenerationPanelStatusProjection>;
  } = {},
): SheinStudioGenerationPanelProjectionInput {
  return {
    actions: buildGenerationPanelActions(overrides.actions),
    form: overrides.form ?? buildGenerationPanelForm(),
    status: buildGenerationPanelStatus(overrides.status),
  };
}

describe("buildSheinStudioGenerationPanelProps", () => {
  it("projects the normal generation panel contract", () => {
    const generate = vi.fn();
    const retryFailedItem = vi.fn().mockResolvedValue(undefined);
    const groupedSelection = {
      baselineReason: "",
      baselineStatus: "ready" as const,
      eligible: true,
      selection: {
        ...selection,
        selectedVariantIds: [102],
        variantId: 102,
        variantLabel: "White / L",
      },
      selectionId: "grouped-102",
      sheinStoreId: "869",
    };
    const input = buildGenerationPanelProjectionInput({
      actions: { generate, retryFailedItem },
      status: {
        currentStoreLabel: "",
        groupedSelections: [groupedSelection],
      },
    });

    const result = buildSheinStudioGenerationPanelProps(input);
    result.actions.onRetryFailedItem?.("failed-item-1");

    expect(result.actions.onGenerate).toBe(generate);
    expect(retryFailedItem).toHaveBeenCalledWith("failed-item-1");
    expect(result.form).toBe(input.form);
    expect(result.status).toMatchObject({
      batchProductCount: 2,
      batchStoreLabel: "未设置",
      createTaskButtonLabel: "为 2 款商品生成 SHEIN 资料",
      failedBatchItems: [],
      generateButtonLabel: "生成款式图",
      generationNotice: "",
      isRetryingFailedItems: false,
      selectedStyleCount: 2,
      selectionReady: true,
      showSavedBatches: true,
    });
  });

  it("projects retry mode as the generation action", () => {
    const generate = vi.fn();
    const retryFailedItems = vi.fn().mockResolvedValue(undefined);
    const input = buildGenerationPanelProjectionInput({
      actions: { generate, retryFailedItems },
      status: {
        hasRetryableFailedItems: true,
        initialBatchId: "batch-1",
        retryableFailedItemCount: 1,
      },
    });

    const result = buildSheinStudioGenerationPanelProps(input);
    result.actions.onGenerate();

    expect(retryFailedItems).toHaveBeenCalledTimes(1);
    expect(generate).not.toHaveBeenCalled();
    expect(result.status).toMatchObject({
      failedBatchItems: [],
      generateButtonLabel: "重试失败批次",
      generationNotice:
        "当前批次有 1 个失败项。点击“重试失败批次”只会重试失败部分，不会重复生成已成功内容。",
      isRetryingFailedItems: true,
      showSavedBatches: false,
    });
  });

  it("projects only failed entries as retryable batch items", () => {
    const failedItem = {
      batchId: "batch-1",
      createdAt: "2026-08-08T00:00:00.000Z",
      id: "item-failed",
      lastError: "upstream timeout",
      selectionCount: 1,
      status: "failed" as const,
      targetGroupKey: "size:1000x1000",
      targetGroupLabel: "黑色 M",
      updatedAt: "2026-08-08T00:01:00.000Z",
    };
    const readyItem = {
      ...failedItem,
      id: "item-ready",
      lastError: undefined,
      status: "review_ready" as const,
    };
    const input = buildGenerationPanelProjectionInput({
      status: {
        hasRetryableFailedItems: true,
        itemizedBatchDetail: {
          batch: {
            createdAt: "2026-08-08T00:00:00.000Z",
            id: "batch-1",
            prompt: "studio prompt",
            sheinStoreId: 869,
            status: "partially_failed",
            styleCount: "2",
            updatedAt: "2026-08-08T00:01:00.000Z",
          },
          items: [
            { designs: [], item: failedItem },
            { designs: [], item: readyItem },
          ],
        },
        retryableFailedItemCount: 1,
      },
    });

    const result = buildSheinStudioGenerationPanelProps(input);

    expect(result.status.failedBatchItems).toEqual([failedItem]);
  });
});

describe("projectBaselineWarmupFeedback", () => {
  it("announces ready baseline warmup without follow-up action", () => {
    expect(
      projectBaselineWarmupFeedback({
        status: "ready",
        reason: "",
      }),
    ).toEqual({
      action: null,
      message: "这款 SDS 商品的 baseline 已通过校验，现在可以继续加入 grouped 批量上品。",
    });
  });

  it("uses the cached baseline fallback message when no reason is available", () => {
    expect(
      projectBaselineWarmupFeedback({
        status: "baseline_cached",
        reason: "",
        reasonCode: "",
      }),
    ).toEqual({
      action: {
        intent: "warm_baseline",
        label: "继续 baseline 校验",
      },
      message: "这款 SDS 商品已经完成 baseline 缓存，当前没有更多校验结果。可以继续使用，必要时再手动复查。",
    });
  });

  it("uses handoff action metadata for blocked baseline warmup", () => {
    expect(
      projectBaselineWarmupFeedback({
        status: "blocked",
        reason: "",
        reasonCode: "login_missing_credentials",
      }),
    ).toEqual({
      action: {
        intent: "open_sds_login",
        label: "去处理 SDS 登录",
      },
      message: "当前 SDS 登录缺少 access token。",
    });
  });
});

describe("projectActiveSelectionBaselineState", () => {
  it("projects a missing active baseline while readiness is loading", () => {
    expect(
      projectActiveSelectionBaselineState({
        activeGroupedSelectionID: "selection-1",
        hasActiveSelection: true,
        baselineStatuses: {},
      }),
    ).toEqual({
      baseline: {
        status: "missing",
        reason: "正在检查 baseline 状态...",
        reasonCode: undefined,
      },
      handoff: null,
      reason: "正在检查 baseline 状态...",
      resolvedBaseline: undefined,
    });
  });

  it("projects resolved active baseline reason and handoff action", () => {
    expect(
      projectActiveSelectionBaselineState({
        activeGroupedSelectionID: "selection-1",
        hasActiveSelection: true,
        baselineStatuses: {
          "selection-1": {
            status: "blocked",
            reason: "",
            reasonCode: "login_missing_credentials",
          },
        },
      }),
    ).toEqual({
      baseline: {
        status: "blocked",
        reason: "",
        reasonCode: "login_missing_credentials",
      },
      handoff: {
        action: "open_sds_login",
        actionLabel: "去处理 SDS 登录",
        message: "当前 SDS 登录缺少 access token。",
      },
      reason: "当前 SDS 登录缺少 access token。",
      resolvedBaseline: {
        status: "blocked",
        reason: "",
        reasonCode: "login_missing_credentials",
      },
    });
  });
});

describe("resolveBaselineReadinessEntries", () => {
  it("maps selection readiness responses to baseline status entries", async () => {
    const getReadiness = vi.fn().mockResolvedValue({
      baselineKey: "baseline-1",
      reason: "ready",
      reasonCode: "ok",
      status: "ready",
    });

    await expect(
      resolveBaselineReadinessEntries({
        getReadiness,
        selections: [selection],
      }),
    ).resolves.toEqual([
      [
        "1:20:100:layer-1:100,101",
        {
          baselineKey: "baseline-1",
          reason: "ready",
          reasonCode: "ok",
          status: "ready",
        },
      ],
    ]);
    expect(getReadiness).toHaveBeenCalledWith({
      parentProductId: 1,
      prototypeGroupId: 20,
      selectedVariantIds: [100, 101],
      variantId: 100,
    });
  });

  it("maps readiness failures to failed baseline status entries", async () => {
    await expect(
      resolveBaselineReadinessEntries({
        getReadiness: vi.fn().mockRejectedValue(new Error("offline")),
        selections: [selection],
      }),
    ).resolves.toEqual([
      [
        "1:20:100:layer-1:100,101",
        {
          reason: "offline",
          reasonCode: undefined,
          status: "failed",
        },
      ],
    ]);
  });
});

describe("useActiveSelectionBaselineStatuses", () => {
  it("loads readiness for the active selection and clears it when no variant is active", async () => {
    const getReadiness = vi.fn().mockResolvedValue({
      baselineKey: "baseline-1",
      reason: "ready",
      reasonCode: "ok",
      status: "ready",
    });
    type HookProps = { activeSelection: typeof selection | undefined };
    const initialProps: HookProps = { activeSelection: selection };
    const { result, rerender } = renderHook(
      ({ activeSelection }: HookProps) =>
        useActiveSelectionBaselineStatuses({
          activeSelection,
          getReadiness,
        }),
      {
        initialProps,
      },
    );

    await waitFor(() =>
      expect(result.current.baselineStatuses).toEqual({
        "1:20:100:layer-1:100,101": {
          baselineKey: "baseline-1",
          reason: "ready",
          reasonCode: "ok",
          status: "ready",
        },
      }),
    );

    rerender({ activeSelection: undefined });

    expect(result.current.baselineStatuses).toEqual({});
  });
});

describe("runBaselineWarmup", () => {
  it("returns null when there is no active variant", async () => {
    const warmBaseline = vi.fn();

    await expect(
      runBaselineWarmup({
        activeSelection: undefined,
        baselineStatuses: {},
        warmBaseline,
      }),
    ).resolves.toBeNull();
    expect(warmBaseline).not.toHaveBeenCalled();
  });

  it("warms the active selection baseline and projects feedback", async () => {
    const warmBaseline = vi.fn().mockResolvedValue({
      baselineKey: "baseline-1",
      reason: "",
      reasonCode: "",
      status: "ready",
    });

    await expect(
      runBaselineWarmup({
        activeSelection: selection,
        baselineStatuses: {
          existing: {
            reason: "existing",
            reasonCode: undefined,
            status: "missing",
          },
        },
        warmBaseline,
      }),
    ).resolves.toEqual({
      baselineStatuses: {
        existing: {
          reason: "existing",
          reasonCode: undefined,
          status: "missing",
        },
        "1:20:100:layer-1:100,101": {
          baselineKey: "baseline-1",
          reason: "",
          reasonCode: "",
          status: "ready",
        },
      },
      feedback: {
        action: null,
        message:
          "这款 SDS 商品的 baseline 已通过校验，现在可以继续加入 grouped 批量上品。",
      },
    });
    expect(warmBaseline).toHaveBeenCalledWith(selection);
  });

  it("returns warning feedback when warmup fails", async () => {
    await expect(
      runBaselineWarmup({
        activeSelection: selection,
        baselineStatuses: {},
        warmBaseline: vi.fn().mockRejectedValue(new Error("warm failed")),
      }),
    ).resolves.toEqual({
      warning: "warm failed",
    });
  });
});

describe("applyBaselineWarmupResult", () => {
  it("applies refreshed statuses and feedback when warmup returns readiness", () => {
    const setBaselineStatuses = vi.fn();
    const setGenerationWarning = vi.fn();
    const setGenerationWarningAction = vi.fn();

    applyBaselineWarmupResult({
      result: {
        baselineStatuses: {
          "selection-1": {
            reason: "",
            reasonCode: "",
            status: "ready",
          },
        },
        feedback: {
          action: {
            intent: "warm_baseline",
            label: "继续 baseline 校验",
          },
          message: "ready",
        },
      },
      setBaselineStatuses,
      setGenerationWarning,
      setGenerationWarningAction,
    });

    expect(setBaselineStatuses).toHaveBeenCalledWith({
      "selection-1": {
        reason: "",
        reasonCode: "",
        status: "ready",
      },
    });
    expect(setGenerationWarning).toHaveBeenCalledWith("ready");
    expect(setGenerationWarningAction).toHaveBeenCalledWith({
      intent: "warm_baseline",
      label: "继续 baseline 校验",
    });
  });

  it("applies warning-only warmup failures without changing statuses", () => {
    const setBaselineStatuses = vi.fn();
    const setGenerationWarning = vi.fn();
    const setGenerationWarningAction = vi.fn();

    applyBaselineWarmupResult({
      result: {
        warning: "warmup failed",
      },
      setBaselineStatuses,
      setGenerationWarning,
      setGenerationWarningAction,
    });

    expect(setBaselineStatuses).not.toHaveBeenCalled();
    expect(setGenerationWarning).toHaveBeenCalledWith("warmup failed");
    expect(setGenerationWarningAction).not.toHaveBeenCalled();
  });
});

describe("useBaselineWarmupAction", () => {
  it("clears generation errors, warms baseline, and applies feedback", async () => {
    const setBaselineStatuses = vi.fn();
    const setGenerationError = vi.fn();
    const setGenerationWarning = vi.fn();
    const setGenerationWarningAction = vi.fn();
    const warmBaseline = vi.fn().mockResolvedValue({
      baselineKey: "baseline-1",
      reason: "",
      reasonCode: "",
      status: "ready",
    });

    const { result } = renderHook(() =>
      useBaselineWarmupAction({
        activeSelection: selection,
        baselineStatuses: {},
        setBaselineStatuses,
        setGenerationError,
        setGenerationWarning,
        setGenerationWarningAction,
        warmBaseline,
      }),
    );

    await act(async () => {
      await result.current.handleWarmBaselineAction();
    });

    expect(setGenerationError).toHaveBeenCalledWith("");
    expect(warmBaseline).toHaveBeenCalledWith(selection);
    expect(setBaselineStatuses).toHaveBeenCalledWith({
      "1:20:100:layer-1:100,101": {
        baselineKey: "baseline-1",
        reason: "",
        reasonCode: "",
        status: "ready",
      },
    });
    expect(setGenerationWarning).toHaveBeenCalledWith(
      "这款 SDS 商品的 baseline 已通过校验，现在可以继续加入 grouped 批量上品。",
    );
    expect(setGenerationWarningAction).toHaveBeenCalledWith(null);
    expect(result.current.isExecutingWarningAction).toBe(false);
  });
});

describe("useSheinStudioBatchGenerationContext", () => {
  const buildDraftInput = vi.fn();
  const getHydratedBatch = vi.fn();
  const saveBatch = vi.fn();
  const setActiveBatchId = vi.fn();
  const setActiveSavedBatchId = vi.fn();
  const setActiveBatchRunId = vi.fn();
  const setBatchRunError = vi.fn();
  const setSavedBatches = vi.fn();
  const startBatchRun = vi.fn();

  beforeEach(() => {
    buildDraftInput.mockReset();
    getHydratedBatch.mockReset();
    saveBatch.mockReset();
    setActiveBatchId.mockReset();
    setActiveSavedBatchId.mockReset();
    setActiveBatchRunId.mockReset();
    setBatchRunError.mockReset();
    setSavedBatches.mockReset();
    startBatchRun.mockReset();

    buildDraftInput.mockReturnValue({
      prompt: "prompt",
      styleCount: "1",
      updatedAt: "2026-06-22T01:00:00.000Z",
    } satisfies Partial<SheinStudioSaveInput>);
    getHydratedBatch.mockResolvedValue({
      savedBatch: {
        ...buildSavedBatch("batch-existing"),
        draftUpdatedAt: "2026-06-22T02:00:00.000Z",
      },
      detail: {
        batch: {
          id: "batch-existing",
          draftUpdatedAt: "2026-06-22T03:00:00.000Z",
        },
        items: [],
      },
    });
    saveBatch.mockResolvedValue(buildSavedBatch("batch-saved"));
    startBatchRun.mockResolvedValue({
      run: { id: "run-1" },
      items: [],
    });
  });

  it("does not provide batch generation context without an active selection", () => {
    const { result } = renderHook(() =>
      useSheinStudioBatchGenerationContext({
        activeBatchId: "",
        buildDraftInput,
        currentGenerationJobId: "",
        enabled: false,
        generationError: "",
        getHydratedBatch,
        initialBatchId: "",
        saveBatch,
        setActiveBatchId,
        setActiveBatchRunId,
        setActiveSavedBatchId,
        setBatchRunError,
        setSavedBatches,
        startBatchRun,
      }),
    );

    expect(result.current.batchGenerationContext).toBeUndefined();
  });

  it("saves and activates the current batch before starting generation", async () => {
    const { result } = renderHook(() =>
      useSheinStudioBatchGenerationContext({
        activeBatchId: "",
        buildDraftInput,
        currentGenerationJobId: "job-1",
        enabled: true,
        generationError: "previous warning",
        getHydratedBatch,
        initialBatchId: "batch-existing",
        saveBatch,
        setActiveBatchId,
        setActiveBatchRunId,
        setActiveSavedBatchId,
        setBatchRunError,
        setSavedBatches,
        startBatchRun,
      }),
    );

    const saved = await result.current.batchGenerationContext?.ensureBatch();

    expect(getHydratedBatch).toHaveBeenCalledWith("batch-existing");
    expect(buildDraftInput).toHaveBeenCalledWith({
      createdTasks: [],
      designs: [],
      generationError: "previous warning",
      generationJobId: "job-1",
      generationJobs: [],
      selectedIds: [],
    });
    expect(saveBatch).toHaveBeenCalledWith(
      expect.objectContaining({
        id: "batch-existing",
        updatedAt: "2026-06-22T03:00:00.000Z",
      }),
      { makeActive: false },
    );
    expect(saved).toEqual(expect.objectContaining({ id: "batch-saved" }));
    expect(setActiveBatchId).toHaveBeenCalledWith("batch-saved");
    expect(setActiveSavedBatchId).toHaveBeenCalledWith("batch-saved");
    expect(setSavedBatches).toHaveBeenCalled();
  });

  it("starts a backend generation run for the saved batch", async () => {
    const { result } = renderHook(() =>
      useSheinStudioBatchGenerationContext({
        activeBatchId: "",
        buildDraftInput,
        currentGenerationJobId: "",
        enabled: true,
        generationError: "",
        getHydratedBatch,
        initialBatchId: "",
        saveBatch,
        setActiveBatchId,
        setActiveBatchRunId,
        setActiveSavedBatchId,
        setBatchRunError,
        setSavedBatches,
        startBatchRun,
      }),
    );

    await result.current.batchGenerationContext?.startGenerationRun(
      buildSavedBatch("batch-saved"),
    );

    await waitFor(() => expect(setActiveBatchRunId).toHaveBeenCalledWith("run-1"));
    expect(setBatchRunError).toHaveBeenCalledWith("");
    expect(startBatchRun).toHaveBeenCalledWith(["batch-saved"], "generate");
  });
});

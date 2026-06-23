import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  buildSheinStudioDraftPersistenceState,
  resetDedicatedBatchPromptOverrides,
  useSheinStudioDedicatedDraftPersistence,
} from "@/components/listingkit/shein-studio/shein-studio-persistence-controller";

describe("useSheinStudioDedicatedDraftPersistence", () => {
  const buildDraftInput = vi.fn();
  const saveLocalSnapshot = vi.fn();

  beforeEach(() => {
    buildDraftInput.mockReset();
    saveLocalSnapshot.mockReset();
    resetDedicatedBatchPromptOverrides();
    buildDraftInput.mockImplementation((overrides = {}) => ({
      prompt: "base prompt",
      designs: [],
      selectedIds: [],
      createdTasks: [],
      generationJobs: [],
      generationError: "",
      generationJobId: "",
      updatedAt: "2026-06-24T00:00:00.000Z",
      ...overrides,
    }));
  });

  it("saves a dedicated batch snapshot and records prompt overrides", () => {
    const { result, rerender } = renderHook(
      ({ batchId }) =>
        useSheinStudioDedicatedDraftPersistence({
          buildDraftInput,
          createdTasks: [],
          currentGenerationJobId: "",
          designs: [],
          generationError: "",
          generationJobs: [],
          initialBatchId: batchId,
          saveLocalSnapshot,
          selectedIds: [],
        }),
      {
        initialProps: {
          batchId: "batch-1",
        },
      },
    );

    act(() => {
      result.current.saveDedicatedBatchDraftSnapshot({
        prompt: "edited prompt",
      });
    });
    rerender({ batchId: "batch-1" });

    expect(saveLocalSnapshot).toHaveBeenCalledWith(
      expect.objectContaining({
        prompt: "edited prompt",
      }),
      {
        batchId: "batch-1",
      },
    );
    expect(result.current.promptOverride).toBe("edited prompt");
  });

  it("builds result-backed draft input from the latest generation result fields", () => {
    const designs = [{ id: "design-1" }];
    const createdTasks = [
      { id: "task-1", title: "Task 1", designId: "design-1" },
    ];
    const generationJobs = [{ jobId: "job-1", status: "succeeded" as const }];

    const { result } = renderHook(() =>
      useSheinStudioDedicatedDraftPersistence({
        buildDraftInput,
        createdTasks,
        currentGenerationJobId: "job-1",
        designs,
        generationError: "",
        generationJobs,
        initialBatchId: "batch-1",
        saveLocalSnapshot,
        selectedIds: ["design-1"],
      }),
    );

    const input = result.current.buildResultBackedDraftInput();

    expect(buildDraftInput).toHaveBeenLastCalledWith({
      createdTasks,
      designs,
      generationError: "",
      generationJobId: "job-1",
      generationJobs,
      selectedIds: ["design-1"],
    });
    expect(input).toEqual(
      expect.objectContaining({
        createdTasks,
        designs,
        generationJobId: "job-1",
        selectedIds: ["design-1"],
      }),
    );
  });
});

describe("buildSheinStudioDraftPersistenceState", () => {
  it("projects workbench values into draft persistence state", () => {
    const setDraftWarning = vi.fn();
    const setPersistedUpdatedAt = vi.fn();

    const state = buildSheinStudioDraftPersistenceState({
      activeSelection: {
        productId: 1,
        parentProductId: 1,
        variantId: 100,
        prototypeGroupId: 200,
        layerId: "layer-1",
        productName: "tee",
        variantLabel: "M / black",
      },
      artworkModel: "model-a",
      createdTasks: [],
      currentGenerationJobId: "job-1",
      designs: [{ id: "design-1" }],
      generationError: "",
      generationJobs: [{ jobId: "job-1", status: "running" }],
      groups: [],
      groupedImageMode: "shared_by_size",
      groupedSelections: [],
      imageStrategy: "hybrid",
      isCreatingTasks: false,
      isGenerating: true,
      isLoadingWorkspace: false,
      persistedUpdatedAt: "2026-06-24T00:00:00.000Z",
      productImageCount: "1",
      productImagePrompt: "image prompt",
      productImagePrompts: [],
      prompt: "prompt",
      promptMode: "managed",
      regeneratingId: "",
      renderSizeImagesWithSds: true,
      selectedIds: ["design-1"],
      selectedSdsImages: [],
      setDraftWarning,
      setPersistedUpdatedAt,
      sheinStoreId: "store-1",
      styleCount: "2",
      transparentBackground: false,
      variationIntensity: "medium",
    });

    expect(state).toMatchObject({
      artworkModel: "model-a",
      generationJobId: "job-1",
      isGenerating: true,
      prompt: "prompt",
      selectedIds: ["design-1"],
      sheinStoreId: "store-1",
    });
    expect(state.setDraftWarning).toBe(setDraftWarning);
    expect(state.setPersistedUpdatedAt).toBe(setPersistedUpdatedAt);
  });
});

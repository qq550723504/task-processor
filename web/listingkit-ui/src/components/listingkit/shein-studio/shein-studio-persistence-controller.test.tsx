import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
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

import { useCallback } from "react";

import { buildSheinStudioDraftInput } from "@/lib/shein-studio/draft-input";
import { saveLocalSheinStudioDraftSnapshot } from "@/lib/shein-studio/local-draft-cache";
import type {
  SheinStudioCreatedTask,
  SheinStudioGenerationJob,
  SheinStudioGeneratedDesign,
} from "@/lib/types/shein-studio";

type DraftInput = ReturnType<typeof buildSheinStudioDraftInput>;

type ResultBackedDraftOverrides = Partial<{
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
  generationJobs: SheinStudioGenerationJob[];
  generationError: string;
  generationJobId: string;
}>;

type BuildDraftInput = (overrides?: ResultBackedDraftOverrides) => DraftInput;

type DedicatedDraftPersistenceParams = {
  buildDraftInput: BuildDraftInput;
  createdTasks: SheinStudioCreatedTask[];
  currentGenerationJobId: string;
  designs: SheinStudioGeneratedDesign[];
  generationError: string;
  generationJobs: SheinStudioGenerationJob[];
  initialBatchId?: string;
  saveLocalSnapshot?: typeof saveLocalSheinStudioDraftSnapshot;
  selectedIds: string[];
};

const dedicatedBatchPromptOverrides = new Map<string, string>();

export function resetDedicatedBatchPromptOverrides() {
  dedicatedBatchPromptOverrides.clear();
}

export function getDedicatedBatchPromptOverride(batchId?: string) {
  const resolvedBatchId = batchId?.trim();
  return resolvedBatchId
    ? dedicatedBatchPromptOverrides.get(resolvedBatchId)
    : undefined;
}

export function useSheinStudioDedicatedDraftPersistence({
  buildDraftInput,
  createdTasks,
  currentGenerationJobId,
  designs,
  generationError,
  generationJobs,
  initialBatchId,
  saveLocalSnapshot = saveLocalSheinStudioDraftSnapshot,
  selectedIds,
}: DedicatedDraftPersistenceParams) {
  const buildResultBackedDraftInput = useCallback(
    () =>
      buildDraftInput({
        designs,
        selectedIds,
        createdTasks,
        generationJobs,
        generationError,
        generationJobId: currentGenerationJobId,
      }),
    [
      buildDraftInput,
      createdTasks,
      currentGenerationJobId,
      designs,
      generationError,
      generationJobs,
      selectedIds,
    ],
  );

  const saveDedicatedBatchDraftSnapshot = useCallback(
    (overrides?: Partial<DraftInput>) => {
      if (!initialBatchId) {
        return;
      }
      if (typeof overrides?.prompt === "string") {
        dedicatedBatchPromptOverrides.set(initialBatchId, overrides.prompt);
      }
      saveLocalSnapshot(
        {
          ...buildDraftInput(),
          ...overrides,
        },
        {
          batchId: initialBatchId,
        },
      );
    },
    [buildDraftInput, initialBatchId, saveLocalSnapshot],
  );

  return {
    buildResultBackedDraftInput,
    promptOverride: getDedicatedBatchPromptOverride(initialBatchId),
    saveDedicatedBatchDraftSnapshot,
  };
}

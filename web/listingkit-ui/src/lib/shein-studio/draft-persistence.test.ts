import { describe, expect, it, vi } from "vitest";

import {
  appendDraftSaveWarning,
  getSheinStudioAutosaveDelayMs,
  runSheinStudioDraftAutosave,
  clearDraftSaveWarning,
  persistSheinStudioDraft,
} from "@/lib/shein-studio/draft-persistence";
import type { SheinStudioSaveInput } from "@/lib/utils/shein-studio-batches";

function buildDraftInput(
  overrides: Partial<SheinStudioSaveInput> = {},
): SheinStudioSaveInput {
  return {
    prompt: "draft prompt",
    styleCount: "1",
    variationIntensity: "medium",
    productImageCount: "5",
    productImagePrompt: "",
    productImagePrompts: [],
    artworkModel: "nanobanana",
    transparentBackground: false,
    sheinStoreId: "869",
    imageStrategy: "ai_generated",
    groupedImageMode: "shared_by_size",
    selectedSdsImages: [],
    renderSizeImagesWithSds: true,
    groupedSelections: [],
    ...overrides,
  };
}

describe("SHEIN Studio draft persistence", () => {
  it("appends and clears the draft save warning idempotently", () => {
    const warning = appendDraftSaveWarning("已有提示");

    expect(appendDraftSaveWarning(warning)).toBe(warning);
    expect(clearDraftSaveWarning(warning)).toBe("已有提示");
    expect(clearDraftSaveWarning(appendDraftSaveWarning(""))).toBe("");
  });

  it("persists an active batch through batch save while keeping local snapshots in sync", async () => {
    const draftInput = buildDraftInput({ updatedAt: "2026-06-22T00:00:00.000Z" });
    const savedDraft = buildDraftInput({
      id: "batch-1",
      prompt: "saved prompt",
      updatedAt: "2026-06-22T00:00:01.000Z",
    });
    const saveLocalSnapshot = vi.fn();
    const saveBatch = vi.fn().mockResolvedValue(savedDraft);
    const saveDraft = vi.fn();
    const setPersistedUpdatedAt = vi.fn();
    const setDraftWarning = vi.fn();

    const result = await persistSheinStudioDraft({
      activeBatchId: "batch-1",
      draftInput,
      saveLocalSnapshot,
      saveBatch,
      saveDraft,
      setPersistedUpdatedAt,
      setDraftWarning,
    });

    expect(result).toBe(savedDraft);
    expect(saveLocalSnapshot).toHaveBeenNthCalledWith(1, draftInput, {
      batchId: "batch-1",
    });
    expect(saveBatch).toHaveBeenCalledWith(
      { ...draftInput, id: "batch-1" },
      { makeActive: false },
    );
    expect(saveDraft).not.toHaveBeenCalled();
    expect(saveLocalSnapshot).toHaveBeenNthCalledWith(2, savedDraft, {
      batchId: "batch-1",
    });
    expect(setPersistedUpdatedAt).toHaveBeenCalledWith(
      "2026-06-22T00:00:01.000Z",
    );
    expect(setDraftWarning.mock.calls[0]?.[0]("old warning")).toBe("old warning");
  });

  it("persists a standalone draft through the draft save dependency", async () => {
    const draftInput = buildDraftInput();
    const savedDraft = buildDraftInput({
      updatedAt: "2026-06-22T00:00:02.000Z",
    });
    const options = { source: "autosave", warnOnFailure: false };
    const saveDraft = vi.fn().mockResolvedValue(savedDraft);

    await persistSheinStudioDraft({
      activeBatchId: "",
      draftInput,
      options,
      saveLocalSnapshot: vi.fn(),
      saveBatch: vi.fn(),
      saveDraft,
      setPersistedUpdatedAt: vi.fn(),
      setDraftWarning: vi.fn(),
    });

    expect(saveDraft).toHaveBeenCalledWith(draftInput, options);
  });

  it("returns null without warning when an aborted save rejects", async () => {
    const controller = new AbortController();
    controller.abort();
    const setDraftWarning = vi.fn();

    const result = await persistSheinStudioDraft({
      activeBatchId: "",
      draftInput: buildDraftInput(),
      options: { signal: controller.signal },
      saveLocalSnapshot: vi.fn(),
      saveBatch: vi.fn(),
      saveDraft: vi.fn().mockRejectedValue(new Error("aborted")),
      setPersistedUpdatedAt: vi.fn(),
      setDraftWarning,
    });

    expect(result).toBeNull();
    expect(setDraftWarning).not.toHaveBeenCalled();
  });

  it("warns on failed saves unless warnOnFailure is false", async () => {
    const error = new Error("timeout");
    const setDraftWarning = vi.fn();

    await expect(
      persistSheinStudioDraft({
        activeBatchId: "",
        draftInput: buildDraftInput(),
        saveLocalSnapshot: vi.fn(),
        saveBatch: vi.fn(),
        saveDraft: vi.fn().mockRejectedValue(error),
        setPersistedUpdatedAt: vi.fn(),
        setDraftWarning,
      }),
    ).rejects.toBe(error);

    expect(setDraftWarning.mock.calls[0]?.[0]("")).toBe(
      appendDraftSaveWarning(""),
    );

    const quietWarning = vi.fn();
    await expect(
      persistSheinStudioDraft({
        activeBatchId: "",
        draftInput: buildDraftInput(),
        options: { warnOnFailure: false },
        saveLocalSnapshot: vi.fn(),
        saveBatch: vi.fn(),
        saveDraft: vi.fn().mockRejectedValue(error),
        setPersistedUpdatedAt: vi.fn(),
        setDraftWarning: quietWarning,
      }),
    ).rejects.toBe(error);

    expect(quietWarning).not.toHaveBeenCalled();
  });

  it("uses a shorter autosave delay for active batch drafts", () => {
    expect(getSheinStudioAutosaveDelayMs("batch-1")).toBe(250);
    expect(getSheinStudioAutosaveDelayMs("")).toBe(1200);
  });

  it("skips autosave when persistence is disabled, workspace is loading, or work is busy", () => {
    const saveLocalSnapshot = vi.fn();
    const setDraftWarning = vi.fn();
    const fingerprintRef = { current: "" };

    for (const state of [
      { persistenceEnabled: false },
      { isLoadingWorkspace: true },
      { isGenerating: true },
      { isCreatingTasks: true },
      { regeneratingId: "design-1" },
    ]) {
      expect(
        runSheinStudioDraftAutosave({
          activeBatchId: "",
          draftInput: buildDraftInput(),
          fingerprintRef,
          persistenceEnabled: state.persistenceEnabled ?? true,
          isLoadingWorkspace: Boolean(state.isLoadingWorkspace),
          isGenerating: Boolean(state.isGenerating),
          isCreatingTasks: Boolean(state.isCreatingTasks),
          regeneratingId: state.regeneratingId ?? "",
          saveLocalSnapshot,
          setDraftWarning,
        }),
      ).toBe(false);
    }

    expect(saveLocalSnapshot).not.toHaveBeenCalled();
    expect(setDraftWarning).not.toHaveBeenCalled();
  });

  it("autosaves changed drafts once and clears stale draft warnings", () => {
    const draftInput = buildDraftInput({ prompt: "changed" });
    const saveLocalSnapshot = vi.fn();
    const setDraftWarning = vi.fn();
    const fingerprintRef = { current: "" };

    expect(
      runSheinStudioDraftAutosave({
        activeBatchId: "batch-1",
        draftInput,
        fingerprintRef,
        persistenceEnabled: true,
        isLoadingWorkspace: false,
        isGenerating: false,
        isCreatingTasks: false,
        regeneratingId: "",
        saveLocalSnapshot,
        setDraftWarning,
      }),
    ).toBe(true);

    expect(saveLocalSnapshot).toHaveBeenCalledWith(draftInput, {
      batchId: "batch-1",
    });
    expect(setDraftWarning.mock.calls[0]?.[0](appendDraftSaveWarning(""))).toBe(
      "",
    );

    expect(
      runSheinStudioDraftAutosave({
        activeBatchId: "batch-1",
        draftInput,
        fingerprintRef,
        persistenceEnabled: true,
        isLoadingWorkspace: false,
        isGenerating: false,
        isCreatingTasks: false,
        regeneratingId: "",
        saveLocalSnapshot,
        setDraftWarning,
      }),
    ).toBe(false);
    expect(saveLocalSnapshot).toHaveBeenCalledTimes(1);
  });

  it("allows active batch autosave while the workspace is loading", () => {
    const saveLocalSnapshot = vi.fn();

    expect(
      runSheinStudioDraftAutosave({
        activeBatchId: "batch-1",
        draftInput: buildDraftInput(),
        fingerprintRef: { current: "" },
        persistenceEnabled: true,
        isLoadingWorkspace: true,
        isGenerating: false,
        isCreatingTasks: false,
        regeneratingId: "",
        saveLocalSnapshot,
        setDraftWarning: vi.fn(),
      }),
    ).toBe(true);
  });
});

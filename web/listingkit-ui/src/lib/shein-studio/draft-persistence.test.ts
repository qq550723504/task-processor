import { describe, expect, it, vi } from "vitest";

import {
  appendDraftSaveWarning,
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
});

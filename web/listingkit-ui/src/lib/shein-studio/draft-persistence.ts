import type { SheinStudioSaveInput } from "@/lib/utils/shein-studio-batches";

export const DRAFT_SAVE_WARNING =
  "款式图已生成，但草稿保存失败，刷新后可能丢失。可继续审核，或先保存批次。";

export type DraftSaveOptions = {
  navigationTriggered?: boolean;
  source?: string;
  signal?: AbortSignal;
  warnOnFailure?: boolean;
};

type PersistedDraft = (SheinStudioSaveInput & { updatedAt?: string }) | null;

type PersistSheinStudioDraftInput = {
  activeBatchId: string;
  draftInput: SheinStudioSaveInput;
  options?: DraftSaveOptions;
  saveLocalSnapshot: (
    input: SheinStudioSaveInput | PersistedDraft,
    options?: { batchId?: string },
  ) => void;
  saveBatch: (
    input: SheinStudioSaveInput,
    options?: { makeActive?: boolean },
  ) => Promise<PersistedDraft>;
  saveDraft: (
    input: SheinStudioSaveInput,
    options?: DraftSaveOptions,
  ) => Promise<PersistedDraft>;
  setPersistedUpdatedAt: (value: string) => void;
  setDraftWarning: (value: (current: string) => string) => void;
};

export function appendDraftSaveWarning(current: string) {
  if (current.includes(DRAFT_SAVE_WARNING)) {
    return current;
  }
  return current ? `${current} ${DRAFT_SAVE_WARNING}` : DRAFT_SAVE_WARNING;
}

export function clearDraftSaveWarning(current: string) {
  return current.replace(DRAFT_SAVE_WARNING, "").trim();
}

export async function persistSheinStudioDraft({
  activeBatchId,
  draftInput,
  options,
  saveLocalSnapshot,
  saveBatch,
  saveDraft,
  setPersistedUpdatedAt,
  setDraftWarning,
}: PersistSheinStudioDraftInput) {
  saveLocalSnapshot(draftInput, {
    batchId: activeBatchId,
  });
  try {
    const draft = activeBatchId
      ? await saveBatch(
          {
            ...draftInput,
            id: activeBatchId,
          },
          { makeActive: false },
        )
      : await saveDraft(draftInput, options);
    saveLocalSnapshot(draft, {
      batchId: activeBatchId,
    });
    if (draft?.updatedAt) {
      setPersistedUpdatedAt(draft.updatedAt);
    }
    setDraftWarning((current) => clearDraftSaveWarning(current));
    return draft;
  } catch (error) {
    if (options?.signal?.aborted) {
      return null;
    }
    if (options?.warnOnFailure !== false) {
      setDraftWarning((current) => appendDraftSaveWarning(current));
    }
    throw error;
  }
}

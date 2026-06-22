import { normalizeDraft } from "@/lib/shein-studio/storage-shared";
import type { SheinStudioDraft } from "@/lib/types/shein-studio";
import type { SheinStudioSaveInput } from "@/lib/utils/shein-studio-batches";

const LOCAL_DRAFT_SNAPSHOT_KEY = "listingkit:shein-studio:recent-draft";

type LocalDraftSnapshotInput =
  | SheinStudioSaveInput
  | SheinStudioDraft
  | Partial<SheinStudioDraft>
  | null
  | undefined;

export type LocalDraftSnapshotPayload = {
  batchId?: string;
  draft: SheinStudioDraft;
};

function canUseLocalDraftSnapshot() {
  return typeof window !== "undefined" && typeof window.localStorage !== "undefined";
}

function parseLocalSheinStudioDraftSnapshot() {
  if (!canUseLocalDraftSnapshot()) {
    return null;
  }
  const raw = window.localStorage.getItem(LOCAL_DRAFT_SNAPSHOT_KEY);
  if (!raw) {
    return null;
  }
  try {
    const parsed = JSON.parse(raw) as {
      batchId?: unknown;
      draft?: unknown;
    };
    const normalizedDraft = normalizeDraft(
      parsed && typeof parsed === "object" && "draft" in parsed
        ? (parsed.draft as Partial<SheinStudioDraft> | null | undefined)
        : (parsed as Partial<SheinStudioDraft> | null | undefined),
    );
    if (!normalizedDraft) {
      return null;
    }
    return {
      batchId: typeof parsed?.batchId === "string" ? parsed.batchId : undefined,
      draft: normalizedDraft,
    } satisfies LocalDraftSnapshotPayload;
  } catch (error) {
    console.warn(
      "shein studio local draft snapshot parse failed",
      error instanceof Error ? error.message : error,
    );
    return null;
  }
}

export function loadLocalSheinStudioDraftSnapshot() {
  return parseLocalSheinStudioDraftSnapshot()?.draft ?? null;
}

export function loadLocalSheinStudioDraftSnapshotDetail() {
  return parseLocalSheinStudioDraftSnapshot();
}

export function saveLocalSheinStudioDraftSnapshot(
  input: LocalDraftSnapshotInput,
  options?: {
    batchId?: string;
  },
) {
  if (!canUseLocalDraftSnapshot() || !input) {
    return;
  }
  const draft = normalizeDraft({
    ...input,
    updatedAt:
      "updatedAt" in input && typeof input.updatedAt === "string"
        ? input.updatedAt
        : new Date().toISOString(),
  } as Partial<SheinStudioDraft>);
  if (!draft) {
    return;
  }
  const payload = {
    batchId: options?.batchId?.trim() || undefined,
    draft,
  };
  try {
    window.localStorage.setItem(LOCAL_DRAFT_SNAPSHOT_KEY, JSON.stringify(payload));
  } catch (error) {
    console.warn(
      "shein studio local draft snapshot save failed",
      error instanceof Error ? error.message : error,
    );
  }
}

export function clearLocalSheinStudioDraftSnapshot() {
  if (!canUseLocalDraftSnapshot()) {
    return;
  }
  window.localStorage.removeItem(LOCAL_DRAFT_SNAPSHOT_KEY);
}

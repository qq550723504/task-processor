const STORAGE_KEY = "listingkit:sds:grouped-candidate-handoff";

export type SDSGroupedCandidateHandoff = {
  createdAt: string;
  message: string;
};

function canUseStorage() {
  return typeof window !== "undefined" && typeof window.localStorage !== "undefined";
}

export function saveSDSGroupedCandidateHandoff(message: string) {
  if (!canUseStorage()) {
    return;
  }
  window.localStorage.setItem(
    STORAGE_KEY,
    JSON.stringify({
      createdAt: new Date().toISOString(),
      message,
    } satisfies SDSGroupedCandidateHandoff),
  );
}

export function consumeSDSGroupedCandidateHandoff() {
  if (!canUseStorage()) {
    return null;
  }
  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) {
    return null;
  }
  window.localStorage.removeItem(STORAGE_KEY);
  try {
    const parsed = JSON.parse(raw) as Partial<SDSGroupedCandidateHandoff>;
    if (typeof parsed.message !== "string" || !parsed.message.trim()) {
      return null;
    }
    return {
      createdAt:
        typeof parsed.createdAt === "string"
          ? parsed.createdAt
          : new Date().toISOString(),
      message: parsed.message,
    } satisfies SDSGroupedCandidateHandoff;
  } catch {
    return null;
  }
}

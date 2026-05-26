const STORAGE_KEY = "listingkit:sds:grouped-candidate-handoff";

export type SDSGroupedCandidateHandoff = {
  action?: "focus_generate";
  actionLabel?: string;
  createdAt: string;
  message: string;
};

function canUseStorage() {
  return typeof window !== "undefined" && typeof window.localStorage !== "undefined";
}

export function saveSDSGroupedCandidateHandoff(
  handoff:
    | string
    | Pick<SDSGroupedCandidateHandoff, "action" | "actionLabel" | "message">,
) {
  if (!canUseStorage()) {
    return;
  }
  const nextHandoff =
    typeof handoff === "string"
      ? { message: handoff }
      : handoff;
  window.localStorage.setItem(
    STORAGE_KEY,
    JSON.stringify({
      action: nextHandoff.action,
      actionLabel: nextHandoff.actionLabel,
      createdAt: new Date().toISOString(),
      message: nextHandoff.message,
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
      action: parsed.action === "focus_generate" ? parsed.action : undefined,
      actionLabel:
        typeof parsed.actionLabel === "string" && parsed.actionLabel.trim()
          ? parsed.actionLabel
          : undefined,
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

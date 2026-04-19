export type TaskCreateDraft = {
  text: string;
  imageUrls: string;
  productUrl: string;
  platforms: string[];
};

const prefix = "listingkit:create-draft:";

function storageKey(taskId: string) {
  return `${prefix}${taskId}`;
}

export function saveTaskCreateDraft(taskId: string, draft: TaskCreateDraft) {
  if (typeof window === "undefined") return;
  window.sessionStorage.setItem(storageKey(taskId), JSON.stringify(draft));
}

export function loadTaskCreateDraft(taskId: string) {
  if (typeof window === "undefined") return null;

  const raw = window.sessionStorage.getItem(storageKey(taskId));
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw) as Partial<TaskCreateDraft>;
    return {
      text: parsed.text ?? "",
      imageUrls: parsed.imageUrls ?? "",
      productUrl: parsed.productUrl ?? "",
      platforms: Array.isArray(parsed.platforms) ? parsed.platforms : [],
    } satisfies TaskCreateDraft;
  } catch {
    return null;
  }
}

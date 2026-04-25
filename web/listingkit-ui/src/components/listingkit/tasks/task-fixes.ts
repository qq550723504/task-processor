import type { ListingKitTaskResult } from "@/lib/types/listingkit";
import type { TaskCreateDraft } from "@/components/listingkit/tasks/task-create-draft";

function primaryTaskError(task: ListingKitTaskResult) {
  if (task.error) return task.error;
  const failedChild = task.result?.child_tasks?.find((child) => child.error);
  return failedChild?.error;
}

export function extractTaskFixes(task?: ListingKitTaskResult | null) {
  const error = task ? primaryTaskError(task) : undefined;
  if (!error) {
    return [];
  }

  const matches = Array.from(error.matchAll(/\d+\.\s+([^\n\r]+)/g));
  return matches.map((match) => match[1].trim()).filter(Boolean);
}

export function inferTaskDraftFocus(task?: ListingKitTaskResult | null) {
  const fixes = extractTaskFixes(task);

  if (fixes.some((fix) => fix.includes("链接") || fix.includes("1688") || fix.toLowerCase().includes("url"))) {
    return "productUrl" as const;
  }

  if (fixes.some((fix) => fix.includes("图片"))) {
    return "imageUrls" as const;
  }

  if (fixes.some((fix) => fix.includes("描述") || fix.includes("字符") || fix.includes("文本"))) {
    return "text" as const;
  }

  return undefined;
}

export function inferTaskDraftFocusFromDraft(draft?: Partial<TaskCreateDraft> | null) {
  const imageCount =
    draft?.imageUrls
      ?.split(/\r?\n/)
      .map((value) => value.trim())
      .filter(Boolean).length ?? 0;
  const textLength = draft?.text?.trim().length ?? 0;
  const hasProductUrl = Boolean(draft?.productUrl?.trim());

  if (hasProductUrl && imageCount === 0 && textLength === 0) {
    return "productUrl" as const;
  }

  if (imageCount < 3) {
    return "imageUrls" as const;
  }
  if (textLength < 50) {
    return "text" as const;
  }

  return undefined;
}

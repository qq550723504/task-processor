import type { ListingKitTaskResult } from "@/lib/types/listingkit";

type EmptyStateCopy = {
  title: string;
  description: string;
};

function primaryTaskError(task?: ListingKitTaskResult | null) {
  if (!task) return undefined;
  if (task.error) return task.error;
  return task.result?.child_tasks?.find((child) => child.error)?.error;
}

export function shouldSuppressResolvedActionSummary(
  task: ListingKitTaskResult | null | undefined,
  options: { hasPreviewSvg: boolean; queueTotal: number },
) {
  return (
    task?.status === "failed" &&
    !options.hasPreviewSvg &&
    options.queueTotal === 0
  );
}

export function deriveTaskPreviewEmptyState(
  task?: ListingKitTaskResult | null,
): EmptyStateCopy | undefined {
  if (!task?.status || task.status === "completed") {
    return undefined;
  }

  if (task.status === "failed") {
    return {
      title: "Preview unavailable",
      description:
        primaryTaskError(task) ??
        "The task failed before any previewable SVG sidecar was generated.",
    };
  }

  if (task.status === "pending" || task.status === "processing") {
    return {
      title: "Preview pending",
      description:
        "The task is still processing. Preview content will appear after generation finishes.",
    };
  }

  return undefined;
}

export function deriveTaskQueueEmptyState(
  task?: ListingKitTaskResult | null,
): EmptyStateCopy | undefined {
  if (!task?.status || task.status === "completed") {
    return undefined;
  }

  if (task.status === "failed") {
    return {
      title: "No generation queue items",
      description:
        primaryTaskError(task) ??
        "The task failed before generation queue items were produced.",
    };
  }

  if (task.status === "pending" || task.status === "processing") {
    return {
      title: "Generation queue pending",
      description:
        "The task is still processing. Queue items will appear after generation planning completes.",
    };
  }

  return undefined;
}

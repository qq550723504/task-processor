import type {
  ActionExecutionRequest,
  QueueItem,
  QueueQuery,
} from "@/lib/types/listingkit";

export type QueueItemAction = {
  kind: "review" | "retry" | "inspect";
  label: string;
  workspaceQuery?: QueueQuery;
  request?: ActionExecutionRequest;
};

export function deriveQueueItemAction(item: QueueItem): QueueItemAction {
  const workspaceQuery: QueueQuery = {
    platform: item.platform,
    slot: item.slot,
  };

  if (item.preview_capabilities?.[0]) {
    workspaceQuery.preview_capability = item.preview_capabilities[0];
  }

  if (item.render_preview_available) {
    return {
      kind: "review",
      label: "Review",
      workspaceQuery,
    };
  }

  if (item.retryable) {
    return {
      kind: "retry",
      label: "Retry",
      request: {
        action_key: "retry_section_generation",
        response_mode: "patch_only",
        target: {
          action_key: "retry_section_generation",
          interaction_mode: "retry_only",
          filters: {
            platforms: item.platform ? [item.platform] : [],
            quality_grade: item.quality_grade,
            retryable_only: true,
            execution_quality: item.execution_quality,
          },
        },
      },
    };
  }

  return {
    kind: "inspect",
    label: "Inspect",
    workspaceQuery,
  };
}

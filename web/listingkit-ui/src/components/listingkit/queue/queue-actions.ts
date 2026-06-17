import type {
  ActionExecutionRequest,
  QueueItem,
  QueueQuery,
} from "@/lib/types/listingkit";

export type QueueItemAction = {
  kind: "review" | "retry" | "inspect";
  label: string;
  semanticLabel: string;
  ownerLabel: string;
  ownerKind: "operations" | "engineering";
  description: string;
  failureReviewFields: Array<{ label: string; value: string }>;
  workspaceQuery?: QueueQuery;
  request?: ActionExecutionRequest;
};

function queueFailureReviewFields(item: QueueItem) {
  return [
    { label: "Platform", value: item.platform },
    { label: "Slot", value: item.slot },
    { label: "Generation task", value: item.generation_task },
    { label: "State", value: item.state },
    { label: "Execution quality", value: item.execution_quality },
    { label: "Quality grade", value: item.quality_grade },
    { label: "Retry hint", value: item.retry_hint },
  ].filter((field): field is { label: string; value: string } => Boolean(field.value));
}

export function deriveQueueItemAction(item: QueueItem): QueueItemAction {
  const failureReviewFields = queueFailureReviewFields(item);
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
      semanticLabel: "Review",
      ownerLabel: "运营可处理",
      ownerKind: "operations",
      description: "打开预览并完成内容复核。",
      failureReviewFields,
      workspaceQuery,
    };
  }

  if (item.retryable) {
    return {
      kind: "retry",
      label: "Retry",
      semanticLabel: "Retry",
      ownerLabel: "运营可处理",
      ownerKind: "operations",
      description: "按当前失败范围重试生成，不重复处理其他项。",
      failureReviewFields,
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
    semanticLabel: "Inspect",
    ownerLabel: "工程介入候选",
    ownerKind: "engineering",
    description: "当前缺少可预览或可重试路径，需要查看上下文定位原因。",
    failureReviewFields,
    workspaceQuery,
  };
}

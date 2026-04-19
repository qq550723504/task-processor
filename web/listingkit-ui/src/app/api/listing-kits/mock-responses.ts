import type { NextRequest } from "next/server";

import { buildMockShared } from "@/app/api/listing-kits/mock-shared";
import type { ListingKitMockBundle } from "@/app/api/listing-kits/mock-types";

export function shouldUseListingKitMock(taskId: string) {
  return (
    process.env.LISTINGKIT_UI_USE_MOCK === "1" ||
    taskId === "demo-task" ||
    taskId.startsWith("mock-")
  );
}

function buildMockBundle(taskId: string, focusCapability?: string): ListingKitMockBundle {
  const shared = buildMockShared(taskId);
  const resolvedCapability = focusCapability ?? shared.reviewSession.focus_capability;

  return {
    createTask: {
      task_id: taskId,
      status: "pending",
      created_at: "2026-04-19T00:00:00Z",
    },
    taskResult: {
      task_id: taskId,
      status: "completed",
      created_at: "2026-04-19T00:00:00Z",
      result: {
        task_id: taskId,
        status: "completed",
        summary: {
          source_type: "images_and_text",
          image_count: shared.queueItems.length,
          variant_count: 1,
          needs_review: true,
        },
      },
    },
    preview: {
      task_id: taskId,
      status: "completed",
      selected_platform: shared.reviewSession.selected_platform,
      platforms: ["shein", "temu"],
      needs_review: true,
      overview: {
        country: "US",
        language: "en",
        image_count: 12,
        variant_count: 3,
        status_message: "Mock preview loaded",
        platform_cards: shared.platformCards,
      },
      asset_generation_overview: shared.overview,
      asset_generation_queue: {
        summary: shared.queueSummary,
        items: shared.queueItems,
      },
    },
    queue: {
      task_id: taskId,
      delta_token: "mock-queue-token",
      conditional: {
        delta_token: "mock-queue-token",
        etag: "mock-queue-token",
      },
      resource_descriptors: shared.recoveryDescriptors,
      recovery_summary: shared.recoverySummary,
      resolved_action_summary: shared.resolvedActionSummary,
      summary: shared.queueSummary,
      page: 1,
      page_size: 50,
      total: shared.queueItems.length,
      items: shared.queueItems,
    },
    reviewSession: {
      task_id: taskId,
      delta_token: "mock-session-token",
      response_mode: "",
      conditional: {
        delta_token: "mock-session-token",
        etag: "mock-session-token",
      },
      resource_descriptors: shared.recoveryDescriptors,
      recovery_summary: shared.recoverySummary,
      resolved_action_summary: shared.resolvedActionSummary,
      session: {
        ...shared.reviewSession,
        focus_capability: resolvedCapability,
      },
      patch: {
        delta_token: "mock-session-token",
        selected_platform: shared.reviewSession.selected_platform,
        selected_slot: shared.reviewSession.selected_slot,
        focus_capability: resolvedCapability,
        focused_section_key: shared.reviewSession.focused_section_key,
        focused_target: shared.focusedTarget,
        focused_render_preview: shared.focusedPreview,
        focused_toolbar: shared.focusedToolbar,
        queue_summary: shared.queueSummary,
        review_summary: shared.reviewSummary,
      },
    },
    reviewPreview: {
      task_id: taskId,
      delta_token: "mock-preview-token",
      conditional: {
        delta_token: "mock-preview-token",
        etag: "mock-preview-token",
      },
      resource_descriptors: shared.recoveryDescriptors,
      recovery_summary: shared.recoverySummary,
      resolved_action_summary: shared.resolvedActionSummary,
      preview: shared.focusedPreview,
      review_target: shared.focusedTarget,
      toolbar: shared.focusedToolbar,
      revision_status: "match",
    },
    dispatch: {
      dispatch_kind: "review_session",
      conditional: {
        delta_token: "mock-dispatch-token",
        etag: "mock-dispatch-token",
      },
      resource_descriptors: shared.recoveryDescriptors,
      recovery_summary: shared.recoverySummary,
      resolved_action_summary: shared.resolvedActionSummary,
      review_session: {
        task_id: taskId,
        session: shared.reviewSession,
        resolved_action_summary: shared.resolvedActionSummary,
        recovery_summary: shared.recoverySummary,
      },
      review_preview: {
        task_id: taskId,
        preview: shared.focusedPreview,
        toolbar: shared.focusedToolbar,
        review_target: shared.focusedTarget,
      },
      panel_update: {
        dispatch_kind: "review_session",
        delta_token: "mock-dispatch-token",
        conditional: {
          delta_token: "mock-dispatch-token",
          etag: "mock-dispatch-token",
        },
        overview: shared.overview,
        queue_summary: shared.queueSummary,
        review_summary: shared.reviewSummary,
        focused_target: shared.focusedTarget,
        focused_render_preview: shared.focusedPreview,
        focused_toolbar: shared.focusedToolbar,
        primary_recovery_descriptor: shared.recoveryDescriptors[0],
        recommended_recovery_descriptors: shared.recoveryDescriptors,
      },
    },
    action: {
      action_key: "review_detail_previews",
      response_mode: "patch_only",
      delta_token: "mock-action-token",
      conditional: {
        delta_token: "mock-action-token",
        etag: "mock-action-token",
      },
      resource_descriptors: shared.recoveryDescriptors,
      recovery_summary: shared.recoverySummary,
      resolved_action_summary: shared.resolvedActionSummary,
      overview: shared.overview,
      queue: {
        task_id: taskId,
        page: 1,
        page_size: 50,
        total: shared.queueItems.length,
        summary: shared.queueSummary,
        items: shared.queueItems,
      },
      review_session: shared.reviewSession,
      review_patch: {
        delta_token: "mock-action-token",
        selected_platform: shared.reviewSession.selected_platform,
        selected_slot: shared.reviewSession.selected_slot,
        focus_capability: shared.reviewSession.focus_capability,
        focused_target: shared.focusedTarget,
        focused_render_preview: shared.focusedPreview,
        focused_toolbar: shared.focusedToolbar,
        queue_summary: shared.queueSummary,
        review_summary: shared.reviewSummary,
      },
      review_workflow: {
        action_key: "retry_section_generation",
        status: "completed",
        platform: "temu",
        slot: "gallery",
        capability: "detail_preview",
        message: "Mock retry executed.",
      },
    },
  };
}

export async function buildListingKitMockResponse(
  request: NextRequest,
  path: string[],
): Promise<ListingKitMockBundle | undefined> {
  if (
    request.method === "POST" &&
    path.length === 1 &&
    path[0] === "generate" &&
    process.env.LISTINGKIT_UI_USE_MOCK === "1"
  ) {
    return buildMockBundle("mock-created-task");
  }

  const [, taskId] = path;
  if (!taskId || !shouldUseListingKitMock(taskId)) {
    return undefined;
  }
  const query = request.nextUrl.searchParams;
  const focusCapability =
    query.get("preview_capability") ?? undefined;

  return buildMockBundle(taskId, focusCapability);
}

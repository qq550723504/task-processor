import type { ListingKitMockShared } from "@/app/api/listing-kits/mock-types";
import type {
  PlatformCard,
  PreviewSlot,
  QueueItem,
  RecoveryDescriptor,
  ReviewSession,
  ReviewSlot,
  ReviewSummary,
  ReviewTarget,
  ReviewToolbar,
} from "@/lib/types/listingkit";

const previewSvg = `
<svg viewBox="0 0 1200 900" xmlns="http://www.w3.org/2000/svg">
  <rect width="1200" height="900" rx="48" fill="#f5f3ff"/>
  <rect x="70" y="70" width="640" height="760" rx="36" fill="#ffffff" stroke="#e4e4e7" stroke-width="4"/>
  <rect x="780" y="120" width="320" height="88" rx="24" fill="#18181b"/>
  <rect x="780" y="240" width="260" height="26" rx="13" fill="#a1a1aa"/>
  <rect x="780" y="290" width="210" height="26" rx="13" fill="#d4d4d8"/>
  <rect x="780" y="410" width="300" height="160" rx="28" fill="#ede9fe"/>
  <rect x="780" y="602" width="300" height="120" rx="28" fill="#faf5ff"/>
  <path d="M308 232c82 0 139 50 139 134v244c0 49-39 88-88 88h-102c-49 0-88-39-88-88V366c0-84 57-134 139-134z" fill="#27272a"/>
  <path d="M257 302h102l44 74-32 38 26 188H219l26-188-32-38 44-74z" fill="#a78bfa"/>
  <circle cx="860" cy="490" r="38" fill="#8b5cf6"/>
  <text x="916" y="499" font-size="42" font-family="Arial" fill="#312e81">Detail</text>
  <text x="824" y="660" font-size="34" font-family="Arial" fill="#6d28d9">Size notes</text>
</svg>
`.trim();

export function buildMockShared(taskId: string): ListingKitMockShared {
  const focusedTarget: ReviewTarget = {
    platform: "shein",
    slot: "main",
    capability: "detail_preview",
    section_key: "detail_preview-main",
    navigation_target: {
      dispatch_kind: "review_session",
      session_query: {
        platform: "shein",
        slot: "main",
        preview_capability: "detail_preview",
      },
      preview_query: {
        platform: "shein",
        slot: "main",
        preview_capability: "detail_preview",
      },
    },
  };

  const focusedPreview: PreviewSlot = {
    slot: "main",
    asset_id: "asset_mock_main",
    state_label: "ready",
    retry_hint: "no_retry",
    template_label: "hero_detail",
    render_profile: "selling_point_detail",
    preview_svg: previewSvg,
    visual_mode: "selling_point",
    layout_engine: "scene_renderer",
    layer_types: ["subject", "detail", "text"],
    regions: ["subject", "badge", "copy"],
    style_tokens: ["detail_preview", "studio_card"],
  };

  const focusedToolbar: ReviewToolbar = {
    platform: "shein",
    slot: "main",
    capability: "detail_preview",
    asset_id: "asset_mock_main",
    visual_mode: "selling_point",
    preview_format: "svg",
    focus_regions: ["subject", "copy"],
    focus_layer_types: ["detail", "text"],
    focus_style_tokens: ["detail_preview", "studio_card"],
    section_actions: [
      {
        key: "open_preview_svg",
        label: "Open Preview",
        kind: "viewer",
        enabled: true,
        navigation_target: focusedTarget.navigation_target,
      },
    ],
    preview_actions: [
      {
        key: "approve_section_review",
        label: "Approve",
        kind: "workflow",
        enabled: true,
        action_target: {
          action_key: "approve_section_review",
          interaction_mode: "review_only",
          navigation_target: focusedTarget.navigation_target,
        },
      },
      {
        key: "retry_section_generation",
        label: "Retry Section",
        kind: "workflow",
        enabled: true,
        action_target: {
          action_key: "retry_section_generation",
          interaction_mode: "retry_only",
          filters: {
            platforms: ["shein"],
            quality_grade: "provisional",
            retryable_only: true,
            preview_capability: "detail_preview",
          },
        },
      },
    ],
  };

  const reviewSummary: ReviewSummary = {
    total_sections: 3,
    approved_sections: 1,
    deferred_sections: 1,
    pending_sections: 1,
  };

  const queueSummary = {
    total_items: 12,
    ready_items: 8,
    fallback_items: 2,
    missing_items: 1,
    queued_items: 0,
    running_items: 0,
    completed_items: 11,
    failed_items: 1,
    retryable_items: 3,
    previewable_items: 7,
    preview_capability_counts: {
      detail_preview: 4,
      badge_preview: 2,
      measurement_preview: 1,
    },
    quality_grade_counts: {
      ideal: 6,
      provisional: 5,
      missing: 1,
    },
    approved_sections: 1,
    deferred_sections: 1,
    review_pending_sections: 1,
  };

  const recoveryDescriptors: RecoveryDescriptor[] = [
    {
      role: "focused_resource",
      platform: "shein",
      slot: "main",
      capability: "detail_preview",
      recovery_hint: "review_fallback",
      recovery_scope: "focused_resource",
      recovery_severity: "medium",
      recovery_urgency: "now",
      recovery_cta_kind: "review",
      recovery_target: focusedTarget.navigation_target,
    },
    {
      role: "queue_item",
      platform: "temu",
      slot: "gallery",
      recovery_hint: "retry_dispatch",
      recovery_scope: "queue_item",
      retryable: true,
      recovery_severity: "high",
      recovery_urgency: "now",
      recovery_cta_kind: "retry",
      recovery_action_key: "retry_section_generation",
      recovery_target: {
        dispatch_kind: "action",
        action_target: {
          action_key: "retry_section_generation",
          interaction_mode: "retry_only",
          filters: {
            platforms: ["temu"],
            quality_grade: "provisional",
            retryable_only: true,
          },
        },
      },
    },
  ];

  const recoverySummary = {
    title: "Two recovery paths need attention",
    summary: "One preview can be reviewed now and one gallery slot should be retried.",
    severity: "medium",
    urgency: "now",
    cta_kind: "review",
    action_key: "review_detail_previews",
    recommended_count: 2,
    primary_descriptor: recoveryDescriptors[0],
    recommended_descriptors: recoveryDescriptors,
  };

  const resolvedActionSummary = {
    source_kind: "review",
    title: "Review detail previews",
    summary: "The shein main slot has a ready SVG preview and a pending review decision.",
    cta_kind: "review",
    action_key: "review_detail_previews",
    navigation_target: focusedTarget.navigation_target,
  };

  const overview = {
    primary_action: "Review detail previews",
    primary_action_key: "review_detail_previews",
    primary_cta_kind: "review",
    primary_navigation_target: focusedTarget.navigation_target,
    primary_action_reason: "A preview-ready section is pending review.",
    resolved_action_summary: resolvedActionSummary,
    dominant_quality_grade: "ideal",
    dominant_quality_grade_label: "Ideal",
    previewable_items: 7,
    preview_ready_platforms: ["shein", "temu"],
    preview_capability_counts: {
      detail_preview: 4,
      badge_preview: 2,
      measurement_preview: 1,
    },
    retryable_count: 3,
    approved_sections: 1,
    deferred_sections: 1,
    review_pending_sections: 1,
    recovery_summary: recoverySummary,
  };

  const platformCards: PlatformCard[] = [
    {
      platform: "shein",
      status: "review_ready",
      summary: "Main hero preview is ready for review.",
      needs_review: true,
      previewable_items: 4,
      preview_capability_counts: { detail_preview: 3, badge_preview: 1 },
      quality_grade_counts: { ideal: 3, provisional: 1 },
      dominant_quality_grade: "ideal",
      dominant_quality_grade_label: "Ideal",
      approved_sections: 1,
      deferred_sections: 0,
      review_pending_sections: 1,
      primary_navigation_target: focusedTarget.navigation_target,
      resolved_action_summary: resolvedActionSummary,
      recovery_summary: recoverySummary,
    },
    {
      platform: "temu",
      status: "retry_needed",
      summary: "One gallery slot is still provisional and retryable.",
      needs_review: false,
      previewable_items: 2,
      preview_capability_counts: { detail_preview: 1, measurement_preview: 1 },
      quality_grade_counts: { provisional: 2 },
      dominant_quality_grade: "provisional",
      dominant_quality_grade_label: "Provisional",
      approved_sections: 0,
      deferred_sections: 1,
      review_pending_sections: 0,
      recovery_summary: recoverySummary,
    },
  ];

  const slotNavigation: ReviewSlot[] = [
    {
      platform: "shein",
      slot: "main",
      state: "ready",
      quality_grade: "ideal",
      quality_grade_label: "Ideal",
      asset_id: "asset_mock_main",
      render_preview_available: true,
      preview_capabilities: ["detail_preview"],
      focus_capability: "detail_preview",
      review_target: focusedTarget,
    },
    {
      platform: "shein",
      slot: "gallery",
      state: "fallback",
      quality_grade: "provisional",
      quality_grade_label: "Provisional",
      asset_id: "asset_mock_gallery",
      render_preview_available: false,
      preview_capabilities: ["badge_preview"],
      focus_capability: "badge_preview",
      review_target: {
        platform: "shein",
        slot: "gallery",
        capability: "badge_preview",
        section_key: "badge_preview-gallery",
        navigation_target: {
          dispatch_kind: "review_session",
          session_query: {
            platform: "shein",
            slot: "gallery",
            preview_capability: "badge_preview",
          },
        },
      },
    },
  ];

  const reviewSession: ReviewSession = {
    selected_platform: "shein",
    selected_slot: "main",
    focus_capability: "detail_preview",
    focused_section_key: "detail_preview-main",
    default_target: focusedTarget,
    focused_target: focusedTarget,
    focused_render_preview: focusedPreview,
    focused_toolbar: focusedToolbar,
    review_summary: reviewSummary,
    overview,
    platform_cards: platformCards,
    slot_navigation: slotNavigation,
    sections: [
      {
        capability: "detail_preview",
        capability_label: "Detail Preview",
        section_key: "detail_preview-main",
        title: "Detail Preview",
        description: "Hero detail composition with subject and supporting copy.",
        empty_state: "",
        selected: true,
        item_count: 1,
        review_target: focusedTarget,
        toolbar_actions: focusedToolbar.section_actions,
        workflow_actions: focusedToolbar.preview_actions,
        review_status: "pending",
        slots: [slotNavigation[0]],
      },
      {
        capability: "badge_preview",
        capability_label: "Badge Preview",
        section_key: "badge_preview-gallery",
        title: "Badge Preview",
        description: "Gallery badge variant is provisional.",
        empty_state: "",
        selected: false,
        item_count: 1,
        review_status: "deferred",
        slots: [slotNavigation[1]],
      },
    ],
  };

  const queueItems: QueueItem[] = [
    {
      task_id: taskId,
      generation_task: "gen_shein_main",
      platform: "shein",
      slot: "main",
      purpose: "hero",
      state: "ready",
      retryable: false,
      retry_hint: "no_retry",
      quality_grade: "ideal",
      quality_grade_label: "Ideal",
      execution_quality: "completed",
      render_preview_available: true,
      preview_capabilities: ["detail_preview"],
      review_decision: "",
      review_status: "pending",
      selected_asset_id: "asset_mock_main",
      resource_descriptors: [recoveryDescriptors[0]],
    },
    {
      task_id: taskId,
      generation_task: "gen_temu_gallery",
      platform: "temu",
      slot: "gallery",
      purpose: "secondary",
      state: "fallback",
      retryable: true,
      retry_hint: "retry_dispatch",
      quality_grade: "provisional",
      quality_grade_label: "Provisional",
      execution_quality: "failed",
      render_preview_available: false,
      preview_capabilities: ["detail_preview"],
      review_status: "deferred",
      selected_asset_id: "asset_mock_gallery",
      resource_descriptors: [recoveryDescriptors[1]],
    },
  ];

  return {
    taskId,
    previewSvg,
    focusedTarget,
    focusedToolbar,
    focusedPreview,
    queueSummary,
    overview,
    reviewSummary,
    recoveryDescriptors,
    recoverySummary,
    resolvedActionSummary,
    platformCards,
    slotNavigation,
    reviewSession,
    queueItems,
  };
}

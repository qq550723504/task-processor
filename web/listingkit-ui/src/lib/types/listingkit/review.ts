import type {
  ActionTarget,
  ConditionalState,
  NavigationTarget,
  RecoveryDescriptor,
  RecoverySummary,
  ResolvedActionSummary,
} from "./navigation";
import type {
  PlatformCard,
  PreviewSlot,
  QueueItem,
  QueuePage,
  QueueSummary,
  ReviewTarget,
  ReviewToolbar,
  ScenePresetSummary,
  ToolbarAction,
} from "./preview";
export type ReviewSlot = {
  platform?: string;
  slot?: string;
  purpose?: string;
  state?: string;
  quality_grade?: string;
  quality_grade_label?: string;
  asset_id?: string;
  template_label?: string;
  render_preview_available?: boolean;
  preview_capabilities?: string[];
  focus_capability?: string;
  review_target?: ReviewTarget;
};

export type ReviewSection = {
  capability?: string;
  capability_label?: string;
  section_key?: string;
  title?: string;
  description?: string;
  empty_state?: string;
  selected?: boolean;
  item_count: number;
  primary_action_key?: string;
  primary_action_target?: ReviewTarget;
  review_target?: ReviewTarget;
  toolbar_actions?: ToolbarAction[];
  workflow_actions?: ToolbarAction[];
  workflow_state?: string;
  workflow_message?: string;
  review_decision?: string;
  review_status?: string;
  slots?: ReviewSlot[];
};

export type ReviewSummary = {
  total_sections?: number;
  approved_sections?: number;
  deferred_sections?: number;
  pending_sections?: number;
};

export type AssetGenerationOverview = {
  primary_action?: string;
  primary_action_key?: string;
  primary_action_target?: ActionTarget;
  primary_cta_kind?: string;
  primary_navigation_target?: NavigationTarget;
  primary_action_reason?: string;
  resolved_action_summary?: ResolvedActionSummary;
  dominant_quality_grade?: string;
  dominant_quality_grade_label?: string;
  previewable_items?: number;
  preview_ready_platforms?: string[];
  preview_capability_counts?: Record<string, number>;
  retryable_count?: number;
  approved_sections?: number;
  deferred_sections?: number;
  review_pending_sections?: number;
  recovery_summary?: RecoverySummary;
};

export type ReviewSession = {
  selected_platform?: string;
  selected_slot?: string;
  focus_capability?: string;
  focused_section_key?: string;
  default_target?: ReviewTarget;
  focused_target?: ReviewTarget;
  focused_render_preview?: PreviewSlot;
  focused_scene_preset?: ScenePresetSummary;
  focused_toolbar?: ReviewToolbar;
  review_summary?: ReviewSummary;
  queue?: {
    summary?: QueueSummary;
    items?: QueueItem[];
  };
  overview?: AssetGenerationOverview;
  platform_cards?: PlatformCard[];
  slot_navigation?: ReviewSlot[];
  sections?: ReviewSection[];
};

export type ReviewPatch = {
  delta_token?: string;
  selected_platform?: string;
  selected_slot?: string;
  focus_capability?: string;
  focused_section_key?: string;
  focused_target?: ReviewTarget;
  focused_render_preview?: PreviewSlot;
  focused_toolbar?: ReviewToolbar;
  queue_summary?: QueueSummary;
  review_summary?: ReviewSummary;
};

export type ReviewSessionResponse = {
  task_id?: string;
  delta_token?: string;
  not_modified?: boolean;
  response_mode?: string;
  conditional?: ConditionalState;
  resource_descriptors?: RecoveryDescriptor[];
  recovery_summary?: RecoverySummary;
  resolved_action_summary?: ResolvedActionSummary;
  patch?: ReviewPatch;
  session?: ReviewSession;
};

export type ReviewPreviewResponse = {
  task_id?: string;
  delta_token?: string;
  not_modified?: boolean;
  conditional?: ConditionalState;
  resource_descriptors?: RecoveryDescriptor[];
  recovery_summary?: RecoverySummary;
  resolved_action_summary?: ResolvedActionSummary;
  preview?: PreviewSlot;
  scene_preset?: ScenePresetSummary;
  review_target?: ReviewTarget;
  toolbar?: ReviewToolbar;
  revision_status?: string;
  revision_mismatch_reason?: string;
};

export type PanelUpdate = {
  dispatch_kind?: string;
  response_mode?: string;
  delta_token?: string;
  no_changes?: boolean;
  conditional?: ConditionalState;
  focused_resolution?: {
    source_kind?: string;
    source_step?: number;
    via_fallback?: boolean;
    fallback_reason?: string;
  };
  primary_recovery_descriptor?: RecoveryDescriptor;
  recommended_recovery_descriptors?: RecoveryDescriptor[];
  overview?: AssetGenerationOverview;
  queue_summary?: QueueSummary;
  review_summary?: ReviewSummary;
  focused_target?: ReviewTarget;
  focused_render_preview?: PreviewSlot;
  focused_toolbar?: ReviewToolbar;
  review_patch?: ReviewPatch;
  review_session?: ReviewSessionResponse;
  review_preview?: ReviewPreviewResponse;
};

export type NavigationDispatchResponse = {
  dispatch_kind?: string;
  not_modified?: boolean;
  conditional?: ConditionalState;
  resource_descriptors?: RecoveryDescriptor[];
  recovery_summary?: RecoverySummary;
  resolved_action_summary?: ResolvedActionSummary;
  queue?: QueuePage;
  review_session?: ReviewSessionResponse;
  review_preview?: ReviewPreviewResponse;
  action?: ActionExecutionResult;
  panel_update?: PanelUpdate;
};

export type ActionExecutionRequest = {
  action_key?: string;
  response_mode?: string;
  target?: ActionTarget;
};

export type ActionExecutionResult = {
  action_key?: string;
  response_mode?: string;
  delta_token?: string;
  conditional?: ConditionalState;
  resource_descriptors?: RecoveryDescriptor[];
  recovery_summary?: RecoverySummary;
  resolved_action_summary?: ResolvedActionSummary;
  overview?: AssetGenerationOverview;
  resolved_target?: ActionTarget;
  queue?: QueuePage;
  review_session?: ReviewSession;
  review_patch?: ReviewPatch;
  review_workflow?: {
    action_key?: string;
    status?: string;
    platform?: string;
    slot?: string;
    capability?: string;
    message?: string;
  };
};

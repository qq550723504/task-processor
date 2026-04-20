export type ConditionalState = {
  delta_token?: string;
  etag?: string;
  not_modified?: boolean;
  no_changes?: boolean;
};

export type QueueQuery = {
  platform?: string;
  slot?: string;
  from_platform?: string;
  from_slot?: string;
  from_capability?: string;
  from_section_key?: string;
  asset_id?: string;
  asset_revision?: string;
  preview_revision?: string;
  task_revision?: string;
  delta_token?: string;
  if_match?: string;
  response_mode?: string;
  state?: string;
  execution_mode?: string;
  execution_quality?: string;
  quality_grade?: string;
  quality_grade_label?: string;
  preview_capability?: string;
  render_preview_available?: boolean;
  retryable?: boolean;
  page?: number;
  page_size?: number;
  sort_by?: string;
  sort_order?: string;
};

export type NavigationDispatchPlanStep = {
  kind?: string;
  response_mode?: string;
  query?: QueueQuery;
  cache_preference?: string;
  requires_revalidate?: boolean;
};

export type NavigationDispatchPlan = {
  strategy?: string;
  stop_on_not_modified?: boolean;
  stop_on_first_success?: boolean;
  stop_on_error?: boolean;
  fallback_strategy?: string;
  max_parallelism?: number;
  dedupe_policy?: string;
  winner_policy?: string;
  steps?: NavigationDispatchPlanStep[];
};

export type NavigationDescriptor = {
  resource_kind?: string;
  cache_key?: string;
  cache_policy?: string;
  supports_stale_while_revalidate?: boolean;
  revalidate_after_action?: boolean;
  refresh_scope?: string;
  invalidates?: string[];
  conditional?: ConditionalState;
  dispatch_plan?: NavigationDispatchPlan;
};

export type RecommendedFilters = {
  platforms?: string[];
  slots?: string[];
  quality_grade?: string;
  quality_grade_label?: string;
  retryable_only?: boolean;
  execution_quality?: string;
  render_preview_available?: boolean;
  preview_capability?: string;
};

export type ActionTarget = {
  action_key?: string;
  interaction_mode?: string;
  filters?: RecommendedFilters;
  navigation_target?: NavigationTarget;
  queue_query?: QueueQuery;
  retry_request?: {
    task_ids?: string[];
    slots?: string[];
    execution_quality?: string;
    quality_grade?: string;
    fallback_only?: boolean;
    renderer_only?: boolean;
  };
};

export type NavigationTarget = {
  dispatch_kind?: string;
  conditional?: ConditionalState;
  resource_kind?: string;
  cache_key?: string;
  cache_policy?: string;
  revalidate_after_action?: boolean;
  descriptor?: NavigationDescriptor;
  queue_query?: QueueQuery;
  session_query?: QueueQuery;
  preview_query?: QueueQuery;
  action_target?: ActionTarget;
};

export type ResolvedActionSummary = {
  source_kind?: string;
  title?: string;
  summary?: string;
  cta_kind?: string;
  action_key?: string;
  navigation_target?: NavigationTarget;
  action_target?: ActionTarget;
};

export type RecoveryDescriptor = {
  role?: string;
  platform?: string;
  slot?: string;
  capability?: string;
  section_key?: string;
  source_kind?: string;
  source_step?: number;
  via_fallback?: boolean;
  fallback_reason?: string;
  recovery_scope?: string;
  recovery_hint?: string;
  retryable?: boolean;
  recovery_severity?: string;
  recovery_urgency?: string;
  recovery_cta_kind?: string;
  recovery_action_key?: string;
  recovery_target?: NavigationTarget;
  recovery_dispatch_plan?: NavigationDispatchPlan;
  descriptor?: NavigationDescriptor;
};

export type RecoverySummary = {
  title?: string;
  summary?: string;
  severity?: string;
  urgency?: string;
  cta_kind?: string;
  action_key?: string;
  recommended_count?: number;
  primary_descriptor?: RecoveryDescriptor;
  recommended_descriptors?: RecoveryDescriptor[];
};

export type PlatformPreviewSummary = {
  total_previews?: number;
  main_available?: boolean;
  gallery_count?: number;
  auxiliary_count?: number;
  capability_counts?: Record<string, number>;
  visual_modes?: string[];
};

export type PlatformCard = {
  platform: string;
  status: string;
  summary?: string;
  needs_review?: boolean;
  previewable_items?: number;
  preview_capability_counts?: Record<string, number>;
  quality_grade_counts?: Record<string, number>;
  dominant_quality_grade?: string;
  dominant_quality_grade_label?: string;
  preview_summary?: PlatformPreviewSummary;
  approved_sections?: number;
  deferred_sections?: number;
  review_pending_sections?: number;
  primary_action_key?: string;
  primary_action_target?: ActionTarget;
  primary_cta_kind?: string;
  primary_navigation_target?: NavigationTarget;
  resolved_action_summary?: ResolvedActionSummary;
  recovery_summary?: RecoverySummary;
};

export type ListingKitPreviewHeader = {
  country?: string;
  language?: string;
  source_type?: string;
  image_count?: number;
  variant_count?: number;
  status_message?: string;
  warnings?: string[];
  platform_cards?: PlatformCard[];
};

export type QueueSummary = {
  total_items: number;
  ready_items: number;
  fallback_items: number;
  missing_items: number;
  queued_items: number;
  running_items: number;
  completed_items: number;
  failed_items: number;
  retryable_items: number;
  previewable_items: number;
  preview_capability_counts?: Record<string, number>;
  quality_grade_counts?: Record<string, number>;
  approved_sections: number;
  deferred_sections: number;
  review_pending_sections: number;
};

export type QueueItem = {
  task_id?: string;
  generation_task?: string;
  platform?: string;
  slot?: string;
  purpose?: string;
  state?: string;
  retryable?: boolean;
  retry_hint?: string;
  quality_grade?: string;
  quality_grade_label?: string;
  execution_quality?: string;
  render_preview_available?: boolean;
  preview_capabilities?: string[];
  review_decision?: string;
  review_status?: string;
  review_blocked?: boolean;
  selected_asset_id?: string;
  scene_preset?: ScenePresetSummary;
  resource_descriptors?: RecoveryDescriptor[];
};

export type QueuePage = {
  task_id: string;
  delta_token?: string;
  not_modified?: boolean;
  conditional?: ConditionalState;
  resource_descriptors?: RecoveryDescriptor[];
  recovery_summary?: RecoverySummary;
  resolved_action_summary?: ResolvedActionSummary;
  summary?: QueueSummary;
  page: number;
  page_size: number;
  total: number;
  items?: QueueItem[];
};

export type ReviewTarget = {
  platform?: string;
  slot?: string;
  capability?: string;
  section_key?: string;
  navigation_target?: NavigationTarget;
};

export type ToolbarAction = {
  key?: string;
  label?: string;
  kind?: string;
  selected?: boolean;
  enabled?: boolean;
  target?: ReviewTarget;
  action_target?: ActionTarget;
  navigation_target?: NavigationTarget;
};

export type PreviewSlot = {
  slot?: string;
  asset_id?: string;
  state_label?: string;
  retry_hint?: string;
  template_label?: string;
  render_profile?: string;
  preview_svg?: string;
  visual_mode?: string;
  layout_engine?: string;
  layer_types?: string[];
  regions?: string[];
  style_tokens?: string[];
};

export type ReviewToolbar = {
  platform?: string;
  slot?: string;
  capability?: string;
  asset_id?: string;
  visual_mode?: string;
  preview_format?: string;
  focus_regions?: string[];
  focus_layer_types?: string[];
  focus_style_tokens?: string[];
  section_actions?: ToolbarAction[];
  preview_actions?: ToolbarAction[];
};

export type ScenePresetSummary = {
  prompt_key?: string;
  defaults_source?: string;
  scene_category?: string;
  scene_style?: string;
  background_tone?: string;
  composition?: string;
  props_level?: string;
  audience_hint?: string;
  custom_scene_hint?: string;
};

export type ReviewSlot = {
  platform?: string;
  slot?: string;
  purpose?: string;
  state?: string;
  quality_grade?: string;
  quality_grade_label?: string;
  asset_id?: string;
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

export type ListingKitPreview = {
  task_id: string;
  status: string;
  selected_platform?: string;
  platforms?: string[];
  needs_review?: boolean;
  overview?: ListingKitPreviewHeader;
  asset_generation_overview?: AssetGenerationOverview;
  asset_generation_queue?: {
    summary?: QueueSummary;
    items?: QueueItem[];
  };
  asset_generation_tasks?: unknown[];
  asset_render_previews?: PreviewSlot[];
  platform_asset_render_previews?: unknown[];
  amazon?: { render_previews?: unknown };
  shein?: { render_previews?: unknown };
  temu?: { render_previews?: unknown };
  walmart?: { render_previews?: unknown };
};

export type ListingKitChildTask = {
  kind?: string;
  task_id?: string;
  status?: string;
  error?: string;
};

export type ListingKitTaskResultData = {
  task_id?: string;
  status?: string;
  platforms?: string[];
  country?: string;
  language?: string;
  summary?: {
    source_type?: string;
    image_count?: number;
    variant_count?: number;
    needs_review?: boolean;
  };
  child_tasks?: ListingKitChildTask[];
  created_at?: string;
  updated_at?: string;
};

export type ListingKitTaskResult = {
  task_id?: string;
  status?: string;
  result?: ListingKitTaskResultData;
  error?: string;
  created_at?: string;
  completed_at?: string;
};

export type CreateListingKitTaskRequest = {
  image_urls?: string[];
  text?: string;
  product_url?: string;
  platforms: string[];
  country?: string;
  language?: string;
  options?: {
    scene?: {
      scene_category?: string;
      scene_style?: string;
      background_tone?: string;
      composition?: string;
      props_level?: string;
      audience_hint?: string;
      custom_scene_hint?: string;
    };
  };
};

export type CreateListingKitTaskResponse = {
  task_id: string;
  status?: string;
  created_at?: string;
};

export type UploadImagesResponse = {
  image_urls?: string[];
};

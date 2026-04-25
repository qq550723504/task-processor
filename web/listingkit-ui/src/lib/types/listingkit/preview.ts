import type {
  ActionTarget,
  ConditionalState,
  NavigationTarget,
  RecoveryDescriptor,
  RecoverySummary,
  ResolvedActionSummary,
} from "./navigation";
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
  review_reasons?: string[];
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
  asset_url?: string;
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

export type PlatformScenePresetSummary = {
  slot?: string;
  purpose?: string;
  asset_id?: string;
  scene_preset?: ScenePresetSummary;
};

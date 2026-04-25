export type ConditionalState = {
  delta_token?: string;
  etag?: string;
  not_modified?: boolean;
  no_changes?: boolean;
};

export type QueueQuery = {
  status?: string;
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
  kind?: string;
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

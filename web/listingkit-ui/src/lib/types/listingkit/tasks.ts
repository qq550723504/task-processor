import type {
  ListingKitPreviewHeader,
  PreviewSlot,
  QueueItem,
  QueueSummary,
} from "./preview";
import type {
  PlatformPreviewPayload,
  PodExecutionSummary,
  SheinPreviewPayload,
  SheinStoreResolutionSummary,
  SheinStatusOverview,
} from "./shein";
import type { AssetGenerationOverview } from "./review";

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
  amazon?: PlatformPreviewPayload;
  shein?: SheinPreviewPayload;
  temu?: PlatformPreviewPayload;
  walmart?: PlatformPreviewPayload;
};

export type ListingKitChildTask = {
  kind?: string;
  task_id?: string;
  status?: string;
  error?: string;
};

export type ListingKitWorkflowStage = {
  kind?: string;
  status?: "pending" | "running" | "completed" | "skipped" | "degraded" | "failed" | string;
  task_id?: string;
  error?: string;
  started_at?: string;
  finished_at?: string;
  duration_ms?: number;
};

export type ListingKitWorkflowIssue = {
  code?: string;
  severity?: "info" | "warning" | "review" | "blocking" | string;
  stage?: string;
  message?: string;
  detail?: string;
};

export type SDSSyncSummary = {
  variant_id?: number;
  product_id?: number;
  prototype_group_id?: number;
  layer_id?: string;
  material_id?: number;
  mockup_image_urls?: string[];
  status?: string;
  error?: string;
};

export type ListingKitTaskResultData = {
  task_id?: string;
  tenant_id?: string;
  status?: string;
  review_reasons?: string[];
  platforms?: string[];
  country?: string;
  language?: string;
  summary?: {
    source_type?: string;
    image_count?: number;
    variant_count?: number;
    needs_review?: boolean;
    issue_count?: number;
    warning_count?: number;
    review_count?: number;
    blocking_count?: number;
    warnings?: string[];
  };
  pod_execution?: PodExecutionSummary;
  sds_sync?: SDSSyncSummary;
  sds_design_result?: SDSSyncSummary;
  child_tasks?: ListingKitChildTask[];
  workflow_stages?: ListingKitWorkflowStage[];
  workflow_issues?: ListingKitWorkflowIssue[];
  shein_store_resolution?: SheinStoreResolutionSummary;
  canonical_product?: CanonicalProduct;
  created_at?: string;
  updated_at?: string;
};

export type RetryableBlock = {
  reason_code?: string;
  reason_message?: string;
  blocked_at?: string;
  last_retry_at?: string;
  next_retry_at?: string;
  retry_attempts?: number;
  max_auto_retry_attempts?: number;
  recovery_scope?: string;
  auto_resume_enabled?: boolean;
  auto_retry_paused?: boolean;
};

export type ListingKitTaskResult = {
  task_id?: string;
  tenant_id?: string;
  status?: string;
  shein_workflow_status?: string;
  shein_latest_submission_status?: string;
  shein_latest_submission_error?: string;
  shein_submission_remote_status?: string;
  result?: ListingKitTaskResultData;
  retryable_block?: RetryableBlock;
  error?: string;
  review_reasons?: string[];
  created_at?: string;
  completed_at?: string;
};

export type RecoverTaskNowResponse = {
  task?: {
    id?: string;
    tenant_id?: string;
    status?: string;
    retryable_block?: RetryableBlock;
    error?: string;
    created_at?: string;
    updated_at?: string;
  };
};

export type BulkRecoverTasksRequest = {
  due_before?: string;
  recover_at?: string;
  limit?: number;
};

export type BulkRecoverTasksResponse = {
  recovered_count: number;
};

export type ListingKitTaskListQuery = {
  tenant_id?: string;
  status?: string;
  platform?: string;
  source_type?: string;
  readiness_status?: string;
  shein_workflow_status?: string;
  shein_latest_submission_status?: string;
  shein_blocker_key?: string;
  shein_warning_key?: string;
  shein_work_queue?: string;
  shein_action_queue?: string;
  include_summary?: boolean;
  page?: number;
  page_size?: number;
};

export type ListingKitTaskFacetDescriptor = {
  key: string;
  label?: string;
  description?: string;
  severity?: string;
};

export type ListingKitTaskListItem = {
  task_id: string;
  tenant_id?: string;
  status?: string;
  pod_execution?: PodExecutionSummary;
  platforms?: string[];
  title?: string;
  image_count?: number;
  source_type?: string;
  product_name?: string;
  variant_label?: string;
  sds_sync_status?: string;
  shein_workflow_status?: string;
  shein_blocking_keys?: string[];
  shein_warning_keys?: string[];
  shein_work_queue?: string;
  shein_action_queue?: string;
  shein_store_id?: number;
  shein_store_site?: string;
  shein_store_profile_id?: number;
  shein_store_resolved_at?: string;
  shein_store_strategy?: string;
  shein_store_reason?: string;
  shein_store_matched_rule_kinds?: string[];
  shein_store_manual_override?: boolean;
  shein_store_fallback?: boolean;
  shein_status_overview?: SheinStatusOverview;
  shein_latest_submission_status?: string;
  shein_latest_submission_error?: string;
  shein_submission_remote_status?: string;
  shein_submission_remote_checked_at?: string;
  shein_submission_remote_record_id?: string;
  error?: string;
  created_at?: string;
  updated_at?: string;
  completed_at?: string;
};

export type ListingKitTaskListSummary = {
  status_counts?: Record<string, number>;
  shein_workflow_status_counts?: Record<string, number>;
  shein_work_queue_counts?: Record<string, number>;
  shein_action_queue_counts?: Record<string, number>;
  shein_blocker_counts?: Record<string, number>;
  shein_warning_counts?: Record<string, number>;
};

export type ListingKitTaskListTaxonomy = {
  shein_workflow_statuses?: ListingKitTaskFacetDescriptor[];
  shein_work_queues?: ListingKitTaskFacetDescriptor[];
  shein_action_queues?: ListingKitTaskFacetDescriptor[];
  shein_blockers?: ListingKitTaskFacetDescriptor[];
  shein_warnings?: ListingKitTaskFacetDescriptor[];
};

export type ListingKitTaskListPage = {
  page: number;
  page_size: number;
  total: number;
  summary?: ListingKitTaskListSummary;
  taxonomy?: ListingKitTaskListTaxonomy;
  items?: ListingKitTaskListItem[];
};

export type CreateListingKitTaskRequest = {
  tenant_id?: string;
  image_urls?: string[];
  text?: string;
  product_url?: string;
  platforms: string[];
  shein_store_id?: number;
  country?: string;
  language?: string;
  options?: {
    image_strategy?: "ai_generated" | "sds_official" | "hybrid";
    process_images?: boolean;
    scene?: {
      scene_category?: string;
      scene_style?: string;
      background_tone?: string;
      composition?: string;
      props_level?: string;
      audience_hint?: string;
      custom_scene_hint?: string;
    };
    shein_studio?: {
      style_id?: string;
      style_name?: string;
      source_design_urls?: string[];
      source_design_width?: number;
      source_design_height?: number;
      product_image_urls?: string[];
      selected_sds_images?: Array<{
        image_url?: string;
        variant_sku?: string;
        color?: string;
      }>;
      variant_product_images?: Array<{
        variant_sku?: string;
        color?: string;
        image_urls?: string[];
      }>;
      size_reference_image_urls?: string[];
      render_size_images_with_sds?: boolean;
    };
    sds?: {
      variant_id: number;
      parent_product_id?: number;
      prototype_group_id?: number;
      layer_id?: string;
      design_type?: string;
      fit_level?: number;
      resize_mode?: number;
      product_name?: string;
      product_sku?: string;
      product_english_name?: string;
      category_path?: string[];
      material?: string;
      material_description?: string;
      production_process?: string;
      product_performance?: string;
      applicable_scenarios?: string;
      washing_instructions?: string;
      special_description?: string;
      product_size?: string;
      packaging_specification?: string;
      design_area?: string;
      picture_request?: string;
      is_electricity?: number;
      variant_sku?: string;
      variant_size?: string;
      variant_color?: string;
      variant_price?: number;
      variant_weight?: number;
      production_cycle?: number;
      blank_design_url?: string;
      template_image_url?: string;
      mask_image_url?: string;
      printable_width?: number;
      printable_height?: number;
      mockup_image_urls?: string[];
      style_id?: string;
      style_name?: string;
      variants?: Array<{
        variant_id: number;
        variant_sku?: string;
        size?: string;
        color?: string;
        price?: number;
        weight?: number;
        box_length?: number;
        box_width?: number;
        box_height?: number;
        production_cycle?: number;
        prototype_group_id?: number;
        layer_id?: string;
        template_image_url?: string;
        mask_image_url?: string;
        blank_design_url?: string;
        mockup_image_url?: string;
        mockup_image_urls?: string[];
      }>;
    };
  };
};

export type CreateListingKitTaskResponse = {
  task_id: string;
  tenant_id?: string;
  status?: string;
  created_at?: string;
};

export type UploadImagesResponse = {
  image_urls?: string[];
};

export type CanonicalFieldTrace = {
  sources?: Array<{
    type?: string;
    detail?: string;
    value?: string;
  }>;
  confidence?: number;
  needs_review?: boolean;
  review_reason?: string;
};

export type CanonicalAttribute = {
  value?: string;
  unit?: string;
  trace?: CanonicalFieldTrace;
};

export type CanonicalProduct = {
  title?: string;
  brand?: string;
  description?: string;
  category_path?: string[];
  images?: Array<{
    url?: string;
    alt?: string;
    role?: string;
  }>;
  attributes?: Record<string, CanonicalAttribute>;
  specifications?: {
    dimensions?: Record<string, unknown>;
    weight?: Record<string, unknown>;
    package?: Record<string, unknown>;
    technical?: Record<string, string>;
    [key: string]: unknown;
  };
  variants?: Array<{
    sku?: string;
    title?: string;
    price?: Record<string, unknown>;
    stock?: number;
    attributes?: Record<string, CanonicalAttribute>;
    images?: Array<{ url?: string; alt?: string; role?: string }>;
    [key: string]: unknown;
  }>;
  field_traces?: Record<string, CanonicalFieldTrace>;
  needs_review?: boolean;
  [key: string]: unknown;
};

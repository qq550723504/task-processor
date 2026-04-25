import type { PlatformScenePresetSummary } from "./preview";

export type SheinReadinessReason = {
  code?: string;
  category?: string;
  summary?: string;
};

export type SheinRepairValidationPreview = {
  valid?: boolean;
  status?: string;
  affected_sections?: string[];
};

export type SheinRepairHint = {
  action?: string;
  priority?: string;
  target?: string;
  editor_section?: string;
  editor_focus?: string[];
  revision_path?: string;
  description?: string;
  field_paths?: string[];
  validation?: SheinRepairValidationPreview;
};

export type SheinReadinessItem = {
  key?: string;
  label?: string;
  message?: string;
  field_paths?: string[];
  suggested_action?: string;
  reason?: SheinReadinessReason;
  repair_hints?: SheinRepairHint[];
};

export type SheinReadinessCheck = {
  key?: string;
  label?: string;
  status?: string;
  message?: string;
  field_paths?: string[];
  suggested_action?: string;
  reason?: SheinReadinessReason;
  repair_hints?: SheinRepairHint[];
};

export type SheinSubmitReadiness = {
  ready?: boolean;
  status?: string;
  summary?: string[];
  blocking_items?: SheinReadinessItem[];
  warning_items?: SheinReadinessItem[];
  checks?: SheinReadinessCheck[];
};

export type SheinChecklistGroupItem = {
  key?: string;
  label?: string;
  status?: string;
  message?: string;
  field_paths?: string[];
  suggested_action?: string;
  reason?: SheinReadinessReason;
  repair_hints?: SheinRepairHint[];
};

export type SheinSubmitChecklist = {
  required?: SheinChecklistGroupItem[];
  recommended?: SheinChecklistGroupItem[];
  optional?: SheinChecklistGroupItem[];
};

export type SheinStatusOverview = {
  status?: string;
  headline?: string;
  subheadline?: string;
  needs_review?: boolean;
  blocking_count?: number;
  warning_count?: number;
  highlights?: string[];
  primary_action?: string;
  primary_action_key?: string;
  next_actions?: string[];
};

export type SheinWorkspaceSubmitState = {
  status?: string;
  ready?: boolean;
  blocking_count?: number;
  warning_count?: number;
  summary?: string[];
};

export type SheinWorkspaceOverview = {
  status?: string;
  headline?: string;
  subheadline?: string;
  primary_action?: string;
  primary_action_key?: string;
  primary_view?: string;
  needs_review?: boolean;
  blocking_count?: number;
  warning_count?: number;
  highlights?: string[];
  next_actions?: string[];
  submit_state?: SheinWorkspaceSubmitState;
};

export type SheinResolutionCacheInfo = {
  status?: string;
  source?: string;
  short_key?: string;
  hit_count?: number;
  updated_at?: string;
  manual?: boolean;
  clearable?: boolean;
};

export type SheinResolutionCacheSummary = {
  category?: SheinResolutionCacheInfo;
  attributes?: SheinResolutionCacheInfo;
  sale_attributes?: SheinResolutionCacheInfo;
};

export type PlatformPreviewPayload = {
  render_previews?: unknown;
  scene_presets?: PlatformScenePresetSummary[];
};

export type SheinImageInfo = {
  main_image?: string;
  white_bg?: string;
  source?: string[];
  gallery?: string[];
  image_info_list?: Array<{
    image_url?: string;
    imageUrl?: string;
  }>;
};

export type SheinSKUDraftPreview = {
  main_image?: string;
  image_info?: SheinImageInfo;
};

export type SheinSKCDraftPreview = {
  image_info?: SheinImageInfo;
  sku_list?: SheinSKUDraftPreview[];
};

export type SheinRequestDraftPreview = {
  image_info?: SheinImageInfo;
  skc_list?: SheinSKCDraftPreview[];
};

export type SheinPreviewProductSKU = {
  image_info?: SheinImageInfo;
};

export type SheinPreviewProductSKC = {
  image_info?: SheinImageInfo;
  sku_list?: SheinPreviewProductSKU[];
};

export type SheinPreviewProductPayload = {
  image_info?: SheinImageInfo;
  skc_list?: SheinPreviewProductSKC[];
};

export type SheinCategorySuggestion = {
  source?: string;
  reason?: string;
  matched_path?: string[];
  category_id?: number;
  category_id_list?: number[];
  product_type_id?: number;
  top_category_id?: number;
};

export type SheinInspectionCategoryPayload = {
  status?: string;
  source?: string;
  category_name?: string;
  category_path?: string[];
  category_id?: number;
  category_id_list?: number[];
  product_type_id?: number;
  top_category_id?: number;
  suggested_category?: SheinCategorySuggestion;
  review_notes?: string[];
};

export type SheinResolvedAttribute = {
  name?: string;
  value?: string;
  attribute_id?: number;
  attribute_value_id?: number;
  attribute_extra_value?: string;
  matched_by?: string;
  required?: boolean;
  skc_scope?: boolean;
};

export type SheinInspectionAttributePayload = {
  status?: string;
  source?: string;
  template_count?: number;
  resolved_count?: number;
  unresolved_count?: number;
  resolved_attributes?: SheinResolvedAttribute[];
  review_notes?: string[];
};

export type SheinResolvedSaleAttribute = {
  scope?: string;
  name?: string;
  value?: string;
  attribute_id?: number;
  attribute_value_id?: number;
  matched_by?: string;
};

export type SheinSaleAttributeCandidateInfo = {
  source_dimension?: string;
  name?: string;
  attribute_id?: number;
  skc_scope?: boolean;
  required?: boolean;
  selected_scope?: string;
  reasons?: string[];
};

export type SheinInspectionSaleAttributePayload = {
  status?: string;
  source?: string;
  recommend_category_review?: boolean;
  category_review_reason?: string;
  primary_attribute_id?: number;
  secondary_attribute_id?: number;
  selection_summary?: string[];
  skc_attributes?: SheinResolvedSaleAttribute[];
  sku_attributes?: SheinResolvedSaleAttribute[];
  candidate_count?: number;
  candidates?: SheinSaleAttributeCandidateInfo[];
  review_notes?: string[];
};

export type SheinRevisionSaleAttributePatch = {
  recommend_category_review?: boolean;
  category_review_reason?: string;
};

export type SheinEditorContext = {
  category?: {
    current?: SheinInspectionCategoryPayload;
  };
  attributes?: {
    current?: SheinInspectionAttributePayload;
  };
  sale_attributes?: {
    current?: SheinInspectionSaleAttributePayload;
  };
  revision_skeleton?: {
    shein?: {
      sale_attribute_resolution?: SheinRevisionSaleAttributePatch;
    };
  };
};

export type SheinPreviewPayload = PlatformPreviewPayload & {
  source_product?: {
    title?: string;
    sku?: string;
    category_path?: string[];
    attributes?: Record<string, string>;
    variant_sku?: string;
    variant_size?: string;
    variant_color?: string;
    variant_price?: number;
    variant_weight?: number;
    production_cycle?: string;
    image_urls?: string[];
  };
  request_draft?: SheinRequestDraftPreview;
  preview_product?: SheinPreviewProductPayload;
  editor_context?: SheinEditorContext;
  submit_readiness?: SheinSubmitReadiness;
  submit_checklist?: SheinSubmitChecklist;
  image_upload?: SheinImageUploadPreflight;
  resolution_cache?: SheinResolutionCacheSummary;
  status_overview?: SheinStatusOverview;
  workspace_overview?: SheinWorkspaceOverview;
  submission?: SheinSubmissionReport;
};

export type SheinImageUploadPreflight = {
  total_image_references?: number;
  unique_image_urls?: number;
  pending_upload_urls?: number;
  shein_uploaded_urls?: number;
  sds_mockup_urls?: number;
  uses_sds_mockups?: boolean;
  ready_for_upload?: boolean;
  summary?: string[];
};

export type SheinSubmissionResponse = {
  code?: string;
  message?: string;
  success?: boolean;
  spu_name?: string;
  version?: string;
  validation_notes?: string[];
};

export type SheinSubmissionRecord = {
  action?: string;
  status?: string;
  error?: string;
  submitted_at?: string;
  result?: SheinSubmissionResponse;
};

export type SheinSubmissionReport = {
  last_action?: string;
  last_status?: string;
  last_error?: string;
  submitted_at?: string;
  save_draft?: SheinSubmissionRecord;
  publish?: SheinSubmissionRecord;
  last_result?: SheinSubmissionResponse;
};

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

export type SheinManualCategoryCandidate = {
  category_id?: number;
  category_id_list?: number[];
  category_path?: string[];
  product_type_id?: number;
  top_category_id?: number;
  source?: string;
  match_reason?: string;
};

export type SheinManualCategorySearchResult = {
  task_id?: string;
  query?: string;
  items?: SheinManualCategoryCandidate[];
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
  attribute_type?: number;
  attribute_mode?: number;
  data_dimension?: number;
  cascade_attribute_id?: number;
  matched_by?: string;
  required?: boolean;
  important?: boolean;
  skc_scope?: boolean;
};

export type SheinAttributeValueCandidate = {
  attribute_value_id?: number;
  value?: string;
  value_en?: string;
};

export type SheinPendingAttributeCandidate = {
  name?: string;
  attribute_id?: number;
  attribute_name?: string;
  attribute_name_en?: string;
  attribute_type?: number;
  attribute_mode?: number;
  data_dimension?: number;
  cascade_attribute_id?: number;
  required?: boolean;
  important?: boolean;
  skc_scope?: boolean;
  attribute_value_list?: SheinAttributeValueCandidate[];
};

export type SheinSourceAttribute = {
  name?: string;
  value?: string;
};

export type SheinInspectionAttributePayload = {
  status?: string;
  source?: string;
  template_count?: number;
  resolved_count?: number;
  unresolved_count?: number;
  resolved_attributes?: SheinResolvedAttribute[];
  pending_attributes?: SheinSourceAttribute[];
  pending_attribute_candidates?: SheinPendingAttributeCandidate[];
  recommended_attribute_candidates?: SheinPendingAttributeCandidate[];
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
  status?: string;
  source?: string;
  recommend_category_review?: boolean;
  category_review_reason?: string;
  primary_attribute_id?: number;
  secondary_attribute_id?: number;
  skc_attributes?: SheinResolvedSaleAttribute[];
  sku_attributes?: SheinResolvedSaleAttribute[];
  selection_summary?: string[];
  review_notes?: string[];
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
  category_id?: number;
  category_path?: string[];
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
  pricing?: SheinPricingReview;
  final_review?: SheinFinalReview;
  store_resolution?: SheinStoreResolutionSummary;
  submission_events?: SheinSubmissionEvent[];
};

export type SheinPricingRule = {
  source_currency?: string;
  target_currency?: string;
  exchange_rate?: number;
  markup_multiplier?: number;
  minimum_price?: number;
  round_to?: number;
  price_ending?: number;
};

export type SheinSKUPriceReview = {
  supplier_sku?: string;
  supplier_code?: string;
  cost_cny?: number;
  calculated_price?: number;
  final_price?: number;
  currency?: string;
  manual?: boolean;
};

export type SheinPricingReview = {
  rule_snapshot?: SheinPricingRule;
  sku_prices?: SheinSKUPriceReview[];
  manual_overrides?: Record<string, number>;
  missing_price_skus?: string[];
  ready?: boolean;
  updated_at?: string;
};

export type SheinFinalReviewSKU = {
  supplier_code?: string;
  supplier_sku?: string;
  color?: string;
  size?: string;
  price?: number;
  currency?: string;
  stock?: number;
  weight?: number;
};

export type SheinFinalReviewImage = {
  url?: string;
  role?: string;
  sort?: number;
  final?: boolean;
  main?: boolean;
  swatch?: boolean;
  size_map?: boolean;
};

export type SheinFinalReview = {
  confirmed?: boolean;
  submit_mode?: "publish" | "save_draft";
  store_id?: number;
  site?: string;
  source_product?: SheinPreviewPayload["source_product"];
  title?: string;
  description?: string;
  category_path?: string[];
  category_id?: number;
  attributes?: SheinResolvedAttribute[];
  sale_attributes?: SheinResolvedSaleAttribute[];
  skus?: SheinFinalReviewSKU[];
  images?: SheinFinalReviewImage[];
  blocking_items?: SheinReadinessItem[];
};

export type SheinStoreResolutionSummary = {
  store_id?: number;
  site?: string;
  strategy?: string;
  reason?: string;
  matched_rule_kinds?: string[];
  matched_profile_id?: number;
  manual_override?: boolean;
  fallback?: boolean;
  resolved_at?: string;
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
  request_id?: string;
  phase?: string;
  started_at?: string;
  finished_at?: string;
  attempt?: number;
  supplier_code?: string;
  remote_record_id?: string;
  remote_state?: number;
  remote_audit_state?: number;
  remote_message?: string;
  remote_checked_at?: string;
};

export type SheinSubmissionReport = {
  last_action?: string;
  last_status?: string;
  last_error?: string;
  submitted_at?: string;
  save_draft?: SheinSubmissionRecord;
  publish?: SheinSubmissionRecord;
  last_result?: SheinSubmissionResponse;
  current_action?: string;
  current_phase?: string;
  current_request_id?: string;
  in_flight_started_at?: string;
  lease_expires_at?: string;
  remote_status?: "confirmed" | "pending" | "failed" | string;
  remote_checked_at?: string;
  attempt_count?: number;
};

export type SheinSubmissionEvent = {
  id?: string;
  task_id?: string;
  platform?: string;
  action?: string;
  phase?: string;
  status?: string;
  request_id?: string;
  started_at?: string;
  finished_at?: string;
  detail?: string;
  remote_record_id?: string;
  error_message?: string;
  validation_notes?: string[];
  response?: SheinSubmissionResponse;
  store_resolution?: SheinStoreResolutionSummary;
};

export type SheinSettings = {
  default_store_id?: number;
  enabled_store_ids?: number[];
  fallback_store_id?: number;
  available_stores?: Array<{
    id: number;
    store_id?: string;
    name?: string;
    platform?: string;
    region?: string;
  }>;
  site?: string;
  warehouse_code?: string;
  default_stock?: number;
  default_submit_mode?: "publish" | "save_draft";
  pricing?: SheinPricingRule;
  updated_at?: string;
};

export type AIClientSettings = {
  scope?: "tenant" | "user" | string;
  client_name?: string;
  api_key?: string;
  api_key_set?: boolean;
  base_url?: string;
  model?: string;
  timeout_second?: number;
  enabled?: boolean;
  updated_at?: string;
};

export type ListingKitStoreMatchRule = {
  kind?: string;
  values?: string[];
};

export type ListingKitStoreProfileStoreOption = {
  id: number;
  store_id?: string;
  name?: string;
  platform?: string;
  region?: string;
};

export type ListingKitStoreProfile = {
  id?: number;
  tenant_id?: number;
  store_id: number;
  enabled?: boolean;
  priority?: number;
  is_fallback?: boolean;
  site?: string;
  warehouse_code?: string;
  default_stock?: number;
  default_submit_mode?: "publish" | "save_draft";
  pricing?: SheinPricingRule;
  match_rules?: ListingKitStoreMatchRule[];
  updated_at?: string;
  store?: ListingKitStoreProfileStoreOption;
};

export type ListingKitStoreRoutingSettings = {
  tenant_id?: number;
  selection_strategy?: "manual" | "priority" | "country" | string;
  fallback_store_id?: number;
  allow_manual_override?: boolean;
  allow_fallback?: boolean;
  updated_at?: string;
};

export type ListingKitSettingsScopeDefinition = {
  id: string;
  label: string;
  description?: string;
};

export type ListingKitSettingsFieldDefinition = {
  key: string;
  label: string;
  type: string;
  required?: boolean;
  description?: string;
};

export type ListingKitSettingsNamespaceSchema = {
  namespace: string;
  label: string;
  description: string;
  supported_scopes?: ListingKitSettingsScopeDefinition[];
  fields?: ListingKitSettingsFieldDefinition[];
  supports_status_toggle?: boolean;
};

export type ListingKitSettingsNamespaceListResponse = {
  items: ListingKitSettingsNamespaceSchema[];
};

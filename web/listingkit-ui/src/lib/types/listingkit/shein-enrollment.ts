export type SheinSyncTriggerMode = "manual" | "schedule";

export type SheinSyncedProductQuery = {
  skc_name?: string;
  is_active?: boolean;
  page?: number;
  page_size?: number;
};

export type SheinActivityCandidateQuery = {
  activity_type: string;
  activity_key?: string;
  skc_name?: string;
  candidate_version?: string;
  executable_only?: boolean;
  page?: number;
  page_size?: number;
};

export type SheinEnrollmentSummaryQuery = {
  activity_type?: string;
  page?: number;
  page_size?: number;
};

export type SheinEnrollmentRunQuery = {
  activity_type?: string;
  activity_key?: string;
  page?: number;
  page_size?: number;
};

export type SheinEnrollmentRunItemQuery = {
  status?: string;
  include_payload?: boolean;
  page?: number;
  page_size?: number;
};

export type SheinReviewActivityCandidateInput = {
  store_id: number;
  review_status:
    | "pending_review"
    | "approved"
    | "rejected"
    | "auto_queued"
    | "enrolled"
    | "failed";
  auto_mode_eligible?: boolean;
  selected_for_run?: boolean;
};

export type ReviewSheinActivityCandidateInput =
  SheinReviewActivityCandidateInput;

export type SheinExecuteEnrollmentInput = {
  activity_type: string;
  activity_key?: string;
  trigger_mode?: "manual_confirmed" | "auto_schedule";
  candidate_ids?: number[];
};

export type SheinRefreshCandidatesInput = {
  activity_type: string;
};

export type SheinActivityPriceMode = "DISCOUNT" | "PROFIT";
export type SheinActivityPartakeType = "REGULAR" | "LIMITED" | "BOTH";

export type SheinActivityStrategyRecord = {
  id?: number;
  tenant_id?: number;
  store_id?: number;
  activity_type?: string;
  activity_price_mode?: SheinActivityPriceMode | string;
  activity_partake_type?: SheinActivityPartakeType | string;
  activity_discount_rate?: number;
  activity_limited_discount_rate?: number;
  activity_stock_ratio?: number;
  activity_min_profit_rate?: number;
  activity_limited_min_profit_rate?: number;
  fixed_price_adjustment?: number;
};

export type SheinActivityStrategyResponse = {
  configured?: boolean;
  strategy?: SheinActivityStrategyRecord | null;
};

export type SheinUpdateActivityStrategyInput = {
  activity_type?: string;
  activity_price_mode: SheinActivityPriceMode;
  activity_partake_type: SheinActivityPartakeType;
  activity_discount_rate?: number;
  activity_limited_discount_rate?: number;
  activity_stock_ratio?: number;
  activity_min_profit_rate?: number;
  activity_limited_min_profit_rate?: number;
  fixed_price_adjustment?: number;
};

export type SheinUpdateSyncedProductCostInput = {
  manual_cost_price?: number | null;
};

export type SheinSDSCostGroupQuery = {
  page?: number;
  page_size?: number;
};

export type SheinUpdateSDSCostGroupInput = {
  group_label?: string;
  manual_cost_price?: number | null;
};

export type SheinSyncedProductRecord = {
  id?: number;
  tenant_id?: number;
  store_id?: number;
  spu_name?: string;
  spu_code?: string;
  skc_name?: string;
  skc_code?: string;
  supplier_code?: string;
  category_id?: number;
  brand_name?: string;
  product_name_multi?: string;
  main_image_url?: string;
  sale_name?: string;
  shelf_status?: string;
  publish_time?: string;
  first_shelf_time?: string;
  currency?: string;
  price_snapshot?: string;
  supply_price?: number | null;
  supply_price_currency?: string;
  inventory_snapshot?: string;
  site_snapshot?: string;
  auto_cost_price?: number | null;
  manual_cost_price?: number | null;
  effective_cost_price?: number | null;
  cost_price_source?: "none" | "auto" | "manual" | string;
  sync_version?: string;
  last_sync_at?: string;
  is_active?: boolean;
  created_at?: string;
  updated_at?: string;
};

export type SheinSyncedProductListResponse = {
  items?: SheinSyncedProductRecord[];
  total?: number;
};

export type SheinSDSCostGroupRecord = {
  id?: number;
  tenant_id?: number;
  store_id?: number;
  group_key?: string;
  group_label?: string;
  manual_cost_price?: number | null;
  created_at?: string;
  updated_at?: string;
};

export type SheinSDSCostGroupListResponse = {
  items?: SheinSDSCostGroupRecord[];
  total?: number;
};

export type SheinSourceSDSCostGroupRecord = {
  group_key?: string;
  group_label?: string;
  source_code?: string;
  sku_codes?: string[];
  sku_groups?: SheinSourceSDSSKUCostGroupRecord[];
  legacy_group_keys?: string[];
  product_count?: number;
  products?: SheinSyncedProductRecord[];
  manual_cost_price?: number | null;
};

export type SheinSourceSDSSKUCostGroupRecord = {
  group_key?: string;
  group_label?: string;
  source_code?: string;
  sku_code?: string;
  variant_label?: string;
  sku_codes?: string[];
  product_count?: number;
  products?: SheinSyncedProductRecord[];
  legacy_group_keys?: string[];
  manual_cost_price?: number | null;
};

export type SheinSourceSDSCostGroupListResponse = {
  items?: SheinSourceSDSCostGroupRecord[];
  total?: number;
};

export type SheinSourceSDSMetadataRecord = {
  source_code?: string;
  title?: string;
  product_sku?: string;
  variant_sku?: string;
  price?: number;
  variant_label?: string;
  image_url?: string;
};

export type SheinSourceSDSMetadataResponse = {
  items?: SheinSourceSDSMetadataRecord[];
};

export type UpdateSheinSDSCostGroupResponse = {
  group?: SheinSDSCostGroupRecord;
};

export type SheinSyncJobRecord = {
  id?: number;
  tenant_id?: number;
  store_id?: number;
  trigger_mode?: "manual" | "schedule" | string;
  status?:
    | "pending"
    | "running"
    | "succeeded"
    | "partially_succeeded"
    | "failed"
    | string;
  started_at?: string;
  finished_at?: string;
  fetched_count?: number;
  inserted_count?: number;
  updated_count?: number;
  deactivated_count?: number;
  skipped_count?: number;
  error_summary?: string;
  created_at?: string;
  updated_at?: string;
};

export type TriggerSheinStoreSyncResponse = {
  job?: SheinSyncJobRecord;
};

export type SyncSheinSourceSDSProductResponse = {
  source_code?: string;
  synced_count?: number;
};

export type SheinActivityCandidateRecord = {
  id?: number;
  tenant_id?: number;
  store_id?: number;
  synced_product_id?: number;
  activity_type?: string;
  activity_key?: string;
  skc_name?: string;
  candidate_version?: string;
  effective_cost_price?: number | null;
  price_snapshot?: string;
  inventory_snapshot?: string;
  calculated_profit_rate?: number | null;
  main_image_url?: string;
  eligibility_status?: "eligible" | "ineligible" | string;
  eligibility_reason?: string;
  review_status?:
    | "pending_review"
    | "approved"
    | "rejected"
    | "auto_queued"
    | "enrolled"
    | "failed"
    | string;
  last_enrollment_error?: string;
  auto_mode_eligible?: boolean;
  selected_for_run?: boolean;
  created_at?: string;
  updated_at?: string;
};

export type SheinActivityCandidateListResponse = {
  items?: SheinActivityCandidateRecord[];
  total?: number;
};

export type SheinRefreshCandidatesResult = {
  activity_type?: string;
  activity_key?: string;
  candidate_version?: string;
  processed_count?: number;
  eligible_count?: number;
  ineligible_count?: number;
};

export type RefreshSheinActivityCandidatesResponse = {
  result?: SheinRefreshCandidatesResult;
};

export type ReviewSheinActivityCandidateResponse = {
  candidate?: SheinActivityCandidateRecord;
};

export type SheinActivityEnrollmentRunRecord = {
  id?: number;
  tenant_id?: number;
  store_id?: number;
  activity_type?: string;
  activity_key?: string;
  trigger_mode?: "manual_confirmed" | "auto_schedule" | string;
  status?:
    | "pending"
    | "running"
    | "succeeded"
    | "partially_succeeded"
    | "failed"
    | "cancelled"
    | string;
  candidate_count?: number;
  submitted_count?: number;
  succeeded_count?: number;
  failed_count?: number;
  started_at?: string;
  finished_at?: string;
  error_summary?: string;
  created_at?: string;
  updated_at?: string;
};

export type SheinActivityEnrollmentItemRecord = {
  id?: number;
  run_id?: number;
  candidate_id?: number;
  store_id?: number;
  activity_key?: string;
  candidate_version?: string;
  synced_product_id?: number;
  skc_name?: string;
  status?: "succeeded" | "failed" | string;
  request_payload?: string;
  response_payload?: string;
  error_message?: string;
  created_at?: string;
  updated_at?: string;
};

export type SheinEnrollmentStoreSummary = {
  store_id?: number;
  store_name?: string;
  store_username?: string;
  platform?: string;
  region?: string;
  enable_auto_listing?: boolean;
  activity_type?: string;
  synced_product_count?: number;
  missing_cost_count?: number;
  pending_review_count?: number;
  ready_to_enroll_count?: number;
  last_sync_at?: string;
  last_sync_status?: string;
  last_sync_job?: SheinSyncJobRecord;
  last_enrollment_at?: string;
  last_enrollment_run?: SheinActivityEnrollmentRunRecord;
};

export type SheinEnrollmentDashboardResponse = {
  items?: SheinEnrollmentStoreSummary[];
  total?: number;
  activity_type?: string;
};

export type SheinEnrollmentStoreSummaryResponse = {
  summary?: SheinEnrollmentStoreSummary;
};

export type SheinEnrollmentRunListResponse = {
  items?: SheinActivityEnrollmentRunRecord[];
  total?: number;
};

export type SheinEnrollmentRunItemListResponse = {
  items?: SheinActivityEnrollmentItemRecord[];
  total?: number;
};

export type ExecuteSheinActivityEnrollmentResponse = {
  run?: SheinActivityEnrollmentRunRecord;
};

export type SDSRetirementRunStatus =
  | "draft"
  | "ready"
  | "running"
  | "succeeded"
  | "partially_succeeded"
  | "failed"
  | "cancelled";

export type SDSRetirementItemStatus =
  | "pending"
  | "selected"
  | "running"
  | "succeeded"
  | "succeeded_already_off_shelf"
  | "failed"
  | "skipped";

export type SDSRetirementRun = {
  id: string;
  tenant_id?: string;
  platform: "shein" | string;
  store_id: number;
  parent_product_id: number;
  prototype_group_id: number;
  variant_id: number;
  selected_variant_ids?: string;
  baseline_key?: string;
  validation_status?: string;
  reason_code?: string;
  reason?: string;
  status: SDSRetirementRunStatus;
  created_by?: string;
  confirmed_by?: string;
  confirmed_at?: string;
  started_at?: string;
  finished_at?: string;
  created_at?: string;
  updated_at?: string;
};

export type SDSRetirementItem = {
  id: string;
  run_id: string;
  tenant_id?: string;
  platform: "shein" | string;
  store_id: number;
  task_id?: string;
  synced_product_id?: number;
  spu_name?: string;
  skc_name?: string;
  skc_code?: string;
  supplier_code?: string;
  business_model?: number;
  shelf_status_before?: string;
  selected: boolean;
  site_selection?: string;
  request_snapshot?: string;
  response_snapshot?: string;
  status: SDSRetirementItemStatus;
  error?: string;
  started_at?: string;
  finished_at?: string;
  created_at?: string;
  updated_at?: string;
};

export type SDSRetirementRunDetail = {
  run: SDSRetirementRun;
  items: SDSRetirementItem[];
  reason?: string;
};

export type CreateSDSRetirementRunInput = {
  tenant_id?: string;
  platform: "shein";
  store_id: number;
  parent_product_id: number;
  prototype_group_id: number;
  variant_id: number;
  selected_variant_ids?: number[];
  source_task_id?: string;
  created_by?: string;
};

export type SDSRetirementSelectionUpdate = {
  item_id: string;
  selected: boolean;
  site_selection?: string;
};

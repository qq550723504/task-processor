export type ImageAgentRunMode = "manual";

export type ImageAgentRunStatus =
  | "planning"
  | "awaiting_plan_approval"
  | "executing"
  | "evaluating"
  | "repairing"
  | "awaiting_final_approval"
  | "blocked"
  | "completed"
  | "failed"
  | "cancelled";

export type ImageAgentSlotStatus =
  | "pending"
  | "executing"
  | "evaluating"
  | "accepted"
  | "rejected"
  | "blocked";

export type ImageAgentSlotRole =
  | "main"
  | "scene"
  | "detail"
  | "selling_point"
  | "size";

export type ImageAgentAction =
  | "edit_plan"
  | "retry_slot"
  | "approve_results"
  | "cancel"
  | "switch_manual";

export type ImageAgentSlot = {
  id: string;
  role: ImageAgentSlotRole;
  source_asset_ids: string[];
  style_reference_ids?: string[];
  brief?: string;
  idempotency_key: string;
  status: ImageAgentSlotStatus;
};

export type ImageAgentPlan = {
  revision: number;
  parent_revision: number;
  idempotency_key: string;
  source_asset_ids: string[];
  style_reference_ids?: string[];
  slots: ImageAgentSlot[];
  created_by?: string;
};

export type ImageAgentBudget = {
  max_images: number;
  max_agent_steps: number;
  max_model_calls: number;
  max_repair_attempts_per_slot: number;
  max_cost_micros: number;
  max_elapsed: number;
};

export type ImageAgentBudgetUsage = {
  images: number;
  agent_steps: number;
  model_calls: number;
  estimated_cost_micros: number;
  elapsed: number;
};

export type ImageAgentBlock = {
  code: string;
  message: string;
  slot_id?: string;
};

export type ImageAgentRun = {
  id: string;
  business_task_id: string;
  tenant_id: string;
  user_id: string;
  mode: ImageAgentRunMode;
  idempotency_key: string;
  status: ImageAgentRunStatus;
  current_node: string;
  active_plan_revision: number;
  version: number;
  budget: ImageAgentBudget;
  usage: ImageAgentBudgetUsage;
  block?: ImageAgentBlock;
};

export type ImageAgentAuthorizedAsset = {
  id: string;
  type: "source" | "style";
  display_url?: string;
  label?: string;
};

export type ImageAgentPendingCommand = {
  action_id: string;
  kind: string;
  phase: string;
  status: "pending";
  plan_revision: number;
  slot_id?: string;
};

export type ImageAgentCandidate = {
  asset_id: string;
  url: string;
  source_asset_id?: string;
  metadata?: Record<string, string>;
};

export type ImageAgentSlotProjection = {
  slot: ImageAgentSlot;
  attempt: number;
  candidates: ImageAgentCandidate[];
  error_code?: string;
};

export type ImageAgentProjection = {
  run: ImageAgentRun;
  plan: ImageAgentPlan;
  slots: ImageAgentSlotProjection[];
  result_digest?: string;
  actions: ImageAgentAction[];
  last_event_id: number;
  projection_version: number;
  asset_catalog: ImageAgentAuthorizedAsset[];
  pending_command?: ImageAgentPendingCommand;
};

export type ImageAgentProjectionEvent = {
  schema_version: "image-agent.projection.v1";
  type: string;
  projection_version: number;
};

export type CreateImageAgentRunInput = {
  run_id: string;
  business_task_id: string;
  mode: "manual";
  idempotency_key: string;
  plan: ImageAgentPlan;
  budget: ImageAgentBudget;
  max_concurrent_slots?: number;
};

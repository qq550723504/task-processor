type ImageAgentRunMode = "manual";

type ImageAgentRunStatus =
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

type ImageAgentSlotStatus =
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
  | "restart"
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

type ImageAgentBudget = {
  max_images: number;
  max_agent_steps: number;
  max_model_calls: number;
  max_repair_attempts_per_slot: number;
  max_cost_micros: number;
  max_elapsed: number;
  enabled_limits?: Array<
    | "max_images"
    | "max_agent_steps"
    | "max_model_calls"
    | "max_repair_attempts_per_slot"
    | "max_cost_micros"
    | "max_elapsed"
  >;
};

type ImageAgentBudgetUsage = {
  images: number;
  agent_steps: number;
  model_calls: number;
  estimated_cost_micros: number;
  elapsed: number;
};

type ImageAgentBlock = {
  code: string;
  message: string;
  slot_id?: string;
};

type ImageAgentRun = {
  id: string;
  business_task_id: string;
  target_platform?: string;
  tenant_id: string;
  user_id: string;
  mode: ImageAgentRunMode;
  idempotency_key: string;
  status: ImageAgentRunStatus;
  current_node: string;
  active_plan_revision: number;
  version: number;
  max_concurrent_slots?: number;
  budget: ImageAgentBudget;
  usage: ImageAgentBudgetUsage;
  block?: ImageAgentBlock;
};

type ImageAgentWorkspaceAsset = {
  id: string;
  label: string;
  display_url: string;
};

export type ImageAgentWorkspaceAssets = {
  target_platform?: string;
  source_assets: ImageAgentWorkspaceAsset[];
  style_candidates: ImageAgentWorkspaceAsset[];
};

export type ImageAgentAuthorizedAsset = {
  id: string;
  type: "source" | "style";
  display_url?: string;
  label?: string;
};

type ImageAgentPendingCommand = {
  action_id: string;
  kind: string;
  phase: string;
  status: "pending";
  plan_revision: number;
  slot_id?: string;
  failure_code?: string;
  failure_category?: string;
  failure_message?: string;
  last_failed_at?: string;
  attempt?: number;
};

type ImageAgentCommandIngress = {
  used: number;
  limit: number;
  exhausted: boolean;
  reason?: string;
};

type ImageAgentCandidate = {
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
  command_ingress?: ImageAgentCommandIngress;
};

export type ImageAgentProjectionEvent = {
  schema_version: "image-agent.projection.v1";
  type: string;
  projection_version: number;
};

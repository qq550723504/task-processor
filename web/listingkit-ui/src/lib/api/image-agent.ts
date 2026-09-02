import { z } from "zod";

import { ApiError } from "@/lib/api/api-error";
import type {
  ImageAgentPlan,
  ImageAgentProjection,
} from "@/lib/types/image-agent";

const imageAgentBffBase = "/api/listing-kits/image-agent/runs";

const slotRoleSchema = z.enum([
  "main",
  "scene",
  "detail",
  "selling_point",
  "size",
]);
const slotStatusSchema = z.enum([
  "pending",
  "executing",
  "evaluating",
  "accepted",
  "rejected",
  "blocked",
]);
const slotSchema = z
  .object({
    id: z.string(),
    role: slotRoleSchema,
    source_asset_ids: z.array(z.string()),
    style_reference_ids: z.array(z.string()).optional(),
    brief: z.string().optional(),
    idempotency_key: z.string(),
    status: slotStatusSchema,
  })
  .passthrough();
const planSchema = z
  .object({
    revision: z.number().int().positive(),
    parent_revision: z.number().int().nonnegative(),
    idempotency_key: z.string(),
    source_asset_ids: z.array(z.string()),
    style_reference_ids: z.array(z.string()).optional(),
    slots: z.array(slotSchema),
    created_by: z.string().optional(),
  })
  .passthrough();
const budgetSchema = z
  .object({
    max_images: z.number(),
    max_agent_steps: z.number(),
    max_model_calls: z.number(),
    max_repair_attempts_per_slot: z.number(),
    max_cost_micros: z.number(),
    max_elapsed: z.number(),
    enabled_limits: z
      .array(
        z.enum([
          "max_images",
          "max_agent_steps",
          "max_model_calls",
          "max_repair_attempts_per_slot",
          "max_cost_micros",
          "max_elapsed",
        ]),
      )
      .optional(),
  })
  .passthrough();
const usageSchema = z
  .object({
    images: z.number(),
    agent_steps: z.number(),
    model_calls: z.number(),
    estimated_cost_micros: z.number(),
    elapsed: z.number(),
  })
  .passthrough();
const projectionSchema = z
  .object({
    run: z
      .object({
        id: z.string(),
        business_task_id: z.string(),
			target_platform: z.string().optional(),
        tenant_id: z.string(),
        user_id: z.string(),
        mode: z.literal("manual"),
        idempotency_key: z.string(),
        status: z.enum([
          "planning",
          "awaiting_plan_approval",
          "executing",
          "evaluating",
          "repairing",
          "awaiting_final_approval",
          "blocked",
          "completed",
          "failed",
          "cancelled",
        ]),
        current_node: z.string(),
        active_plan_revision: z.number().int().nonnegative(),
        version: z.number().int().nonnegative(),
        max_concurrent_slots: z.number().int().positive().optional(),
        budget: budgetSchema,
        usage: usageSchema,
        block: z
          .object({
            code: z.string(),
            message: z.string(),
            slot_id: z.string().optional(),
          })
          .passthrough()
          .optional(),
      })
      .passthrough(),
    plan: planSchema,
    slots: z.array(
      z
        .object({
          slot: slotSchema,
          attempt: z.number().int().nonnegative(),
          candidates: z.array(
            z
              .object({
                asset_id: z.string(),
                url: z.string(),
                source_asset_id: z.string().optional(),
                metadata: z.record(z.string(), z.string()).optional(),
              })
              .passthrough(),
          ),
          error_code: z.string().optional(),
        })
        .passthrough(),
    ),
    result_digest: z.string().optional(),
    actions: z.array(
      z.enum([
        "edit_plan",
        "retry_slot",
        "approve_results",
        "cancel",
        "restart",
        "switch_manual",
      ]),
    ),
    last_event_id: z.number().int().nonnegative(),
    projection_version: z.number().int().nonnegative(),
    asset_catalog: z.array(z.object({
      id: z.string(),
      type: z.enum(["source", "style"]),
      display_url: z.string().optional(),
      label: z.string().optional(),
    }).passthrough()),
    pending_command: z.object({
      action_id: z.string(), kind: z.string(), phase: z.string(),
      status: z.literal("pending"), plan_revision: z.number().int().positive(),
      slot_id: z.string().optional(),
    }).passthrough().optional(),
  })
  .passthrough();

export function parseImageAgentProjection(payload: unknown): ImageAgentProjection {
  return projectionSchema.parse(payload) as ImageAgentProjection;
}

export async function getImageAgentRun(runId: string, signal?: AbortSignal) {
	const payload = await imageAgentRequest<unknown>(encodeId(runId), { signal });
  return parseImageAgentProjection(payload);
}

export function replaceImageAgentPlan(
  runId: string,
  expectedRevision: number,
  plan: ImageAgentPlan,
	actionId: string,
	signal?: AbortSignal,
) {
  return command(`${encodeId(runId)}/plan`, "PUT", {
    expected_revision: expectedRevision,
    action_id: actionId,
    plan,
	}, signal);
}

export function retryImageAgentSlot(
  runId: string,
  slotId: string,
  planRevision: number,
	actionId: string,
	signal?: AbortSignal,
) {
  return command(`${encodeId(runId)}/slots/${encodeId(slotId)}/retry`, "POST", {
    plan_revision: planRevision,
    action_id: actionId,
	}, signal);
}

export function approveImageAgentResults(
  runId: string,
  planRevision: number,
  resultDigest: string,
	actionId: string,
	signal?: AbortSignal,
) {
  return command(`${encodeId(runId)}/results/approve`, "POST", {
    plan_revision: planRevision,
    result_digest: resultDigest,
    action_id: actionId,
	}, signal);
}

export function cancelImageAgentRun(
  runId: string,
  planRevision: number,
	actionId: string,
	signal?: AbortSignal,
) {
  return command(`${encodeId(runId)}/cancel`, "POST", {
    plan_revision: planRevision,
    action_id: actionId,
	}, signal);
}

export function restartFailedImageAgentRun(runId: string, signal?: AbortSignal) {
  return command(`${encodeId(runId)}/restart`, "POST", undefined, signal);
}

export function resumeImageAgentCommand(runId: string, actionId: string, signal?: AbortSignal) {
  return imageAgentRequest<{ run_id: string; plan_revision: number; action_id: string; status: string }>(
    `${encodeId(runId)}/commands/${encodeId(actionId)}/resume`,
		{ method: "POST", signal },
  );
}

export function imageAgentEventsUrl(runId: string, afterCursor?: number) {
  const base = `${imageAgentBffBase}/${encodeId(runId)}/events`;
  if (!Number.isSafeInteger(afterCursor) || Number(afterCursor) <= 0) return base;
  return `${base}?after_cursor=${afterCursor}`;
}

async function command(
  path: string,
  method: "POST" | "PUT",
  body?: unknown,
  signal?: AbortSignal,
) {
  await imageAgentRequest(path, {
    method,
		body, signal,
  });
}

async function imageAgentRequest<T>(
  path: string,
	{ method = "GET", body, signal }: { method?: "GET" | "POST" | "PUT"; body?: unknown; signal?: AbortSignal } = {},
): Promise<T> {
  const response = await fetch(
    path ? `${imageAgentBffBase}/${path}` : imageAgentBffBase,
    {
      method,
      headers: {
        Accept: "application/json",
        ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      },
		body: body === undefined ? undefined : JSON.stringify(body),
		signal,
    },
  );
  if (response.ok) {
    const text = await response.text();
    return (text ? JSON.parse(text) : undefined) as T;
  }
  const payload = await response.json().catch(() => null);
  const message =
    payload &&
    typeof payload === "object" &&
    "message" in payload &&
    typeof payload.message === "string"
      ? payload.message
      : `Image Agent request failed: ${response.status}`;
  throw new ApiError(message, response.status, payload);
}

function encodeId(value: string) {
  return encodeURIComponent(value.trim());
}

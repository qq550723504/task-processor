import { z } from "zod";

import { apiRequest, ApiError } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";

export const subscriptionStatusSchema = z.enum([
  "active",
  "trialing",
  "expired",
  "disabled",
]);

export const subscriptionModuleSchema = z
  .object({
    code: z.string(),
    name: z.string(),
    description: z.string().optional(),
    sort_order: z.number(),
    active: z.boolean(),
    created_at: z.string().optional(),
    updated_at: z.string().optional(),
  })
  .passthrough();

export const subscriptionEntitlementSchema = z
  .object({
    id: z.number(),
    tenant_id: z.string(),
    module_code: z.string(),
    status: subscriptionStatusSchema,
    starts_at: z.string().optional(),
    expires_at: z.string().optional(),
    limits: z.record(z.string(), z.number()).optional(),
    created_at: z.string().optional(),
    updated_at: z.string().optional(),
  })
  .passthrough();

export const subscriptionUsageCounterSchema = z
  .object({
    id: z.number(),
    tenant_id: z.string(),
    module_code: z.string(),
    period_key: z.string(),
    metric: z.string(),
    used: z.number(),
    updated_at: z.string().optional(),
  })
  .passthrough();

export const subscriptionEntitlementViewSchema = z
  .object({
    module: subscriptionModuleSchema,
    entitlement: subscriptionEntitlementSchema.optional(),
    usage: z.array(subscriptionUsageCounterSchema),
    allowed: z.boolean(),
    reason: z.string().optional(),
    limits: z.record(z.string(), z.number()).optional(),
    used: z.record(z.string(), z.number()).optional(),
  })
  .passthrough();

export const subscriptionSummarySchema = z
  .object({
    tenant_id: z.string(),
    modules: z.array(subscriptionModuleSchema),
    entitlements: z.array(subscriptionEntitlementViewSchema),
  })
  .passthrough();

const subscriptionModuleListSchema = z
  .object({
    items: z.array(subscriptionModuleSchema),
  })
  .passthrough();

const subscriptionRequiredPayloadSchema = z
  .object({
    error: z.enum(["subscription_required", "quota_exceeded"]),
    module_code: z.string().optional(),
    metric: z.string().optional(),
    limit: z.number().optional(),
    used: z.number().optional(),
    message: z.string().optional(),
    reason: z.string().optional(),
  })
  .passthrough();

export type SubscriptionStatus = z.infer<typeof subscriptionStatusSchema>;
export type SubscriptionModule = z.infer<typeof subscriptionModuleSchema>;
export type SubscriptionEntitlement = z.infer<
  typeof subscriptionEntitlementSchema
>;
export type SubscriptionEntitlementView = z.infer<
  typeof subscriptionEntitlementViewSchema
>;
export type SubscriptionSummary = z.infer<typeof subscriptionSummarySchema>;
export type SubscriptionRequiredPayload = z.infer<
  typeof subscriptionRequiredPayloadSchema
>;

export type SubscriptionEntitlementInput = {
  status: SubscriptionStatus;
  starts_at?: string;
  expires_at?: string;
  limits?: Record<string, number>;
};

export function parseSubscriptionSummary(payload: unknown): SubscriptionSummary {
  return parseApiResponseShape(
    payload,
    subscriptionSummarySchema,
    "ListingKit API returned an unexpected subscription summary response",
  );
}

export function parseSubscriptionModuleList(
  payload: unknown,
): SubscriptionModule[] {
  return parseApiResponseShape(
    payload,
    subscriptionModuleListSchema,
    "ListingKit API returned an unexpected subscription module response",
  ).items;
}

export function parseSubscriptionEntitlement(
  payload: unknown,
): SubscriptionEntitlement {
  return parseApiResponseShape(
    payload,
    subscriptionEntitlementSchema,
    "ListingKit API returned an unexpected subscription entitlement response",
  );
}

export function parseSubscriptionRequiredPayload(
  payload: unknown,
): SubscriptionRequiredPayload | null {
  const result = subscriptionRequiredPayloadSchema.safeParse(payload);
  return result.success ? result.data : null;
}

export function formatSubscriptionApiError(error: unknown): string {
  if (!(error instanceof ApiError) || error.status !== 402) {
    return error instanceof Error ? error.message : String(error);
  }
  const payload = parseSubscriptionRequiredPayload(error.payload);
  if (!payload) {
    return error.message;
  }
  if (payload.error === "quota_exceeded") {
    return `模块额度不足：${payload.module_code ?? "unknown"} ${payload.metric ?? ""} ${payload.used ?? 0}/${payload.limit ?? 0}`;
  }
  return `模块未开通或已失效：${payload.module_code ?? "unknown"}`;
}

export async function getCurrentSubscription(): Promise<SubscriptionSummary> {
  const payload = await apiRequest<unknown>("/subscription/me");
  return parseSubscriptionSummary(payload);
}

export async function getSubscriptionModules(): Promise<SubscriptionModule[]> {
  const payload = await apiRequest<unknown>("/admin/subscription/modules");
  return parseSubscriptionModuleList(payload);
}

export async function updateSubscriptionEntitlement(
  moduleCode: string,
  input: SubscriptionEntitlementInput,
): Promise<SubscriptionEntitlement> {
  const payload = await apiRequest<unknown>(
    `/admin/subscription/entitlements/${encodeURIComponent(moduleCode)}`,
    {
      method: "PUT",
      body: input,
    },
  );
  return parseSubscriptionEntitlement(payload);
}

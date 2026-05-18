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

export const subscriptionPlanSchema = z
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

export const subscriptionPlanModuleSchema = z
  .object({
    plan_code: z.string(),
    module_code: z.string(),
    limits: z.record(z.string(), z.number()).optional(),
    sort_order: z.number(),
  })
  .passthrough();

export const subscriptionPlanBundleSchema = z
  .object({
    plan: subscriptionPlanSchema,
    modules: z.array(subscriptionPlanModuleSchema),
  })
  .passthrough();

export const tenantSubscriptionSchema = z
  .object({
    id: z.number(),
    tenant_id: z.string(),
    plan_code: z.string(),
    status: subscriptionStatusSchema,
    starts_at: z.string().optional(),
    expires_at: z.string().optional(),
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

const subscriptionUsageListSchema = z
  .array(subscriptionUsageCounterSchema)
  .nullish()
  .transform((value) => value ?? []);

export const subscriptionAuditLogSchema = z
  .object({
    id: z.number(),
    tenant_id: z.string(),
    module_code: z.string().optional(),
    action: z.string(),
    actor_id: z.string().optional(),
    reason: z.string().optional(),
    payload: z.string().optional(),
    created_at: z.string(),
  })
  .passthrough();

export const subscriptionEntitlementViewSchema = z
  .object({
    module: subscriptionModuleSchema,
    entitlement: subscriptionEntitlementSchema.optional(),
    usage: subscriptionUsageListSchema,
    allowed: z.boolean(),
    reason: z.string().optional(),
    limits: z.record(z.string(), z.number()).optional(),
    used: z.record(z.string(), z.number()).optional(),
  })
  .passthrough();

export const subscriptionSummarySchema = z
  .object({
    tenant_id: z.string(),
    subscription: tenantSubscriptionSchema.optional(),
    current_plan: subscriptionPlanBundleSchema.optional(),
    modules: z.array(subscriptionModuleSchema),
    entitlements: z.array(subscriptionEntitlementViewSchema),
  })
  .passthrough();

export const subscriptionTenantOverviewSchema = z
  .object({
    tenant_id: z.string(),
    tenant_display_name: z.string().optional(),
    entitlement_count: z.number(),
    active_count: z.number(),
    updated_at: z.string().optional(),
  })
  .passthrough();

const subscriptionModuleListSchema = z
  .object({
    items: z.array(subscriptionModuleSchema),
  })
  .passthrough();

const subscriptionPlanListSchema = z
  .object({
    items: z.array(subscriptionPlanBundleSchema),
  })
  .passthrough();

const subscriptionTenantOverviewListSchema = z
  .object({
    items: z.array(subscriptionTenantOverviewSchema),
  })
  .passthrough();

const subscriptionAuditLogListSchema = z
  .object({
    items: z.array(subscriptionAuditLogSchema),
  })
  .passthrough();

const tenantSubscriptionListSchema = z
  .object({
    items: z.array(tenantSubscriptionSchema),
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
export type SubscriptionPlan = z.infer<typeof subscriptionPlanSchema>;
export type SubscriptionPlanBundle = z.infer<typeof subscriptionPlanBundleSchema>;
export type TenantSubscription = z.infer<typeof tenantSubscriptionSchema>;
export type SubscriptionEntitlement = z.infer<
  typeof subscriptionEntitlementSchema
>;
export type SubscriptionEntitlementView = z.infer<
  typeof subscriptionEntitlementViewSchema
>;
export type SubscriptionSummary = z.infer<typeof subscriptionSummarySchema>;
export type SubscriptionTenantOverview = z.infer<
  typeof subscriptionTenantOverviewSchema
>;
export type SubscriptionUsageCounter = z.infer<
  typeof subscriptionUsageCounterSchema
>;
export type SubscriptionAuditLog = z.infer<typeof subscriptionAuditLogSchema>;
export type SubscriptionRequiredPayload = z.infer<
  typeof subscriptionRequiredPayloadSchema
>;

export type SubscriptionEntitlementInput = {
  status: SubscriptionStatus;
  starts_at?: string;
  expires_at?: string;
  limits?: Record<string, number>;
};

export type SubscriptionUsageAdjustmentInput = {
  period_key: string;
  metric: string;
  used: number;
  reason?: string;
};

export type SubscriptionPlanApplyInput = {
  plan_code: string;
  status: SubscriptionStatus;
  starts_at?: string;
  expires_at?: string;
};

export type SubscriptionPlanInput = {
  code: string;
  name: string;
  description?: string;
  sort_order: number;
  active: boolean;
  modules?: SubscriptionPlanModuleInput[];
};

export type SubscriptionPlanModuleInput = {
  module_code?: string;
  limits?: Record<string, number>;
  sort_order: number;
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

export function parseSubscriptionPlanList(
  payload: unknown,
): SubscriptionPlanBundle[] {
  return parseApiResponseShape(
    payload,
    subscriptionPlanListSchema,
    "ListingKit API returned an unexpected subscription plan response",
  ).items;
}

export function parseSubscriptionPlanBundle(
  payload: unknown,
): SubscriptionPlanBundle {
  return parseApiResponseShape(
    payload,
    subscriptionPlanBundleSchema,
    "ListingKit API returned an unexpected subscription plan response",
  );
}

export function parseSubscriptionTenantOverviewList(
  payload: unknown,
): SubscriptionTenantOverview[] {
  return parseApiResponseShape(
    payload,
    subscriptionTenantOverviewListSchema,
    "ListingKit API returned an unexpected subscription tenant list response",
  ).items;
}

export function parseSubscriptionAuditLogList(
  payload: unknown,
): SubscriptionAuditLog[] {
  return parseApiResponseShape(
    payload,
    subscriptionAuditLogListSchema,
    "ListingKit API returned an unexpected subscription audit log response",
  ).items;
}

export function parseTenantSubscriptionList(
  payload: unknown,
): TenantSubscription[] {
  return parseApiResponseShape(
    payload,
    tenantSubscriptionListSchema,
    "ListingKit API returned an unexpected tenant subscription list response",
  ).items;
}

export function parseSubscriptionUsageCounter(
  payload: unknown,
): SubscriptionUsageCounter {
  return parseApiResponseShape(
    payload,
    subscriptionUsageCounterSchema,
    "ListingKit API returned an unexpected subscription usage response",
  );
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

export function parseTenantSubscription(payload: unknown): TenantSubscription {
  return parseApiResponseShape(
    payload,
    tenantSubscriptionSchema,
    "ListingKit API returned an unexpected tenant subscription response",
  );
}

export function parseSubscriptionRequiredPayload(
  payload: unknown,
): SubscriptionRequiredPayload | null {
  const result = subscriptionRequiredPayloadSchema.safeParse(payload);
  return result.success ? result.data : null;
}

export function formatSubscriptionApiError(error: unknown): string {
  if (error instanceof ApiError && error.status === 403) {
    return "没有平台订阅管理权限";
  }
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

export async function getPlatformSubscriptionPlans(): Promise<
  SubscriptionPlanBundle[]
> {
  const payload = await apiRequest<unknown>("/platform/subscription-plans");
  return parseSubscriptionPlanList(payload);
}

export async function getPlatformSubscriptionPlanTenants(
  planCode: string,
): Promise<TenantSubscription[]> {
  const payload = await apiRequest<unknown>(
    `/platform/subscription-plans/${encodeURIComponent(planCode)}/tenants`,
  );
  return parseTenantSubscriptionList(payload);
}

export async function getPlatformSubscriptionPlanAuditLogs(
  planCode: string,
): Promise<SubscriptionAuditLog[]> {
  const payload = await apiRequest<unknown>(
    `/platform/subscription-plans/${encodeURIComponent(planCode)}/audit-logs`,
  );
  return parseSubscriptionAuditLogList(payload);
}

export async function upsertPlatformSubscriptionPlan(
  input: SubscriptionPlanInput,
): Promise<SubscriptionPlanBundle> {
  const payload = await apiRequest<unknown>("/platform/subscription-plans", {
    method: "POST",
    body: input,
  });
  return parseSubscriptionPlanBundle(payload);
}

export async function updatePlatformSubscriptionPlan(
  planCode: string,
  input: SubscriptionPlanInput,
): Promise<SubscriptionPlanBundle> {
  const payload = await apiRequest<unknown>(
    `/platform/subscription-plans/${encodeURIComponent(planCode)}`,
    {
      method: "PUT",
      body: input,
    },
  );
  return parseSubscriptionPlanBundle(payload);
}

export async function updatePlatformSubscriptionPlanModule(
  planCode: string,
  moduleCode: string,
  input: SubscriptionPlanModuleInput,
): Promise<SubscriptionPlanBundle> {
  const payload = await apiRequest<unknown>(
    `/platform/subscription-plans/${encodeURIComponent(planCode)}/modules/${encodeURIComponent(moduleCode)}`,
    {
      method: "PUT",
      body: input,
    },
  );
  return parseSubscriptionPlanBundle(payload);
}

export async function deletePlatformSubscriptionPlanModule(
  planCode: string,
  moduleCode: string,
): Promise<SubscriptionPlanBundle> {
  const payload = await apiRequest<unknown>(
    `/platform/subscription-plans/${encodeURIComponent(planCode)}/modules/${encodeURIComponent(moduleCode)}`,
    { method: "DELETE" },
  );
  return parseSubscriptionPlanBundle(payload);
}

export async function setPlatformSubscriptionPlanStatus(
  planCode: string,
  active: boolean,
): Promise<SubscriptionPlanBundle> {
  const payload = await apiRequest<unknown>(
    `/platform/subscription-plans/${encodeURIComponent(planCode)}/status`,
    {
      method: "PUT",
      body: { active },
    },
  );
  return parseSubscriptionPlanBundle(payload);
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

export async function getPlatformTenantSubscription(
  tenantId: string,
): Promise<SubscriptionSummary> {
  const payload = await apiRequest<unknown>(
    `/platform/subscriptions/${encodeURIComponent(tenantId)}`,
  );
  return parseSubscriptionSummary(payload);
}

export async function getPlatformTenantSubscriptions(): Promise<
  SubscriptionTenantOverview[]
> {
  const payload = await apiRequest<unknown>("/platform/subscriptions");
  return parseSubscriptionTenantOverviewList(payload);
}

export async function updatePlatformTenantSubscriptionEntitlement(
  tenantId: string,
  moduleCode: string,
  input: SubscriptionEntitlementInput,
): Promise<SubscriptionEntitlement> {
  const payload = await apiRequest<unknown>(
    `/platform/subscriptions/${encodeURIComponent(tenantId)}/entitlements/${encodeURIComponent(moduleCode)}`,
    {
      method: "PUT",
      body: input,
    },
  );
  return parseSubscriptionEntitlement(payload);
}

export async function applyPlatformTenantSubscriptionPlan(
  tenantId: string,
  input: SubscriptionPlanApplyInput,
): Promise<TenantSubscription> {
  const payload = await apiRequest<unknown>(
    `/platform/subscriptions/${encodeURIComponent(tenantId)}/plan`,
    {
      method: "PUT",
      body: input,
    },
  );
  return parseTenantSubscription(payload);
}

export async function updatePlatformTenantSubscriptionUsage(
  tenantId: string,
  moduleCode: string,
  input: SubscriptionUsageAdjustmentInput,
): Promise<SubscriptionUsageCounter> {
  const payload = await apiRequest<unknown>(
    `/platform/subscriptions/${encodeURIComponent(tenantId)}/usage/${encodeURIComponent(moduleCode)}/${encodeURIComponent(input.period_key)}/${encodeURIComponent(input.metric)}`,
    {
      method: "PUT",
      body: input,
    },
  );
  return parseSubscriptionUsageCounter(payload);
}

export async function getPlatformTenantSubscriptionAuditLogs(
  tenantId: string,
): Promise<SubscriptionAuditLog[]> {
  const payload = await apiRequest<unknown>(
    `/platform/subscriptions/${encodeURIComponent(tenantId)}/audit-logs`,
  );
  return parseSubscriptionAuditLogList(payload);
}

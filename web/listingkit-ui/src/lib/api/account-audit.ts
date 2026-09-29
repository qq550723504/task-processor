import { z } from "zod";
import { AccountReadError, accountErrorCode } from "./account";
import { InvalidStrictJSONResponseError, readBoundedStrictJSON } from "./strict-json-response";

const identity = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const reference = z.string().uuid();
const resourceReference = z.string().min(1).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
// Resource writers use opaque, trimmed IDs capped at 128 UTF-8 bytes.
// Go strings.TrimSpace uses Unicode White_Space, which differs from JS trim.
const operationReference = z.string().min(1).refine(value => new TextEncoder().encode(value).length <= 128 && !/^\p{White_Space}|\p{White_Space}$/u.test(value));
const version = z.string().regex(/^[1-9][0-9]{0,18}$/);
const sourceOperation = z.enum(["register", "enable", "disable"]);
const resourceOperation = z.enum(["allocate_member_resource", "reclaim_member_resource", "set_member_ai_point_limit"]);
const profileOperation = z.literal("update");
const membershipOperation = z.enum(["invite", "role", "remove"]);
const auditOperation = z.union([sourceOperation, resourceOperation, profileOperation, membershipOperation]);
const sourceEvent = z.object({
  eventType: z.literal("source_account.operation_committed"),
  actor: z.string().min(1).max(256).regex(/^[^\x00-\x1f\x7f]+$/),
  time: z.string().max(40).datetime({ precision: null }),
  objectType: z.literal("source_account"), objectReference: reference,
  operation: sourceOperation, result: z.literal("succeeded"),
  relation: z.object({ type: z.literal("source_account_version"), reference, version }).strict(),
}).strict().refine(value => value.objectReference === value.relation.reference);
const resourceFields = {
  actor: z.string().min(1).max(256).regex(/^[^\x00-\x1f\x7f]+$/),
  time: z.string().max(40).datetime({ precision: null }),
  objectReference: resourceReference, result: z.literal("succeeded"),
  relation: z.object({ type: z.literal("organization_resource_operation"), reference: operationReference, version }).strict(),
};
const exactNonnegative = z.string().refine(value => /^(0|[1-9][0-9]{0,18})$/.test(value) && BigInt(value) <= BigInt("9223372036854775807"));
const resourceEvent = z.object({
  ...resourceFields, eventType: z.literal("account_member_resource.changed"), objectType: z.literal("member_resource"),
  operation: z.enum(["allocate_member_resource", "reclaim_member_resource"]),
  resource: z.object({ type: z.enum(["store_renewal_period", "data_row"]), quantity: exactNonnegative.refine(value => value !== "0") }).strict(),
}).strict();
const limitEvent = z.object({
  ...resourceFields, eventType: z.literal("account_member_ai_point_limit.changed"), objectType: z.literal("member_ai_point_limit"),
  operation: z.literal("set_member_ai_point_limit"),
  resource: z.object({ type: z.literal("ai_point"), quantity: exactNonnegative }).strict(),
}).strict();
const profileEvent = z.object({
  eventType: z.literal("account_business_profile.updated"),
  actor: z.string().min(1).max(256).regex(/^[^\x00-\x1f\x7f]+$/),
  time: z.string().max(40).datetime({ precision: null }),
  objectType: z.literal("account_business_profile"), objectReference: resourceReference,
  operation: profileOperation, result: z.literal("succeeded"),
  relation: z.object({ type: z.literal("account_business_profile_version"), reference: resourceReference, version }).strict(),
}).strict().refine(value => value.objectReference === value.relation.reference);
const membershipEvent = z.object({
  eventType: z.literal("organization_membership.changed"),
  actor: z.string().min(1).max(256).regex(/^[^\x00-\x1f\x7f]+$/),
  time: z.string().max(40).datetime({ precision: null }),
  objectType: z.literal("organization_member"), objectReference: resourceReference,
  operation: membershipOperation, result: z.literal("succeeded"),
  relation: z.object({ type: z.literal("organization_membership_operation"), reference: z.string().uuid(), version }).strict(),
}).strict();
const usageEvent = z.object({
  eventType: z.literal("ai_invocation.usage_observed"), actor: z.literal(""),
  time: z.string().max(40).datetime({ precision: null }),
  objectType: z.literal("ai_invocation"), objectReference: resourceReference,
  operation: z.literal("observe"), result: z.literal("observed"),
  relation: z.object({ type: z.literal("ai_invocation"), reference: resourceReference, version: z.literal("") }).strict(),
  usage: z.object({ memberId: resourceReference, quantity: z.number().int().positive(), metric: z.literal("model_tokens") }).strict(),
}).strict();
const pointEvent = z.object({
  eventType: z.literal("account_ai_points.committed"), actor: z.string().min(1).max(192).regex(/^[^\x00-\x1f\x7f]+$/),
  time: z.string().max(40).datetime({ precision: null }),
  objectType: z.literal("image_generation"), objectReference: resourceReference,
  operation: z.literal("consume"), result: z.literal("succeeded"),
  relation: z.object({ type: z.literal("organization_resource_event"), reference: resourceReference, version: z.literal("") }).strict(),
  points: z.object({ memberId: resourceReference, quantity: z.string().refine(value => /^[1-9][0-9]{0,18}$/.test(value) && BigInt(value) <= BigInt("9223372036854775807")), priceVersion: z.string().min(1).max(192).regex(/^[^\x00-\x1f\x7f]+$/).refine(value => value.trim() === value), intentId: resourceReference }).strict(),
}).strict();
const event = z.union([sourceEvent, resourceEvent, limitEvent, profileEvent, membershipEvent, usageEvent, pointEvent]);
const cursor = z.string().min(1).max(2048).regex(/^[A-Za-z0-9_-]+$/);
const source = z.string().regex(/^source_account_committed_operations(\+account_business_profile_audit)?(\+organization_member_audit)?(\+ai_invocations)?(\+image_ai_point_debits)?(\+member_resource_audit)?$/);
const page = z.object({
  schemaVersion: z.literal("account-audit-v1"), userId: identity, effectiveOrganizationId: identity,
  source, items: z.array(event).max(100), nextCursor: cursor.nullable(),
}).strict().refine(value => value.items.length > 0 || value.nextCursor === null);
export type AccountAuditPage = z.infer<typeof page>;
const summary = z.object({
  schemaVersion: z.literal("account-audit-summary-v1"), userId: identity, effectiveOrganizationId: identity,
  coverage: z.literal("current_account_audit_committed_events"),
  window: z.object({ from: z.string().max(40).datetime({ precision: null }), asOf: z.string().max(40).datetime({ precision: null }) }).strict(),
  counts: z.object({ operations: exactNonnegative, members: exactNonnegative, permissions: exactNonnegative, resources: exactNonnegative }).strict(),
}).strict().refine(value => Date.parse(value.window.from) + 30 * 24 * 60 * 60 * 1000 === Date.parse(value.window.asOf)
  && BigInt(value.counts.permissions) <= BigInt(value.counts.members)
  && BigInt(value.counts.members) + BigInt(value.counts.resources) <= BigInt(value.counts.operations));
export type AccountAuditSummary = z.infer<typeof summary>;
export function parseAccountAuditSummary(value: unknown): AccountAuditSummary {
  const parsed = summary.safeParse(value);
  if (!parsed.success) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
  return parsed.data;
}
export const AUDIT_RESPONSE_MAX_BYTES = 128 * 1024;

export function parseAccountAudit(value: unknown): AccountAuditPage {
  const parsed = page.safeParse(value);
  if (!parsed.success) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
  return parsed.data;
}
export type AuditOptions = { expectedUserId: string; expectedOrganizationId: string; limit?: number; cursor?: string; actor?: string; operation?: z.infer<typeof auditOperation>; signal?: AbortSignal };
export function auditQuery(limit = 20, after?: string, actor?: string, kind?: AuditOptions["operation"]): string {
  if (!Number.isInteger(limit) || limit < 1 || limit > 100 || after !== undefined && !cursor.safeParse(after).success || kind !== undefined && !auditOperation.safeParse(kind).success) throw new AccountReadError(400, "INVALID_REQUEST");
  const query = new URLSearchParams({ limit: String(limit) });
  if (after !== undefined) query.set("cursor", after);
  if (actor !== undefined && actor !== "") query.set("actor", actor);
  if (kind !== undefined) query.set("operation", kind);
  return query.toString();
}
export async function getAccountAudit(options: AuditOptions): Promise<AccountAuditPage> {
  return readAccountAudit(options, false) as Promise<AccountAuditPage>;
}
export async function getAccountAuditSummary(options: Pick<AuditOptions, "expectedUserId" | "expectedOrganizationId" | "signal">): Promise<AccountAuditSummary> {
  return readAccountAudit(options, true) as Promise<AccountAuditSummary>;
}
async function readAccountAudit(options: AuditOptions, isSummary: boolean): Promise<AccountAuditPage | AccountAuditSummary> {
  if (!identity.safeParse(options.expectedUserId).success) throw new AccountReadError(409, "IDENTITY_CONTEXT_CHANGED");
  if (!identity.safeParse(options.expectedOrganizationId).success) throw new AccountReadError(409, "ORGANIZATION_SELECTION_REQUIRED");
  const path = isSummary ? "/api/account/audit/summary" : `/api/account/audit?${auditQuery(options.limit, options.cursor, options.actor, options.operation)}`;
  const controller = new AbortController();
  const abort = () => controller.abort();
  options.signal?.addEventListener("abort", abort, { once: true });
  if (options.signal?.aborted) abort();
  const timer = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    const headers = new Headers({ Accept: "application/json", "X-Expected-User-ID": options.expectedUserId, "X-Expected-Organization-ID": options.expectedOrganizationId });
    const response = await fetch(path, { method: "GET", headers, credentials: "same-origin", cache: "no-store", redirect: "error", signal: controller.signal });
    const payload = await readBoundedStrictJSON(response, AUDIT_RESPONSE_MAX_BYTES, controller.signal);
    controller.signal.throwIfAborted();
    if (response.status !== 200) throw new AccountReadError(response.status, accountErrorCode(response.status, payload));
    const result = isSummary ? parseAccountAuditSummary(payload) : parseAccountAudit(payload);
    if (result.userId !== options.expectedUserId) throw new AccountReadError(409, "IDENTITY_CONTEXT_CHANGED");
    if (result.effectiveOrganizationId !== options.expectedOrganizationId) throw new AccountReadError(409, "ORGANIZATION_CONTEXT_CHANGED");
    if ("items" in result && result.items.length > (options.limit ?? 20)) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
    return result;
  } catch (error) {
    if (controller.signal.aborted) throw new AccountReadError(504, "DEADLINE_EXCEEDED");
    if (error instanceof AccountReadError) throw error;
    if (error instanceof InvalidStrictJSONResponseError) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
    throw new AccountReadError(502, "DEPENDENCY_UNAVAILABLE");
  } finally { clearTimeout(timer); options.signal?.removeEventListener("abort", abort); }
}

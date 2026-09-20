import { z } from "zod";
import { AccountReadError, accountErrorCode } from "./account";
import { InvalidStrictJSONResponseError, readBoundedStrictJSON } from "./strict-json-response";

const identity = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const reference = z.string().uuid();
const version = z.string().regex(/^[1-9][0-9]{0,18}$/);
const operation = z.enum(["register", "enable", "disable"]);
const event = z.object({
  eventType: z.literal("source_account.operation_committed"),
  actor: z.string().min(1).max(256).regex(/^[^\x00-\x1f\x7f]+$/),
  time: z.string().max(40).datetime({ precision: null }),
  objectType: z.literal("source_account"), objectReference: reference,
  operation, result: z.literal("succeeded"),
  relation: z.object({ type: z.literal("source_account_version"), reference, version }).strict(),
}).strict().refine(value => value.objectReference === value.relation.reference);
const cursor = z.string().min(1).max(2048).regex(/^[A-Za-z0-9_-]+$/);
const page = z.object({
  schemaVersion: z.literal("account-audit-v1"), userId: identity, effectiveOrganizationId: identity,
  source: z.literal("source_account_committed_operations"), items: z.array(event).max(100), nextCursor: cursor.nullable(),
}).strict().refine(value => value.items.length > 0 || value.nextCursor === null);
export type AccountAuditPage = z.infer<typeof page>;
export const AUDIT_RESPONSE_MAX_BYTES = 128 * 1024;

export function parseAccountAudit(value: unknown): AccountAuditPage {
  const parsed = page.safeParse(value);
  if (!parsed.success) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
  return parsed.data;
}
export type AuditOptions = { expectedUserId: string; expectedOrganizationId: string; limit?: number; cursor?: string; signal?: AbortSignal };
export function auditQuery(limit = 20, after?: string): string {
  if (!Number.isInteger(limit) || limit < 1 || limit > 100 || after !== undefined && !cursor.safeParse(after).success) throw new AccountReadError(400, "INVALID_REQUEST");
  const query = new URLSearchParams({ limit: String(limit) });
  if (after !== undefined) query.set("cursor", after);
  return query.toString();
}
export async function getAccountAudit(options: AuditOptions): Promise<AccountAuditPage> {
  if (!identity.safeParse(options.expectedUserId).success) throw new AccountReadError(409, "IDENTITY_CONTEXT_CHANGED");
  if (!identity.safeParse(options.expectedOrganizationId).success) throw new AccountReadError(409, "ORGANIZATION_SELECTION_REQUIRED");
  const query = auditQuery(options.limit, options.cursor);
  const controller = new AbortController();
  const abort = () => controller.abort();
  options.signal?.addEventListener("abort", abort, { once: true });
  if (options.signal?.aborted) abort();
  const timer = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    const headers = new Headers({ Accept: "application/json", "X-Expected-User-ID": options.expectedUserId, "X-Expected-Organization-ID": options.expectedOrganizationId });
    const response = await fetch(`/api/account/audit?${query}`, { method: "GET", headers, credentials: "same-origin", cache: "no-store", redirect: "error", signal: controller.signal });
    const payload = await readBoundedStrictJSON(response, AUDIT_RESPONSE_MAX_BYTES, controller.signal);
    controller.signal.throwIfAborted();
    if (response.status !== 200) throw new AccountReadError(response.status, accountErrorCode(response.status, payload));
    const result = parseAccountAudit(payload);
    if (result.userId !== options.expectedUserId) throw new AccountReadError(409, "IDENTITY_CONTEXT_CHANGED");
    if (result.effectiveOrganizationId !== options.expectedOrganizationId) throw new AccountReadError(409, "ORGANIZATION_CONTEXT_CHANGED");
    if (result.items.length > (options.limit ?? 20)) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
    return result;
  } catch (error) {
    if (controller.signal.aborted) throw new AccountReadError(504, "DEADLINE_EXCEEDED");
    if (error instanceof AccountReadError) throw error;
    if (error instanceof InvalidStrictJSONResponseError) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
    throw new AccountReadError(502, "DEPENDENCY_UNAVAILABLE");
  } finally { clearTimeout(timer); options.signal?.removeEventListener("abort", abort); }
}

import { z } from "zod";
import { readBoundedStrictJSON } from "./strict-json-response";
import { accountErrorCode } from "./account";

const id = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const integer = z.string().regex(/^(0|[1-9][0-9]*)$/).refine(v => { try { return BigInt(v) <= BigInt("9223372036854775807"); } catch { return false; } });
const month = z.string().regex(/^\d{4}-(0[1-9]|1[0-2])-01T00:00:00Z$/).refine(v => Number.isFinite(Date.parse(v)));
const limit = z.object({ organizationId: id, memberId: id, configured: z.boolean(), monthlyLimit: integer, reserved: integer, consumed: integer, remaining: integer, version: integer, monthStart: month, monthEnd: month }).strict().refine(v => {
  try {
  const start = new Date(v.monthStart); start.setUTCMonth(start.getUTCMonth() + 1);
  if (start.toISOString().replace(".000Z", "Z") !== v.monthEnd) return false;
  if (BigInt(v.monthlyLimit) !== BigInt(v.reserved) + BigInt(v.consumed) + BigInt(v.remaining)) return false;
  return v.configured ? BigInt(v.version) > 0 : [v.monthlyLimit, v.reserved, v.consumed, v.remaining, v.version].every(x => x === "0");
  } catch { return false; }
});
const directoryLimit = limit.safeExtend({ displayName: z.string().max(512), loginName: z.string().max(512), roles: z.array(z.string().max(128)).max(32) });
const snapshot = z.object({ schemaVersion: z.literal("member-ai-point-monthly-limit-v1"), organizationId: id, resourceType: z.literal("ai_point"), timezone: z.literal("UTC"), members: z.array(directoryLimit).max(100) }).strict().refine(v => new Set(v.members.map(m => m.memberId)).size === v.members.length && v.members.every(m => m.organizationId === v.organizationId));
export type MemberPointScope = { expectedUserId: string; expectedOrganizationId: string };
export type MemberAIPointLimit = z.infer<typeof limit>;
export type MemberAIPointLimits = z.infer<typeof snapshot>;
export class MemberPointLimitError extends Error {
  constructor(public readonly status: number, public readonly code: string, public readonly outcome?: "unknown") { super("Member AI point limit request could not be completed"); }
}
export function memberPointErrorCode(status: number, payload: unknown): string {
  const rejection = z.object({ code: z.enum(["FORBIDDEN", "CONSUMED_FLOOR"]), message: z.string(), requestId: z.string(), fieldErrors: z.array(z.unknown()).max(0) }).strict().safeParse(payload);
  if (rejection.success && (status === 403 && rejection.data.code === "FORBIDDEN" || status === 409 && rejection.data.code === "CONSUMED_FLOOR")) return rejection.data.code;
  return accountErrorCode(status, payload);
}
export function parseMemberAIPointLimits(payload: unknown, organizationId: string): MemberAIPointLimits {
  const result = snapshot.safeParse(payload);
  if (!result.success || result.data.organizationId !== organizationId) throw new MemberPointLimitError(502, "INVALID_UPSTREAM_RESPONSE");
  return result.data;
}
export function parseMemberAIPointLimit(payload: unknown, organizationId: string, memberId: string): MemberAIPointLimit {
  const result = limit.safeParse(payload);
  if (!result.success || result.data.organizationId !== organizationId || result.data.memberId !== memberId) throw new MemberPointLimitError(502, "RESULT_UNVERIFIED", "unknown");
  return result.data;
}
async function request(scope: MemberPointScope, path: string, init: RequestInit): Promise<unknown> {
  if (!id.safeParse(scope.expectedUserId).success || !id.safeParse(scope.expectedOrganizationId).success) throw new MemberPointLimitError(400, "INVALID_REQUEST");
  if (init.signal?.aborted) throw new MemberPointLimitError(504, "DEADLINE_EXCEEDED");
  const write = init.method === "PUT";
  let response: Response;
  try { response = await fetch(path, { ...init, headers: { Accept: "application/json", "X-Expected-User-ID": scope.expectedUserId, "X-Expected-Organization-ID": scope.expectedOrganizationId, ...(init.headers ?? {}) }, cache: "no-store" }); }
  catch { throw new MemberPointLimitError(502, write ? "RESULT_UNVERIFIED" : "DEPENDENCY_UNAVAILABLE", write ? "unknown" : undefined); }
  let payload: unknown;
  try { payload = await readBoundedStrictJSON(response, response.status === 200 ? 128 * 1024 : 8192); }
  catch { throw new MemberPointLimitError(502, write ? "RESULT_UNVERIFIED" : "INVALID_UPSTREAM_RESPONSE", write ? "unknown" : undefined); }
  if (response.status !== 200) {
    let code: string;
    try { code = memberPointErrorCode(response.status, payload); }
    catch { throw new MemberPointLimitError(502, write ? "RESULT_UNVERIFIED" : "INVALID_UPSTREAM_RESPONSE", write ? "unknown" : undefined); }
    const unknown = write && (code === "RESULT_UNVERIFIED" || typeof payload === "object" && payload !== null && "outcome" in payload && payload.outcome === "unknown");
    throw new MemberPointLimitError(response.status, code, unknown ? "unknown" : undefined);
  }
  return payload;
}
export async function getMemberAIPointLimits(scope: MemberPointScope, signal?: AbortSignal): Promise<MemberAIPointLimits> {
  return parseMemberAIPointLimits(await request(scope, "/api/account/member-ai-point-limits", { method: "GET", signal }), scope.expectedOrganizationId);
}
export async function setMemberAIPointLimit(scope: MemberPointScope, memberId: string, target: string, expectedVersion: string, key: string, signal?: AbortSignal): Promise<MemberAIPointLimit> {
  if (!id.safeParse(memberId).success || !id.safeParse(key).success || !integer.safeParse(target).success || !integer.safeParse(expectedVersion).success) throw new MemberPointLimitError(400, "INVALID_REQUEST");
  return parseMemberAIPointLimit(await request(scope, `/api/account/member-ai-point-limits/${memberId}`, { method: "PUT", signal, headers: { "Content-Type": "application/json", "Idempotency-Key": key }, body: JSON.stringify({ target, expectedVersion }) }), scope.expectedOrganizationId, memberId);
}

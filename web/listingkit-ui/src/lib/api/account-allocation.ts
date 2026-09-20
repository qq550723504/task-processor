import { z } from "zod";
import { readBoundedStrictJSON } from "./strict-json-response";
import { parseWorkbenchErrorEnvelopePayload } from "./workbench-context";

const id = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const integer = z.string().regex(/^(0|[1-9][0-9]*)$/).refine(v => { try { return BigInt(v) <= BigInt("9223372036854775807"); } catch { return false; } });
const timestamp = z.string().max(40).regex(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,9})?Z$/).refine(v => Number.isFinite(Date.parse(v)));
const enterprise = z.object({ total: integer, allocated: integer, unallocated: integer, consumed: integer }).strict();
const allocation = z.object({ metric: z.literal("token"), windowStart: timestamp, windowEnd: timestamp, allocated: integer, consumed: integer, remaining: integer, version: integer, active: z.boolean() }).strict();
const member = z.object({ memberId: id, userId: id, displayName: z.string().max(512), loginName: z.string().max(512), state: z.literal("active"), allocation }).strict();
const snapshot = z.object({ schemaVersion: z.literal("account-member-token-allocation-v1"), organizationId: id, metric: z.literal("token"), windowStart: timestamp, windowEnd: timestamp, enterprise, members: z.array(member).max(100) }).strict();
const mutation = z.object({ metric: z.literal("token"), windowStart: timestamp, windowEnd: timestamp, allocated: integer, consumed: integer, remaining: integer, version: integer, active: z.boolean() }).strict();
export type MemberTokenAllocationSnapshot = z.infer<typeof snapshot>;
export type MemberTokenAllocation = z.infer<typeof allocation>;
export class AccountAllocationError extends Error { constructor(public readonly status: number, public readonly code: string) { super("Account resource allocation request could not be completed"); } }
function parseFailure(payload: unknown, status: number) { const parsed = parseWorkbenchErrorEnvelopePayload(payload); return parsed.success ? parsed.data.code : status === 409 ? "CONFLICT" : "DEPENDENCY_UNAVAILABLE"; }
async function request(expectedOrganizationId: string, init: RequestInit, path = "/api/account/member-allocations"): Promise<unknown> {
  if (!id.safeParse(expectedOrganizationId).success) throw new AccountAllocationError(400, "INVALID_REQUEST");
  const response = await fetch(path, { ...init, headers: { Accept: "application/json", "X-Expected-Organization-ID": expectedOrganizationId, ...(init.headers ?? {}) }, cache: "no-store" });
  const payload = await readBoundedStrictJSON(response, response.status === 200 ? 128 * 1024 : 8192);
  if (response.status !== 200) throw new AccountAllocationError(response.status, parseFailure(payload, response.status));
  return payload;
}
export async function getMemberTokenAllocations(expectedOrganizationId: string, signal?: AbortSignal): Promise<MemberTokenAllocationSnapshot> {
  const payload = await request(expectedOrganizationId, { method: "GET", signal }); const parsed = snapshot.safeParse(payload); if (!parsed.success || parsed.data.organizationId !== expectedOrganizationId) throw new AccountAllocationError(502, "INVALID_UPSTREAM_RESPONSE"); return parsed.data;
}
export async function setMemberTokenAllocation(expectedOrganizationId: string, memberId: string, target: string, expectedVersion: string, idempotencyKey = crypto.randomUUID(), signal?: AbortSignal): Promise<MemberTokenAllocation> {
  if (!id.safeParse(memberId).success || !integer.safeParse(target).success || !integer.safeParse(expectedVersion).success) throw new AccountAllocationError(400, "INVALID_REQUEST");
  const payload = await request(expectedOrganizationId, { method: "PUT", signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey }, body: JSON.stringify({ target, expectedVersion }) }, `/api/account/member-allocations/${memberId}`);
  const parsed = mutation.safeParse(payload); if (!parsed.success) throw new AccountAllocationError(502, "INVALID_UPSTREAM_RESPONSE"); return parsed.data;
}

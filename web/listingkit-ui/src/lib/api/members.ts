import { z } from "zod";
import { accountErrorCode } from "./account";
import { InvalidStrictJSONResponseError, readBoundedStrictJSON } from "./strict-json-response";

export const memberId = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const text = (max: number) => z.string().refine(value => new TextEncoder().encode(value).length <= max);
const role = z.enum(["listingkit_viewer", "listingkit_operator", "listingkit_admin"]);
const timestamp = z.string().max(40).datetime({ precision: null });
const version = z.string().regex(/^[a-f0-9]{64}$/);
const memberSchema = z.object({
  id: memberId, userId: memberId, organizationId: memberId, projectId: memberId,
  displayName: text(512), loginName: text(512), roles: z.array(text(128)).max(32),
  state: z.enum(["active", "inactive"]), createdAt: timestamp, changedAt: timestamp,
  observedVersion: version, canChangeRole: z.boolean(), canRemove: z.boolean(),
}).strict();
const listSchema = z.object({
  schemaVersion: z.literal("membership-v1"), userId: memberId, organizationId: memberId,
  items: z.array(memberSchema).max(100), total: z.number().int().min(0).max(10000),
  canManage: z.boolean(), assignableRoles: z.array(role).max(3),
}).strict().refine(value => value.items.length <= value.total && new Set(value.items.map(item => item.id)).size === value.items.length &&
  value.items.every(item => item.organizationId === value.organizationId) &&
  (value.canManage || (!value.assignableRoles.length && value.items.every(item => !item.canChangeRole && !item.canRemove))));
const acknowledgment = z.object({ id: memberId, at: timestamp }).strict().nullable();
const operationSchema = z.object({
  schemaVersion: z.literal("membership-operation-v1"), userId: memberId, organizationId: memberId, id: z.string().uuid(),
  kind: z.enum(["invite", "role", "remove"]), step: z.enum(["create_user", "create_authorization", "update_authorization", "delete_authorization"]),
  status: z.enum(["pending", "unknown", "acknowledged", "rejected"]), targetUserId: memberId, authorizationId: z.union([memberId, z.literal("")]),
  userEvidence: z.enum(["", "acknowledged", "identity_verified"]), userAcknowledgment: acknowledgment, acknowledgment,
  observation: z.enum(["unavailable", "observed", "not_visible"]), observed: memberSchema.nullable(),
}).strict().refine(value => (value.status !== "acknowledged" || value.acknowledgment !== null) &&
  (value.observation === "observed" ? value.observed !== null : value.observed === null) &&
  (value.observed === null || (value.observed.organizationId === value.organizationId && value.observed.userId === value.targetUserId)));

export type Members = z.infer<typeof listSchema>;
export type MemberOperation = z.infer<typeof operationSchema>;
export type MemberRole = z.infer<typeof role>;
export type MemberScope = { expectedUserId: string; expectedOrganizationId: string; signal?: AbortSignal };
export class MemberError extends Error {
  constructor(readonly status: number, readonly code: string) { super(`Member request failed (${code})`); this.name = "MemberError"; }
}
export function parseMembers(value: unknown): Members { const parsed = listSchema.safeParse(value); if (!parsed.success) throw new MemberError(502, "INVALID_UPSTREAM_RESPONSE"); return parsed.data; }
export function parseMemberOperation(value: unknown): MemberOperation { const parsed = operationSchema.safeParse(value); if (!parsed.success) throw new MemberError(502, "INVALID_UPSTREAM_RESPONSE"); return parsed.data; }
export function memberErrorCode(status: number, value: unknown): string {
  const extra = z.object({ code: z.enum(["MEMBER_NOT_FOUND", "MEMBER_OPERATION_CONFLICT"]), message: z.string(), requestId: z.string(), fieldErrors: z.array(z.unknown()).max(0) }).strict().safeParse(value);
  if (extra.success && ((status === 404 && extra.data.code === "MEMBER_NOT_FOUND") || (status === 409 && extra.data.code === "MEMBER_OPERATION_CONFLICT"))) return extra.data.code;
  try { return accountErrorCode(status, value); } catch { throw new MemberError(502, "INVALID_UPSTREAM_RESPONSE"); }
}

const invitationName = text(200).refine(value => value.trim().length > 0 && !/\p{Cc}/u.test(value));
export const invitationInput = z.object({ email: text(200).email().refine(value => !value.toLowerCase().endsWith("@phone.invalid")), firstName: invitationName, lastName: invitationName, role }).strict();
export const roleInput = z.object({ role, expectedVersion: version }).strict();
export const removeInput = z.object({ expectedVersion: version }).strict();

export function getMembers(scope: MemberScope, offset = 0): Promise<Members> { return requestMembers(`/members?limit=20&offset=${offset}`, scope, "GET", parseMembers); }
export function getMember(scope: MemberScope, id: string): Promise<Members> { return requestMembers(`/members/${memberId.parse(id)}`, scope, "GET", parseMembers); }
export function getMemberOperation(scope: MemberScope, id: string): Promise<MemberOperation> { return requestMembers(`/member-operations/${z.string().uuid().parse(id)}`, scope, "GET", parseMemberOperation); }
export function verifyMemberOperation(scope: MemberScope, id: string): Promise<MemberOperation> { return requestMembers(`/member-operations/${z.string().uuid().parse(id)}/verify`, scope, "POST", parseMemberOperation); }
export function inviteMember(scope: MemberScope, key: string, input: z.infer<typeof invitationInput>): Promise<MemberOperation> { return requestMembers("/members/invitations", scope, "POST", parseMemberOperation, key, invitationInput.parse(input)); }
export function changeMemberRole(scope: MemberScope, key: string, id: string, input: z.infer<typeof roleInput>): Promise<MemberOperation> { return requestMembers(`/members/${memberId.parse(id)}/role`, scope, "POST", parseMemberOperation, key, roleInput.parse(input)); }
export function removeMember(scope: MemberScope, key: string, id: string, input: z.infer<typeof removeInput>): Promise<MemberOperation> { return requestMembers(`/members/${memberId.parse(id)}/remove`, scope, "POST", parseMemberOperation, key, removeInput.parse(input)); }

async function requestMembers<T>(path: string, scope: MemberScope, method: "GET" | "POST", parse: (value: unknown) => T, key?: string, body?: unknown): Promise<T> {
  if (!memberId.safeParse(scope.expectedUserId).success) throw new MemberError(409, "IDENTITY_CONTEXT_CHANGED");
  if (!memberId.safeParse(scope.expectedOrganizationId).success) throw new MemberError(409, "ORGANIZATION_SELECTION_REQUIRED");
  const controller = new AbortController(); const abort = () => controller.abort();
  scope.signal?.addEventListener("abort", abort, { once: true }); if (scope.signal?.aborted) abort();
  const timer = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    const headers = new Headers({ Accept: "application/json", "X-Expected-User-ID": scope.expectedUserId, "X-Expected-Organization-ID": scope.expectedOrganizationId });
    if (key) headers.set("Idempotency-Key", z.string().uuid().parse(key));
    if (body !== undefined) headers.set("Content-Type", "application/json");
    const response = await fetch(`/api/account${path}`, { method, headers, body: body === undefined ? undefined : JSON.stringify(body), signal: controller.signal, credentials: "same-origin", cache: "no-store", redirect: "error" });
    const payload = await readBoundedStrictJSON(response, 1024 * 1024, controller.signal); controller.signal.throwIfAborted();
    if (response.status !== 200) throw new MemberError(response.status, memberErrorCode(response.status, payload));
    const result = parse(payload) as T & { userId: string; organizationId: string };
    if (result.userId !== scope.expectedUserId) throw new MemberError(409, "IDENTITY_CONTEXT_CHANGED");
    if (result.organizationId !== scope.expectedOrganizationId) throw new MemberError(409, "ORGANIZATION_CONTEXT_CHANGED");
    return result;
  } catch (error) {
    if (controller.signal.aborted) throw new MemberError(504, "DEADLINE_EXCEEDED");
    if (error instanceof MemberError) throw error;
    if (error instanceof InvalidStrictJSONResponseError) throw new MemberError(502, "INVALID_UPSTREAM_RESPONSE");
    throw new MemberError(502, "DEPENDENCY_UNAVAILABLE");
  } finally { clearTimeout(timer); scope.signal?.removeEventListener("abort", abort); }
}

import { z } from "zod";
import { assignableRole } from "./enterprise-roles";
import {
  MemberError,
  MemberScope,
  memberId,
  requestMembers,
  memberErrorCode,
} from "./members";
import { readBoundedStrictJSON } from "./strict-json-response";
const date = z.string().datetime({ precision: null }).max(40);
export const invitationInput = z
  .object({
    email: z
      .string()
      .email()
      .max(200)
      .refine((v) => !v.toLowerCase().endsWith("@phone.invalid")),
    role: assignableRole,
  })
  .strict();
const invitation = z
  .object({
    id: z.string().uuid(),
    organizationId: memberId,
    organizationName: z.string().max(512),
    creatorId: memberId,
    contact: z.string().email().max(200),
    role: invitationInput.shape.role,
    permissions: z.array(z.string().max(128)).max(128),
    state: z.enum([
      "pending",
      "accepting",
      "accepted",
      "declined",
      "cancelled",
      "expired",
    ]),
    revision: z.number().int().positive(),
    createdAt: date,
    expiresAt: date,
    recipientId: z.union([memberId, z.literal("")]),
    authorizationId: z.union([memberId, z.literal("")]),
    deliveryState: z.enum([
      "not_attempted",
      "sending",
      "mail_server_accepted",
      "delivery_unknown",
    ]),
    deliveryAttempts: z.number().int().nonnegative(),
    deliveryUpdatedAt: date.nullable(),
  })
  .strict()
  .refine(
    (v) => !(v.state === "accepted" && (!v.recipientId || !v.authorizationId)),
  );
const invitationSchema = z
  .object({
    schemaVersion: z.literal("membership-invitation-v1"),
    userId: memberId,
    organizationId: memberId,
    invitation,
  })
  .strict()
  .refine((v) => v.invitation.organizationId === v.organizationId);
export const invitationsSchema = z
  .object({
    schemaVersion: z.literal("membership-invitations-v1"),
    userId: memberId,
    organizationId: memberId,
    items: z.array(invitation).max(100),
    total: z.number().int().nonnegative(),
    pending: z.number().int().nonnegative(),
    canNotify: z.boolean(),
  })
  .strict()
  .refine(
    (v) =>
      v.items.length <= v.total &&
      v.pending <= v.total &&
      v.items.every((i) => i.organizationId === v.organizationId) &&
      new Set(v.items.map((i) => i.id)).size === v.items.length,
  );
export const invitationSummarySchema = z
  .object({
    schemaVersion: z.literal("membership-invitation-summary-v1"),
    userId: memberId,
    organizationId: memberId,
    pending: z.number().int().nonnegative(),
  })
  .strict();
export type InvitationResult = z.infer<typeof invitationSchema>;
export function parseInvitation(value: unknown, id?: string) {
  const p = invitationSchema.safeParse(value);
  if (!p.success || (id && p.data.invitation.id !== id))
    throw new MemberError(502, "INVALID_UPSTREAM_RESPONSE");
  return p.data;
}
export const getInvitations = (scope: MemberScope) =>
  requestMembers("/member-invitations", scope, "GET", (v) =>
    invitationsSchema.parse(v),
  );
export const getInvitationSummary = (scope: MemberScope) =>
  requestMembers("/member-invitations/summary", scope, "GET", (v) =>
    invitationSummarySchema.parse(v),
  );
export const createInvitation = (
  scope: MemberScope,
  key: string,
  input: z.infer<typeof invitationInput>,
) =>
  requestMembers(
    "/member-invitations",
    scope,
    "POST",
    (v) => parseInvitation(v, key),
    key,
    invitationInput.parse(input),
  );
export const invitationAction = (
  scope: MemberScope,
  id: string,
  action: "cancel" | "resend",
) =>
  requestMembers(
    `/member-invitations/${z.string().uuid().parse(id)}/${action}`,
    scope,
    "POST",
    (v) => parseInvitation(v, id),
  );
export const readAdminInvitation = (scope: MemberScope, id: string) =>
  requestMembers(
    `/member-invitations/${z.string().uuid().parse(id)}`,
    scope,
    "GET",
    (v) => parseInvitation(v, id),
  );
export async function recipientInvitation(
  userId: string,
  id: string,
  action?: "accept" | "decline",
  signal?: AbortSignal,
): Promise<InvitationResult> {
  memberId.parse(userId);
  z.string().uuid().parse(id);
  const controller = new AbortController();
  const abort = () => controller.abort();
  signal?.addEventListener("abort", abort, { once: true });
  if (signal?.aborted) abort();
  const timer = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    const response = await fetch(
      `/api/account/invitations/${id}${action ? `/${action}` : ""}`,
      {
        method: action ? "POST" : "GET",
        headers: { Accept: "application/json", "X-Expected-User-ID": userId },
        credentials: "same-origin",
        cache: "no-store",
        redirect: "error",
        signal: controller.signal,
      },
    );
    const payload = await readBoundedStrictJSON(
      response,
      16384,
      controller.signal,
    );
    controller.signal.throwIfAborted();
    if (response.status !== 200)
      throw new MemberError(
        response.status,
        memberErrorCode(response.status, payload),
      );
    const result = parseInvitation(payload, id);
    if (result.userId !== userId)
      throw new MemberError(409, "IDENTITY_CONTEXT_CHANGED");
    return result;
  } catch (error) {
    if (controller.signal.aborted)
      throw new MemberError(504, "DEADLINE_EXCEEDED");
    if (error instanceof MemberError) throw error;
    throw new MemberError(502, "RESULT_UNVERIFIED");
  } finally {
    clearTimeout(timer);
    signal?.removeEventListener("abort", abort);
  }
}

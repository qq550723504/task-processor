import { z } from "zod";
import { InvalidStrictJSONResponseError, readBoundedStrictJSON } from "./strict-json-response";

const id = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const boundedText = (max: number) => z.string().refine(value => new TextEncoder().encode(value).length <= max);
const optionalText = boundedText(512).nullable();
const common = { schemaVersion: z.literal("account-v1"), userId: id, homeOrganizationId: id, readAt: z.string().max(40).datetime({ precision: null }) };
const profileSchema = z.object({ ...common, displayName: optionalText, email: optionalText, emailVerified: z.boolean().nullable(), phoneNumber: optionalText, phoneNumberVerified: z.boolean().nullable(), source: z.literal("zitadel_userinfo") }).strict().refine(value =>
  (value.email === null ? value.emailVerified === null : !value.email.trim().toLowerCase().endsWith("@phone.invalid")) &&
  (value.phoneNumber !== null || value.phoneNumberVerified === null));
const organizationSchema = z.object({ ...common, effectiveOrganizationId: id, name: optionalText, roles: z.array(boundedText(128)).max(32), source: z.literal("zitadel_project_authorizations"), authorizationMaxAgeSeconds: z.literal(60) }).strict();
const businessProfileSchema = z.object({
  schemaVersion: z.literal("account-business-profile-v1"), userId: id,
  userRole: optionalText, shopSituation: optionalText, factorySituation: optionalText,
  platforms: z.array(boundedText(64)).max(16), sites: z.array(boundedText(64)).max(16), shopType: optionalText,
  services: z.array(boundedText(64)).max(16), source: z.literal("account_profile"),
  updatedAt: z.string().max(40).datetime({ precision: null }).nullable(), readAt: z.string().max(40).datetime({ precision: null }),
}).strict();
const businessProfileInputSchema = z.object({
  userRole: z.string().max(128), shopSituation: z.string().max(128), factorySituation: z.string().max(128),
  platforms: z.array(z.string().max(64)).max(16), sites: z.array(z.string().max(64)).max(16), shopType: z.string().max(128), services: z.array(z.string().max(64)).max(16),
}).strict();

export type AccountProfile = z.infer<typeof profileSchema>;
export type AccountOrganization = z.infer<typeof organizationSchema>;
export type AccountBusinessProfile = z.infer<typeof businessProfileSchema>;
export type AccountBusinessProfileInput = z.infer<typeof businessProfileInputSchema>;
export class AccountReadError extends Error {
  constructor(readonly status: number, readonly code: string) { super(`Account read failed (${code})`); this.name = "AccountReadError"; }
}

const errorCodes: Record<number, readonly string[]> = {
  400: ["INVALID_REQUEST"], 401: ["AUTHENTICATION_REQUIRED"],
  403: ["PERMISSION_DENIED", "ORGANIZATION_ACCESS_DENIED", "ORGANIZATION_ACCESS_REVOKED", "ORGANIZATION_SUSPENDED"],
  405: ["INVALID_REQUEST"], 409: ["IDENTITY_CONTEXT_CHANGED", "ORGANIZATION_CONTEXT_CHANGED", "ORGANIZATION_SELECTION_REQUIRED"],
  502: ["INVALID_UPSTREAM_RESPONSE", "DEPENDENCY_UNAVAILABLE"], 503: ["ACCOUNT_NOT_CONFIGURED", "DEPENDENCY_UNAVAILABLE"], 504: ["DEADLINE_EXCEEDED"],
};
// Shared wire validation for the dedicated BFF; UI consumes only the typed client.
export function parseAccountPayload(kind: "profile", value: unknown): AccountProfile;
export function parseAccountPayload(kind: "organization", value: unknown): AccountOrganization;
export function parseAccountPayload(kind: "business-profile", value: unknown): AccountBusinessProfile;
export function parseAccountPayload(kind: "profile" | "organization" | "business-profile", value: unknown): AccountProfile | AccountOrganization | AccountBusinessProfile;
export function parseAccountPayload(kind: "profile" | "organization" | "business-profile", value: unknown) {
  const result = (kind === "profile" ? profileSchema : kind === "organization" ? organizationSchema : businessProfileSchema).safeParse(value);
  if (!result.success) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
  return result.data;
}
export function accountErrorCode(status: number, value: unknown): string {
  const envelope = z.object({ code: z.string(), message: z.string(), requestId: z.string(), fieldErrors: z.array(z.unknown()).max(0) }).strict().safeParse(value);
  if (!envelope.success || !errorCodes[status]?.includes(envelope.data.code)) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
  return envelope.data.code;
}

type ProfileOptions = { expectedUserId: string; signal?: AbortSignal };
type OrganizationOptions = ProfileOptions & { expectedOrganizationId: string };
export function getAccountProfile(options: ProfileOptions): Promise<AccountProfile> { return readAccount("profile", options) as Promise<AccountProfile>; }
export function getAccountOrganization(options: OrganizationOptions): Promise<AccountOrganization> { return readAccount("organization", options) as Promise<AccountOrganization>; }
export function getAccountBusinessProfile(options: OrganizationOptions): Promise<AccountBusinessProfile> { return readAccount("business-profile", options) as Promise<AccountBusinessProfile>; }
export async function updateAccountBusinessProfile(options: ProfileOptions & { expectedOrganizationId: string; input: AccountBusinessProfileInput }): Promise<AccountBusinessProfile> {
  return writeBusinessProfile(options);
}

async function readAccount(kind: "profile" | "organization" | "business-profile", options: ProfileOptions | OrganizationOptions) {
  if (!id.safeParse(options.expectedUserId).success) throw new AccountReadError(409, "IDENTITY_CONTEXT_CHANGED");
  const organization = "expectedOrganizationId" in options ? options.expectedOrganizationId : undefined;
  if ((kind === "organization" || kind === "business-profile") && !id.safeParse(organization).success) throw new AccountReadError(409, "ORGANIZATION_SELECTION_REQUIRED");
  const controller = new AbortController();
  const abort = () => controller.abort();
  options.signal?.addEventListener("abort", abort, { once: true });
  if (options.signal?.aborted) abort();
  const timer = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    const headers = new Headers({ Accept: "application/json", "X-Expected-User-ID": options.expectedUserId });
    if (organization) headers.set("X-Expected-Organization-ID", organization);
    const response = await fetch(`/api/account/${kind}`, { method: "GET", headers, credentials: "same-origin", cache: "no-store", redirect: "error", signal: controller.signal });
    const payload = await readBoundedStrictJSON(response, 16 * 1024, controller.signal);
    controller.signal.throwIfAborted();
    if (response.status !== 200) throw new AccountReadError(response.status, accountErrorCode(response.status, payload));
    const result = parseAccountPayload(kind, payload);
    if (result.userId !== options.expectedUserId) throw new AccountReadError(409, "IDENTITY_CONTEXT_CHANGED");
    if ("effectiveOrganizationId" in result && result.effectiveOrganizationId !== organization) throw new AccountReadError(409, "ORGANIZATION_CONTEXT_CHANGED");
    return result;
  } catch (error) {
    if (controller.signal.aborted) throw new AccountReadError(504, "DEADLINE_EXCEEDED");
    if (error instanceof AccountReadError) throw error;
    if (error instanceof InvalidStrictJSONResponseError) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
    throw new AccountReadError(502, "DEPENDENCY_UNAVAILABLE");
  } finally { clearTimeout(timer); options.signal?.removeEventListener("abort", abort); }
}

async function writeBusinessProfile(options: ProfileOptions & { expectedOrganizationId: string; input: AccountBusinessProfileInput }) {
  if (!id.safeParse(options.expectedUserId).success) throw new AccountReadError(409, "IDENTITY_CONTEXT_CHANGED");
  if (!id.safeParse(options.expectedOrganizationId).success) throw new AccountReadError(409, "ORGANIZATION_CONTEXT_CHANGED");
  const controller = new AbortController(); const abort = () => controller.abort(); options.signal?.addEventListener("abort", abort, { once: true }); if (options.signal?.aborted) abort();
  const timer = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    const response = await fetch("/api/account/business-profile", { method: "PUT", headers: { Accept: "application/json", "Content-Type": "application/json", "X-Expected-User-ID": options.expectedUserId, "X-Expected-Organization-ID": options.expectedOrganizationId }, body: JSON.stringify(options.input), credentials: "same-origin", cache: "no-store", redirect: "error", signal: controller.signal });
    const payload = await readBoundedStrictJSON(response, 16 * 1024, controller.signal); controller.signal.throwIfAborted();
    if (response.status !== 200) throw new AccountReadError(response.status, accountErrorCode(response.status, payload));
    const result = parseAccountPayload("business-profile", payload); if (result.userId !== options.expectedUserId) throw new AccountReadError(409, "IDENTITY_CONTEXT_CHANGED"); return result;
  } catch (error) {
    if (controller.signal.aborted) throw new AccountReadError(504, "DEADLINE_EXCEEDED");
    if (error instanceof AccountReadError) throw error;
    if (error instanceof InvalidStrictJSONResponseError) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
    throw new AccountReadError(502, "DEPENDENCY_UNAVAILABLE");
  } finally { clearTimeout(timer); options.signal?.removeEventListener("abort", abort); }
}

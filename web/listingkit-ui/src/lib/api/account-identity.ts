import { z } from "zod";
import { AccountReadError, accountErrorCode } from "./account";
import { InvalidStrictJSONResponseError, readBoundedStrictJSON } from "./strict-json-response";

const operationResponseSchema = z.object({
  schemaVersion: z.literal("account-identity-operation-v1"),
  operation: z.string().min(1).max(32),
  state: z.enum(["updated", "verification_pending", "verification_sent", "verified"]),
  source: z.literal("zitadel_auth_v1"),
}).strict();
const id = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const email = z.string().email().max(320);
const phone = z.string().regex(/^\+[1-9][0-9]{6,14}$/);
const code = z.string().min(1).max(64);
const password = z.string().min(1).max(512);
const profileResponseSchema = z.object({
  schemaVersion: z.literal("account-identity-profile-v1"),
  userId: id,
  firstName: z.string().max(200),
  lastName: z.string().max(200),
  nickName: z.string().max(200),
  displayName: z.string().max(200),
  preferredLanguage: z.string().max(32),
  gender: z.string().max(32),
  source: z.literal("zitadel_auth_v1"),
}).strict();
const profileInput = z.object({
  firstName: z.string().max(200),
  lastName: z.string().max(200),
  nickName: z.string().max(200),
  displayName: z.string().max(200),
  preferredLanguage: z.string().max(32),
  gender: z.enum(["", "GENDER_UNSPECIFIED", "GENDER_FEMALE", "GENDER_MALE", "GENDER_DIVERSE"]),
}).strict();

export type AccountIdentityOperation = z.infer<typeof operationResponseSchema>;
export type AccountIdentityProfile = z.infer<typeof profileResponseSchema>;
export type AccountIdentityProfileInput = z.infer<typeof profileInput>;
export type AccountIdentityOptions = { expectedUserId: string; signal?: AbortSignal };

export function getAccountIdentityProfile(options: AccountIdentityOptions): Promise<AccountIdentityProfile> {
  if (!id.safeParse(options.expectedUserId).success) return Promise.reject(new AccountReadError(400, "INVALID_REQUEST"));
  return requestIdentityProfile(options.expectedUserId, options.signal);
}

export function updateAccountIdentityProfile(options: AccountIdentityOptions & { input: AccountIdentityProfileInput }) {
  if (!id.safeParse(options.expectedUserId).success || !profileInput.safeParse(options.input).success) return Promise.reject(new AccountReadError(400, "INVALID_REQUEST"));
  return requestIdentity("profile", "PUT", options.expectedUserId, options.input, options.signal);
}

export function setAccountEmail(options: AccountIdentityOptions & { email: string }) {
  if (!id.safeParse(options.expectedUserId).success || !email.safeParse(options.email).success) return Promise.reject(new AccountReadError(400, "INVALID_REQUEST"));
  return requestIdentity("email", "PUT", options.expectedUserId, { email: options.email }, options.signal);
}
export function setAccountPhone(options: AccountIdentityOptions & { phone: string }) {
  if (!id.safeParse(options.expectedUserId).success || !phone.safeParse(options.phone).success) return Promise.reject(new AccountReadError(400, "INVALID_REQUEST"));
  return requestIdentity("phone", "PUT", options.expectedUserId, { phone: options.phone }, options.signal);
}
export function resendAccountEmailVerification(options: AccountIdentityOptions) { return requestIdentity("email/resend", "POST", options.expectedUserId, {}, options.signal); }
export function verifyAccountEmail(options: AccountIdentityOptions & { code: string }) {
  if (!code.safeParse(options.code).success) return Promise.reject(new AccountReadError(400, "INVALID_REQUEST"));
  return requestIdentity("email/verify", "POST", options.expectedUserId, { code: options.code }, options.signal);
}
export function resendAccountPhoneVerification(options: AccountIdentityOptions) { return requestIdentity("phone/resend", "POST", options.expectedUserId, {}, options.signal); }
export function verifyAccountPhone(options: AccountIdentityOptions & { code: string }) {
  if (!code.safeParse(options.code).success) return Promise.reject(new AccountReadError(400, "INVALID_REQUEST"));
  return requestIdentity("phone/verify", "POST", options.expectedUserId, { code: options.code }, options.signal);
}
export function updateAccountPassword(options: AccountIdentityOptions & { oldPassword: string; newPassword: string }) {
  if (!password.safeParse(options.oldPassword).success || !password.safeParse(options.newPassword).success) return Promise.reject(new AccountReadError(400, "INVALID_REQUEST"));
  return requestIdentity("password", "PUT", options.expectedUserId, { oldPassword: options.oldPassword, newPassword: options.newPassword }, options.signal);
}

async function requestIdentity(operation: string, method: "PUT" | "POST", expectedUserId: string, body: Record<string, string>, signal?: AbortSignal): Promise<AccountIdentityOperation> {
  const controller = new AbortController(); const abort = () => controller.abort(); signal?.addEventListener("abort", abort, { once: true }); if (signal?.aborted) abort();
  const timer = setTimeout(abort, 15000);
  let dispatched = false;
  try {
    controller.signal.throwIfAborted();
    dispatched = true;
    const response = await fetch(`/api/account/identity/${operation}`, { method, headers: { Accept: "application/json", "Content-Type": "application/json", "X-Expected-User-ID": expectedUserId }, body: JSON.stringify(body), credentials: "same-origin", cache: "no-store", redirect: "error", signal: controller.signal });
    let payload: unknown;
    try { payload = await readBoundedStrictJSON(response, 16 * 1024, controller.signal); }
    catch (error) { if (dispatched) throw new AccountReadError(controller.signal.aborted ? 504 : 502, "RESULT_UNVERIFIED"); throw error; }
    controller.signal.throwIfAborted();
    if (response.status !== 200) {
      let code: string;
      try { code = accountErrorCode(response.status, payload); }
      catch (error) { if (dispatched) throw new AccountReadError(502, "RESULT_UNVERIFIED"); throw error; }
      throw new AccountReadError(response.status, code);
    }
    const parsed = operationResponseSchema.safeParse(payload); if (!parsed.success) throw new AccountReadError(502, "RESULT_UNVERIFIED");
    return parsed.data;
  } catch (error) {
    if (error instanceof AccountReadError) throw error;
    if (dispatched) throw new AccountReadError(controller.signal.aborted ? 504 : 502, "RESULT_UNVERIFIED");
    if (controller.signal.aborted) throw new AccountReadError(504, "DEADLINE_EXCEEDED");
    if (error instanceof InvalidStrictJSONResponseError) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
    throw new AccountReadError(502, "DEPENDENCY_UNAVAILABLE");
  } finally { clearTimeout(timer); signal?.removeEventListener("abort", abort); }
}

async function requestIdentityProfile(expectedUserId: string, signal?: AbortSignal): Promise<AccountIdentityProfile> {
  const controller = new AbortController(); const abort = () => controller.abort(); signal?.addEventListener("abort", abort, { once: true }); if (signal?.aborted) abort();
  const timer = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    const response = await fetch("/api/account/identity/profile", { method: "GET", headers: { Accept: "application/json", "X-Expected-User-ID": expectedUserId }, credentials: "same-origin", cache: "no-store", redirect: "error", signal: controller.signal });
    const payload = await readBoundedStrictJSON(response, 16 * 1024, controller.signal); controller.signal.throwIfAborted();
    if (response.status !== 200) throw new AccountReadError(response.status, accountErrorCode(response.status, payload));
    const parsed = profileResponseSchema.safeParse(payload); if (!parsed.success) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
    if (parsed.data.userId !== expectedUserId) throw new AccountReadError(409, "IDENTITY_CONTEXT_CHANGED");
    return parsed.data;
  } catch (error) {
    if (controller.signal.aborted) throw new AccountReadError(504, "DEADLINE_EXCEEDED");
    if (error instanceof AccountReadError) throw error;
    if (error instanceof InvalidStrictJSONResponseError) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
    throw new AccountReadError(502, "DEPENDENCY_UNAVAILABLE");
  } finally { clearTimeout(timer); signal?.removeEventListener("abort", abort); }
}

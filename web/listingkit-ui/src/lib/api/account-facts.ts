import { z } from "zod";
import { AccountReadError, accountErrorCode } from "./account";
import { readBoundedStrictJSON } from "./strict-json-response";
export const accountFactSubject = z
  .string()
  .regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const timestamp = z.string().max(40).datetime({ precision: null });
const region = z
  .string()
  .refine(
    (value) =>
      new TextEncoder().encode(value).length <= 128 && !/\p{Cc}/u.test(value),
  );
export const regionInputSchema = z
  .object({ country: region, province: region, city: region })
  .strict();
export const accountFactsSchema = z
  .object({
    schemaVersion: z.literal("account-identity-facts-v1"),
    userId: accountFactSubject,
    registeredAt: timestamp,
    lastLogin: timestamp.nullable(),
    passwordChangedAt: timestamp.nullable(),
    source: z.literal("zitadel_auth_v1"),
    readAt: timestamp,
  })
  .strict();
export const accountPreferencesSchema = regionInputSchema
  .extend({
    schemaVersion: z.literal("account-preferences-v1"),
    userId: accountFactSubject,
    updatedAt: timestamp.nullable(),
    readAt: timestamp,
    source: z.literal("account_profile"),
  })
  .strict();
export type AccountPreferences = z.infer<typeof accountPreferencesSchema>;
export type RegionInput = z.infer<typeof regionInputSchema>;
type Scope = { expectedUserId: string; signal?: AbortSignal };
export function getAccountFacts(scope: Scope) {
  return request("facts", scope, accountFactsSchema);
}
export function getAccountPreferences(scope: Scope) {
  return request("preferences", scope, accountPreferencesSchema);
}
export function saveAccountPreferences(scope: Scope & { input: RegionInput }) {
  return request("preferences", scope, accountPreferencesSchema, scope.input);
}
async function request<T extends { userId: string }>(
  kind: "facts" | "preferences",
  scope: Scope,
  schema: z.ZodType<T>,
  input?: RegionInput,
): Promise<T> {
  if (
    !accountFactSubject.safeParse(scope.expectedUserId).success ||
    (input && !regionInputSchema.safeParse(input).success)
  )
    throw new AccountReadError(400, "INVALID_REQUEST");
  const controller = new AbortController();
  const abort = () => controller.abort();
  scope.signal?.addEventListener("abort", abort, { once: true });
  if (scope.signal?.aborted) abort();
  const timer = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    const headers = new Headers({
      Accept: "application/json",
      "X-Expected-User-ID": scope.expectedUserId,
    });
    if (input) headers.set("Content-Type", "application/json");
    const response = await fetch(`/api/account/${kind}`, {
      method: input ? "PUT" : "GET",
      headers,
      ...(input ? { body: JSON.stringify(input) } : {}),
      credentials: "same-origin",
      cache: "no-store",
      redirect: "error",
      signal: controller.signal,
    });
    const value = await readBoundedStrictJSON(
      response,
      16384,
      controller.signal,
    );
    controller.signal.throwIfAborted();
    if (response.status !== 200)
      throw new AccountReadError(
        response.status,
        accountErrorCode(response.status, value),
      );
    const parsed = schema.safeParse(value);
    if (!parsed.success)
      throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
    if (parsed.data.userId !== scope.expectedUserId)
      throw new AccountReadError(409, "IDENTITY_CONTEXT_CHANGED");
    return parsed.data;
  } catch (error) {
    if (controller.signal.aborted)
      throw new AccountReadError(
        504,
        input ? "RESULT_UNVERIFIED" : "DEADLINE_EXCEEDED",
      );
    if (error instanceof AccountReadError) throw error;
    throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE");
  } finally {
    clearTimeout(timer);
    scope.signal?.removeEventListener("abort", abort);
  }
}

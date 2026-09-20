import { z } from "zod";
import { AccountReadError, accountErrorCode } from "./account";
import { readBoundedStrictJSON } from "./strict-json-response";

const amount = z.string().regex(/^(0|[1-9][0-9]{0,18})$/);
const earnings = z.object({ schemaVersion: z.literal("referral-earnings-v1"), referrer: z.string().min(1), currency: z.literal("CNY"), pendingMinor: amount, availableMinor: amount, reservedMinor: amount, adjustmentMinor: z.string().regex(/^-?(0|[1-9][0-9]{0,18})$/), version: amount, updatedAt: z.string().datetime({ precision: null }).nullable(), source: z.literal("referral_earnings_projection") }).strict();
const payoutMethods = z.object({ schemaVersion: z.literal("payout-methods-v1"), methods: z.array(z.object({ methodId: z.string().min(1), type: z.enum(["ALIPAY", "BANK_TRANSFER"]), displayName: z.string().min(1), maskedDestination: z.string().min(1), version: amount }).strict()) }).strict();
const withdrawal = z.object({ schemaVersion: z.literal("referral-withdrawal-v1"), id: z.string().min(1), currency: z.literal("CNY"), method: z.enum(["ALIPAY", "BANK_TRANSFER"]), payoutMethodId: z.string().min(1), amountMinor: amount, status: z.enum(["REQUESTED", "APPROVED", "PAID", "CANCELED", "REJECTED"]), payoutReference: z.string(), version: amount, createdAt: z.string().datetime({ precision: null }), updatedAt: z.string().datetime({ precision: null }) }).strict();
export type ReferralEarnings = z.infer<typeof earnings>;
export type PayoutMethod = z.infer<typeof payoutMethods>["methods"][number];
export type ReferralWithdrawal = z.infer<typeof withdrawal>;
async function request<T>(path: string, schema: z.ZodType<T>, expectedUserId: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers); headers.set("Accept", "application/json"); headers.set("X-Expected-User-ID", expectedUserId);
  const response = await fetch(path, { ...init, headers, credentials: "same-origin", cache: "no-store", redirect: "error" });
  const payload = await readBoundedStrictJSON(response, 128 * 1024);
  if (!response.ok) throw new AccountReadError(response.status, accountErrorCode(response.status, payload));
  const parsed = schema.safeParse(payload); if (!parsed.success) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE"); return parsed.data;
}
export const getReferralEarnings = (expectedUserId: string, signal?: AbortSignal) => request("/api/account/referral-earnings", earnings, expectedUserId, { signal });
export const getReferralPayoutMethods = (expectedUserId: string, signal?: AbortSignal) => request("/api/account/referral-payout-methods", payoutMethods, expectedUserId, { signal });
export function requestReferralWithdrawal(expectedUserId: string, body: { amountMinor: string; payoutMethodId: string; expectedVersion: string }, idempotencyKey: string, signal?: AbortSignal) { return request("/api/account/referral-withdrawals", withdrawal, expectedUserId, { method: "POST", signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey }, body: JSON.stringify(body) }); }
export function cancelReferralWithdrawal(expectedUserId: string, withdrawalId: string, expectedVersion: string, idempotencyKey: string, signal?: AbortSignal) { return request(`/api/account/referral-withdrawals/${encodeURIComponent(withdrawalId)}/cancel`, withdrawal, expectedUserId, { method: "POST", signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey }, body: JSON.stringify({ expectedVersion }) }); }

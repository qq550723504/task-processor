import { z } from "zod";
import { AccountReadError, accountErrorCode } from "./account";
import { readBoundedStrictJSON } from "./strict-json-response";

const amount = z.string().regex(/^(0|[1-9][0-9]{0,18})$/);
const earningsEntry = z.object({ entryId: z.string().min(1).max(200), referrer: z.string().min(1).max(200), currency: z.literal("CNY"), paymentId: z.string().min(1).max(200), entryType: z.enum(["COMMISSION", "REFUND_ADJUSTMENT", "CHARGEBACK_ADJUSTMENT", "REVERSAL"]), amountMinor: z.string().regex(/^-?(0|[1-9][0-9]{0,18})$/), referenceId: z.string().min(1).max(200), occurredAt: z.string().datetime({ precision: null }) }).strict();
const referralEarnings = z.object({ schemaVersion: z.literal("referral-earnings-v1"), referrer: z.string().min(1), currency: z.literal("CNY"), pendingMinor: amount, availableMinor: amount, reservedMinor: amount, adjustmentMinor: z.string().regex(/^-?(0|[1-9][0-9]{0,18})$/), version: amount, updatedAt: z.string().datetime({ precision: null }).nullable(), source: z.literal("referral_earnings_projection"), entryLimit: z.literal(100), entries: z.array(earningsEntry).max(100).optional() }).strict();
const referralRules = z.object({ schemaVersion: z.literal("referral-rules-v1"), currency: z.literal("CNY"), commissionRateBps: z.number().int().nonnegative(), settlementPeriodDays: z.number().int().positive(), minimumWithdrawalMinor: amount, withdrawalReview: z.literal("manual"), earningsBasis: z.literal("canonical_settled_payment_refund_chargeback"), source: z.literal("referral_economics_contract") }).strict();
const payoutMethods = z.object({ schemaVersion: z.literal("payout-methods-v1"), methods: z.array(z.object({ methodId: z.string().min(1), type: z.enum(["ALIPAY", "BANK_TRANSFER"]), displayName: z.string().min(1), maskedDestination: z.string().min(1), version: amount }).strict()) }).strict();
const payoutMethod = z.object({ schemaVersion: z.literal("payout-method-v1"), methodId: z.string().min(1), type: z.enum(["ALIPAY", "BANK_TRANSFER"]), displayName: z.string().min(1), maskedDestination: z.string().min(1), version: amount }).strict();
const withdrawal = z.object({ schemaVersion: z.literal("referral-withdrawal-v1"), id: z.string().min(1), currency: z.literal("CNY"), method: z.enum(["ALIPAY", "BANK_TRANSFER"]), payoutMethodId: z.string().min(1), amountMinor: amount, status: z.enum(["REQUESTED", "APPROVED", "PAID", "CANCELED", "REJECTED"]), payoutReference: z.string(), version: amount, createdAt: z.string().datetime({ precision: null }), updatedAt: z.string().datetime({ precision: null }) }).strict();
const withdrawals = z.object({ schemaVersion: z.literal("referral-withdrawals-v1"), withdrawals: z.array(withdrawal.omit({ schemaVersion: true })).max(100) }).strict();
export type PayoutMethod = z.infer<typeof payoutMethods>["methods"][number];
export type ReferralWithdrawal = z.infer<typeof withdrawal>;
export type ReferralEarnings = z.infer<typeof referralEarnings>;
export type ReferralRules = z.infer<typeof referralRules>;
async function request<T>(path: string, schema: z.ZodType<T>, expectedUserId: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers); headers.set("Accept", "application/json"); headers.set("X-Expected-User-ID", expectedUserId);
  const response = await fetch(path, { ...init, headers, credentials: "same-origin", cache: "no-store", redirect: "error" });
  const payload = await readBoundedStrictJSON(response, 128 * 1024);
  if (!response.ok) throw new AccountReadError(response.status, accountErrorCode(response.status, payload));
  const parsed = schema.safeParse(payload); if (!parsed.success) throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE"); return parsed.data;
}
export const getReferralPayoutMethods = (expectedUserId: string, signal?: AbortSignal) => request("/api/account/referral-payout-methods", payoutMethods, expectedUserId, { signal });
export const getReferralEarnings = (expectedUserId: string, signal?: AbortSignal) => request("/api/account/referral-earnings", referralEarnings, expectedUserId, { signal });
export const getReferralRules = (expectedUserId: string, signal?: AbortSignal) => request("/api/account/referral-rules", referralRules, expectedUserId, { signal });
export function createReferralPayoutMethod(expectedUserId: string, body: { type: "ALIPAY" | "BANK_TRANSFER"; displayName: string; destination: string }, idempotencyKey: string, signal?: AbortSignal) { return request("/api/account/referral-payout-methods", payoutMethod, expectedUserId, { method: "POST", signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey }, body: JSON.stringify(body) }); }
export const getReferralWithdrawals = (expectedUserId: string, signal?: AbortSignal) => request("/api/account/referral-withdrawals", withdrawals, expectedUserId, { signal });
export function requestReferralWithdrawal(expectedUserId: string, body: { amountMinor: string; payoutMethodId: string; expectedVersion: string }, idempotencyKey: string, signal?: AbortSignal) { return request("/api/account/referral-withdrawals", withdrawal, expectedUserId, { method: "POST", signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey }, body: JSON.stringify(body) }); }
export function cancelReferralWithdrawal(expectedUserId: string, withdrawalId: string, expectedVersion: string, idempotencyKey: string, signal?: AbortSignal) { return request(`/api/account/referral-withdrawals/${encodeURIComponent(withdrawalId)}/cancel`, withdrawal, expectedUserId, { method: "POST", signal, headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey }, body: JSON.stringify({ expectedVersion }) }); }

import { z } from "zod";
import { parseCommercialSubscriptionOrder } from "./commercial-billing";
import { readBoundedStrictJSON } from "./strict-json-response";
import { parseWorkbenchErrorEnvelopePayload } from "./workbench-context";

const id = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const planCode = z.string().min(1).refine(value => value.trim() === value && new TextEncoder().encode(value).length <= 64 && !/[\u0000-\u001f\u007f-\u009f]/.test(value));
const displayName = z.string().min(1).max(256).refine(value => value.trim() === value && !/[\u0000-\u001f\u007f-\u009f]/.test(value));
const pricingVersion = z.string().min(1).max(128).refine(value => value.trim() === value && !/[\u0000-\u001f\u007f-\u009f]/.test(value));
const nonnegative = z.string().regex(/^(0|[1-9][0-9]*)$/).max(19).refine(value => BigInt(value) <= BigInt("9223372036854775807"));
const termMonths = nonnegative.refine(value => BigInt(value) > BigInt(0) && BigInt(value) <= BigInt(120));
const timestamp = z.string().max(40).regex(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,9})?Z$/).refine(value => Number.isFinite(Date.parse(value)));
const settlement = z.enum(["ZERO_PRICE", "WALLET", "EXTERNAL_PAYMENT"]);
const offer = z.object({ offer_id: id, plan_code: planCode, plan_name: z.union([displayName, z.literal("")]), term_months: termMonths, settlement_mode: settlement, currency: z.literal("CNY"), total_minor: nonnegative, pricing_version: pricingVersion, availability: z.enum(["available", "current_plan", "active_subscription_conflict", "payment_unavailable", "offer_unavailable"]) }).strict().refine(value => (value.availability === "offer_unavailable" || value.plan_name !== "") && (value.settlement_mode === "ZERO_PRICE" ? value.total_minor === "0" : value.total_minor !== "0"));
const offers = z.object({ organization_id: id, items: z.array(offer).max(64) }).strict().refine(value => new Set(value.items.map(item => item.offer_id)).size === value.items.length);
const quote = z.object({ quote_id: id, organization_id: id, offer_id: id, product_kind: z.literal("SUBSCRIPTION_PLAN"), plan_code: planCode, plan_fingerprint: id, term_months: termMonths, settlement_mode: settlement, currency: z.literal("CNY"), total_minor: nonnegative, pricing_version: pricingVersion, expires_at: timestamp, fingerprint: id, created_at: timestamp }).strict().refine(value => Date.parse(value.created_at) < Date.parse(value.expires_at) && (value.settlement_mode === "ZERO_PRICE" ? value.total_minor === "0" : value.total_minor !== "0"));

export type SubscriptionOffer = z.infer<typeof offer>;
export type SubscriptionOffers = z.infer<typeof offers>;
export type SubscriptionQuote = z.infer<typeof quote>;
export type SubscriptionOrder = NonNullable<ReturnType<typeof parseCommercialSubscriptionOrder>>;

const errorStatuses: Record<string, readonly number[]> = {
  INVALID_REQUEST: [400, 413], AUTHENTICATION_REQUIRED: [401], FORBIDDEN: [403], PERMISSION_DENIED: [403], ORGANIZATION_ACCESS_DENIED: [403], ORGANIZATION_ACCESS_REVOKED: [403], ORGANIZATION_SUSPENDED: [403],
  ORGANIZATION_SELECTION_REQUIRED: [409], ORGANIZATION_CONTEXT_CHANGED: [409], IDENTITY_CONTEXT_CHANGED: [409], OFFER_UNAVAILABLE: [503], QUOTE_EXPIRED: [409], PAYMENT_METHOD_UNAVAILABLE: [409], INSUFFICIENT_FUNDS: [409], ACTIVE_SUBSCRIPTION_EXISTS: [409], PLAN_CHANGED: [409], IDEMPOTENCY_CONFLICT: [409], CONFLICT: [409], RECONCILIATION_REQUIRED: [409], NOT_FOUND: [404], FEATURE_UNAVAILABLE: [503], DEPENDENCY_UNAVAILABLE: [503], DEADLINE_EXCEEDED: [504], INVALID_UPSTREAM_RESPONSE: [502],
};

export class SubscriptionPurchaseError extends Error {
  constructor(public readonly status: number, public readonly code: string, public readonly requestId = "") { super("Subscription purchase request could not be completed"); }
}

async function request<T>(path: string, expectedUserId: string, expectedOrganizationId: string, parse: (value: unknown) => T | null, init: { body?: object; idempotencyKey?: string; signal?: AbortSignal } = {}): Promise<T> {
  if (!id.safeParse(expectedUserId).success || !id.safeParse(expectedOrganizationId).success || (init.idempotencyKey !== undefined && (!init.idempotencyKey.trim() || init.idempotencyKey.length > 192))) throw new SubscriptionPurchaseError(400, "INVALID_REQUEST");
  const controller = new AbortController();
  const abort = () => controller.abort();
  init.signal?.addEventListener("abort", abort, { once: true });
  if (init.signal?.aborted) abort();
  const timeout = setTimeout(abort, 15_000);
  try {
    controller.signal.throwIfAborted();
    const headers = new Headers({ Accept: "application/json", "X-Expected-User-ID": expectedUserId, "X-Expected-Organization-ID": expectedOrganizationId });
    if (init.body) headers.set("Content-Type", "application/json");
    if (init.idempotencyKey) headers.set("Idempotency-Key", init.idempotencyKey);
    const response = await fetch(path, { method: init.body ? "POST" : "GET", headers, body: init.body ? JSON.stringify(init.body) : undefined, cache: "no-store", redirect: "manual", signal: controller.signal });
    const success = response.status === (init.body ? 201 : 200);
    const payload = await readBoundedStrictJSON(response, success ? 64 * 1024 : 8192, controller.signal);
    controller.signal.throwIfAborted();
    if (success) {
      const parsed = parse(payload);
      if (parsed) return parsed;
    } else {
      const failure = parseWorkbenchErrorEnvelopePayload(payload);
      if (failure.success && errorStatuses[failure.data.code]?.includes(response.status)) throw new SubscriptionPurchaseError(response.status, failure.data.code, failure.data.requestId);
    }
    throw new SubscriptionPurchaseError(502, "INVALID_UPSTREAM_RESPONSE");
  } catch (error) {
    if (controller.signal.aborted) throw new SubscriptionPurchaseError(504, "DEADLINE_EXCEEDED");
    if (error instanceof SubscriptionPurchaseError) throw error;
    throw new SubscriptionPurchaseError(503, "DEPENDENCY_UNAVAILABLE");
  } finally {
    clearTimeout(timeout);
    init.signal?.removeEventListener("abort", abort);
  }
}

export function getSubscriptionOffers(userId: string, organizationId: string, signal?: AbortSignal) {
  return request("/api/workbench/commercial/subscription-offers", userId, organizationId, value => { const parsed = offers.safeParse(value); return parsed.success && parsed.data.organization_id === organizationId ? parsed.data : null; }, { signal });
}
export function createSubscriptionQuote(userId: string, organizationId: string, offerId: string, signal?: AbortSignal) {
  if (!id.safeParse(offerId).success) throw new SubscriptionPurchaseError(400, "INVALID_REQUEST");
  return request("/api/workbench/commercial/subscription-quotes", userId, organizationId, value => { const parsed = quote.safeParse(value); return parsed.success && parsed.data.organization_id === organizationId && parsed.data.offer_id === offerId ? parsed.data : null; }, { body: { offer_id: offerId }, signal });
}
export function createSubscriptionOrder(userId: string, organizationId: string, quoteId: string, idempotencyKey: string, signal?: AbortSignal) {
  if (!id.safeParse(quoteId).success) throw new SubscriptionPurchaseError(400, "INVALID_REQUEST");
  return request("/api/workbench/commercial/subscription-orders", userId, organizationId, value => { const parsed = parseCommercialSubscriptionOrder(value); return parsed && parsed.organization_id === organizationId && parsed.quote_id === quoteId ? parsed : null; }, { body: { quote_id: quoteId }, idempotencyKey, signal });
}
export function getSubscriptionOrder(userId: string, organizationId: string, orderId: string, signal?: AbortSignal) {
  if (!id.safeParse(orderId).success) throw new SubscriptionPurchaseError(400, "INVALID_REQUEST");
  return request(`/api/workbench/commercial/subscription-orders/${encodeURIComponent(orderId)}`, userId, organizationId, value => { const parsed = parseCommercialSubscriptionOrder(value); return parsed && parsed.organization_id === organizationId && parsed.order_id === orderId ? parsed : null; }, { signal });
}

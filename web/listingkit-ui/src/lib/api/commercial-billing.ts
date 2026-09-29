import { z } from "zod";
import { readBoundedStrictJSON } from "./strict-json-response";
import { parseWorkbenchErrorEnvelopePayload } from "./workbench-context";

const MAX_BYTES = 64 * 1024;
const id = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const text = z.string().min(1).max(512).refine(value => value.trim() === value && !/[\u0000-\u001f\u007f-\u009f]/.test(value));
const int64 = z.string().max(20).regex(/^(0|-?[1-9][0-9]*)$/).refine(value => {
  try { const parsed = BigInt(value); return parsed >= BigInt("-9223372036854775808") && parsed <= BigInt("9223372036854775807"); } catch { return false; }
});
const nonnegative = int64.refine(value => /^(0|[1-9][0-9]*)$/.test(value));
export const COMMERCIAL_RESOURCE_MAX_BYTES = 16 * 1024;
const timestamp = z.string().max(40).regex(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,9})?Z$/).refine(value => Number.isFinite(Date.parse(value)));
const resourceTimestamp = timestamp.refine(value => {
  try { return Number(value.slice(0, 4)) > 0 && new Date(value).toISOString().slice(0, 19) === value.slice(0, 19); } catch { return false; }
});
// Do not call BigInt in a refinement that can run after an invalid string.
const resourceQuantity = int64.refine(value => /^(0|[1-9][0-9]*)$/.test(value));
const resourceType = z.enum(["store_renewal_period", "ai_point", "data_row"]);
const resourceBase = { resource_type: resourceType, unit: z.enum(["period", "point", "row"]) };
const resourceBalance = z.discriminatedUnion("state", [
  z.object({ ...resourceBase, state: z.literal("recorded"), available: resourceQuantity, allocated: resourceQuantity, reserved: resourceQuantity, consumed: resourceQuantity, debt: resourceQuantity, updated_at: resourceTimestamp }).strict(),
  z.object({ ...resourceBase, state: z.literal("not_recorded"), available: z.null(), allocated: z.null(), reserved: z.null(), consumed: z.null(), debt: z.null(), updated_at: z.null() }).strict(),
]).refine(value => value.unit === ({ store_renewal_period: "period", ai_point: "point", data_row: "row" } as const)[value.resource_type] && (value.state !== "recorded" || ((value.debt === "0" || value.available === "0") && (value.resource_type !== "ai_point" || value.allocated === "0"))));
const resourceBalances = z.object({ schema_version: z.literal("organization-resource-balances-v1"), organization_id: id, observed_at: resourceTimestamp, resources: z.array(resourceBalance).length(3) }).strict().refine(value => new Set(value.resources.map(v => v.resource_type)).size === 3);
export {resourceBalances as commercialResourceBalancesSchema};
export type CommercialResources = z.infer<typeof resourceBalances>;
export const parseCommercialResources = (value: unknown) => { const parsed = resourceBalances.safeParse(value); return parsed.success ? parsed.data : null; };
const wallet = z.object({ organization_id: id, currency: z.literal("CNY"), available_minor: nonnegative, reserved_minor: nonnegative, debt_minor: nonnegative, lifetime_topup_minor: nonnegative, lifetime_spend_minor: nonnegative, version: nonnegative.refine(value => value !== "0"), observed_at: timestamp }).strict().refine(value => value.available_minor === "0" || value.debt_minor === "0");
const walletEntry = z.object({ entry_id: id, currency: z.literal("CNY"), entry_type: z.enum(["TOP_UP_CREDIT", "PURCHASE_RESERVE", "PURCHASE_COMMIT", "PURCHASE_RELEASE", "REFUND_REVERSAL", "CHARGEBACK_REVERSAL", "DEBT_REPAYMENT", "REFUND_RESERVE", "REFUND_CONFIRM", "REFUND_RELEASE"]), available_delta_minor: int64, reserved_delta_minor: int64, debt_delta_minor: int64, available_after_minor: nonnegative, reserved_after_minor: nonnegative, debt_after_minor: nonnegative, order_id: id.optional(), payment_id: id.optional(), source_id: text, occurred_at: timestamp }).strict();
const walletEntryPage = z.object({ organization_id: id, items: z.array(walletEntry).max(50), next_cursor: z.string().max(2048) }).strict();
const resourceProductKind = z.enum(["STORE_RENEWAL_PERIOD", "AI_POINT", "DATA_ROW"]);
const orderItem = z.object({ order_item_id: id, product_kind: resourceProductKind, resource_type: z.enum(["store_renewal_period", "ai_point", "data_row"]), resource_quantity: nonnegative, amount_minor: nonnegative }).strict();
const orderStatus = z.enum(["PENDING", "FUNDS_RESERVED", "FULFILLING", "FULFILLED", "CANCELLED", "RECONCILIATION_REQUIRED"]);
const orderBase = z.object({ order_id: id, organization_id: id, description: text, quote_id: id.optional(), currency: z.literal("CNY"), total_minor: nonnegative, status: orderStatus, wallet_reservation_id: id.optional(), items: z.array(orderItem).max(64), created_at: timestamp, updated_at: timestamp });
const topUpSummary = z.object({ provider: z.enum(["ALIPAY", "WECHAT_PAY"]), phase: z.enum(["CREATED", "AWAITING_PAYMENT", "RECONCILIATION_REQUIRED", "PAID_PENDING_CREDIT", "COMPLETED", "CLOSED_UNPAID"]), attempt_id: id, version: nonnegative.refine(v => v !== "0"), expires_at: timestamp, close_requested: z.boolean(), late_payment_corrected: z.boolean() }).strict();
const walletTopUpOrder = orderBase.extend({ kind: z.literal("WALLET_TOP_UP"), top_up: topUpSummary.optional(), failure_code: z.enum(["INSUFFICIENT_FUNDS", "RESOURCE_GRANT_REJECTED"]).optional() }).strict();
const resourcePurchaseOrder = orderBase.extend({ kind: z.literal("RESOURCE_PURCHASE"), failure_code: z.enum(["INSUFFICIENT_FUNDS", "RESOURCE_GRANT_REJECTED"]).optional(), product_kind: resourceProductKind }).strict();
const order = z.union([walletTopUpOrder, resourcePurchaseOrder]);
const orderPage = z.object({ organization_id: id, items: z.array(order).max(50), next_cursor: z.string().max(2048) }).strict().refine(value => value.items.every(item => item.organization_id === value.organization_id));
const orderSummary = z.object({ organization_id: id, currency: z.literal("CNY"), from: timestamp, until: timestamp, spend_minor: nonnegative, store_renewal_spend_minor: nonnegative, ai_point_spend_minor: nonnegative, data_row_spend_minor: nonnegative, other_spend_minor: nonnegative, observed_at: timestamp }).strict().refine(value => Date.parse(value.from) < Date.parse(value.until));

export type CommercialWallet = z.infer<typeof wallet>;
export const parseCommercialOrder = (value: unknown) => { const parsed = order.safeParse(value); return parsed.success ? parsed.data : null; };
export type CommercialWalletEntryPage = z.infer<typeof walletEntryPage>;
export type CommercialOrder = z.infer<typeof order>;
export type CommercialOrderPage = z.infer<typeof orderPage>;
export type CommercialOrderSummary = z.infer<typeof orderSummary>;
export type CommercialOrderFilters = { query?: string; kind?: CommercialOrder["kind"]; status?: CommercialOrder["status"]; from?: string; until?: string; cursor?: string };

export const parseCommercialWallet = (value: unknown) => { const parsed = wallet.safeParse(value); return parsed.success ? parsed.data : null; };
export const parseCommercialWalletEntries = (value: unknown) => { const parsed = walletEntryPage.safeParse(value); return parsed.success ? parsed.data : null; };
export const parseCommercialOrderPage = (value: unknown) => { const parsed = orderPage.safeParse(value); return parsed.success ? parsed.data : null; };
export const parseCommercialOrderSummary = (value: unknown) => { const parsed = orderSummary.safeParse(value); return parsed.success ? parsed.data : null; };

const errorStatuses: Readonly<Record<string, number>> = { INVALID_REQUEST: 400, AUTHENTICATION_REQUIRED: 401, PERMISSION_DENIED: 403, ORGANIZATION_ACCESS_DENIED: 403, ORGANIZATION_ACCESS_REVOKED: 403, ORGANIZATION_SUSPENDED: 403, ORGANIZATION_SELECTION_REQUIRED: 409, ORGANIZATION_CONTEXT_CHANGED: 409, IDENTITY_CONTEXT_CHANGED: 409, DEPENDENCY_UNAVAILABLE: 503, FEATURE_UNAVAILABLE: 503, DEADLINE_EXCEEDED: 504, INVALID_UPSTREAM_RESPONSE: 502 };
export class CommercialBillingReadError extends Error {
  constructor(public readonly status: number, public readonly code: string, public readonly requestId = "") { super("Commercial billing read could not be completed"); }
}

export function parseCommercialBillingFailure(payload: unknown, status: number) {
  const parsed = parseWorkbenchErrorEnvelopePayload(payload);
  return parsed.success && errorStatuses[parsed.data.code] === status ? parsed.data : null;
}

export async function readCommercialBilling<T extends { organization_id: string }>(path: string, expectedUserId: string, expectedOrganizationId: string, parser: (payload: unknown) => T | null, signal?: AbortSignal, maxBytes = MAX_BYTES): Promise<T> {
  if (!id.safeParse(expectedUserId).success || !id.safeParse(expectedOrganizationId).success) throw new CommercialBillingReadError(400, "INVALID_REQUEST");
  const controller = new AbortController();
  const abort = () => controller.abort();
  signal?.addEventListener("abort", abort, { once: true });
  if (signal?.aborted) abort();
  const timeout = setTimeout(abort, 15_000);
  try {
    controller.signal.throwIfAborted();
    const response = await fetch(path, { method: "GET", headers: { Accept: "application/json", "X-Expected-User-ID": expectedUserId, "X-Expected-Organization-ID": expectedOrganizationId }, cache: "no-store", redirect: "manual", signal: controller.signal });
    const payload = await readBoundedStrictJSON(response, response.status === 200 ? maxBytes : 8192, controller.signal);
    controller.signal.throwIfAborted();
    if (response.status === 200) {
      const result = parser(payload);
      if (result && result.organization_id === expectedOrganizationId) return result;
    }
    const failure = parseCommercialBillingFailure(payload, response.status);
    throw failure ? new CommercialBillingReadError(response.status, failure.code, failure.requestId) : new CommercialBillingReadError(502, "INVALID_UPSTREAM_RESPONSE");
  } catch (error) {
    if (controller.signal.aborted) throw new CommercialBillingReadError(504, "DEADLINE_EXCEEDED");
    if (error instanceof CommercialBillingReadError) throw error;
    throw new CommercialBillingReadError(502, "INVALID_UPSTREAM_RESPONSE");
  } finally {
    clearTimeout(timeout);
    signal?.removeEventListener("abort", abort);
  }
}

export function getCommercialWallet(userId: string, organizationId: string, signal?: AbortSignal) {
  return readCommercialBilling("/api/workbench/commercial/wallet", userId, organizationId, parseCommercialWallet, signal);
}
export function getCommercialResources(userId: string, organizationId: string, signal?: AbortSignal) {
  return readCommercialBilling("/api/workbench/commercial/resources", userId, organizationId, parseCommercialResources, signal, COMMERCIAL_RESOURCE_MAX_BYTES);
}
export function getCommercialWalletEntries(userId: string, organizationId: string, signal?: AbortSignal, cursor?: string) {
  const query = new URLSearchParams({ limit: "50" });
  if (cursor) query.set("cursor", cursor);
  return readCommercialBilling(`/api/workbench/commercial/wallet/entries?${query}`, userId, organizationId, parseCommercialWalletEntries, signal);
}
export function getCommercialOrders(userId: string, organizationId: string, filters: CommercialOrderFilters = {}, signal?: AbortSignal) {
  const query = new URLSearchParams({ limit: "50" });
  if (filters.query) query.set("q", filters.query);
  if (filters.kind) query.set("kind", filters.kind);
  if (filters.status) query.set("status", filters.status);
  if (filters.from) query.set("from", filters.from);
  if (filters.until) query.set("until", filters.until);
  if (filters.cursor) query.set("cursor", filters.cursor);
  return readCommercialBilling(`/api/workbench/commercial/orders?${query}`, userId, organizationId, parseCommercialOrderPage, signal);
}
export function getCommercialOrderSummary(userId: string, organizationId: string, signal?: AbortSignal) {
  return readCommercialBilling("/api/workbench/commercial/orders/summary", userId, organizationId, parseCommercialOrderSummary, signal);
}
export function getCommercialOrder(userId: string, organizationId: string, orderId: string, signal?: AbortSignal) {
  if (!id.safeParse(orderId).success) throw new CommercialBillingReadError(400, "INVALID_REQUEST");
  return readCommercialBilling(`/api/workbench/commercial/orders/${encodeURIComponent(orderId)}`, userId, organizationId, value => {
    const parsed = order.safeParse(value);
    return parsed.success && parsed.data.order_id === orderId ? parsed.data : null;
  }, signal);
}

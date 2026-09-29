import { z } from "zod";
import {
  parseCommercialOrder,
  type CommercialOrder,
} from "./commercial-billing";
import { readBoundedStrictJSON } from "./strict-json-response";
import { parseWorkbenchErrorEnvelopePayload } from "./workbench-context";

const id = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const version = z
  .string()
  .min(1)
  .max(128)
  .refine((v) => v.trim() === v && !/[\u0000-\u001f\u007f-\u009f]/.test(v));
export function positiveInt64(v: unknown): v is string {
  return (
    typeof v === "string" &&
    /^[1-9][0-9]{0,18}$/.test(v) &&
    BigInt(v) <= BigInt("9223372036854775807")
  );
}
const positive = z.string().refine(positiveInt64);
const nonnegative = z.string().refine((v) => v === "0" || positiveInt64(v));
const timestamp = z
  .string()
  .max(40)
  .regex(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,9})?Z$/)
  .refine((v) => Number.isFinite(Date.parse(v)));
const kind = z.enum(["STORE_RENEWAL_PERIOD", "AI_POINT", "DATA_ROW"]);
const resource = z.enum(["store_renewal_period", "ai_point", "data_row"]);
const types = {
  STORE_RENEWAL_PERIOD: "store_renewal_period",
  AI_POINT: "ai_point",
  DATA_ROW: "data_row",
} as const;
const offer = z
  .object({
    offer_id: id,
    product_kind: kind,
    resource_type: resource,
    currency: z.literal("CNY"),
    unit_price_minor: positive,
    min_quantity: positive,
    max_quantity: positive,
    pricing_version: version,
  })
  .strict()
  .refine(
    (v) =>
      v.resource_type === types[v.product_kind] &&
      positiveInt64(v.min_quantity) &&
      positiveInt64(v.max_quantity) &&
      BigInt(v.min_quantity) <= BigInt(v.max_quantity),
  );
const offers = z
  .object({ organization_id: id, items: z.array(offer).max(100) })
  .strict()
  .refine(
    (v) => new Set(v.items.map((o) => o.offer_id)).size === v.items.length,
  );
const quote = z
  .object({
    quote_id: id,
    organization_id: id,
    offer_id: id,
    product_kind: kind,
    resource_type: resource,
    resource_quantity: positive,
    currency: z.literal("CNY"),
    total_minor: positive,
    pricing_version: version,
    expires_at: timestamp,
    fingerprint: id,
    created_at: timestamp,
    amount_minor: positive.optional(),
    unit_price_minor: positive.optional(),
    remainder_minor: nonnegative.optional(),
  })
  .strict()
  .refine((v) => {
    if (
      v.resource_type !== types[v.product_kind] ||
      Date.parse(v.created_at) >= Date.parse(v.expires_at)
    )
      return false;
    const amountFields = [
      v.amount_minor,
      v.unit_price_minor,
      v.remainder_minor,
    ];
    if (amountFields.every((f) => f === undefined)) return true;
    if (
      v.product_kind === "STORE_RENEWAL_PERIOD" ||
      !positiveInt64(v.amount_minor) ||
      !positiveInt64(v.unit_price_minor) ||
      !(v.remainder_minor === "0" || positiveInt64(v.remainder_minor)) ||
      !positiveInt64(v.resource_quantity) ||
      !positiveInt64(v.total_minor)
    )
      return false;
    return (
      BigInt(v.resource_quantity) * BigInt(v.unit_price_minor) ===
        BigInt(v.total_minor) &&
      BigInt(v.total_minor) + BigInt(v.remainder_minor) ===
        BigInt(v.amount_minor) &&
      BigInt(v.remainder_minor) < BigInt(v.unit_price_minor)
    );
  });
export type ResourceOffer = z.infer<typeof offer>;
export type ResourceOffers = z.infer<typeof offers>;
export type ResourceQuote = z.infer<typeof quote>;
export type ResourceOrder = Extract<
  CommercialOrder,
  { kind: "RESOURCE_PURCHASE" }
>;
export const parseResourceOffers = (v: unknown) => {
  const result = offers.safeParse(v);
  return result.success ? result.data : null;
};
export const parseResourceQuote = (v: unknown) => {
  const result = quote.safeParse(v);
  return result.success ? result.data : null;
};
const errorStatuses: Record<string, readonly number[]> = {
  INVALID_REQUEST: [400, 413],
  AUTHENTICATION_REQUIRED: [401],
  FORBIDDEN: [403],
  PERMISSION_DENIED: [403],
  ORGANIZATION_ACCESS_DENIED: [403],
  ORGANIZATION_ACCESS_REVOKED: [403],
  ORGANIZATION_SUSPENDED: [403],
  ORGANIZATION_SELECTION_REQUIRED: [409],
  ORGANIZATION_CONTEXT_CHANGED: [409],
  IDENTITY_CONTEXT_CHANGED: [409],
  OFFER_UNAVAILABLE: [503],
  QUOTE_EXPIRED: [409],
  PAYMENT_METHOD_UNAVAILABLE: [409],
  INSUFFICIENT_FUNDS: [409],
  IDEMPOTENCY_CONFLICT: [409],
  CONFLICT: [409],
  RECONCILIATION_REQUIRED: [409],
  NOT_FOUND: [404],
  FEATURE_UNAVAILABLE: [503],
  DEPENDENCY_UNAVAILABLE: [503],
  DEADLINE_EXCEEDED: [504],
  INVALID_UPSTREAM_RESPONSE: [502],
};
class ResourcePurchaseError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    public readonly requestId = "",
  ) {
    super("Resource purchase request could not be completed");
  }
}
async function request<T>(
  path: string,
  user: string,
  org: string,
  parse: (v: unknown) => T | null,
  init: { body?: object; idempotencyKey?: string; signal?: AbortSignal } = {},
): Promise<T> {
  if (
    !id.safeParse(user).success ||
    !id.safeParse(org).success ||
    (init.idempotencyKey !== undefined &&
      (!init.idempotencyKey.trim() || init.idempotencyKey.length > 192))
  )
    throw new ResourcePurchaseError(400, "INVALID_REQUEST");
  const controller = new AbortController();
  const abort = () => controller.abort();
  init.signal?.addEventListener("abort", abort, { once: true });
  if (init.signal?.aborted) abort();
  const timeout = setTimeout(abort, 15_000);
  try {
    controller.signal.throwIfAborted();
    const headers = new Headers({
      Accept: "application/json",
      "X-Expected-User-ID": user,
      "X-Expected-Organization-ID": org,
    });
    if (init.body) headers.set("Content-Type", "application/json");
    if (init.idempotencyKey)
      headers.set("Idempotency-Key", init.idempotencyKey);
    const response = await fetch(path, {
      method: init.body ? "POST" : "GET",
      headers,
      body: init.body ? JSON.stringify(init.body) : undefined,
      cache: "no-store",
      redirect: "manual",
      signal: controller.signal,
    });
    const success = response.status === (init.body ? 201 : 200);
    // Order conflicts may carry the original durable order, not an error envelope.
    const persistedOrder =
      init.body !== undefined &&
      path === "/api/workbench/commercial/orders" &&
      response.status === 409;
    const payload = await readBoundedStrictJSON(
      response,
      success || persistedOrder ? 64 * 1024 : 8192,
      controller.signal,
    );
    controller.signal.throwIfAborted();
    if (success || persistedOrder) {
      const parsed = parse(payload);
      if (parsed) return parsed;
    }
    if (!success) {
      const failure = parseWorkbenchErrorEnvelopePayload(payload);
      if (
        failure.success &&
        errorStatuses[failure.data.code]?.includes(response.status)
      )
        throw new ResourcePurchaseError(
          response.status,
          failure.data.code,
          failure.data.requestId,
        );
    }
    throw new ResourcePurchaseError(502, "INVALID_UPSTREAM_RESPONSE");
  } catch (error) {
    if (controller.signal.aborted)
      throw new ResourcePurchaseError(504, "DEADLINE_EXCEEDED");
    if (error instanceof ResourcePurchaseError) throw error;
    throw new ResourcePurchaseError(503, "DEPENDENCY_UNAVAILABLE");
  } finally {
    clearTimeout(timeout);
    init.signal?.removeEventListener("abort", abort);
  }
}
export function getResourceOffers(
  user: string,
  org: string,
  signal?: AbortSignal,
) {
  return request(
    "/api/workbench/commercial/resource-offers",
    user,
    org,
    (v) => {
      const parsed = parseResourceOffers(v);
      return parsed?.organization_id === org ? parsed : null;
    },
    { signal },
  );
}
export async function createResourceQuote(
  user: string,
  org: string,
  offerId: string,
  selection:
    | { quantity: string; amountMinor?: never }
    | { amountMinor: string; quantity?: never },
  signal?: AbortSignal,
) {
  if (
    !id.safeParse(offerId).success ||
    (selection.quantity !== undefined) ===
      (selection.amountMinor !== undefined) ||
    !positiveInt64(selection.quantity ?? selection.amountMinor)
  )
    throw new ResourcePurchaseError(400, "INVALID_REQUEST");
  return request(
    "/api/workbench/commercial/quotes",
    user,
    org,
    (v) => {
      const parsed = parseResourceQuote(v);
      return parsed?.organization_id === org &&
        parsed.offer_id === offerId &&
        (selection.quantity === undefined
          ? parsed.amount_minor === selection.amountMinor
          : parsed.resource_quantity === selection.quantity)
        ? parsed
        : null;
    },
    {
      body: {
        offer_id: offerId,
        ...(selection.quantity !== undefined
          ? { quantity: selection.quantity }
          : { amount_minor: selection.amountMinor }),
      },
      signal,
    },
  );
}
export function createResourceOrder(
  user: string,
  org: string,
  quoteId: string,
  key: string,
  signal?: AbortSignal,
) {
  if (!id.safeParse(quoteId).success)
    throw new ResourcePurchaseError(400, "INVALID_REQUEST");
  return request(
    "/api/workbench/commercial/orders",
    user,
    org,
    (v) => {
      const parsed = parseCommercialOrder(v);
      return parsed?.kind === "RESOURCE_PURCHASE" &&
        parsed.organization_id === org &&
        parsed.quote_id === quoteId
        ? parsed
        : null;
    },
    { body: { quote_id: quoteId }, idempotencyKey: key, signal },
  );
}

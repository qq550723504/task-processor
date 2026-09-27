import { z } from "zod";
import {
  parseCommercialOrder,
  type CommercialOrder,
} from "./commercial-billing";
import { readBoundedStrictJSON } from "./strict-json-response";
import { parseWorkbenchErrorEnvelopePayload } from "./workbench-context";

const id = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const minor = z
  .string()
  .max(19)
  .regex(/^(0|[1-9][0-9]*)$/)
  .refine((v) => BigInt(v) <= BigInt("9223372036854775807"));
export const paymentProvider = z.enum(["ALIPAY", "WECHAT_PAY"]);
export type PaymentProvider = z.infer<typeof paymentProvider>;
const timestamp = z.string().max(40).datetime({ precision: null });
const channels = z.discriminatedUnion("provider", [
  z
    .object({
      provider: z.literal("ALIPAY"),
      product: z.literal("PAGE_PAY"),
      available: z.boolean(),
      reason: z.string().max(64),
    })
    .strict(),
  z
    .object({
      provider: z.literal("WECHAT_PAY"),
      product: z.literal("NATIVE"),
      available: z.boolean(),
      reason: z.string().max(64),
    })
    .strict(),
]);
const options = z
  .object({
    organization_id: id,
    currency: z.literal("CNY"),
    min_minor: minor,
    max_minor: minor,
    quick_amounts_minor: z.array(minor).max(12),
    channels: z.array(channels).length(2),
  })
  .strict()
  .refine(
    (v) =>
      new Set(v.channels.map((c) => c.provider)).size === 2 &&
      (!v.channels.some((c) => c.available) ||
        (BigInt(v.min_minor) > BigInt(0) &&
          BigInt(v.max_minor) >= BigInt(v.min_minor) &&
          v.quick_amounts_minor.length > 0 &&
          v.quick_amounts_minor.every(
            (a) =>
              BigInt(a) >= BigInt(v.min_minor) &&
              BigInt(a) <= BigInt(v.max_minor),
          ))),
  );
const checkout = z
  .object({
    organization_id: id,
    order_id: id,
    attempt_id: id,
    provider: paymentProvider,
    kind: z.enum(["REDIRECT", "QR_CODE"]),
    expires_at: timestamp,
    payload: z
      .string()
      .min(1)
      .max(64 * 1024),
  })
  .strict()
  .refine((v) => {
    try {
      const u = new URL(v.payload);
      if (u.username || u.password || u.hash) return false;
      return v.provider === "ALIPAY"
        ? v.kind === "REDIRECT" &&
            u.protocol === "https:" &&
            ["openapi.alipay.com", "openapi-sandbox.dl.alipaydev.com"].includes(
              u.host,
            ) &&
            u.pathname === "/gateway.do"
        : v.kind === "QR_CODE" &&
            u.protocol === "weixin:" &&
            u.host === "wxpay" &&
            u.pathname === "/bizpayurl" &&
            !!u.searchParams.get("pr");
    } catch {
      return false;
    }
  });
export type TopUpOptions = z.infer<typeof options>;
export type TopUpCheckout = z.infer<typeof checkout>;
export type TopUpOrder = Extract<CommercialOrder, { kind: "WALLET_TOP_UP" }>;
export const parseTopUpOptions = (v: unknown) => {
  const p = options.safeParse(v);
  return p.success ? p.data : null;
};
export const parseTopUpCheckout = (v: unknown) => {
  const p = checkout.safeParse(v);
  return p.success ? p.data : null;
};
export function yuanToMinor(value: string): string | null {
  if (!/^(0|[1-9][0-9]*)(\.[0-9]{1,2})?$/.test(value) || value.length > 22)
    return null;
  const [whole, fraction = ""] = value.split(".");
  const result = BigInt(whole) * BigInt(100) + BigInt(fraction.padEnd(2, "0"));
  return result > BigInt(0) && result <= BigInt("9223372036854775807")
    ? result.toString()
    : null;
}
export function topUpMoney(value: string) {
  const n = BigInt(value);
  return `${(n / BigInt(100)).toString()}.${(n % BigInt(100)).toString().padStart(2, "0")}`;
}
export class WalletTopUpError extends Error {
  constructor(public readonly code: string) {
    super("Wallet top-up request could not be completed");
  }
}
async function request<T extends { organization_id: string }>(
  path: string,
  user: string,
  org: string,
  parse: (v: unknown) => T | null,
  status: number,
  body?: object,
  key?: string,
  signal?: AbortSignal,
): Promise<T> {
  if (!id.safeParse(user).success || !id.safeParse(org).success)
    throw new WalletTopUpError("INVALID_REQUEST");
  const controller = new AbortController();
  const abort = () => controller.abort();
  signal?.addEventListener("abort", abort, { once: true });
  if (signal?.aborted) abort();
  const timeout = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    const headers = new Headers({
      Accept: "application/json",
      "X-Expected-User-ID": user,
      "X-Expected-Organization-ID": org,
    });
    if (body) headers.set("Content-Type", "application/json");
    if (key) headers.set("Idempotency-Key", key);
    const res = await fetch(path, {
      method: body ? "POST" : "GET",
      headers,
      body: body ? JSON.stringify(body) : undefined,
      redirect: "manual",
      cache: "no-store",
      signal: controller.signal,
    });
    const raw = await readBoundedStrictJSON(
      res,
      res.status === status ? 64 * 1024 : 8192,
      controller.signal,
    );
    controller.signal.throwIfAborted();
    if (res.status === status) {
      const result = parse(raw);
      if (result && result.organization_id === org) return result;
    }
    const failure = parseWorkbenchErrorEnvelopePayload(raw);
    if (res.status >= 400 && failure.success)
      throw new WalletTopUpError(failure.data.code);
    throw new WalletTopUpError("INVALID_UPSTREAM_RESPONSE");
  } catch (e) {
    if (controller.signal.aborted)
      throw new WalletTopUpError("DEADLINE_EXCEEDED");
    if (e instanceof WalletTopUpError) throw e;
    throw new WalletTopUpError("DEPENDENCY_UNAVAILABLE");
  } finally {
    clearTimeout(timeout);
    signal?.removeEventListener("abort", abort);
  }
}
function parseTopUpOrder(v: unknown): TopUpOrder | null {
  const o = parseCommercialOrder(v);
  return o?.kind === "WALLET_TOP_UP" && o.top_up ? o : null;
}
export function getTopUpOptions(
  user: string,
  org: string,
  signal?: AbortSignal,
) {
  return request(
    "/api/workbench/commercial/wallet/top-up-options",
    user,
    org,
    parseTopUpOptions,
    200,
    undefined,
    undefined,
    signal,
  );
}
export function createTopUp(
  user: string,
  org: string,
  provider: PaymentProvider,
  amount: string,
  key: string,
  signal?: AbortSignal,
) {
  if (
    !paymentProvider.safeParse(provider).success ||
    !minor.safeParse(amount).success ||
    amount === "0" ||
    !key ||
    key.length > 192
  )
    throw new WalletTopUpError("INVALID_REQUEST");
  return request(
    "/api/workbench/commercial/wallet/top-up-intents",
    user,
    org,
    (v) => {
      const o = parseTopUpOrder(v);
      return o?.top_up?.provider === provider && o.total_minor === amount
        ? o
        : null;
    },
    201,
    { provider, amount_minor: amount },
    key,
    signal,
  );
}
export function checkoutTopUp(
  user: string,
  org: string,
  order: TopUpOrder,
  signal?: AbortSignal,
) {
  if (!order.top_up) throw new WalletTopUpError("INVALID_REQUEST");
  const a = order.top_up;
  return request(
    `/api/workbench/commercial/orders/${encodeURIComponent(order.order_id)}/checkout`,
    user,
    org,
    (v) => {
      const p = parseTopUpCheckout(v);
      return p?.order_id === order.order_id &&
        p.attempt_id === a.attempt_id &&
        p.provider === a.provider &&
        p.expires_at === a.expires_at
        ? p
        : null;
    },
    200,
    { expected_version: a.version },
    undefined,
    signal,
  );
}
export function cancelTopUp(
  user: string,
  org: string,
  order: TopUpOrder,
  signal?: AbortSignal,
) {
  if (!order.top_up) throw new WalletTopUpError("INVALID_REQUEST");
  return request(
    `/api/workbench/commercial/orders/${encodeURIComponent(order.order_id)}/cancel-payment`,
    user,
    org,
    (v) => {
      const p = parseTopUpOrder(v);
      return p?.order_id === order.order_id ? p : null;
    },
    200,
    { expected_version: order.top_up.version },
    undefined,
    signal,
  );
}

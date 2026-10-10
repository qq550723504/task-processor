import { z } from "zod";
import { readBoundedStrictJSON } from "./strict-json-response";
export const observationID = z
  .string()
  .regex(
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
  )
  .refine((v) => v !== "00000000-0000-0000-0000-000000000000");
const text = (n: number) =>
  z
    .string()
    .refine(
      (v) => v.trim() === v && !/\p{Cc}/u.test(v) && Array.from(v).length <= n,
    );
const identity = text(128).refine((v) => v.length > 0);
const count = z.number().int().min(0).max(Number.MAX_SAFE_INTEGER);
const code = z.number().int().min(0).max(1_000_000).nullable();
const date = z.iso.datetime({ offset: true });
const optionalDate = z.union([date, z.literal("")]);
const image = text(2048).refine((v) => {
  if (!v) return true;
  try {
    const u = new URL(v);
    return u.protocol === "https:" && !u.username && !u.password && !u.hash;
  } catch {
    return false;
  }
});
const observationPriceSchema = z
  .object({
    currency: z.string().regex(/^(?:[A-Z]{3})?$/),
    value: z.string().regex(/^(?:[0-9]{1,20}(?:\.[0-9]{1,8})?)?$/),
    special: z.string().regex(/^(?:[0-9]{1,20}(?:\.[0-9]{1,8})?)?$/),
  })
  .strict();
const sku = z
  .object({
    id: identity,
    sellerSku: text(800),
    prices: z.array(observationPriceSchema).max(100),
    costs: z.array(observationPriceSchema).max(100),
    inventory: z
      .array(
        z
          .object({ warehouseId: identity, quantity: count.nullable() })
          .strict(),
      )
      .max(200),
  })
  .strict();
const skc = z
  .object({
    id: identity,
    sellerCode: text(800),
    title: text(1000),
    imageUrl: image,
    site: z.enum(["shein-us", ""]),
    siteStatus: code,
    skus: z.array(sku).max(400),
  })
  .strict();
const product = z
  .object({ id: identity, skcs: z.array(skc).max(100) })
  .strict();
const order = z
  .object({
    id: identity,
    site: z.literal("shein-us"),
    status: code,
    stockMode: code,
    type: code,
    tag: code,
    reasons: z.array(z.number().int().min(0).max(1_000_000)).max(50),
    items: z
      .array(
        z
          .object({
            id: identity,
            sku: text(128),
            sellerSku: text(800),
            title: text(1000),
            imageUrl: image,
            status: code,
            exchangeTag: code,
          })
          .strict(),
      )
      .max(1000),
    packages: z
      .array(
        z
          .object({
            id: text(128),
            waybill: text(128),
            carrier: text(200),
            label: text(128),
          })
          .strict(),
      )
      .max(1000),
    amount: observationPriceSchema.nullable(),
    supplyCost: observationPriceSchema.nullable(),
    createdAt: optionalDate,
    updatedAt: optionalDate,
    issuedAt: optionalDate,
    needDeliveryAt: optionalDate,
    handoverAt: optionalDate,
    expectedCollectAt: optionalDate,
  })
  .strict();
export const observationRecordSchema = z
  .object({
    storeId: observationID,
    syncId: observationID,
    id: identity,
    observedAt: date,
    stale: z.boolean().optional(),
    product: product.optional(),
    order: order.optional(),
  })
  .strict()
  .refine(
    (v) =>
      Boolean(v.product) !== Boolean(v.order) &&
      (v.product?.id ?? v.order?.id) === v.id,
  );
export const observationKindSchema = z.enum(["products", "orders"]);
export type ObservationKind = z.infer<typeof observationKindSchema>;
const window = z.object({ start: date, end: date }).strict();
export const observationSyncSchema = z
  .object({
    id: observationID,
    commandId: observationID,
    storeId: observationID,
    kind: observationKindSchema,
    status: z.enum([
      "pending",
      "running",
      "completed",
      "partial",
      "failed",
      "suspended",
    ]),
    progress: z
      .object({
        page: z.number().int().min(1).max(5001),
        windows: z.array(window).max(512),
        expectedTotal: count.nullable(),
        seen: count,
        pages: count,
        incomplete: z.boolean(),
        notes: z.array(text(64)).max(20),
      })
      .strict(),
    range: window.nullable(),
    createdAt: date,
    observedAt: date.nullable(),
    errorCode: text(64),
  })
  .strict();
export const observationBeginSchema = z
  .object({
    kind: observationKindSchema,
    stores: z.array(observationID).max(500),
    start: date.nullable().optional(),
    end: date.nullable().optional(),
  })
  .strict()
  .refine(
    (v) =>
      new Set(v.stores).size === v.stores.length &&
      (v.kind !== "products" || (!v.start && !v.end)),
  );
export const observationCommandSchema = z
  .object({
    id: observationID,
    input: observationBeginSchema,
    createdAt: date,
    syncs: z.array(observationSyncSchema).max(500),
  })
  .strict();
export const observationListSchema = z
  .object({
    items: z.array(observationRecordSchema).max(50),
    next: text(4096),
    summary: z
      .object({
        total: count,
        active: count,
        offShelf: count,
        today: count,
        todayUnknown: count,
        pending: count,
        transit: count,
        exceptional: count,
        unknown: count,
      })
      .strict(),
    syncs: z.array(observationSyncSchema).max(500),
    latest: z.array(observationSyncSchema).max(500),
    complete: z.boolean(),
  })
  .strict();
export const observationTracksSchema = z
  .array(
    z
      .object({
        carrier: text(200),
        waybill: text(128),
        nodes: z
          .array(
            z
              .object({
                description: text(2000),
                code: text(128),
                name: text(500),
                atMillis: count,
              })
              .strict(),
          )
          .max(1000),
      })
      .strict(),
  )
  .max(50);
export const observationCapabilitiesSchema = z
  .object({
    available: z.boolean(),
    canSync: z.boolean(),
    kind: observationKindSchema,
    site: z.literal("shein-us"),
    platformUrl: z.literal("https://sellerhub.shein.com/"),
  })
  .strict()
  .refine((v) => !v.canSync || v.available);
export type ObservationRecord = z.infer<typeof observationRecordSchema>;
export type ObservationSync = z.infer<typeof observationSyncSchema>;
export type ObservationPrice = z.infer<typeof observationPriceSchema>;
export type ObservationOrder = NonNullable<ObservationRecord["order"]>;
export type ObservationScope = { organizationId: string; userId: string };
export const observationEnvelopeSchema = (schema: z.ZodType) =>
  z
    .object({ organizationId: identity, userId: identity, data: schema })
    .strict();
export class ObservationError extends Error {
  constructor(
    public code: string,
    public status: number,
  ) {
    super(code);
  }
}
export async function observationRequest<T>(
  path: string,
  scope: ObservationScope,
  schema: z.ZodType<T>,
  signal?: AbortSignal,
  write?: { key?: string; input?: unknown },
): Promise<T> {
  const headers = new Headers({
    "X-Expected-Organization-ID": scope.organizationId,
    "X-Expected-User-ID": scope.userId,
    Accept: "application/json",
  });
  let body: string | undefined;
  if (write?.input !== undefined) {
    headers.set("Content-Type", "application/json");
    body = JSON.stringify(write.input);
  }
  if (write?.key) headers.set("Idempotency-Key", write.key);
  let response: Response;
  try {
    response = await fetch(`/api/workbench/store-observations/${path}`, {
      method: write ? "POST" : "GET",
      headers,
      body,
      signal,
      cache: "no-store",
      redirect: "error",
    });
  } catch {
    throw new ObservationError("DEPENDENCY_UNAVAILABLE", 0);
  }
  const payload = await readBoundedStrictJSON(response, 2 << 20, signal).catch(
    () => null,
  );
  if (!response.ok) {
    const e = z.object({ code: z.string() }).passthrough().safeParse(payload);
    throw new ObservationError(
      e.success ? e.data.code : "INVALID_UPSTREAM_RESPONSE",
      response.status,
    );
  }
  const parsed = observationEnvelopeSchema(schema).safeParse(payload);
  if (!parsed.success)
    throw new ObservationError("INVALID_UPSTREAM_RESPONSE", 502);
  if (
    parsed.data.organizationId !== scope.organizationId ||
    parsed.data.userId !== scope.userId
  )
    throw new ObservationError("ORGANIZATION_CONTEXT_CHANGED", 409);
  return schema.parse(parsed.data.data);
}
export function orderStatus(status: number | null) {
  const labels: Record<number, string> = {
    1: "待处理",
    2: "待发货",
    3: "待 SHEIN 发货",
    4: "已发货",
    5: "已签收",
    6: "用户已退款",
    7: "待揽收",
    8: "已报损",
    9: "已拒收",
  };
  return status === null
    ? "平台未提供"
    : (labels[status] ?? `未识别 (${status})`);
}
export function orderProblems(v: ObservationOrder) {
  const labels: Record<number, string> = {
    1: "系统处理",
    2: "客服验证中",
    3: "需要巴西发票",
    4: "未生成包裹或预测未成功",
    5: "系统处理",
    6: "部分商品缺货",
    7: "系统处理",
    8: "系统处理",
    9: "部分商品缺货",
    10: "未设置卖家仓库",
    11: "未设置卖家仓库",
    12: "需要确认拆包",
    13: "CTE 发票开票中",
  };
  const out = v.reasons.map((n) => labels[n] ?? `平台问题代码 ${n}`);
  if (v.tag === 1) out.push("平台问题标签");
  if (v.status === 8 || v.status === 9) out.push(orderStatus(v.status));
  for (const p of v.packages) {
    if (p.label === "1" || p.label === "2")
      out.push(`包裹 ${p.id}：平台异常标签 ${p.label}`);
  }
  return out;
}

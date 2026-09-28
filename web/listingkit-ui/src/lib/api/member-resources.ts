import { z } from "zod";
import { readBoundedStrictJSON } from "./strict-json-response";
import { accountErrorCode } from "./account";

export const memberResourceID = z
  .string()
  .regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
export const resourceInteger = z
  .string()
  .refine(
    (v) =>
      v.length <= 19 &&
      /^(0|[1-9][0-9]*)$/.test(v) &&
      BigInt(v) <= BigInt("9223372036854775807"),
  );
const positive = resourceInteger.refine((v) => v !== "0");
const timestamp = z.string().datetime({ precision: null });
const resource = z.enum(["store_renewal_period", "data_row"]);
const position = z
  .object({
    free: resourceInteger,
    reserved: resourceInteger,
    consumed: resourceInteger,
    version: resourceInteger,
    recorded: z.boolean(),
  })
  .strict()
  .refine((v) =>
    v.recorded
      ? v.version !== "0"
      : [v.free, v.reserved, v.consumed, v.version].every((n) => n === "0"),
  );
const member = z
  .object({
    memberId: memberResourceID,
    userId: z.string().max(128),
    displayName: z.string().max(512),
    loginName: z.string().max(512),
    state: z.string().max(32),
    roles: z.array(z.string().max(128)).max(32),
    storeCount: resourceInteger.nullable(),
    periods: position,
    dataRows: position,
  })
  .strict();
const directory = z
  .object({
    schemaVersion: z.literal("member-resource-directory-v1"),
    organizationId: memberResourceID,
    observedAt: timestamp,
    members: z.array(member).max(200),
  })
  .strict()
  .refine(
    (v) => new Set(v.members.map((m) => m.memberId)).size === v.members.length,
  );
const offer = z
  .object({
    offerId: memberResourceID,
    pricingVersion: memberResourceID,
    currency: z.literal("CNY"),
    unitPriceMinor: positive,
    minQuantity: positive,
    maxQuantity: positive,
  })
  .strict()
  .refine(
    (v) =>
      [v.minQuantity, v.maxQuantity].every(
        (n) => resourceInteger.safeParse(n).success,
      ) && BigInt(v.minQuantity) <= BigInt(v.maxQuantity),
  );
const prices = z
  .object({ organizationId: memberResourceID, offers: z.array(offer).max(100) })
  .strict();
const quote = z
  .object({
    organizationId: memberResourceID,
    quoteId: memberResourceID,
    quantity: positive,
    unitPriceMinor: positive,
    pricingVersion: memberResourceID,
    currency: z.literal("CNY"),
    expiresAt: timestamp,
    amountMinor: positive,
    remainderMinor: resourceInteger,
  })
  .strict()
  .refine(
    (v) =>
      [v.quantity, v.unitPriceMinor, v.remainderMinor, v.amountMinor].every(
        (n) => resourceInteger.safeParse(n).success,
      ) &&
      BigInt(v.quantity) * BigInt(v.unitPriceMinor) +
        BigInt(v.remainderMinor) ===
        BigInt(v.amountMinor) &&
      BigInt(v.remainderMinor) < BigInt(v.unitPriceMinor),
  );
const transfer = z
  .object({
    organizationId: memberResourceID,
    memberId: memberResourceID,
    resourceType: resource,
    operationId: memberResourceID,
    position,
    unallocated: resourceInteger,
    allocated: resourceInteger,
    grossCredit: resourceInteger,
    debtRepaid: resourceInteger,
    netCredit: resourceInteger,
    replayed: z.boolean(),
  })
  .strict()
  .refine(
    (v) =>
      [v.grossCredit, v.debtRepaid, v.netCredit].every(
        (n) => resourceInteger.safeParse(n).success,
      ) && BigInt(v.grossCredit) === BigInt(v.debtRepaid) + BigInt(v.netCredit),
  );
const grant = z
  .object({
    organizationId: memberResourceID,
    memberId: memberResourceID,
    storeId: z.uuid(),
    active: z.boolean(),
    version: resourceInteger,
  })
  .strict();
const grantReceipt = grant.extend({ operationId: z.uuid() }).strict();
const storePage = z
  .object({
    organizationId: memberResourceID,
    memberId: memberResourceID,
    page: z.number().int().positive(),
    pageSize: z.number().int().min(1).max(100),
    total: resourceInteger,
    observedAt: timestamp,
    items: z
      .array(
        z
          .object({
            id: z.uuid(),
            name: z.string().max(512),
            recordStatus: z.string().max(32),
            version: z.number().int().positive(),
            grantVersion: positive,
            serviceExpiresAt: timestamp.nullable(),
          })
          .strict(),
      )
      .max(100),
  })
  .strict();
export const memberTransferInput = z
  .object({
    resourceType: resource,
    action: z.enum(["allocate", "reclaim"]),
    quantity: positive,
    expectedVersion: resourceInteger,
    quoteId: memberResourceID.optional(),
  })
  .strict();
export const memberGrantInput = z
  .object({ active: z.boolean(), expectedVersion: resourceInteger })
  .strict();
export const memberQuoteInput = z
  .object({ offerId: memberResourceID, amountMinor: positive })
  .strict();
export type MemberResourceScope = {
  expectedUserId: string;
  expectedOrganizationId: string;
};
export type MemberResourceEntry = z.infer<typeof member>;
type MemberResourceDirectory = z.infer<typeof directory>;
export type MemberDataQuote = z.infer<typeof quote>;
export type MemberTransferInput = z.infer<typeof memberTransferInput>;
export type MemberGrantInput = z.infer<typeof memberGrantInput>;
export class MemberResourceError extends Error {
  constructor(
    public status: number,
    public code: string,
    public outcome?: "unknown",
  ) {
    super("Member resource request could not be completed");
  }
}
export function parseMemberResourceResult(
  payload: unknown,
  path: string,
  method: string,
  org: string,
  key?: string,
) {
  const parts = path.split("/").filter(Boolean);
  const schema =
    path === ""
      ? directory
      : path === "/data-prices"
        ? prices
        : path === "/data-quotes"
          ? quote
          : parts[2] === "transfers"
            ? transfer
            : parts[2] === "stores" && parts.length === 3
              ? storePage
              : method === "PUT"
                ? grantReceipt
                : grant;
  const result = schema.safeParse(payload);
  if (
    !result.success ||
    result.data.organizationId !== org ||
    ("memberId" in result.data && result.data.memberId !== parts[1]) ||
    ("storeId" in result.data && result.data.storeId !== parts[3]) ||
    ("operationId" in result.data && result.data.operationId !== key)
  )
    throw new MemberResourceError(
      502,
      method === "GET" ? "INVALID_UPSTREAM_RESPONSE" : "RESULT_UNVERIFIED",
      method === "GET" ? undefined : "unknown",
    );
  return result.data;
}
export function memberResourceErrorCode(
  status: number,
  payload: unknown,
): string {
  const envelope = z
    .object({
      code: z.string(),
      message: z.string(),
      requestId: z.string(),
      fieldErrors: z.array(z.unknown()).max(0),
      outcome: z.literal("unknown").optional(),
    })
    .strict()
    .safeParse(payload);
  const owner: Record<number, string[]> = {
    403: ["FORBIDDEN"],
    404: ["STORE_NOT_FOUND"],
    409: [
      "RESOURCE_INSUFFICIENT_BALANCE",
      "RESOURCE_DEBT_OUTSTANDING",
      "QUOTE_EXPIRED",
    ],
    503: ["DATA_PRICE_UNAVAILABLE"],
  };
  if (envelope.success && owner[status]?.includes(envelope.data.code))
    return envelope.data.code;
  return accountErrorCode(status, payload);
}
async function request(
  scope: MemberResourceScope,
  path: string,
  method = "GET",
  body?: unknown,
  key?: string,
  signal?: AbortSignal,
) {
  if (
    !memberResourceID.safeParse(scope.expectedUserId).success ||
    !memberResourceID.safeParse(scope.expectedOrganizationId).success
  )
    throw new MemberResourceError(400, "INVALID_REQUEST");
  const write = method !== "GET";
  let sent = false;
  try {
    signal?.throwIfAborted();
    sent = true;
    const response = await fetch(`/api/account/member-resources${path}`, {
      method,
      signal,
      cache: "no-store",
      headers: {
        Accept: "application/json",
        "X-Expected-User-ID": scope.expectedUserId,
        "X-Expected-Organization-ID": scope.expectedOrganizationId,
        ...(write ? { "Content-Type": "application/json" } : {}),
        ...(key ? { "Idempotency-Key": key } : {}),
      },
      ...(write ? { body: JSON.stringify(body) } : {}),
    });
    const payload = await readBoundedStrictJSON(
      response,
      response.status === 200 ? 256 * 1024 : 8192,
      signal,
    );
    if (response.status !== 200) {
      const code = memberResourceErrorCode(response.status, payload);
      throw new MemberResourceError(
        response.status,
        code,
        write && (response.status >= 500 || code === "RESULT_UNVERIFIED")
          ? "unknown"
          : undefined,
      );
    }
    return parseMemberResourceResult(
      payload,
      path.split("?")[0],
      method,
      scope.expectedOrganizationId,
      key,
    );
  } catch (error) {
    if (error instanceof MemberResourceError) throw error;
    throw new MemberResourceError(
      502,
      write && sent ? "RESULT_UNVERIFIED" : "DEPENDENCY_UNAVAILABLE",
      write && sent ? "unknown" : undefined,
    );
  }
}
export async function getMemberResources(
  scope: MemberResourceScope,
  signal?: AbortSignal,
) {
  return (await request(
    scope,
    "",
    "GET",
    undefined,
    undefined,
    signal,
  )) as MemberResourceDirectory;
}
export async function getMemberDataPrices(
  scope: MemberResourceScope,
  signal?: AbortSignal,
) {
  return (await request(
    scope,
    "/data-prices",
    "GET",
    undefined,
    undefined,
    signal,
  )) as z.infer<typeof prices>;
}
export async function quoteMemberData(
  scope: MemberResourceScope,
  offerId: string,
  amountMinor: string,
) {
  return (await request(
    scope,
    "/data-quotes",
    "POST",
    memberQuoteInput.parse({ offerId, amountMinor }),
  )) as MemberDataQuote;
}
export async function transferMemberResource(
  scope: MemberResourceScope,
  memberId: string,
  input: MemberTransferInput,
  key: string,
) {
  memberResourceID.parse(memberId);
  memberResourceID.parse(key);
  return (await request(
    scope,
    `/members/${memberId}/transfers`,
    "POST",
    memberTransferInput.parse(input),
    key,
  )) as z.infer<typeof transfer>;
}
export async function getMemberStores(
  scope: MemberResourceScope,
  memberId: string,
  page = 1,
  signal?: AbortSignal,
) {
  memberResourceID.parse(memberId);
  return (await request(
    scope,
    `/members/${memberId}/stores?page=${page}&pageSize=20`,
    "GET",
    undefined,
    undefined,
    signal,
  )) as z.infer<typeof storePage>;
}
export async function getMemberStoreGrant(
  scope: MemberResourceScope,
  memberId: string,
  storeId: string,
  signal?: AbortSignal,
) {
  memberResourceID.parse(memberId);
  z.uuid().parse(storeId);
  return (await request(
    scope,
    `/members/${memberId}/stores/${storeId}/grant`,
    "GET",
    undefined,
    undefined,
    signal,
  )) as z.infer<typeof grant>;
}
export async function setMemberStoreGrant(
  scope: MemberResourceScope,
  memberId: string,
  storeId: string,
  input: MemberGrantInput,
  key: string,
) {
  memberResourceID.parse(memberId);
  z.uuid().parse(storeId);
  z.uuid().parse(key);
  return (await request(
    scope,
    `/members/${memberId}/stores/${storeId}/grant`,
    "PUT",
    memberGrantInput.parse(input),
    key,
  )) as z.infer<typeof grantReceipt>;
}

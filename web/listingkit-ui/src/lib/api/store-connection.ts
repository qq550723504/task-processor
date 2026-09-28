import { z } from "zod";
import {
  officialConnectionViewSchema,
  officialConnectionBeginSchema,
  officialConnectionCompleteSchema,
  officialConnectionQuerySchema,
  type OfficialConnectionCallback,
} from "@/lib/contracts/store-connection";
import { memberResourceID, type MemberResourceScope } from "./member-resources";
import { readBoundedStrictJSON } from "./strict-json-response";
import { parseWorkbenchErrorEnvelopePayload } from "./workbench-context";
export class StoreConnectionError extends Error {
  constructor(
    public status: number,
    public code: string,
    public unknown = false,
  ) {
    super("Official connection could not be completed");
  }
}
async function request<T>(
  scope: MemberResourceScope,
  storeId: string,
  action: string,
  schema: z.ZodType<T>,
  init: RequestInit = {},
): Promise<T> {
  memberResourceID.parse(scope.expectedUserId);
  memberResourceID.parse(scope.expectedOrganizationId);
  z.uuid().parse(storeId);
  const write = init.method === "POST";
  try {
    const response = await fetch(
      `/api/workbench/stores/${storeId}/connection${action ? `/${action}` : ""}`,
      {
        ...init,
        cache: "no-store",
        credentials: "same-origin",
        headers: {
          Accept: "application/json",
          "X-Expected-User-ID": scope.expectedUserId,
          "X-Expected-Organization-ID": scope.expectedOrganizationId,
          ...(init.headers ?? {}),
        },
      },
    );
    const payload = await readBoundedStrictJSON(
      response,
      16384,
      init.signal ?? undefined,
    );
    if (response.status !== 200) {
      const error = parseWorkbenchErrorEnvelopePayload(payload);
      if (!error.success)
        throw new StoreConnectionError(502, "INVALID_UPSTREAM_RESPONSE", write);
      const code = error.data.code;
      throw new StoreConnectionError(
        response.status,
        code,
        write &&
          response.status >= 500 &&
          code !== "STORE_OFFICIAL_SETUP_UNAVAILABLE",
      );
    }
    const result = schema.safeParse(payload);
    if (!result.success)
      throw new StoreConnectionError(502, "INVALID_UPSTREAM_RESPONSE", write);
    return result.data;
  } catch (error) {
    if (error instanceof StoreConnectionError) throw error;
    throw new StoreConnectionError(502, "RESULT_UNVERIFIED", write);
  }
}
export function getStoreConnection(
  scope: MemberResourceScope,
  storeId: string,
  signal?: AbortSignal,
) {
  return request(scope, storeId, "", officialConnectionViewSchema, {
    method: "GET",
    signal,
  });
}
export function beginStoreConnection(
  scope: MemberResourceScope,
  storeId: string,
  version: number,
  key: string,
) {
  z.uuid().parse(key);
  return request(scope, storeId, "begin", officialConnectionBeginSchema, {
    method: "POST",
    headers: { "If-Match": `"${version}"`, "Idempotency-Key": key },
  });
}
export async function completeStoreConnection(
  scope: MemberResourceScope,
  storeId: string,
  input: OfficialConnectionCallback,
) {
  const body = officialConnectionCompleteSchema.parse(input);
  return request(
    scope,
    storeId,
    "complete",
    officialConnectionViewSchema.refine(
      (view) => view.attemptId === body.attemptId,
    ),
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
  );
}
export function queryStoreConnection(
  scope: MemberResourceScope,
  storeId: string,
  attemptId: string,
) {
  return request(
    scope,
    storeId,
    "query",
    officialConnectionViewSchema.refine((view) => view.attemptId === attemptId),
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(officialConnectionQuerySchema.parse({ attemptId })),
    },
  );
}
export function disconnectStoreConnection(
  scope: MemberResourceScope,
  storeId: string,
  version: number,
  key: string,
) {
  z.uuid().parse(key);
  return request(scope, storeId, "disconnect", officialConnectionViewSchema, {
    method: "POST",
    headers: { "If-Match": `"${version}"`, "Idempotency-Key": key },
  });
}
const pendingSchema = z
  .object({
    expectedUserId: memberResourceID,
    expectedOrganizationId: memberResourceID,
    storeId: z.uuid(),
    attemptId: z.uuid(),
    expiresAt: z.string().datetime({ precision: null }),
  })
  .strict();
const pendingKey = "listingkit.shein.pending";
// Contains local routing metadata only. Consent state, tempToken, merchant
// credentials and authorization URL never enter browser persistence.
export function rememberStoreAuthorization(
  value: z.infer<typeof pendingSchema>,
) {
  sessionStorage.setItem(
    pendingKey,
    JSON.stringify(pendingSchema.parse(value)),
  );
}
export function readStoreAuthorization() {
  try {
    return pendingSchema.parse(
      JSON.parse(sessionStorage.getItem(pendingKey) ?? "null"),
    );
  } catch {
    return null;
  }
}
export function clearStoreAuthorization() {
  sessionStorage.removeItem(pendingKey);
}

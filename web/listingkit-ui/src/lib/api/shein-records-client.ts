import { InvalidStrictJSONResponseError, readBoundedStrictJSON } from "./strict-json-response";
import { parseSheinRecordList, parseSheinRecordListFailure, sheinRecordCursorSchema, sheinRecordLimitSchema, sheinRecordOrganizationSchema, type SheinRecordList, type SheinRecordListFailure } from "./shein-records";

const MAX_BYTES = 128 * 1024;
export class SheinRecordListError extends Error {
  constructor(public readonly status: number, public readonly code: string, public readonly payload: SheinRecordListFailure) {
    super(`SHEIN record list request failed (${code})`);
    this.name = "SheinRecordListError";
  }
}
const invalidResponse = () => new SheinRecordListError(502, "INVALID_UPSTREAM_RESPONSE", { code: "INVALID_UPSTREAM_RESPONSE", message: "SHEIN record list response is invalid", requestId: "", fieldErrors: [] });

export async function fetchSheinRecords(input: { organizationId: string; limit?: number; cursor?: string; signal?: AbortSignal }): Promise<SheinRecordList> {
  const limit = input.limit ?? 20;
  if (!sheinRecordOrganizationSchema.safeParse(input.organizationId).success || !sheinRecordLimitSchema.safeParse(limit).success || (input.cursor !== undefined && !sheinRecordCursorSchema.safeParse(input.cursor).success)) {
    throw new SheinRecordListError(400, "invalid_request", { error: "invalid_request" });
  }
  const query = new URLSearchParams({ limit: String(limit) });
  if (input.cursor !== undefined) query.set("cursor", input.cursor);
  const response = await fetch(`/api/listing/shein-records?${query}`, {
    method: "GET", credentials: "same-origin", cache: "no-store", redirect: "error",
    headers: { Accept: "application/json", "X-Expected-Organization-ID": input.organizationId },
    ...(input.signal ? { signal: input.signal } : {}),
  });
  let payload: unknown;
  try { payload = await readBoundedStrictJSON(response, MAX_BYTES, input.signal); }
  catch (error) {
    input.signal?.throwIfAborted();
    if (error instanceof InvalidStrictJSONResponseError) throw invalidResponse();
    throw error;
  }
  if (response.status === 200) {
    const parsed = parseSheinRecordList(payload);
    if (!parsed) throw invalidResponse();
    return parsed;
  }
  const failure = parseSheinRecordListFailure(payload, response.status);
  if (!failure) throw invalidResponse();
  throw new SheinRecordListError(response.status, "error" in failure ? failure.error : failure.code, failure);
}

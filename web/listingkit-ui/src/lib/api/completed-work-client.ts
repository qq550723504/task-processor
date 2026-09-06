import { InvalidStrictJSONResponseError, readBoundedStrictJSON } from "./strict-json-response";
import { parseSheinRecordListFailure, sheinRecordCursorSchema, sheinRecordLimitSchema, sheinRecordOrganizationSchema } from "./shein-records";
import { SheinRecordListError } from "./shein-records-client";
import { parseCompletedWorkList, type CompletedWorkList, type CompletedWorkFailure } from "./completed-work";

export { SheinRecordListError as CompletedWorkError } from "./shein-records-client";
const invalidResponse = () => new SheinRecordListError(502, "INVALID_UPSTREAM_RESPONSE", { code: "INVALID_UPSTREAM_RESPONSE", message: "Completed work response is invalid", requestId: "", fieldErrors: [] });

export async function fetchCompletedWork(input: { organizationId: string; limit?: number; cursor?: string; signal?: AbortSignal }): Promise<CompletedWorkList> {
  const limit = input.limit ?? 20;
  if (!sheinRecordOrganizationSchema.safeParse(input.organizationId).success || !sheinRecordLimitSchema.safeParse(limit).success || (input.cursor !== undefined && !sheinRecordCursorSchema.safeParse(input.cursor).success)) {
    throw new SheinRecordListError(400, "invalid_request", { error: "invalid_request" });
  }
  input.signal?.throwIfAborted();
  const query = new URLSearchParams({ source: "listing-local-preparation", limit: String(limit) });
  if (input.cursor !== undefined) query.set("cursor", input.cursor);
  const response = await fetch(`/api/workbench/completed-work?${query}`, {
    method: "GET", credentials: "same-origin", cache: "no-store", redirect: "error",
    headers: { Accept: "application/json", "X-Expected-Organization-ID": input.organizationId },
    ...(input.signal ? { signal: input.signal } : {}),
  });
  let payload: unknown;
  try { payload = await readBoundedStrictJSON(response, 128 * 1024, input.signal); }
  catch (error) {
    input.signal?.throwIfAborted();
    if (error instanceof InvalidStrictJSONResponseError) throw invalidResponse();
    throw error;
  }
  input.signal?.throwIfAborted();
  if (response.status === 200) {
    const parsed = parseCompletedWorkList(payload);
    if (!parsed) throw invalidResponse();
    return parsed;
  }
  const failure: CompletedWorkFailure | null = parseSheinRecordListFailure(payload, response.status);
  if (!failure) throw invalidResponse();
  throw new SheinRecordListError(response.status, "error" in failure ? failure.error : failure.code, failure);
}

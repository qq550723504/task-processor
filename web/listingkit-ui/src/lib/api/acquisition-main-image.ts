import { AcquisitionAPIError, type AcquisitionContext } from "./product-acquisition";
import { acquisitionErrorStatuses, isAcquisitionUUID } from "../contracts/product-acquisition";
import { mainImageAcceptedSchema, mainImageApprovalRequestSchema, mainImageCandidatesSchema, mainImageResultSchema, mainImageStartRequestSchema, type MainImageCandidate, type MainImageResult } from "../contracts/acquisition-main-image";

const safeContextID = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/;
const errorStatuses: Readonly<Record<string, number>> = {
  ...acquisitionErrorStatuses,
  INVALID_IMAGE_REQUEST: 400, INVALID_REQUEST: 400, FORBIDDEN: 403, IMAGE_NOT_FOUND: 404,
  IMAGE_CONFLICT: 409, IMAGE_BLOCKED: 409, IMAGE_UNAVAILABLE: 503, OUTCOME_UNKNOWN: 503,
  IDENTITY_CONTEXT_CHANGED: 409, ORGANIZATION_CONTEXT_CHANGED: 409,
};
type Approval = { planRevision: number; resultDigest: string; actionId: string };

function base(operationId: string) {
  if (!isAcquisitionUUID(operationId)) throw new AcquisitionAPIError("INVALID_IMAGE_REQUEST", 400);
  return `/api/workbench/sourcing/1688/acquisitions/${operationId}/main-image`;
}

export function readMainImageCandidates(operationId: string, context: AcquisitionContext, signal?: AbortSignal): Promise<{ operationId: string; candidates: MainImageCandidate[] }> {
  return send(`${base(operationId)}/candidates`, "GET", context, undefined, undefined, mainImageCandidatesSchema, operationId, signal);
}
export function startMainImage(operationId: string, sourceImageId: string, key: string, context: AcquisitionContext, signal?: AbortSignal) {
  const input = mainImageStartRequestSchema.safeParse({ sourceImageId });
  if (!input.success || !isAcquisitionUUID(key)) throw new AcquisitionAPIError("INVALID_IMAGE_REQUEST", 400);
  return send(base(operationId), "POST", context, input.data, key, mainImageAcceptedSchema, undefined, signal);
}
export function readMainImage(operationId: string, runId: string, context: AcquisitionContext, signal?: AbortSignal): Promise<MainImageResult> {
  if (!isAcquisitionUUID(runId)) throw new AcquisitionAPIError("INVALID_IMAGE_REQUEST", 400);
  return send(`${base(operationId)}/runs/${runId}`, "GET", context, undefined, undefined, mainImageResultSchema, runId, signal);
}
export function approveMainImage(operationId: string, runId: string, approval: Approval, context: AcquisitionContext, signal?: AbortSignal) {
  if (!isAcquisitionUUID(runId)) throw new AcquisitionAPIError("INVALID_IMAGE_REQUEST", 400);
  const input = mainImageApprovalRequestSchema.safeParse(approval);
  if (!input.success) throw new AcquisitionAPIError("INVALID_IMAGE_REQUEST", 400);
  return send(`${base(operationId)}/runs/${runId}/approve`, "POST", context, input.data, undefined, mainImageAcceptedSchema, runId, signal);
}

// A caller retains its original Start key or approval action ID after an
// uncertain response. This transport never allocates an ID or retries.
async function send<T>(path: string, method: "GET" | "POST", context: AcquisitionContext, input: object | undefined, key: string | undefined, schema: { safeParse: (value: unknown) => { success: true; data: T } | { success: false } }, expectedID: string | undefined, signal?: AbortSignal): Promise<T> {
  if (!safeContextID.test(context.userId) || !safeContextID.test(context.organizationId)) throw new AcquisitionAPIError("INVALID_IMAGE_REQUEST", 400);
  if (signal?.aborted) throw new AcquisitionAPIError("DEADLINE_EXCEEDED", 504);
  const controller = new AbortController();
  const abort = () => controller.abort();
  signal?.addEventListener("abort", abort, { once: true });
  const timeout = setTimeout(abort, 25_000);
  const uncertain = () => new AcquisitionAPIError(method === "POST" ? "OUTCOME_UNKNOWN" : "IMAGE_UNAVAILABLE", method === "POST" ? 503 : 502);
  try {
    const headers = new Headers({ Accept: "application/json", "X-Expected-Organization-ID": context.organizationId, "X-Expected-User-ID": context.userId });
    if (input !== undefined) headers.set("Content-Type", "application/json");
    if (key) headers.set("Idempotency-Key", key);
    const response = await fetch(path, { method, headers, body: input === undefined ? undefined : JSON.stringify(input), credentials: "same-origin", cache: "no-store", redirect: "error", signal: controller.signal });
    if (!/^application\/json(?:\s*;|$)/i.test(response.headers.get("content-type") ?? "")) throw uncertain();
    const payload = await boundedJSON(response, controller.signal);
    if (!response.ok) {
      const code = payload && typeof payload === "object" && "code" in payload && typeof payload.code === "string" ? payload.code : "";
      if (errorStatuses[code] === response.status) throw new AcquisitionAPIError(code, response.status);
      throw uncertain();
    }
    const parsed = schema.safeParse(payload);
    if (!parsed.success || response.status !== (method === "POST" ? 202 : 200)) throw uncertain();
    const data = parsed.data;
    if (expectedID && typeof data === "object" && data &&
      (("operationId" in data && data.operationId !== expectedID) || ("runId" in data && data.runId !== expectedID))) throw uncertain();
    return data;
  } catch (error) {
    if (error instanceof AcquisitionAPIError) throw error;
    throw uncertain();
  } finally {
    clearTimeout(timeout);
    signal?.removeEventListener("abort", abort);
  }
}

async function boundedJSON(response: Response, signal: AbortSignal): Promise<unknown> {
  const maximum = 128 * 1024;
  const declared = Number(response.headers.get("content-length") ?? 0);
  if (declared > maximum) { await response.body?.cancel(); throw new Error("oversize"); }
  const reader = response.body?.getReader();
  if (!reader) throw new Error("empty response");
  const cancel = () => void reader.cancel().catch(() => undefined);
  signal.addEventListener("abort", cancel, { once: true });
  const chunks: Uint8Array[] = []; let length = 0;
  try {
    while (true) {
      signal.throwIfAborted();
      const next = await reader.read();
      signal.throwIfAborted();
      if (next.done) break;
      length += next.value.length;
      if (length > maximum) { await reader.cancel(); throw new Error("oversize"); }
      chunks.push(next.value);
    }
  } finally { signal.removeEventListener("abort", cancel); reader.releaseLock(); }
  const bytes = new Uint8Array(length); let offset = 0;
  for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.length; }
  return JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes));
}

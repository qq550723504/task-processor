import { BROWSER_CAPTURE_BASE, BROWSER_CAPTURE_MAX_BYTES, browserCaptureSchema } from "@/lib/contracts/browser-capture";
import { acquisitionErrorStatuses, acquisitionResultSchema, ACQUISITION_RESPONSE_MAX_BYTES, isAcquisitionUUID, type AcquisitionResult } from "@/lib/contracts/product-acquisition";
import { WorkbenchContextError } from "./workbench-context";
import type { AcquisitionContext } from "./product-acquisition";
export type BrowserCaptureIntent = AcquisitionContext & Readonly<{ key: string; body: string }>;
const error = (code: string, status = acquisitionErrorStatuses[code] ?? 503) => new WorkbenchContextError(status, code, "", []);
export function capture1688(intent: BrowserCaptureIntent, signal?: AbortSignal) { return sendIntent(intent, "", signal); }
export function verifyBrowserCapture(intent: BrowserCaptureIntent, signal?: AbortSignal) { return sendIntent(intent, "/verify", signal); }
function sendIntent(intent: BrowserCaptureIntent, suffix: string, signal?: AbortSignal) {
  let payload;
  try {
    if (new TextEncoder().encode(intent.body).byteLength > BROWSER_CAPTURE_MAX_BYTES) throw error("SOURCE_TOO_LARGE");
    payload = browserCaptureSchema.parse(JSON.parse(intent.body));
  } catch { return Promise.reject(error("INVALID_ACQUISITION")); }
  if (!isAcquisitionUUID(intent.key)) return Promise.reject(error("INVALID_ACQUISITION"));
  return send(intent, suffix, "POST", signal, intent, undefined, `crawler:1688:${payload.evidence.offerID}`);
}
export function readBrowserCaptureByKey(context: AcquisitionContext, key: string, signal?: AbortSignal) {
  if (!isAcquisitionUUID(key)) return Promise.reject(error("INVALID_ACQUISITION"));
  return send(context, `/by-key/${key}`, "GET", signal);
}
export function readBrowserCapture(context: AcquisitionContext, operationId: string, signal?: AbortSignal) {
  if (!isAcquisitionUUID(operationId)) return Promise.reject(error("INVALID_ACQUISITION"));
  return send(context, `/${operationId}`, "GET", signal, undefined, operationId);
}
async function send(context: AcquisitionContext, suffix: string, method: "GET" | "POST", signal?: AbortSignal, intent?: BrowserCaptureIntent, expectedId?: string, expectedProduct?: string): Promise<AcquisitionResult> {
  if (![context.userId, context.organizationId].every((v) => /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(v))) throw error("INVALID_REQUEST");
  if (signal?.aborted) throw error("DEADLINE_EXCEEDED");
  const controller = new AbortController(); const abort = () => controller.abort();
  signal?.addEventListener("abort", abort, { once: true });
  const timeout = setTimeout(abort, 25_000);
  const unavailable = () => error(method === "POST" ? "OUTCOME_UNKNOWN" : "ACQUISITION_UNAVAILABLE");
  try {
    const headers = new Headers({ Accept: "application/json", "X-Expected-Organization-ID": context.organizationId, "X-Expected-User-ID": context.userId });
    if (intent) { headers.set("Content-Type", "application/json"); headers.set("Idempotency-Key", intent.key); }
    const response = await fetch(`${BROWSER_CAPTURE_BASE}${suffix}`, { method, headers, body: intent?.body, credentials: "same-origin", cache: "no-store", redirect: "error", signal: controller.signal });
    const raw = await boundedJSON(response, controller.signal);
    if (!response.ok) {
      const code = raw && typeof raw === "object" && "code" in raw ? raw.code : undefined;
      if (typeof code === "string" && acquisitionErrorStatuses[code] === response.status) throw error(code, response.status);
      throw unavailable();
    }
    const parsed = acquisitionResultSchema.safeParse(raw);
    if (response.status !== 200 || !parsed.success || (expectedId && parsed.data.operationId !== expectedId) || (expectedProduct && parsed.data.productKey && parsed.data.productKey !== expectedProduct)) throw unavailable();
    return parsed.data;
  } catch (failure) {
    if (failure instanceof WorkbenchContextError) throw failure;
    throw unavailable();
  } finally { clearTimeout(timeout); signal?.removeEventListener("abort", abort); }
}
async function boundedJSON(response: Response, signal: AbortSignal): Promise<unknown> {
  if (!/^application\/json(?:\s*;|$)/i.test(response.headers.get("content-type") ?? "") || Number(response.headers.get("content-length")) > ACQUISITION_RESPONSE_MAX_BYTES) {
    void response.body?.cancel().catch(() => undefined); throw new Error("Invalid response");
  }
  const reader = response.body?.getReader(); if (!reader) throw new Error("Missing response");
  const cancel = () => void reader.cancel().catch(() => undefined);
  signal.addEventListener("abort", cancel, { once: true });
  const chunks: Uint8Array[] = []; let count = 0;
  try {
    while (true) {
      signal.throwIfAborted(); const { done, value } = await reader.read(); signal.throwIfAborted();
      if (done) break;
      count += value.byteLength;
      if (count > ACQUISITION_RESPONSE_MAX_BYTES) { cancel(); throw new Error("Response too large"); }
      chunks.push(value);
    }
    const bytes = new Uint8Array(count); let offset = 0;
    for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
    return JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes));
  } finally { signal.removeEventListener("abort", cancel); reader.releaseLock(); }
}

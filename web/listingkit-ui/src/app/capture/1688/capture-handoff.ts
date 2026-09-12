import { z } from "zod";
import { browserCaptureSchema, type BrowserCapturePayload } from "@/lib/contracts/browser-capture";
import { isAcquisitionUUID } from "@/lib/contracts/product-acquisition";

export type CaptureHandoff = Readonly<{ kind: "handoff"; extensionId: string; handoffId: string; key: string }>;
export type CaptureEntry = CaptureHandoff | Readonly<{ kind: "recovery"; key: string }>;
export function parseCaptureEntry(rawURL: string): CaptureEntry | null {
  try {
    const url = new URL(rawURL);
    if (!["http:", "https:"].includes(url.protocol) || url.username || url.password || url.pathname !== "/capture/1688" || url.search || rawURL.split("#")[0]!.endsWith("?")) return null;
    const params = new URLSearchParams(url.hash.slice(1));
    const names = [...params.keys()];
    if (new Set(names).size !== names.length) return null;
    const recovery = params.get("operationKey");
    if (names.length === 1 && recovery && isAcquisitionUUID(recovery)) return { kind: "recovery", key: recovery };
    if (names.length !== 3 || !names.every((name) => ["extensionId", "handoffId", "idempotencyKey"].includes(name))) return null;
    const extensionId = params.get("extensionId") ?? "", handoffId = params.get("handoffId") ?? "", key = params.get("idempotencyKey") ?? "";
    return /^[a-p]{32}$/.test(extensionId) && isAcquisitionUUID(handoffId) && isAcquisitionUUID(key) ? { kind: "handoff", extensionId, handoffId, key } : null;
  } catch { return null; }
}
type ExternalRuntime = { lastError?: unknown; sendMessage: (id: string, message: unknown, callback: (response: unknown) => void) => void };
function externalMessage(entry: CaptureHandoff, message: unknown, signal?: AbortSignal): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const runtime = (globalThis as typeof globalThis & { chrome?: { runtime?: ExternalRuntime } }).chrome?.runtime;
    if (!runtime || signal?.aborted) { reject(new Error("Handoff unavailable")); return; }
    let settled = false;
    const finish = (ok: boolean, value?: unknown) => {
      if (settled) return; settled = true; clearTimeout(timer); signal?.removeEventListener("abort", abort);
      if (ok) resolve(value); else reject(new Error("Handoff unavailable"));
    };
    const abort = () => finish(false);
    const timer = setTimeout(abort, 1_000);
    signal?.addEventListener("abort", abort, { once: true });
    try { runtime.sendMessage(entry.extensionId, message, (response) => finish(!runtime.lastError, response)); } catch { finish(false); }
  });
}
export async function readCaptureHandoff(entry: CaptureHandoff, signal: AbortSignal): Promise<BrowserCapturePayload> {
  const responseSchema = z.object({ version: z.literal(1), type: z.literal("capture.payload"), handoffId: z.literal(entry.handoffId), idempotencyKey: z.literal(entry.key), payload: browserCaptureSchema }).strict();
  for (let attempt = 0; attempt < 3; attempt++) {
    signal.throwIfAborted();
    try {
      const response = await externalMessage(entry, { version: 1, type: "capture.read", handoffId: entry.handoffId, idempotencyKey: entry.key }, signal);
      const parsed = responseSchema.safeParse(response);
      if (parsed.success) return parsed.data.payload;
    } catch { signal.throwIfAborted(); }
    if (attempt < 2) await new Promise<void>((resolve) => setTimeout(resolve, 250));
  }
  throw new Error("Handoff unavailable");
}
export async function notifyCaptureStatus(entry: CaptureHandoff, outcome: "processing" | "published" | "failed" | "outcome_unknown", operationId?: string) {
  if (outcome === "published" && (!operationId || !isAcquisitionUUID(operationId))) return;
  try {
    await externalMessage(entry, { version: 1, type: "capture.status", handoffId: entry.handoffId, idempotencyKey: entry.key, outcome, ...(operationId ? { operationId } : {}) });
  } catch { /* Ephemeral worker status is never publication evidence. */ }
}

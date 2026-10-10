import { z } from "zod";
import { reportIntentSchema, reportResultSchema, type ReportIntent, type ReportScope } from "@/lib/contracts/report-center";
import { readBoundedStrictJSON } from "./strict-json-response";
export class ReportError extends Error { constructor(public code: string) { super(code); } }
export async function reportRequest<T>(scope: ReportScope, path: string, schema: z.ZodType<T>, signal?: AbortSignal, intent?: ReportIntent): Promise<T> {
  const write = !!intent; const headers = new Headers({ Accept: "application/json", "X-Expected-User-ID": scope.userId, "X-Expected-Organization-ID": scope.organizationId });
  if (write) { headers.set("Content-Type", "application/json"); headers.set("Idempotency-Key", intent.key); }
  const body = intent ? intent.operation === "save" ? { source: intent.source } : { favorite: intent.favorite } : undefined;
  let dispatched = false;
  try {
    const deadline = AbortSignal.timeout(15000), combined = signal ? AbortSignal.any([signal, deadline]) : deadline;
    combined.throwIfAborted(); dispatched = true;
    const response = await fetch(`/api/report-center/reports${path}`, { method: write ? "POST" : "GET", credentials: "same-origin", cache: "no-store", redirect: "error", headers, signal: combined, body: body ? JSON.stringify(body) : undefined });
    const payload = await readBoundedStrictJSON(response, 160 << 10, combined);
    if (response.status !== 200) { const failure = z.strictObject({ code: z.string().regex(/^[A-Z_]{1,80}$/) }).safeParse(payload); throw new ReportError(write && response.status >= 500 ? "OUTCOME_UNKNOWN" : failure.success ? failure.data.code : write ? "OUTCOME_UNKNOWN" : "DEPENDENCY_UNAVAILABLE"); }
    const result = schema.safeParse(payload); if (!result.success) throw new ReportError(write ? "OUTCOME_UNKNOWN" : "INVALID_UPSTREAM_RESPONSE");
    if (intent) { const receipt = reportResultSchema.parse(result.data); if (receipt.commandId !== intent.key || (intent.operation === "save" ? JSON.stringify(receipt.report.ref) !== JSON.stringify(intent.source) : receipt.report.id !== intent.id)) throw new ReportError("OUTCOME_UNKNOWN"); }
    return result.data;
  } catch (e) { if (e instanceof ReportError) throw e; throw new ReportError(write && dispatched ? "OUTCOME_UNKNOWN" : "DEPENDENCY_UNAVAILABLE"); }
}
const storageKey = (scope: ReportScope) => `personal-report-intent:${scope.userId}:${scope.organizationId}`;
export function readReportIntent(scope: ReportScope): ReportIntent | null {
  try { const raw = sessionStorage.getItem(storageKey(scope)); if (!raw) return null; return reportIntentSchema.parse(JSON.parse(raw)); } catch { throw new ReportError("INTENT_STORAGE_UNAVAILABLE"); }
}
export function writeReportIntent(scope: ReportScope, intent: ReportIntent | null) {
  try { if (intent) sessionStorage.setItem(storageKey(scope), JSON.stringify(reportIntentSchema.parse(intent))); else sessionStorage.removeItem(storageKey(scope)); } catch { throw new ReportError("INTENT_STORAGE_UNAVAILABLE"); }
}

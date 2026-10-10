import { z } from "zod";
import { reportEndpoint, reportID, reportResultSchema, sourceRef } from "@/lib/contracts/report-center";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { hasTrustedSameOriginWrite } from "./same-origin-write";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
const safe = { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" };
export const reportFailure = (status: number, code: string) => Response.json({ code }, { status, headers: safe });
export async function proxyReportCenter(request: Request, token: string, userId: string): Promise<Response> {
  if (!token || !userId) return reportFailure(401, "AUTHENTICATION_REQUIRED");
  if (request.headers.get("X-Expected-User-ID") !== userId) return reportFailure(409, "IDENTITY_CONTEXT_CHANGED");
  const url = new URL(request.url), route = reportEndpoint(url, request.method);
  if (!route || request.url.endsWith("?") || request.headers.has("content-encoding")) return reportFailure(400, "INVALID_REQUEST");
  const write = !!route.input;
  if (write && !hasTrustedSameOriginWrite(request)) return reportFailure(403, "FORBIDDEN");
  if (!write && (request.body || request.headers.has("transfer-encoding") || request.headers.has("content-length") && request.headers.get("content-length") !== "0")) return reportFailure(400, "INVALID_REQUEST");
  const cookies = (request.headers.get("cookie") ?? "").split(";").map(v => v.trim()).filter(v => v.startsWith(WORKBENCH_COOKIE_NAME + "="));
  let org = "";
  try { if (cookies.length === 1) org = decodeURIComponent(cookies[0].slice(WORKBENCH_COOKIE_NAME.length + 1)); } catch {}
  if (!/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(org) || org !== request.headers.get("X-Expected-Organization-ID")) return reportFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
  const controller = new AbortController(), abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true }); if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 12000); let dispatched = false;
  try {
    const headers = new Headers({ Accept: "application/json", Authorization: `Bearer ${token}`, "X-Requested-Organization-ID": org });
    let body: string | undefined, payload: unknown;
    if (write) {
      const key = request.headers.get("Idempotency-Key"); if (!reportID.safeParse(key).success) return reportFailure(400, "INVALID_REQUEST");
      payload = await readBoundedStrictJSON(new Response(request.body, { headers: { "Content-Type": request.headers.get("Content-Type") ?? "" } }), 2048, controller.signal);
      const checked = route.input!.safeParse(payload); if (!checked.success) return reportFailure(400, "INVALID_REQUEST");
      body = JSON.stringify(checked.data); headers.set("Idempotency-Key", key!); headers.set("Content-Type", "application/json");
    }
    const base = new URL(process.env.LISTINGKIT_SERVICE_API_BASE ?? "");
    if (!["http:", "https:"].includes(base.protocol) || base.username || base.password || base.search || base.hash || !["/api/v1", "/api/v1/"].includes(base.pathname)) return reportFailure(503, "DEPENDENCY_UNAVAILABLE");
    controller.signal.throwIfAborted(); dispatched = true;
    const response = await fetch(base.origin + "/api/v1/workbench/reports" + route.suffix + url.search, { method: request.method, headers, body, signal: controller.signal, cache: "no-store", redirect: "manual" });
    const raw = await readBoundedStrictJSON(response, 160 << 10, controller.signal); controller.signal.throwIfAborted();
    if (response.status !== 200) {
      const failure = z.strictObject({ code: z.string().regex(/^[A-Z_]{1,80}$/) }).safeParse(raw);
      return reportFailure(response.status >= 400 && response.status < 600 ? response.status : 502, write && response.status >= 500 || !failure.success && write ? "OUTCOME_UNKNOWN" : failure.success ? failure.data.code : "INVALID_UPSTREAM_RESPONSE");
    }
    const checked = route.output.safeParse(raw); if (!checked.success) throw new Error();
    if (write) {
      const result = reportResultSchema.parse(checked.data);
      if (result.commandId !== request.headers.get("Idempotency-Key")) throw new Error();
      if (route.operation === "save") { const ref = sourceRef.parse((payload as { source: unknown }).source); if (JSON.stringify(ref) !== JSON.stringify(result.report.ref)) throw new Error(); }
      else if (result.report.id !== route.suffix.split("/")[1]) throw new Error();
    } else if (route.suffix.startsWith("/sources/")) {
      const ref = (checked.data as { ref: { kind: string; id: string } }).ref; if (`/sources/${ref.kind}/${ref.id}` !== route.suffix) throw new Error();
    } else if (/^\/[0-9a-f-]{36}$/.test(route.suffix) && (!('id' in checked.data) || checked.data.id !== route.suffix.slice(1))) throw new Error();
    return Response.json(checked.data, { headers: safe });
  } catch { return reportFailure(controller.signal.aborted ? 504 : dispatched ? 502 : 400, write && dispatched ? "OUTCOME_UNKNOWN" : dispatched ? "DEPENDENCY_UNAVAILABLE" : "INVALID_REQUEST"); }
  finally { clearTimeout(timer); request.signal.removeEventListener("abort", abort); }
}

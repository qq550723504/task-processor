import { z } from "zod";
import { cockpitEndpoint, cockpitID } from "@/lib/contracts/operations-cockpit";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { hasTrustedSameOriginWrite } from "./same-origin-write";
import { hasEmptyBody } from "./members-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
const safe = { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" };
export const cockpitFailure = (status: number, code: string) => Response.json({ code }, { status, headers: safe });
export async function proxyCockpit(request: Request, token: string, userId: string): Promise<Response> {
 if (!token || !userId) return cockpitFailure(401, "AUTHENTICATION_REQUIRED");
 if (request.headers.get("X-Expected-User-ID") !== userId) return cockpitFailure(409, "IDENTITY_CONTEXT_CHANGED");
 const url = new URL(request.url), prefix = "/api/operations-cockpit/", path = url.pathname.startsWith(prefix) ? url.pathname.slice(prefix.length) : "", route = cockpitEndpoint(path, request.method), write = request.method === "POST";
 if (!route || url.search.length > 16384 || request.url.endsWith("?") || request.headers.has("content-encoding")) return cockpitFailure(400, "INVALID_REQUEST");
 let org = "";
 const cookies = (request.headers.get("cookie") ?? "").split(";").map(v => v.trim()).filter(v => v.startsWith(WORKBENCH_COOKIE_NAME + "="));
 try { if (cookies.length === 1) org = decodeURIComponent(cookies[0].slice(WORKBENCH_COOKIE_NAME.length + 1)); } catch {}
 if (!/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(org) || org !== request.headers.get("X-Expected-Organization-ID")) return cockpitFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
 if (write && !hasTrustedSameOriginWrite(request)) return cockpitFailure(403, "FORBIDDEN");
 const controller = new AbortController(), abort = () => controller.abort();
 request.signal.addEventListener("abort", abort, { once: true }); if (request.signal.aborted) abort();
 const timer = setTimeout(abort, 12000); let dispatched = false;
 try {
  const allowed = write || ["capabilities", "goals/head"].includes(path) || /^facts\/[0-9a-f-]{36}$/.test(path) ? [] : path.endsWith("history") ? ["beforeRevision"] : path === "facts" ? ["storeId", "page"] : ["startDate", "endDate", "storeId", "page", "state", "search", "sort", "level", "kind"];
  for (const key of url.searchParams.keys()) if (!allowed.includes(key) || key !== "storeId" && url.searchParams.getAll(key).length !== 1) return cockpitFailure(400, "INVALID_REQUEST");
  const headers = new Headers({ Accept: "application/json", Authorization: "Bearer " + token, "X-Requested-Organization-ID": org });
  let body: string | undefined, intent: Record<string, unknown> | undefined;
  if (write) {
   const key = request.headers.get("Idempotency-Key"); if (!cockpitID.safeParse(key).success) return cockpitFailure(400, "INVALID_REQUEST");
   const input = await readBoundedStrictJSON(new Response(request.body, { headers: { "Content-Type": request.headers.get("Content-Type") ?? "" } }), 16384, controller.signal);
   const parsed = route.input!.safeParse(input); if (!parsed.success) return cockpitFailure(400, "INVALID_REQUEST");
   intent = parsed.data as Record<string, unknown>; body = JSON.stringify(intent);
   headers.set("Content-Type", "application/json"); headers.set("Idempotency-Key", key!);
  } else if (!(await hasEmptyBody(request, controller.signal))) return cockpitFailure(400, "INVALID_REQUEST");
  let origin: string;
  try { const upstream = new URL(process.env.LISTINGKIT_SERVICE_API_BASE ?? ""); if (!["http:", "https:"].includes(upstream.protocol) || upstream.username || upstream.password || upstream.search || upstream.hash || !["/api/v1", "/api/v1/"].includes(upstream.pathname)) throw new Error(); origin = upstream.origin; } catch { return cockpitFailure(503, "DEPENDENCY_UNAVAILABLE"); }
  controller.signal.throwIfAborted(); dispatched = true;
  const response = await fetch(origin + "/api/v1/workbench/operations-cockpit/" + path + url.search, { method: request.method, headers, body, signal: controller.signal, cache: "no-store", redirect: "manual" });
  const payload = await readBoundedStrictJSON(response, 1 << 20, controller.signal); controller.signal.throwIfAborted();
  if (response.status !== 200) {
   const error = z.strictObject({ code: z.string().regex(/^[A-Z_]{1,80}$/) }).safeParse(payload);
   if (!error.success) throw new Error();
   return cockpitFailure(response.status >= 400 && response.status < 600 ? response.status : 502, write && response.status >= 500 ? "OUTCOME_UNKNOWN" : error.data.code);
  }
  const parsed = route.output.safeParse(payload); if (!parsed.success) throw new Error();
  if (!write) {
   const parts=path.split("/");
   if(parts[0]==="facts" && cockpitID.safeParse(parts[1]).success) {
    const records=Array.isArray(parsed.data)?parsed.data:[parsed.data];
    if(records.some(v=>(v as {id:string}).id!==parts[1]))throw new Error();
   }
   if(parts[0]==="stores" && cockpitID.safeParse(parts[1]).success) {
    const detail=parsed.data as {store:{id:string};aggregate:{storeId:string}};
    if(detail.store.id!==parts[1]||detail.aggregate.storeId!==parts[1])throw new Error();
   }
   if(path==="facts") {const storeId=url.searchParams.get("storeId");if((parsed.data as {storeId:string}[]).some(v=>v.storeId!==storeId))throw new Error();}
  }
  if (write) {
   const receipt = parsed.data as { commandId: string; id: string; operation: string }, operation = path === "goals/restore" ? "goal_restore" : `${path === "facts" ? "fact" : "goal"}_${intent?.expectedRevision === "0" ? "create" : "update"}`;
   if (receipt.commandId !== request.headers.get("Idempotency-Key") || receipt.id !== intent?.id || receipt.operation !== operation) throw new Error();
  }
  return Response.json(parsed.data, { headers: safe });
 } catch { return cockpitFailure(controller.signal.aborted ? 504 : dispatched ? 502 : 400, write && dispatched ? "OUTCOME_UNKNOWN" : dispatched ? "DEPENDENCY_UNAVAILABLE" : "INVALID_REQUEST"); }
 finally { clearTimeout(timer); request.signal.removeEventListener("abort", abort); }
}

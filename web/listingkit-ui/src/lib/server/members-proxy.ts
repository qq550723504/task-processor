import { z } from "zod";
import { MemberError, invitationInput, memberErrorCode, memberId, parseMemberOperation, parseMembers, removeInput, roleInput } from "@/lib/api/members";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
import { hasTrustedSameOriginWrite } from "./same-origin-write";

const responseHeaders = { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" };
export function membersFailure(status: number, code: string) { return Response.json({ code, message: "Member request could not be completed", requestId: "", fieldErrors: [] }, { status, headers: responseHeaders }); }
function serviceOrigin(): string | null {
  try { const url = new URL(process.env.LISTINGKIT_SERVICE_API_BASE ?? ""); return ["http:", "https:"].includes(url.protocol) && !url.username && !url.password && !url.search && !url.hash && ["/api/v1", "/api/v1/"].includes(url.pathname) ? url.origin : null; } catch { return null; }
}

function endpoint(url: URL, method: string) {
  const base = "/api/account/"; if (!url.pathname.startsWith(base)) return null;
  const path = url.pathname.slice(base.length); const parts = path.split("/");
  if (method === "GET" && parts[0] === "members" && parts.length === 1) {
    if (url.search.length > 128) return null;
    for (const [key, value] of url.searchParams) {
      if (!["limit", "offset"].includes(key) || url.searchParams.getAll(key).length !== 1 || !/^\d+$/.test(value)) return null;
      const n = Number(value); if ((key === "limit" && (n < 1 || n > 100)) || (key === "offset" && (n < 0 || n > 10000))) return null;
    }
    return { path, operation: false, schema: null };
  }
  if (url.search) return null;
  if (method === "GET" && parts[0] === "members" && parts.length === 2 && memberId.safeParse(parts[1]).success) return { path, operation: false, schema: null };
  if (parts[0] === "member-operations" && z.string().uuid().safeParse(parts[1]).success && ((method === "GET" && parts.length === 2) || (method === "POST" && parts.length === 3 && parts[2] === "verify"))) return { path, operation: true, schema: null };
  if (method !== "POST" || parts[0] !== "members") return null;
  if (parts.length === 2 && parts[1] === "invitations") return { path, operation: true, schema: invitationInput };
  if (parts.length === 3 && memberId.safeParse(parts[1]).success) {
    if (parts[2] === "role") return { path, operation: true, schema: roleInput };
    if (parts[2] === "remove") return { path, operation: true, schema: removeInput };
  }
  return null;
}

export async function proxyMembers(request: Request, token: string, sessionUserId: string): Promise<Response> {
  if (!["GET", "POST"].includes(request.method)) return membersFailure(405, "INVALID_REQUEST");
  if (!token || !memberId.safeParse(sessionUserId).success) return membersFailure(401, "AUTHENTICATION_REQUIRED");
  if (request.headers.get("X-Expected-User-ID") !== sessionUserId) return membersFailure(409, "IDENTITY_CONTEXT_CHANGED");
  const url = new URL(request.url);
  if (request.method === "POST" && !hasTrustedSameOriginWrite(request)) return membersFailure(403, "PERMISSION_DENIED");
  const route = endpoint(url, request.method); if (!route || request.url.endsWith("?")) return membersFailure(400, "INVALID_REQUEST");
  const cookies = (request.headers.get("cookie") ?? "").split(";").map(value => value.trim()).filter(value => value.startsWith(`${WORKBENCH_COOKIE_NAME}=`));
  if (cookies.length === 0) return membersFailure(409, "ORGANIZATION_SELECTION_REQUIRED");
  if (cookies.length !== 1) return membersFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
  let organization: string;
  try { organization = decodeURIComponent(cookies[0].slice(WORKBENCH_COOKIE_NAME.length + 1)); } catch { return membersFailure(409, "ORGANIZATION_CONTEXT_CHANGED"); }
  if (!memberId.safeParse(organization).success || organization !== request.headers.get("X-Expected-Organization-ID")) return membersFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
  const origin = serviceOrigin(); if (!origin) return membersFailure(503, "ACCOUNT_NOT_CONFIGURED");
  const controller = new AbortController(); const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true }); if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    let body: string | undefined;
    const headers = new Headers({ Accept: "application/json", Authorization: `Bearer ${token}`, "X-Requested-Organization-ID": organization });
    if (route.schema) {
      const key = request.headers.get("idempotency-key");
      if (!z.string().uuid().safeParse(key).success || request.headers.get("content-type")?.split(";")[0].trim() !== "application/json") return membersFailure(400, "INVALID_REQUEST");
      let input: unknown;
      try { input = await readBoundedStrictJSON(new Response(request.body, { headers: { "Content-Type": "application/json" } }), 16384, controller.signal); } catch { return controller.signal.aborted ? membersFailure(504, "DEADLINE_EXCEEDED") : membersFailure(400, "INVALID_REQUEST"); }
      const parsed = route.schema.safeParse(input); if (!parsed.success) return membersFailure(400, "INVALID_REQUEST");
      body = JSON.stringify(parsed.data); headers.set("Content-Type", "application/json"); headers.set("Idempotency-Key", key!);
    } else if (!(await hasEmptyBody(request, controller.signal))) {
      return membersFailure(400, "INVALID_REQUEST");
    }
    controller.signal.throwIfAborted();
    const response = await fetch(`${origin}/api/v1/account/${route.path}${url.search}`, { method: request.method, headers, body, signal: controller.signal, cache: "no-store", redirect: "manual" });
    let payload: unknown;
    try { payload = await readBoundedStrictJSON(response, 1024 * 1024, controller.signal); } catch { throw new MemberError(502, "INVALID_UPSTREAM_RESPONSE"); }
    controller.signal.throwIfAborted();
    if (response.status !== 200) return membersFailure(response.status, memberErrorCode(response.status, payload));
    const result = route.operation ? parseMemberOperation(payload) : parseMembers(payload);
    if (result.userId !== sessionUserId) return membersFailure(409, "IDENTITY_CONTEXT_CHANGED");
    if (result.organizationId !== organization) return membersFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
    return Response.json(result, { headers: responseHeaders });
  } catch (error) {
    if (controller.signal.aborted) return membersFailure(504, "DEADLINE_EXCEEDED");
    return error instanceof MemberError ? membersFailure(error.status, error.code) : membersFailure(502, "DEPENDENCY_UNAVAILABLE");
  } finally { clearTimeout(timer); request.signal.removeEventListener("abort", abort); }
}

// Node/Next exposes a closed stream even for a zero-byte POST. Inspect actual
// bytes; Content-Length alone is not proof of an empty verification request.
async function hasEmptyBody(request: Request, signal: AbortSignal): Promise<boolean> {
  const reader = request.body?.getReader();
  if (!reader) return !request.headers.has("transfer-encoding") && (!request.headers.has("content-length") || request.headers.get("content-length") === "0");
  const cancel = () => { void reader.cancel().catch(() => undefined); };
  signal.addEventListener("abort", cancel, {once:true});
  try {
    while (true) {
      signal.throwIfAborted(); const chunk = await reader.read(); signal.throwIfAborted();
      if (chunk.done) return true;
      if (chunk.value.byteLength) { cancel(); return false; }
    }
  } finally { signal.removeEventListener("abort",cancel); reader.releaseLock(); }
}

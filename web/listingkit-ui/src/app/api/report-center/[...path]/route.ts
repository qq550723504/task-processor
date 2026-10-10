import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "@/lib/server/zitadel-auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";
import { proxyReportCenter, reportFailure } from "@/lib/server/report-center-proxy";
const authenticated = serverAuth(async (request: NextRequest & { auth?: unknown }) => proxyReportCenter(request, readZitadelServerAccessToken(request.auth as never), String(readZitadelIdentityFromSession(request.auth as never)?.userId ?? "")));
async function handle(request: NextRequest) {
  const controller = new AbortController(), abort = () => controller.abort();
  const failure = () => reportFailure(504, request.method === "GET" ? "DEPENDENCY_UNAVAILABLE" : "OUTCOME_UNKNOWN");
  request.signal.addEventListener("abort", abort, { once: true }); if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 15000); let finish = () => {};
  const ended = new Promise<Response>(resolve => { finish = () => resolve(failure()); controller.signal.addEventListener("abort", finish, { once: true }); if (controller.signal.aborted) finish(); });
  try { return await Promise.race([authenticated(new NextRequest(request, { signal: controller.signal }), { params: Promise.resolve({}) }), ended]) ?? reportFailure(503, "DEPENDENCY_UNAVAILABLE"); }
  catch { return reportFailure(503, request.method === "GET" ? "DEPENDENCY_UNAVAILABLE" : "OUTCOME_UNKNOWN"); }
  finally { clearTimeout(timer); request.signal.removeEventListener("abort", abort); controller.signal.removeEventListener("abort", finish); }
}
export const GET = handle, POST = handle;
export const PUT = () => reportFailure(405, "INVALID_REQUEST");
export const PATCH = PUT, DELETE = PUT, OPTIONS = PUT, HEAD = PUT;

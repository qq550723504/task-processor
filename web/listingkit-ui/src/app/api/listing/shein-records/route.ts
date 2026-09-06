import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { proxySheinRecords } from "@/lib/server/shein-records-proxy";
import { workbenchProtocolError } from "@/lib/server/workbench-proxy";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";

export const dynamic = "force-dynamic";
const deadlineResponse = () => workbenchProtocolError(504, "DEADLINE_EXCEEDED", "SHEIN record list request ended before completion");
const authenticatedGET = serverAuth(async (request: NextRequest & { auth?: unknown }) => {
  if (request.signal.aborted) return deadlineResponse();
  return proxySheinRecords(request, readZitadelServerAccessToken(request.auth as never));
});

export async function GET(request: NextRequest) {
  if (request.signal.aborted) return deadlineResponse();
  const controller = new AbortController();
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  const timeout = setTimeout(abort, 15000);
  let endResponse: () => void = () => undefined;
  const ended = new Promise<Response>((resolve) => { endResponse = () => resolve(deadlineResponse()); controller.signal.addEventListener("abort", endResponse, { once: true }); });
  try {
    const scoped = new NextRequest(request, { signal: controller.signal });
    const response = await Promise.race([authenticatedGET(scoped, { params: Promise.resolve({}) }), ended]);
    return response ?? workbenchProtocolError(503, "DEPENDENCY_UNAVAILABLE", "Authentication is unavailable");
  } catch {
    return controller.signal.aborted ? deadlineResponse() : workbenchProtocolError(503, "DEPENDENCY_UNAVAILABLE", "Authentication is unavailable");
  } finally {
    clearTimeout(timeout); request.signal.removeEventListener("abort", abort); controller.signal.removeEventListener("abort", endResponse);
  }
}
const rejectMethod = () => workbenchProtocolError(405, "INVALID_REQUEST", "Method is not allowed");
export const POST = rejectMethod;
export const PUT = rejectMethod;
export const PATCH = rejectMethod;
export const DELETE = rejectMethod;
export const HEAD = rejectMethod;
export const OPTIONS = rejectMethod;

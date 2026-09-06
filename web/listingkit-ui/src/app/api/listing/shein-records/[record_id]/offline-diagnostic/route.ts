import type { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { proxySheinDiagnostic } from "@/lib/server/shein-diagnostic-proxy";
import { workbenchProtocolError } from "@/lib/server/workbench-proxy";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";

export const dynamic = "force-dynamic";

const authenticatedGET = serverAuth(async (request: NextRequest & { auth?: unknown }, context: { params: Promise<{ record_id: string }> }) => {
  const { record_id } = await context.params;
  return proxySheinDiagnostic(request, record_id, readZitadelServerAccessToken(request.auth as never));
});
export const GET = authenticatedGET;
const rejectMethod = () => workbenchProtocolError(405, "INVALID_REQUEST", "Method is not allowed");
export const POST = rejectMethod;
export const PUT = rejectMethod;
export const PATCH = rejectMethod;
export const DELETE = rejectMethod;
export const HEAD = rejectMethod;
export const OPTIONS = rejectMethod;

import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "@/lib/server/zitadel-auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";
import { proxyCommercialBilling } from "@/lib/server/commercial-billing-proxy";
import { withCommercialReadDeadline } from "@/lib/server/commercial-route";
import { workbenchProtocolError } from "@/lib/server/workbench-proxy";

export const dynamic = "force-dynamic";
export const GET = withCommercialReadDeadline(serverAuth(async (request: NextRequest & { auth?: unknown }) => request.signal.aborted ? workbenchProtocolError(504, "DEADLINE_EXCEEDED", "Resource read ended before completion") : proxyCommercialBilling(request, readZitadelServerAccessToken(request.auth as never), String(readZitadelIdentityFromSession(request.auth as never)?.userId ?? ""))));
export { POST, PUT, PATCH, DELETE, HEAD, OPTIONS } from "@/lib/server/commercial-route";

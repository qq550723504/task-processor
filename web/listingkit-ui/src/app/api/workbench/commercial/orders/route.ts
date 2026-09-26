import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "@/lib/server/zitadel-auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";
import { proxyCommercialBilling } from "@/lib/server/commercial-billing-proxy";

export const dynamic = "force-dynamic";
const handler = serverAuth(async (request: NextRequest & { auth?: unknown }) => proxyCommercialBilling(request, readZitadelServerAccessToken(request.auth as never), String(readZitadelIdentityFromSession(request.auth as never)?.userId ?? "")));
export const GET = handler;
export const POST = handler;

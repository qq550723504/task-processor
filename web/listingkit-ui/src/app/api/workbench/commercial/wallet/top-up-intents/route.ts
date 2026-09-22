import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";
import { proxyCommercialBilling } from "@/lib/server/commercial-billing-proxy";

export const POST = serverAuth(async (request: NextRequest & { auth?: unknown }) => proxyCommercialBilling(request, readZitadelServerAccessToken(request.auth as never)));

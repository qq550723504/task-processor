import { NextRequest, NextResponse } from "next/server";

import {
  buildListingKitUpstreamHeaders,
  type VerifiedIdentity,
  verifyListingKitRequestIdentity,
} from "@/app/api/listing-kits/proxy-auth";
import { buildSheinLoginURL } from "@/app/api/shein-login/shared";

export const dynamic = "force-dynamic";

export function buildSheinLoginUpstreamHeaders(
  requestHeaders: Headers,
  verifiedIdentity?: VerifiedIdentity,
) {
  return buildListingKitUpstreamHeaders(requestHeaders, verifiedIdentity);
}

async function proxyRequest(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  const url = buildSheinLoginURL(`/${path.join("/")}`);
  const auth = await verifyListingKitRequestIdentity(request);
  if (auth.response) {
    return auth.response;
  }

  const headers = buildSheinLoginUpstreamHeaders(request.headers, auth.identity);
  if (auth.token && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${auth.token}`);
  }

  const response = await fetch(url, {
    method: request.method,
    headers,
    body:
      request.method === "GET" || request.method === "HEAD"
        ? undefined
        : await request.text(),
    cache: "no-store",
  });

  const responseHeaders = new Headers();
  const contentType = response.headers.get("content-type");
  if (contentType) {
    responseHeaders.set("Content-Type", contentType);
  }

  const proxyResponse = new NextResponse(await response.text(), {
    status: response.status,
    headers: responseHeaders,
  });
  return proxyResponse;
}

export const GET = proxyRequest;
export const POST = proxyRequest;
export const DELETE = proxyRequest;

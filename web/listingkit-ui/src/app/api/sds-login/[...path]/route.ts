import { NextResponse } from "next/server";

import { serverAuth } from "@/auth";
import {
  type AuthenticatedListingKitRequest,
  buildListingKitUpstreamHeaders,
  type VerifiedIdentity,
  verifyListingKitRequestIdentity,
} from "@/app/api/listing-kits/proxy-auth";
import { buildSDSLoginURL } from "@/app/api/sds-login/shared";
import { hasPlatformAdminRole } from "@/lib/listingkit-permissions";

export const dynamic = "force-dynamic";

export function buildSDSLoginUpstreamHeaders(
  requestHeaders: Headers,
  verifiedIdentity?: VerifiedIdentity,
) {
  return buildListingKitUpstreamHeaders(requestHeaders, verifiedIdentity);
}

async function proxyRequest(
  request: AuthenticatedListingKitRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  const url = buildSDSLoginURL(`/${path.join("/")}`);
  const auth = await verifyListingKitRequestIdentity(request, request.auth);
  if (auth.response) {
    return auth.response;
  }

  if (auth.identity && !hasPlatformAdminRole(auth.identity.roles)) {
    return NextResponse.json(
      {
        error: "listingkit_permission_denied",
        message: "Platform administrator permission is required to access SDS login",
      },
      { status: 403 },
    );
  }

  const headers = buildSDSLoginUpstreamHeaders(request.headers, auth.identity);
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

  return new NextResponse(await response.text(), {
    status: response.status,
    headers: responseHeaders,
  });
}

const authenticatedProxyRequest = serverAuth(proxyRequest);

export const GET = authenticatedProxyRequest;
export const POST = authenticatedProxyRequest;
export const DELETE = authenticatedProxyRequest;

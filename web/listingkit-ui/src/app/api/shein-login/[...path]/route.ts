import { NextRequest, NextResponse } from "next/server";

import { shouldBypassListingKitProxyAuth } from "@/app/api/listing-kits/proxy-auth";
import { buildSheinLoginURL } from "@/app/api/shein-login/shared";
import {
  fetchZitadelDiscovery,
  getZitadelAuthOptions,
  getZitadelBearerToken,
  verifyZitadelAccessToken,
} from "@/lib/server/zitadel-auth";

export const dynamic = "force-dynamic";

type SheinLoginAuthResult =
  | { token: string; response?: never }
  | { response: NextResponse; token?: never };

function copyHeader(source: Headers, target: Headers, sourceName: string, targetName?: string) {
  const value = source.get(sourceName);
  if (value) {
    target.set(targetName ?? sourceName, value);
  }
}

export function buildSheinLoginUpstreamHeaders(requestHeaders: Headers) {
  const headers = new Headers();
  copyHeader(requestHeaders, headers, "accept", "Accept");
  copyHeader(requestHeaders, headers, "content-type", "Content-Type");
  copyHeader(requestHeaders, headers, "authorization", "Authorization");
  return headers;
}

async function authorizeSheinLoginProxy(
  request: NextRequest,
): Promise<SheinLoginAuthResult> {
  const options = getZitadelAuthOptions();
  if (!options) {
    if (shouldBypassListingKitProxyAuth()) {
      return { token: "" };
    }
    return {
      response: NextResponse.json(
        {
          error: "zitadel_auth_not_configured",
          message: "ZITADEL authentication is not configured",
        },
        { status: 503 },
      ),
    };
  }

  const token = getZitadelBearerToken(request);
  try {
    const discovery = await fetchZitadelDiscovery(options);
    await verifyZitadelAccessToken(token, options, discovery);
    return { token };
  } catch (error) {
    return {
      response: NextResponse.json(
        {
          error: "zitadel_token_invalid",
          message:
            error instanceof Error
              ? error.message
              : "ZITADEL token verification failed",
        },
        { status: 401 },
      ),
    };
  }
}

async function proxyRequest(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  const url = buildSheinLoginURL(`/${path.join("/")}`);
  const auth = await authorizeSheinLoginProxy(request);
  if (auth.response) {
    return auth.response;
  }

  const headers = buildSheinLoginUpstreamHeaders(request.headers);
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

export const GET = proxyRequest;
export const POST = proxyRequest;
export const DELETE = proxyRequest;

import { NextRequest, NextResponse } from "next/server";

import {
  buildListingKitProxyUrl,
  getListingKitUpstreamBase,
} from "@/app/api/listing-kits/proxy-url";
import { buildListingKitMockResponse } from "@/app/api/listing-kits/mock-responses";
import { selectListingKitMockPayload } from "@/app/api/listing-kits/proxy-mock";
import { readProxyRequestBody } from "@/app/api/listing-kits/proxy-request-body";
import { resolveListingKitProxyTimeoutMs } from "@/app/api/listing-kits/proxy-response";
import { buildListingKitProxyResponse } from "@/app/api/listing-kits/proxy-upstream-response";
import {
  buildListingKitUpstreamHeaders,
  shouldBypassListingKitProxyAuth,
  type VerifiedIdentity,
} from "@/app/api/listing-kits/proxy-auth";
import {
  logRequestInfo,
  logRequestWarn,
  newRequestLogId,
} from "@/lib/server/request-log";
import {
  fetchZitadelDiscovery,
  getZitadelAuthOptions,
  getZitadelBearerToken,
  verifyZitadelAccessToken,
} from "@/lib/server/zitadel-auth";

export const dynamic = "force-dynamic";
const PROXY_BODY_READ_TIMEOUT_MS = 15_000;

async function proxyRequest(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const requestId = newRequestLogId();
  const startedAt = Date.now();
  const { path } = await params;
  const proxyPath = `/${path.join("/")}`;
  const mock = await buildListingKitMockResponse(request, path);
  if (mock) {
    if (request.method === "POST" && path.length === 1 && path[0] === "generate") {
      logRequestInfo("listingkit proxy mock response", {
        requestId,
        method: request.method,
        path: proxyPath,
        status: 200,
        durationMs: Date.now() - startedAt,
      });
      return NextResponse.json(mock.createTask, {
        headers: {
          ETag: "mock-create-task",
        },
      });
    }

    const payload = selectListingKitMockPayload({
      bundle: mock,
      method: request.method,
      path,
    });

    logRequestInfo("listingkit proxy mock response", {
      requestId,
      method: request.method,
      path: proxyPath,
      status: 200,
      durationMs: Date.now() - startedAt,
    });
    return NextResponse.json(payload, {
      headers: {
        ETag:
          ("conditional" in payload && payload.conditional?.etag) ||
          ("delta_token" in payload && payload.delta_token) ||
          "mock-token",
      },
    });
  }

  const url = buildListingKitProxyUrl(
    getListingKitUpstreamBase(),
    path,
    request.nextUrl.searchParams.toString(),
  );
  logRequestInfo("listingkit proxy request started", {
    requestId,
    method: request.method,
    path: proxyPath,
  });

  const zitadelOptions = getZitadelAuthOptions();
  const zitadelToken = zitadelOptions ? getZitadelBearerToken(request) : "";
  let verifiedIdentity: VerifiedIdentity | undefined;
  if (zitadelOptions) {
    try {
      const discovery = await fetchZitadelDiscovery(zitadelOptions);
      verifiedIdentity = await verifyZitadelAccessToken(
        zitadelToken,
        zitadelOptions,
        discovery,
      );
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "ZITADEL token verification failed";
      logRequestWarn("listingkit proxy zitadel token verification failed", {
        requestId,
        method: request.method,
        path: proxyPath,
        status: 401,
        durationMs: Date.now() - startedAt,
        error: message,
      });
      return NextResponse.json(
        {
          error: "zitadel_token_invalid",
          message,
        },
        { status: 401 },
      );
    }
  } else if (!shouldBypassListingKitProxyAuth()) {
    logRequestWarn("listingkit proxy zitadel auth not configured", {
      requestId,
      method: request.method,
      path: proxyPath,
      status: 503,
      durationMs: Date.now() - startedAt,
    });
    return NextResponse.json(
      {
        error: "zitadel_auth_not_configured",
        message: "ZITADEL authentication is not configured",
      },
      { status: 503 },
    );
  }

  const headers = buildListingKitUpstreamHeaders(
    request.headers,
    verifiedIdentity,
  );
  if (zitadelToken && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${zitadelToken}`);
  }

  let upstream: Response;
  try {
    const body =
      request.method === "GET" || request.method === "HEAD"
        ? undefined
        : await readProxyRequestBody(request, PROXY_BODY_READ_TIMEOUT_MS);
    const upstreamTimeoutMs = resolveListingKitProxyTimeoutMs(
      request.method,
      path,
    );
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), upstreamTimeoutMs);
    try {
      upstream = await fetch(url, {
        method: request.method,
        headers,
        body,
        cache: "no-store",
        signal: controller.signal,
      });
    } finally {
      clearTimeout(timeout);
    }
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "ListingKit upstream request failed";
    logRequestWarn("listingkit upstream request failed", {
      requestId,
      method: request.method,
      path: proxyPath,
      status: 504,
      durationMs: Date.now() - startedAt,
      error: message,
      timeoutMs: resolveListingKitProxyTimeoutMs(request.method, path),
    });
    return NextResponse.json(
      {
        error: "listingkit_upstream_unavailable",
        message,
      },
      { status: 504 },
    );
  }

  return buildListingKitProxyResponse({
    durationMs: Date.now() - startedAt,
    requestId,
    method: request.method,
    path: proxyPath,
    routePath: path,
    upstream,
  });
}

export async function GET(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
) {
  return proxyRequest(request, context);
}

export async function POST(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
) {
  return proxyRequest(request, context);
}

export async function PATCH(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
) {
  return proxyRequest(request, context);
}

export async function PUT(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
) {
  return proxyRequest(request, context);
}

export async function DELETE(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
) {
  return proxyRequest(request, context);
}


import { NextResponse } from "next/server";

import { shouldProxyListingKitResponseAsBinary } from "@/app/api/listing-kits/proxy-response";
import { logRequestInfo, logRequestWarn } from "@/lib/server/request-log";

type ProxyResponseInput = {
  durationMs: number;
  method: string;
  path: string;
  requestId: string;
  routePath: string[];
  upstream: Response;
};

export async function buildListingKitProxyResponse({
  durationMs,
  method,
  path,
  requestId,
  routePath,
  upstream,
}: ProxyResponseInput) {
  const responseHeaders = buildProxyResponseHeaders(upstream);

  if (upstream.status === 304) {
    logListingKitProxyResponse({
      durationMs,
      method,
      path,
      requestId,
      status: upstream.status,
      upstreamOk: upstream.ok,
    });
    return new NextResponse(null, {
      status: 304,
      headers: responseHeaders,
    });
  }

  try {
    const body = shouldProxyListingKitResponseAsBinary(
      upstream.headers.get("content-type"),
      routePath,
    )
      ? await upstream.arrayBuffer()
      : await upstream.text();

    const upstreamBodyPreview =
      !upstream.ok && typeof body === "string"
        ? body.slice(0, 500)
        : undefined;

    logListingKitProxyResponse({
      durationMs,
      method,
      path,
      requestId,
      status: upstream.status,
      upstreamOk: upstream.ok,
      upstreamBodyPreview,
    });
    return new NextResponse(body, {
      status: upstream.status,
      headers: responseHeaders,
    });
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "ListingKit upstream body read failed";
    logRequestWarn("listingkit upstream body read failed", {
      requestId,
      method,
      path,
      status: 502,
      durationMs,
      error: message,
    });
    return NextResponse.json(
      {
        error: "listingkit_upstream_body_unavailable",
        message,
      },
      { status: 502 },
    );
  }
}

function buildProxyResponseHeaders(upstream: Response) {
  const responseHeaders = new Headers();
  const etag = upstream.headers.get("etag");
  const contentTypeHeader = upstream.headers.get("content-type");

  if (etag) {
    responseHeaders.set("ETag", etag);
  }
  if (contentTypeHeader) {
    responseHeaders.set("Content-Type", contentTypeHeader);
  }

  return responseHeaders;
}

function logListingKitProxyResponse({
  durationMs,
  method,
  path,
  requestId,
  status,
  upstreamOk,
  upstreamBodyPreview,
}: {
  durationMs: number;
  method: string;
  path: string;
  requestId: string;
  status: number;
  upstreamOk: boolean;
  upstreamBodyPreview?: string;
}) {
  const logFields = {
    requestId,
    method,
    path,
    status,
    durationMs,
    upstreamBodyPreview,
  };
  if (!upstreamOk || durationMs > 5_000) {
    logRequestWarn("listingkit proxy response", logFields);
  } else {
    logRequestInfo("listingkit proxy response", logFields);
  }
}

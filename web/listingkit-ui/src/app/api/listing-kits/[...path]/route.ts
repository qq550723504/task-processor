import { NextRequest, NextResponse } from "next/server";

import {
  buildListingKitProxyUrl,
  getListingKitUpstreamBase,
} from "@/app/api/listing-kits/proxy-url";
import { buildListingKitMockResponse } from "@/app/api/listing-kits/mock-responses";
import {
  logRequestInfo,
  logRequestWarn,
  newRequestLogId,
} from "@/lib/server/request-log";

export const dynamic = "force-dynamic";
const PROXY_BODY_READ_TIMEOUT_MS = 15_000;
const PROXY_UPSTREAM_TIMEOUT_MS = 15_000;

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

    const endpoint = path[path.length - 1];
    const payload =
      request.method === "POST" && endpoint === "execute"
        ? mock.action
        : request.method === "POST"
          ? mock.dispatch
          : endpoint === "generation-queue"
            ? mock.queue
            : endpoint === "generation-review-session"
              ? mock.reviewSession
              : endpoint === "generation-review-preview"
                ? mock.reviewPreview
                : path.length === 2 && path[0] === "tasks"
                  ? mock.taskResult
                  : mock.preview;

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

  const headers = new Headers();
  headers.set("Accept", request.headers.get("accept") ?? "application/json");

  const ifNoneMatch = request.headers.get("if-none-match");
  if (ifNoneMatch) {
    headers.set("If-None-Match", ifNoneMatch);
  }

  const contentType = request.headers.get("content-type");
  if (contentType) {
    headers.set("Content-Type", contentType);
  }

  let upstream: Response;
  try {
    const body =
      request.method === "GET" || request.method === "HEAD"
        ? undefined
        : await readProxyRequestBody(request, PROXY_BODY_READ_TIMEOUT_MS);
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), PROXY_UPSTREAM_TIMEOUT_MS);
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
    });
    return NextResponse.json(
      {
        error: "listingkit_upstream_unavailable",
        message,
      },
      { status: 504 },
    );
  }

  const responseHeaders = new Headers();
  const etag = upstream.headers.get("etag");
  const contentTypeHeader = upstream.headers.get("content-type");

  if (etag) {
    responseHeaders.set("ETag", etag);
  }
  if (contentTypeHeader) {
    responseHeaders.set("Content-Type", contentTypeHeader);
  }

  if (upstream.status === 304) {
    logRequestInfo("listingkit proxy response", {
      requestId,
      method: request.method,
      path: proxyPath,
      status: 304,
      durationMs: Date.now() - startedAt,
    });
    return new NextResponse(null, {
      status: 304,
      headers: responseHeaders,
    });
  }

  let body: string;
  try {
    body = await upstream.text();
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "ListingKit upstream body read failed";
    logRequestWarn("listingkit upstream body read failed", {
      requestId,
      method: request.method,
      path: proxyPath,
      status: 502,
      durationMs: Date.now() - startedAt,
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
  const durationMs = Date.now() - startedAt;
  const logFields = {
    requestId,
    method: request.method,
    path: proxyPath,
    status: upstream.status,
    durationMs,
  };
  if (!upstream.ok || durationMs > 5_000) {
    logRequestWarn("listingkit proxy response", logFields);
  } else {
    logRequestInfo("listingkit proxy response", logFields);
  }
  return new NextResponse(body, {
    status: upstream.status,
    headers: responseHeaders,
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

async function readProxyRequestBody(request: NextRequest, timeoutMs: number) {
  const buffer = await withTimeout(
    request.arrayBuffer(),
    timeoutMs,
    "ListingKit proxy request body read timed out",
  );
  if (buffer.byteLength === 0) {
    return undefined;
  }
  return buffer;
}

async function withTimeout<T>(
  promise: Promise<T>,
  timeoutMs: number,
  message: string,
) {
  let timeout: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      promise,
      new Promise<T>((_, reject) => {
        timeout = setTimeout(() => reject(new Error(message)), timeoutMs);
      }),
    ]);
  } finally {
    if (timeout) {
      clearTimeout(timeout);
    }
  }
}

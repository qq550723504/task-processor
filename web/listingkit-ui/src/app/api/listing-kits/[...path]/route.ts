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
const PROXY_SUBMIT_UPSTREAM_TIMEOUT_MS = 180_000;

export function resolveListingKitProxyTimeoutMs(
  method: string,
  path: string[],
) {
  if (
    method.toUpperCase() === "POST" &&
    path.length === 3 &&
    path[0] === "tasks" &&
    path[2] === "submit"
  ) {
    return PROXY_SUBMIT_UPSTREAM_TIMEOUT_MS;
  }
  return PROXY_UPSTREAM_TIMEOUT_MS;
}

export function shouldProxyListingKitResponseAsBinary(
  contentType: string | null,
  path: string[],
) {
  const normalized = (contentType ?? "").toLowerCase();
  if (path.length >= 3 && path[0] === "uploads" && path[1] === "files") {
    return true;
  }
  return (
    normalized.startsWith("image/") ||
    normalized.startsWith("audio/") ||
    normalized.startsWith("video/") ||
    normalized === "application/octet-stream" ||
    normalized.startsWith("application/pdf")
  );
}

type YudaoLoginUserHeader = {
  id?: string | number;
  tenantId?: string | number;
  tenant_id?: string | number;
  userType?: string | number;
  user_type?: string | number;
};

type YudaoVerifiedIdentity = {
  tenantId?: string | number;
  userId?: string | number;
  userType?: string | number;
};

type YudaoCheckTokenOptions = {
  checkTokenUrl: string;
  clientId: string;
  clientSecret: string;
  tenantId?: string | null;
};

type YudaoCheckTokenResponse = {
  code?: number;
  data?: {
    user_id?: string | number;
    user_type?: string | number;
    tenant_id?: string | number;
  };
  msg?: string;
  message?: string;
};

export function shouldBypassYudaoTokenVerification() {
  return (
    process.env.NODE_ENV !== "production" &&
    process.env.YUDAO_DEV_BYPASS_TOKEN_VERIFICATION === "1"
  );
}

export function buildListingKitUpstreamHeaders(
  requestHeaders: Headers,
  verifiedIdentity?: YudaoVerifiedIdentity,
) {
  const headers = new Headers();
  headers.set("Accept", requestHeaders.get("accept") ?? "application/json");

  copyHeader(requestHeaders, headers, "if-none-match", "If-None-Match");
  copyHeader(requestHeaders, headers, "content-type", "Content-Type");
  copyHeader(requestHeaders, headers, "authorization", "Authorization");
  copyHeader(requestHeaders, headers, "tenant-id", "tenant-id");
  copyHeader(requestHeaders, headers, "visit-tenant-id", "visit-tenant-id");
  copyHeader(requestHeaders, headers, "login-user", "login-user");

  const loginUser = parseYudaoLoginUserHeader(requestHeaders.get("login-user"));
  const tenantID = stringifyIdentityValue(
    verifiedIdentity?.tenantId ??
      loginUser?.tenantId ??
      loginUser?.tenant_id ??
      requestHeaders.get("tenant-id"),
  );
  const userID = stringifyIdentityValue(verifiedIdentity?.userId ?? loginUser?.id);
  const userType = stringifyIdentityValue(
    verifiedIdentity?.userType ?? loginUser?.userType ?? loginUser?.user_type,
  );

  if (tenantID) {
    headers.set("tenant-id", tenantID);
    headers.set("X-Tenant-ID", tenantID);
  }
  if (userID) {
    headers.set("X-User-ID", userID);
  }
  if (userType) {
    headers.set("X-User-Type", userType);
  }

  return headers;
}

export async function verifyYudaoAccessToken(
  authorization: string | null,
  options: YudaoCheckTokenOptions,
): Promise<YudaoVerifiedIdentity> {
  if (shouldBypassYudaoTokenVerification()) {
    const token = extractBearerToken(authorization);
    if (!token) {
      throw new Error("Missing yudao bearer token");
    }
    return {
      tenantId: options.tenantId ?? "1",
      userId: "dev-user",
      userType: "1",
    };
  }

  const token = extractBearerToken(authorization);
  if (!token) {
    throw new Error("Missing yudao bearer token");
  }

  const response = await fetch(options.checkTokenUrl, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      ...(options.tenantId ? { "tenant-id": options.tenantId } : {}),
    },
    body: new URLSearchParams({
      client_id: options.clientId,
      client_secret: options.clientSecret,
      token,
    }).toString(),
    cache: "no-store",
  });
  const payload = (await response.json().catch(() => undefined)) as
    | YudaoCheckTokenResponse
    | undefined;

  if (!response.ok || payload?.code !== 0 || !payload.data) {
    throw new Error(
      payload?.msg ??
        payload?.message ??
        `Yudao check-token failed: ${response.status}`,
    );
  }

  return {
    tenantId: payload.data.tenant_id,
    userId: payload.data.user_id,
    userType: payload.data.user_type,
  };
}

function copyHeader(
  source: Headers,
  target: Headers,
  sourceName: string,
  targetName: string,
) {
  const value = source.get(sourceName);
  if (value) {
    target.set(targetName, value);
  }
}

function parseYudaoLoginUserHeader(
  value: string | null,
): YudaoLoginUserHeader | undefined {
  if (!value) {
    return undefined;
  }
  try {
    return JSON.parse(decodeURIComponent(value)) as YudaoLoginUserHeader;
  } catch {
    try {
      return JSON.parse(value) as YudaoLoginUserHeader;
    } catch {
      return undefined;
    }
  }
}

function stringifyIdentityValue(value: unknown) {
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value);
  }
  if (typeof value === "string" && value.trim()) {
    return value.trim();
  }
  return "";
}

function extractBearerToken(authorization: string | null) {
  const match = authorization?.match(/^Bearer\s+(.+)$/i);
  return match?.[1]?.trim() ?? "";
}

export function getYudaoCheckTokenOptions(): YudaoCheckTokenOptions | undefined {
  const checkTokenUrl = process.env.YUDAO_CHECK_TOKEN_URL?.trim();
  const clientId = process.env.YUDAO_OAUTH_CLIENT_ID?.trim();
  const clientSecret = process.env.YUDAO_OAUTH_CLIENT_SECRET?.trim();
  if (!checkTokenUrl || !clientId || !clientSecret) {
    return undefined;
  }
  return { checkTokenUrl, clientId, clientSecret };
}

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

  const yudaoOptions = shouldBypassYudaoTokenVerification()
    ? undefined
    : getYudaoCheckTokenOptions();
  let verifiedIdentity: YudaoVerifiedIdentity | undefined;
  if (yudaoOptions) {
    try {
      verifiedIdentity = await verifyYudaoAccessToken(
        request.headers.get("authorization"),
        {
          ...yudaoOptions,
          tenantId:
            request.headers.get("tenant-id") ??
            request.headers.get("x-tenant-id"),
        },
      );
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Yudao token verification failed";
      logRequestWarn("listingkit proxy yudao token verification failed", {
        requestId,
        method: request.method,
        path: proxyPath,
        status: 401,
        durationMs: Date.now() - startedAt,
        error: message,
      });
      return NextResponse.json(
        {
          error: "yudao_token_invalid",
          message,
        },
        { status: 401 },
      );
    }
  }

  const headers = buildListingKitUpstreamHeaders(
    request.headers,
    verifiedIdentity,
  );

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

  const shouldReadAsBinary = shouldProxyListingKitResponseAsBinary(
    contentTypeHeader,
    path,
  );

  try {
    if (shouldReadAsBinary) {
      const body = await upstream.arrayBuffer();
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

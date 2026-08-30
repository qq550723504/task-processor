import { NextResponse } from "next/server";

export const WORKBENCH_COOKIE_NAME = "shuomi_effective_organization";

const DEFAULT_SERVICE_API_BASE = "http://localhost:8085/api/v1";
const REQUEST_BODY_MAX_BYTES = 4 * 1024;
const UPSTREAM_RESPONSE_MAX_BYTES = 1024 * 1024;

type WorkbenchUpstreamRequest = {
  url: string;
  init: RequestInit;
};

export async function buildWorkbenchUpstreamRequest(
  request: Request,
  path: string[],
  accessToken: string,
): Promise<WorkbenchUpstreamRequest | Response> {
  const route = resolveAllowlistedRoute(request.method, path);
  if (!route) {
    return protocolError(404, "INVALID_REQUEST", "Workbench route is not allowed");
  }

  const headers = new Headers({
    Accept: "application/json",
    Authorization: `Bearer ${accessToken}`,
  });
  let body: string | undefined;

  if (request.method.toUpperCase() === "PUT") {
    const contentLength = readContentLength(request.headers);
    if (contentLength !== null && contentLength > REQUEST_BODY_MAX_BYTES) {
      return protocolError(413, "INVALID_REQUEST", "Request body is too large");
    }

    let rawBody: Uint8Array;
    try {
      rawBody = await readBodyWithinLimit(request.body, REQUEST_BODY_MAX_BYTES);
    } catch (error) {
      if (error instanceof BodyTooLargeError) {
        return protocolError(413, "INVALID_REQUEST", "Request body is too large");
      }
      return protocolError(400, "INVALID_REQUEST", "Request body is invalid");
    }

    const organizationId = parseSwitchOrganizationBody(rawBody);
    if (!organizationId) {
      return protocolError(400, "INVALID_REQUEST", "Request body is invalid");
    }
    body = JSON.stringify({ organizationId });
    headers.set("Content-Type", "application/json");
    headers.set("X-Requested-Organization-ID", organizationId);
  } else {
    const selectedOrganization = readCookie(
      request.headers.get("cookie"),
      WORKBENCH_COOKIE_NAME,
    ).trim();
    if (selectedOrganization) {
      headers.set("X-Requested-Organization-ID", selectedOrganization);
    }
  }

  return {
    url: `${getServiceRoot()}/workbench/${route}`,
    init: {
      method: request.method.toUpperCase(),
      headers,
      body,
      cache: "no-store",
    },
  };
}

export async function buildWorkbenchBrowserResponse(upstream: Response) {
  let body: Uint8Array;
  try {
    const contentLength = readContentLength(upstream.headers);
    if (contentLength !== null && contentLength > UPSTREAM_RESPONSE_MAX_BYTES) {
      throw new BodyTooLargeError();
    }
    body = await readBodyWithinLimit(
      upstream.body,
      UPSTREAM_RESPONSE_MAX_BYTES,
    );
  } catch {
    return protocolError(
      502,
      "DEPENDENCY_UNAVAILABLE",
      "Workbench upstream response is unavailable",
    );
  }

  const responseHeaders = new Headers();
  for (const name of ["Content-Type", "Cache-Control", "ETag", "X-Request-ID"]) {
    const value = upstream.headers.get(name);
    if (value) {
      responseHeaders.set(name, value);
    }
  }

  const responseBody = statusAllowsBody(upstream.status)
    ? Uint8Array.from(body).buffer
    : null;
  const response = new NextResponse(responseBody, {
    status: upstream.status,
    headers: responseHeaders,
  });
  const payload = parseJSONBody(body);

  if (upstream.ok && payload && hasOwn(payload, "effectiveOrganizationId")) {
    const effectiveOrganizationId =
      typeof payload.effectiveOrganizationId === "string"
        ? payload.effectiveOrganizationId.trim()
        : "";
    if (effectiveOrganizationId) {
      response.cookies.set(WORKBENCH_COOKIE_NAME, effectiveOrganizationId, {
        httpOnly: true,
        sameSite: "lax",
        path: "/",
        secure: process.env.NODE_ENV !== "development",
      });
    } else {
      clearSelectionCookie(response);
    }
  } else if (
    payload?.code === "ORGANIZATION_ACCESS_REVOKED" ||
    payload?.code === "ORGANIZATION_ACCESS_DENIED"
  ) {
    clearSelectionCookie(response);
  }

  return response;
}

export function workbenchProtocolError(
  status: number,
  code: string,
  message: string,
) {
  return protocolError(status, code, message);
}

function resolveAllowlistedRoute(method: string, path: string[]) {
  if (!path.every(isSafePathSegment)) {
    return "";
  }
  const verb = method.toUpperCase();
  if (verb === "GET" && path.length === 1 && path[0] === "context") {
    return "context";
  }
  if (
    verb === "PUT" &&
    path.length === 2 &&
    path[0] === "context" &&
    path[1] === "effective-organization"
  ) {
    return "context/effective-organization";
  }
  return "";
}

function isSafePathSegment(value: string) {
  return Boolean(
    value &&
      value !== "." &&
      value !== ".." &&
      !value.includes("/") &&
      !value.includes("\\") &&
      !value.includes("\0"),
  );
}

function getServiceRoot() {
  return (
    process.env.LISTINGKIT_SERVICE_API_BASE?.trim() || DEFAULT_SERVICE_API_BASE
  ).replace(/\/+$/, "");
}

function parseSwitchOrganizationBody(body: Uint8Array) {
  let text: string;
  try {
    text = new TextDecoder("utf-8", { fatal: true }).decode(body);
  } catch {
    return "";
  }
  const match =
    /^\s*\{\s*"organizationId"\s*:\s*("(?:\\["\\/bfnrt]|\\u[0-9a-fA-F]{4}|[^"\\\u0000-\u001F])*")\s*\}\s*$/u.exec(
      text,
    );
  if (!match?.[1]) {
    return "";
  }
  try {
    const value = JSON.parse(match[1]) as unknown;
    return typeof value === "string" ? value.trim() : "";
  } catch {
    return "";
  }
}

function readCookie(header: string | null, name: string) {
  if (!header) return "";
  for (const part of header.split(";")) {
    const separator = part.indexOf("=");
    if (separator < 0 || part.slice(0, separator).trim() !== name) continue;
    try {
      return decodeURIComponent(part.slice(separator + 1).trim());
    } catch {
      return "";
    }
  }
  return "";
}

async function readBodyWithinLimit(
  stream: ReadableStream<Uint8Array> | null,
  limit: number,
) {
  if (!stream) return new Uint8Array();
  const reader = stream.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      if (total + value.byteLength > limit) {
        await reader.cancel().catch(() => undefined);
        throw new BodyTooLargeError();
      }
      chunks.push(value);
      total += value.byteLength;
    }
  } finally {
    reader.releaseLock();
  }
  const joined = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    joined.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return joined;
}

function readContentLength(headers: Headers) {
  const raw = headers.get("content-length");
  if (!raw || !/^\d+$/.test(raw)) return null;
  const value = Number(raw);
  return Number.isSafeInteger(value) ? value : null;
}

function parseJSONBody(body: Uint8Array) {
  try {
    const value = JSON.parse(new TextDecoder().decode(body)) as unknown;
    return value !== null && typeof value === "object"
      ? (value as Record<string, unknown>)
      : null;
  } catch {
    return null;
  }
}

function hasOwn(value: object, key: string) {
  return Object.prototype.hasOwnProperty.call(value, key);
}

function clearSelectionCookie(response: NextResponse) {
  response.cookies.set(WORKBENCH_COOKIE_NAME, "", {
    httpOnly: true,
    sameSite: "lax",
    path: "/",
    secure: process.env.NODE_ENV !== "development",
    maxAge: 0,
  });
}

function statusAllowsBody(status: number) {
  return status !== 204 && status !== 205 && status !== 304;
}

function protocolError(status: number, code: string, message: string) {
  return NextResponse.json(
    { code, message, requestId: "", fieldErrors: [] },
    { status },
  );
}

class BodyTooLargeError extends Error {}

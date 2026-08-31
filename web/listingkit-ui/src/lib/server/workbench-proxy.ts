import { NextResponse } from "next/server";
import { parseTree, type ParseError } from "jsonc-parser";

import {
  parseWorkbenchContextPayload,
  parseWorkbenchErrorEnvelopePayload,
} from "@/lib/api/workbench-context";

export const WORKBENCH_COOKIE_NAME = "shuomi_effective_organization";

const DEFAULT_SERVICE_API_BASE = "http://localhost:8085/api/v1";
const REQUEST_BODY_MAX_BYTES = 4 * 1024;
const REQUEST_BODY_READ_TIMEOUT_MS = 15_000;
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
    return protocolError(
      404,
      "INVALID_REQUEST",
      "Workbench route is not allowed",
    );
  }

  const headers = new Headers({
    Accept: "application/json",
    Authorization: `Bearer ${accessToken}`,
  });
  let body: string | undefined;

  if (request.method.toUpperCase() === "PUT") {
    const contentLength = readContentLength(request.headers);
    if (contentLength !== null && contentLength > REQUEST_BODY_MAX_BYTES) {
      void request.body?.cancel().catch(() => undefined);
      return protocolError(413, "INVALID_REQUEST", "Request body is too large");
    }

    let rawBody: Uint8Array;
    try {
      rawBody = await readBodyWithinLimit(
        request.body,
        REQUEST_BODY_MAX_BYTES,
        REQUEST_BODY_READ_TIMEOUT_MS,
      );
    } catch (error) {
      if (error instanceof BodyReadTimeoutError) {
        return protocolError(
          408,
          "INVALID_REQUEST",
          "Request body read timed out",
        );
      }
      if (error instanceof BodyTooLargeError) {
        return protocolError(
          413,
          "INVALID_REQUEST",
          "Request body is too large",
        );
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
    const selectedCookie = readCookie(
      request.headers.get("cookie"),
      WORKBENCH_COOKIE_NAME,
    );
    if (selectedCookie === null) {
      return protocolError(
        400,
        "INVALID_REQUEST",
        "Selection cookie is invalid",
      );
    }
    const selectedOrganization = selectedCookie.trim();
    if (selectedOrganization) {
      if (!isSafeOrganizationId(selectedOrganization)) {
        return protocolError(
          400,
          "INVALID_REQUEST",
          "Selection cookie is invalid",
        );
      }
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
      void upstream.body?.cancel().catch(() => undefined);
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

  const payload = parseJSONBody(body);
  const parsedContext =
    upstream.status === 200 ? parseWorkbenchContextPayload(payload) : null;
  const parsedError =
    upstream.status >= 400 && upstream.status <= 599
      ? parseWorkbenchErrorEnvelopePayload(payload)
      : null;
  if (upstream.ok ? !parsedContext?.success : !parsedError?.success) {
    return invalidUpstreamResponse();
  }

  const validatedPayload = parsedContext?.success
    ? parsedContext.data
    : parsedError?.success
      ? parsedError.data
      : null;
  if (!validatedPayload) return invalidUpstreamResponse();
  const response = new NextResponse(JSON.stringify(validatedPayload), {
    status: upstream.status,
    headers: safeJSONHeaders(),
  });

  if (parsedContext?.success) {
    const effectiveOrganizationId = parsedContext.data.effectiveOrganizationId;
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
    parsedError?.success &&
    (parsedError.data.code === "ORGANIZATION_ACCESS_REVOKED" ||
      parsedError.data.code === "ORGANIZATION_ACCESS_DENIED")
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
  const errors: ParseError[] = [];
  const root = parseTree(text, errors, {
    allowEmptyContent: false,
    allowTrailingComma: false,
    disallowComments: true,
  });
  if (
    errors.length > 0 ||
    root?.type !== "object" ||
    root.children?.length !== 1
  ) {
    return "";
  }
  const property = root.children[0];
  const key = property?.children?.[0];
  const value = property?.children?.[1];
  if (
    property?.type !== "property" ||
    property.children?.length !== 2 ||
    key?.type !== "string" ||
    key.value !== "organizationId" ||
    value?.type !== "string"
  ) {
    return "";
  }
  const organizationId = String(value.value).trim();
  return isSafeOrganizationId(organizationId) ? organizationId : "";
}

function readCookie(header: string | null, name: string): string | null {
  if (!header) return "";
  for (const part of header.split(";")) {
    const separator = part.indexOf("=");
    if (separator < 0 || part.slice(0, separator).trim() !== name) continue;
    try {
      return decodeURIComponent(part.slice(separator + 1).trim());
    } catch {
      return null;
    }
  }
  return "";
}

async function readBodyWithinLimit(
  stream: ReadableStream<Uint8Array> | null,
  limit: number,
  timeoutMs?: number,
) {
  if (!stream) return new Uint8Array();
  let timeout: ReturnType<typeof setTimeout> | undefined;
  const deadline = timeoutMs
    ? new Promise<never>((_resolve, reject) => {
        timeout = setTimeout(
          () => reject(new BodyReadTimeoutError()),
          timeoutMs,
        );
      })
    : undefined;
  let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;
  const chunks: Uint8Array[] = [];
  let total = 0;
  try {
    reader = stream.getReader();
    while (true) {
      const next = reader.read();
      const { done, value } = deadline
        ? await Promise.race([next, deadline])
        : await next;
      if (done) break;
      if (total + value.byteLength > limit) {
        void reader.cancel().catch(() => undefined);
        throw new BodyTooLargeError();
      }
      chunks.push(value);
      total += value.byteLength;
    }
  } catch (error) {
    if (error instanceof BodyReadTimeoutError && reader) {
      void reader.cancel().catch(() => undefined);
    }
    throw error;
  } finally {
    if (timeout) clearTimeout(timeout);
    reader?.releaseLock();
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
    const value = JSON.parse(
      new TextDecoder("utf-8", { fatal: true }).decode(body),
    ) as unknown;
    return value !== null && typeof value === "object"
      ? (value as Record<string, unknown>)
      : null;
  } catch {
    return null;
  }
}

function isSafeOrganizationId(value: string) {
  return /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(value);
}

function invalidUpstreamResponse() {
  return protocolError(
    502,
    "DEPENDENCY_UNAVAILABLE",
    "Workbench upstream response is invalid",
  );
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

function protocolError(status: number, code: string, message: string) {
  return NextResponse.json(
    { code, message, requestId: "", fieldErrors: [] },
    { status, headers: safeJSONHeaders() },
  );
}

function safeJSONHeaders() {
  return new Headers({
    "Content-Type": "application/json",
    "Cache-Control": "private, no-store",
    "X-Content-Type-Options": "nosniff",
  });
}

class BodyTooLargeError extends Error {}
class BodyReadTimeoutError extends Error {}

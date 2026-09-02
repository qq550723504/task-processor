import { NextResponse } from "next/server";
import {
  findNodeAtLocation,
  parseTree,
  type Node as JSONNode,
  type ParseError,
} from "jsonc-parser";
import { z } from "zod";

import {
  parseWorkbenchContextPayload,
  parseWorkbenchErrorEnvelopePayload,
} from "@/lib/api/workbench-context";
import { newRequestLogId } from "@/lib/server/request-log";

export const WORKBENCH_COOKIE_NAME = "shuomi_effective_organization";
const EXPECTED_ORGANIZATION_ID_HEADER = "X-Expected-Organization-ID";

const DEFAULT_SERVICE_API_BASE = "http://localhost:8085/api/v1";
const SWITCH_REQUEST_BODY_MAX_BYTES = 4 * 1024;
const STORE_REQUEST_BODY_MAX_BYTES = 16 * 1024;
const REQUEST_BODY_READ_TIMEOUT_MS = 15_000;
const UPSTREAM_RESPONSE_MAX_BYTES = 1024 * 1024;
const STORE_QUERY_MAX_BYTES = 2 * 1024;
const REQUEST_ID_MAX_BYTES = 128;

export type WorkbenchResponseContract =
  | "context"
  | "context-switch"
  | "store-list"
  | "store-create"
  | "store-item"
  | "store-delete";

type WorkbenchRequestContract =
  | "context-get"
  | "context-switch"
  | "store-list"
  | "store-create"
  | "store-get"
  | "store-update"
  | "store-delete"
  | "store-enable"
  | "store-disable"
  | "store-resume";

type WorkbenchRouteDescriptor = {
  requestContract: WorkbenchRequestContract;
  responseContract: WorkbenchResponseContract;
  upstreamPath: string;
};

type WorkbenchRouteDefinition = Omit<
  WorkbenchRouteDescriptor,
  "upstreamPath"
> & {
  method: "GET" | "PUT" | "POST" | "DELETE";
  resolveUpstreamPath: (path: string[]) => string | null;
};

type WorkbenchUpstreamRequest = {
  url: string;
  init: RequestInit;
  responseContract: WorkbenchResponseContract;
  expectedStoreId?: string;
};

type ParsedJSONBody = {
  payload: Record<string, unknown>;
  root: JSONNode;
  text: string;
};

const canonicalUUIDSchema = z.string().refine(isCanonicalUUID);
const positiveSafeIntegerSchema = z
  .number()
  .int()
  .min(1)
  .max(Number.MAX_SAFE_INTEGER);
const nonnegativeSafeIntegerSchema = z
  .number()
  .int()
  .min(0)
  .max(Number.MAX_SAFE_INTEGER);
const normalizedPublicString = (minimum: number, maximum: number) =>
  z
    .string()
    .refine(
      (value) =>
        value === value.trim() &&
        !hasUnicodeControl(value) &&
        Array.from(value).length >= minimum &&
        Array.from(value).length <= maximum,
    );
const utcRFC3339Schema = z.string().refine(isUTCRFC3339Timestamp);
const storeResponseSchema = z
  .object({
    id: canonicalUUIDSchema,
    name: normalizedPublicString(1, 120),
    platform: z.literal("shein"),
    region: normalizedPublicString(1, 64),
    externalStoreId: normalizedPublicString(0, 128),
    lifecycleStatus: z.enum([
      "provisioning",
      "active",
      "disabled",
      "deleting",
    ]),
    connectionStatus: z.enum([
      "disconnected",
      "connected",
      "expired",
      "unavailable",
    ]),
    version: positiveSafeIntegerSchema,
    createdAt: utcRFC3339Schema,
    updatedAt: utcRFC3339Schema,
  })
  .strict();
const quotaResponseSchema = z
  .object({
    used: nonnegativeSafeIntegerSchema,
    reserved: nonnegativeSafeIntegerSchema,
    limit: positiveSafeIntegerSchema.nullable(),
    allowed: z.boolean(),
    reason: z.enum(["", "subscription_required", "store_limit_reached"]),
  })
  .strict()
  .superRefine((quota, refinement) => {
    const consumed = quota.used + quota.reserved;
    const valid =
      Number.isSafeInteger(consumed) && quota.limit === null
        ? !quota.allowed && quota.reason === "subscription_required"
        : quota.limit !== null &&
          Number.isSafeInteger(consumed) &&
          quota.allowed === consumed < quota.limit &&
          quota.reason ===
            (consumed < quota.limit
              ? ""
              : "store_limit_reached");
    if (!valid) {
      refinement.addIssue({
        code: "custom",
        message: "Quota state is inconsistent",
      });
    }
  });
const listStoresResponseSchema = z
  .object({
    items: z.array(storeResponseSchema).max(100),
    quota: quotaResponseSchema,
    pagination: z
      .object({
        page: positiveSafeIntegerSchema,
        pageSize: positiveSafeIntegerSchema.max(100),
        total: nonnegativeSafeIntegerSchema,
      })
      .strict(),
  })
  .strict()
  .superRefine((response, refinement) => {
    if (
      response.items.length > response.pagination.pageSize ||
      response.items.length > response.pagination.total
    ) {
      refinement.addIssue({
        code: "custom",
        message: "Pagination state is inconsistent",
      });
    }
  });
const deleteStoreResponseSchema = z
  .object({
    id: canonicalUUIDSchema,
    deleted: z.literal(true),
    version: positiveSafeIntegerSchema,
  })
  .strict();

const documentedWorkbenchErrorStatuses: Readonly<Record<string, number>> = {
  INVALID_REQUEST: 400,
  AUTHENTICATION_REQUIRED: 401,
  ORGANIZATION_SELECTION_REQUIRED: 409,
  ORGANIZATION_ACCESS_DENIED: 403,
  ORGANIZATION_ACCESS_REVOKED: 403,
  ORGANIZATION_SUSPENDED: 403,
  PERMISSION_DENIED: 403,
  DEPENDENCY_UNAVAILABLE: 503,
  STORE_NOT_FOUND: 404,
  STORE_ALREADY_EXISTS: 409,
  STORE_VERSION_CONFLICT: 409,
  STORE_INVALID_STATE: 422,
  SUBSCRIPTION_REQUIRED: 409,
  STORE_LIMIT_REACHED: 409,
  ORGANIZATION_CONTEXT_CHANGED: 409,
};

const workbenchRouteAllowlist = [
  routeDefinition("GET", "context-get", "context", (path) =>
    exactPath(path, "context") ? "context" : null,
  ),
  routeDefinition("PUT", "context-switch", "context-switch", (path) =>
    exactPath(path, "context", "effective-organization")
      ? "context/effective-organization"
      : null,
  ),
  routeDefinition("GET", "store-list", "store-list", (path) =>
    exactPath(path, "stores") ? "stores" : null,
  ),
  routeDefinition("POST", "store-create", "store-create", (path) =>
    exactPath(path, "stores") ? "stores" : null,
  ),
  routeDefinition("GET", "store-get", "store-item", storeItemPath),
  routeDefinition("PUT", "store-update", "store-item", storeItemPath),
  routeDefinition("DELETE", "store-delete", "store-delete", storeItemPath),
  routeDefinition("POST", "store-enable", "store-item", (path) =>
    storeActionPath(path, "enable"),
  ),
  routeDefinition("POST", "store-disable", "store-item", (path) =>
    storeActionPath(path, "disable"),
  ),
  routeDefinition("POST", "store-resume", "store-item", (path) =>
    storeActionPath(path, "resume"),
  ),
] as const satisfies readonly WorkbenchRouteDefinition[];

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
  if (
    route.requestContract.startsWith("store-") &&
    new URL(request.url).pathname !==
      `/api/workbench/${route.upstreamPath}`
  ) {
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
  headers.set("X-Request-ID", readRequestId(request.headers));
  let body: string | undefined;
  let query = "";

  if (route.requestContract === "context-switch") {
    const rawBody = await readRequestBody(request, SWITCH_REQUEST_BODY_MAX_BYTES);
    if (rawBody instanceof Response) return rawBody;
    const organizationId = parseSwitchOrganizationBody(rawBody);
    if (!organizationId) {
      return protocolError(400, "INVALID_REQUEST", "Request body is invalid");
    }
    body = JSON.stringify({ organizationId });
    headers.set("Content-Type", "application/json");
    headers.set("X-Requested-Organization-ID", organizationId);
  } else {
    const selectedOrganization = readSelectedOrganization(request);
    if (selectedOrganization instanceof Response) return selectedOrganization;
    if (route.requestContract.startsWith("store-")) {
      const expectedOrganization = readExpectedOrganizationAssertion(
        request.headers,
      );
      if (
        !expectedOrganization ||
        !selectedOrganization ||
        expectedOrganization !== selectedOrganization
      ) {
        return protocolError(
          409,
          "ORGANIZATION_CONTEXT_CHANGED",
          "Organization context changed",
        );
      }
    }
    if (selectedOrganization) {
      headers.set("X-Requested-Organization-ID", selectedOrganization);
    }

    switch (route.requestContract) {
      case "context-get":
        break;
      case "store-list": {
        const parsedQuery = parseStoreListQuery(new URL(request.url).searchParams);
        if (parsedQuery === null) {
          return protocolError(400, "INVALID_REQUEST", "Query is invalid");
        }
        query = parsedQuery ? `?${parsedQuery}` : "";
        break;
      }
      case "store-create": {
        if (!hasNoQuery(request)) {
          return protocolError(400, "INVALID_REQUEST", "Query is not allowed");
        }
        const idempotencyKey = readCanonicalUUIDHeader(
          request.headers,
          "Idempotency-Key",
        );
        if (!idempotencyKey) {
          return protocolError(400, "INVALID_REQUEST", "Idempotency-Key is invalid");
        }
        const rawBody = await readRequestBody(request, STORE_REQUEST_BODY_MAX_BYTES);
        if (rawBody instanceof Response) return rawBody;
        const payload = parseStoreBody(rawBody, "create");
        if (!payload) {
          return protocolError(400, "INVALID_REQUEST", "Request body is invalid");
        }
        body = JSON.stringify(payload);
        headers.set("Content-Type", "application/json");
        headers.set("Idempotency-Key", idempotencyKey);
        break;
      }
      case "store-update": {
        if (!hasNoQuery(request)) {
          return protocolError(400, "INVALID_REQUEST", "Query is not allowed");
        }
        const ifMatch = readIfMatchHeader(request.headers);
        if (!ifMatch) {
          return protocolError(400, "INVALID_REQUEST", "If-Match is invalid");
        }
        const rawBody = await readRequestBody(request, STORE_REQUEST_BODY_MAX_BYTES);
        if (rawBody instanceof Response) return rawBody;
        const payload = parseStoreBody(rawBody, "update");
        if (!payload) {
          return protocolError(400, "INVALID_REQUEST", "Request body is invalid");
        }
        body = JSON.stringify(payload);
        headers.set("Content-Type", "application/json");
        headers.set("If-Match", ifMatch);
        break;
      }
      case "store-delete": {
        if (!hasNoQuery(request) || !(await requestHasNoBody(request))) {
          return protocolError(400, "INVALID_REQUEST", "Request is invalid");
        }
        const idempotencyKey = readCanonicalUUIDHeader(
          request.headers,
          "Idempotency-Key",
        );
        const ifMatch = readIfMatchHeader(request.headers);
        if (!idempotencyKey || !ifMatch) {
          return protocolError(400, "INVALID_REQUEST", "Required header is invalid");
        }
        headers.set("Idempotency-Key", idempotencyKey);
        headers.set("If-Match", ifMatch);
        break;
      }
      case "store-enable":
      case "store-disable":
      case "store-resume": {
        if (!hasNoQuery(request) || !(await requestHasNoBody(request))) {
          return protocolError(400, "INVALID_REQUEST", "Request is invalid");
        }
        const ifMatch = readIfMatchHeader(request.headers);
        if (!ifMatch) {
          return protocolError(400, "INVALID_REQUEST", "If-Match is invalid");
        }
        headers.set("If-Match", ifMatch);
        break;
      }
      case "store-get":
        if (!hasNoQuery(request)) {
          return protocolError(400, "INVALID_REQUEST", "Query is not allowed");
        }
        break;
    }
  }

  return {
    url: `${getServiceRoot()}/workbench/${route.upstreamPath}${query}`,
    init: {
      method: request.method.toUpperCase(),
      headers,
      body,
      cache: "no-store",
    },
    responseContract: route.responseContract,
    expectedStoreId:
      route.responseContract === "store-item" ||
      route.responseContract === "store-delete"
        ? path[1]
        : undefined,
  };
}

export async function buildWorkbenchBrowserResponse(
  upstream: Response,
  contract: WorkbenchResponseContract = "context",
  expectedStoreId?: string,
) {
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

  const parsedBody = parseJSONBody(body);
  const payload = parsedBody?.payload ?? null;
  const parsedError =
    upstream.status >= 400 && upstream.status <= 599
      ? parseWorkbenchErrorEnvelopePayload(payload)
      : null;
  if (!upstream.ok) {
    if (!parsedError?.success) return invalidUpstreamResponse();
    if (
      documentedWorkbenchErrorStatuses[parsedError.data.code] !== undefined &&
      documentedWorkbenchErrorStatuses[parsedError.data.code] !== upstream.status
    ) {
      return invalidUpstreamResponse();
    }
    const response = new NextResponse(JSON.stringify(parsedError.data), {
      status: upstream.status,
      headers: safeJSONHeaders(),
    });
    if (
      contract !== "context-switch" &&
      (parsedError.data.code === "ORGANIZATION_ACCESS_REVOKED" ||
        parsedError.data.code === "ORGANIZATION_ACCESS_DENIED")
    ) {
      clearSelectionCookie(response);
    }
    return response;
  }

  const validatedPayload = parseSuccessfulPayload(
    contract,
    upstream.status,
    parsedBody,
    expectedStoreId,
  );
  if (!validatedPayload) {
    return invalidUpstreamResponse();
  }
  const response = new NextResponse(JSON.stringify(validatedPayload), {
    status: upstream.status,
    headers: safeJSONHeaders(),
  });

  if (contract === "context" || contract === "context-switch") {
    const effectiveOrganizationId = validatedPayload.effectiveOrganizationId;
    if (effectiveOrganizationId) {
      response.cookies.set(WORKBENCH_COOKIE_NAME, String(effectiveOrganizationId), {
        httpOnly: true,
        sameSite: "lax",
        path: "/",
        secure: process.env.NODE_ENV !== "development",
      });
    } else {
      clearSelectionCookie(response);
    }
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

function resolveAllowlistedRoute(
  method: string,
  path: string[],
): WorkbenchRouteDescriptor | null {
  if (!path.every(isSafePathSegment)) return null;
  const verb = method.toUpperCase();
  for (const route of workbenchRouteAllowlist) {
    if (route.method !== verb) continue;
    const upstreamPath = route.resolveUpstreamPath(path);
    if (upstreamPath === null) continue;
    return {
      requestContract: route.requestContract,
      responseContract: route.responseContract,
      upstreamPath,
    };
  }
  return null;
}

function routeDefinition(
  method: WorkbenchRouteDefinition["method"],
  requestContract: WorkbenchRequestContract,
  responseContract: WorkbenchResponseContract,
  resolveUpstreamPath: WorkbenchRouteDefinition["resolveUpstreamPath"],
): WorkbenchRouteDefinition {
  return { method, requestContract, responseContract, resolveUpstreamPath };
}

function exactPath(path: string[], ...segments: string[]) {
  return (
    path.length === segments.length &&
    segments.every((segment, index) => path[index] === segment)
  );
}

function storeItemPath(path: string[]) {
  return path.length === 2 && path[0] === "stores" && isCanonicalUUID(path[1]!)
    ? `stores/${path[1]}`
    : null;
}

function storeActionPath(path: string[], action: "enable" | "disable" | "resume") {
  return path.length === 3 &&
    path[0] === "stores" &&
    isCanonicalUUID(path[1]!) &&
    path[2] === action
    ? `stores/${path[1]}/${action}`
    : null;
}

function isSafePathSegment(value: string) {
  return Boolean(
    value &&
    value !== "." &&
    value !== ".." &&
    !value.includes("/") &&
    !value.includes("\\") &&
    !value.includes("%") &&
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

async function readRequestBody(request: Request, limit: number) {
  const contentLength = readContentLength(request.headers);
  if (contentLength !== null && contentLength > limit) {
    void request.body?.cancel().catch(() => undefined);
    return protocolError(413, "INVALID_REQUEST", "Request body is too large");
  }
  try {
    return await readBodyWithinLimit(
      request.body,
      limit,
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
      return protocolError(413, "INVALID_REQUEST", "Request body is too large");
    }
    return protocolError(400, "INVALID_REQUEST", "Request body is invalid");
  }
}

function readSelectedOrganization(request: Request) {
  const selectedCookie = readCookie(
    request.headers.get("cookie"),
    WORKBENCH_COOKIE_NAME,
  );
  if (selectedCookie === null) {
    return protocolError(400, "INVALID_REQUEST", "Selection cookie is invalid");
  }
  const selectedOrganization = selectedCookie.trim();
  if (selectedOrganization && !isSafeOrganizationId(selectedOrganization)) {
    return protocolError(400, "INVALID_REQUEST", "Selection cookie is invalid");
  }
  return selectedOrganization;
}

function readExpectedOrganizationAssertion(headers: Headers) {
  const value = headers.get(EXPECTED_ORGANIZATION_ID_HEADER);
  return value && !value.includes(",") && isSafeOrganizationId(value)
    ? value
    : "";
}

function hasNoQuery(request: Request) {
  return new URL(request.url).searchParams.size === 0;
}

function parseStoreListQuery(search: URLSearchParams) {
  const allowed = new Set(["page", "pageSize", "platform", "status"]);
  const values = new Map<string, string>();
  for (const [key, value] of search) {
    if (!allowed.has(key) || values.has(key) || value === "") return null;
    values.set(key, value);
  }
  if (!validCanonicalInteger(values.get("page"), 1, Number.MAX_SAFE_INTEGER)) {
    return null;
  }
  if (!validCanonicalInteger(values.get("pageSize"), 1, 100)) return null;
  const platform = values.get("platform");
  if (platform !== undefined && platform !== "shein") return null;
  const status = values.get("status");
  if (
    status !== undefined &&
    !["provisioning", "active", "disabled", "deleting"].includes(status)
  ) {
    return null;
  }
  const rebuilt = new URLSearchParams();
  for (const key of ["page", "pageSize", "platform", "status"]) {
    const value = values.get(key);
    if (value !== undefined) rebuilt.set(key, value);
  }
  const encoded = rebuilt.toString();
  return encoded.length <= STORE_QUERY_MAX_BYTES ? encoded : null;
}

function validCanonicalInteger(
  value: string | undefined,
  minimum: number,
  maximum: number,
) {
  if (value === undefined) return true;
  if (!/^(0|[1-9][0-9]*)$/.test(value)) return false;
  const parsed = Number(value);
  return (
    Number.isSafeInteger(parsed) && parsed >= minimum && parsed <= maximum
  );
}

function parseStoreBody(body: Uint8Array, kind: "create" | "update") {
  let text: string;
  try {
    text = new TextDecoder("utf-8", { fatal: true }).decode(body);
  } catch {
    return null;
  }
  const errors: ParseError[] = [];
  const root = parseTree(text, errors, {
    allowEmptyContent: false,
    allowTrailingComma: false,
    disallowComments: true,
  });
  if (errors.length > 0 || root?.type !== "object") return null;
  const required =
    kind === "create"
      ? new Set(["name", "platform", "region"])
      : new Set(["name", "region"]);
  const allowed =
    kind === "create"
      ? new Set(["name", "platform", "region", "externalStoreId"])
      : new Set(["name", "region"]);
  const values = new Map<string, string>();
  for (const property of root.children ?? []) {
    const key = property.children?.[0];
    const value = property.children?.[1];
    const name = key?.type === "string" ? String(key.value) : "";
    if (
      property.type !== "property" ||
      property.children?.length !== 2 ||
      !allowed.has(name) ||
      values.has(name) ||
      value?.type !== "string"
    ) {
      return null;
    }
    values.set(name, String(value.value));
  }
  if ([...required].some((key) => !values.has(key))) return null;
  const name = normalizeStoreValue(values.get("name") ?? "", 120, true);
  const region = normalizeStoreValue(values.get("region") ?? "", 64, true);
  if (name === null || region === null) return null;
  if (kind === "update") return { name, region };
  if (values.get("platform") !== "shein") return null;
  const externalStoreId = normalizeStoreValue(
    values.get("externalStoreId") ?? "",
    128,
    false,
  );
  if (externalStoreId === null) return null;
  return values.has("externalStoreId")
    ? { name, platform: "shein", region, externalStoreId }
    : { name, platform: "shein", region };
}

function normalizeStoreValue(value: string, maximum: number, required: boolean) {
  if (hasUnicodeControl(value)) return null;
  const normalized = value.trim();
  if ((required && normalized === "") || Array.from(normalized).length > maximum) {
    return null;
  }
  return normalized;
}

function hasUnicodeControl(value: string) {
  return /\p{Cc}/u.test(value);
}

function readCanonicalUUIDHeader(headers: Headers, name: string) {
  const value = headers.get(name);
  return value && isCanonicalUUID(value) ? value : "";
}

function readRequestId(headers: Headers) {
  const value = headers.get("X-Request-ID")?.trim() ?? "";
  if (
    value &&
    new TextEncoder().encode(value).byteLength <= REQUEST_ID_MAX_BYTES &&
    /^[A-Za-z0-9][A-Za-z0-9._:/-]*$/.test(value)
  ) {
    return value;
  }
  return newRequestLogId();
}

function readIfMatchHeader(headers: Headers) {
  const value = headers.get("If-Match") ?? "";
  const matched = /^"([1-9][0-9]*)"$/.exec(value);
  if (!matched) return "";
  const version = Number(matched[1]);
  return Number.isSafeInteger(version) && version > 0 ? value : "";
}

async function requestHasNoBody(request: Request) {
  const contentLength = readContentLength(request.headers);
  if (contentLength !== null && contentLength !== 0) {
    void request.body?.cancel().catch(() => undefined);
    return false;
  }
  if (!request.body) return true;
  try {
    const body = await readBodyWithinLimit(
      request.body,
      0,
      REQUEST_BODY_READ_TIMEOUT_MS,
    );
    return body.byteLength === 0;
  } catch {
    return false;
  }
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

function parseJSONBody(body: Uint8Array): ParsedJSONBody | null {
  try {
    const text = new TextDecoder("utf-8", { fatal: true }).decode(body);
    const value = JSON.parse(text) as unknown;
    if (value === null || typeof value !== "object" || Array.isArray(value)) {
      return null;
    }
    const errors: ParseError[] = [];
    const root = parseTree(text, errors, {
      allowEmptyContent: false,
      allowTrailingComma: false,
      disallowComments: true,
    });
    return errors.length === 0 && root?.type === "object" && hasUniqueObjectKeys(root)
      ? { payload: value as Record<string, unknown>, root, text }
      : null;
  } catch {
    return null;
  }
}

function isSafeOrganizationId(value: string) {
  return /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(value);
}

function isCanonicalUUID(value: string) {
  return (
    value !== "00000000-0000-0000-0000-000000000000" &&
    /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(
      value,
    )
  );
}

function isUTCRFC3339Timestamp(value: string) {
  const match =
    /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?Z$/.exec(
      value,
    );
  if (!match) return false;
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const hour = Number(match[4]);
  const minute = Number(match[5]);
  const second = Number(match[6]);
  if (year < 1 || month < 1 || month > 12 || hour > 23 || minute > 59 || second > 59) {
    return false;
  }
  const daysInMonth = new Date(Date.UTC(year, month, 0)).getUTCDate();
  return day >= 1 && day <= daysInMonth;
}

function hasCanonicalStoreIntegerTokens(
  contract: WorkbenchResponseContract,
  body: ParsedJSONBody,
) {
  if (!hasUniqueObjectKeys(body.root)) return false;
  if (contract === "store-create" || contract === "store-item") {
    return hasCanonicalIntegerNode(
      findNodeAtLocation(body.root, ["version"]),
      body.text,
      false,
    );
  }
  if (contract === "store-delete") {
    return hasCanonicalIntegerNode(
      findNodeAtLocation(body.root, ["version"]),
      body.text,
      false,
    );
  }
  if (contract !== "store-list") return true;

  const items = findNodeAtLocation(body.root, ["items"]);
  if (items?.type === "array") {
    for (const item of items.children ?? []) {
      if (
        !hasCanonicalIntegerNode(
          findNodeAtLocation(item, ["version"]),
          body.text,
          false,
        )
      ) {
        return false;
      }
    }
  }
  return (
    hasCanonicalIntegerNode(
      findNodeAtLocation(body.root, ["quota", "used"]),
      body.text,
      true,
    ) &&
    hasCanonicalIntegerNode(
      findNodeAtLocation(body.root, ["quota", "reserved"]),
      body.text,
      true,
    ) &&
    hasCanonicalIntegerNode(
      findNodeAtLocation(body.root, ["quota", "limit"]),
      body.text,
      false,
    ) &&
    hasCanonicalIntegerNode(
      findNodeAtLocation(body.root, ["pagination", "page"]),
      body.text,
      false,
    ) &&
    hasCanonicalIntegerNode(
      findNodeAtLocation(body.root, ["pagination", "pageSize"]),
      body.text,
      false,
    ) &&
    hasCanonicalIntegerNode(
      findNodeAtLocation(body.root, ["pagination", "total"]),
      body.text,
      true,
    )
  );
}

function hasUniqueObjectKeys(root: JSONNode): boolean {
  const pending = [root];
  while (pending.length > 0) {
    const node = pending.pop()!;
    if (node.type === "object") {
      const names = new Set<string>();
      for (const property of node.children ?? []) {
        const key = property.children?.[0];
        const value = property.children?.[1];
        if (
          property.type !== "property" ||
          key?.type !== "string" ||
          !value ||
          names.has(String(key.value))
        ) {
          return false;
        }
        names.add(String(key.value));
        pending.push(value);
      }
    } else if (node.type === "array") {
      for (const child of node.children ?? []) pending.push(child);
    }
  }
  return true;
}

function hasCanonicalIntegerNode(
  node: JSONNode | undefined,
  text: string,
  allowZero: boolean,
) {
  if (!node || node.type !== "number") return true;
  const token = text.slice(node.offset, node.offset + node.length);
  const canonicalPattern = allowZero ? /^(0|[1-9][0-9]*)$/ : /^[1-9][0-9]*$/;
  if (!canonicalPattern.test(token)) return false;
  try {
    return BigInt(token) <= BigInt("9007199254740991");
  } catch {
    return false;
  }
}

function parseSuccessfulPayload(
  contract: WorkbenchResponseContract,
  status: number,
  body: ParsedJSONBody | null,
  expectedStoreId?: string,
): Record<string, unknown> | null {
  if (!body) return null;
  if (contract === "context" || contract === "context-switch") {
    if (status !== 200) return null;
    const parsed = parseWorkbenchContextPayload(body.payload);
    return parsed.success ? parsed.data : null;
  }
  const parser =
    contract === "store-list"
      ? listStoresResponseSchema
      : contract === "store-delete"
        ? deleteStoreResponseSchema
        : storeResponseSchema;
  const expectedStatus = contract === "store-create" ? 201 : 200;
  if (status !== expectedStatus) return null;
  if (!hasCanonicalStoreIntegerTokens(contract, body)) return null;
  const parsed = parser.safeParse(body.payload);
  if (!parsed.success) return null;
  if (
    (contract === "store-item" || contract === "store-delete") &&
    expectedStoreId !== undefined &&
    (!("id" in parsed.data) || parsed.data.id !== expectedStoreId)
  ) {
    return null;
  }
  return parsed.data;
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

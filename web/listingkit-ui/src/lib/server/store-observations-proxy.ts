import { z } from "zod";
import {
  observationBeginSchema,
  observationCapabilitiesSchema,
  observationCommandSchema,
  observationEnvelopeSchema,
  observationID,
  observationKindSchema,
  observationListSchema,
  observationRecordSchema,
  observationSyncSchema,
  observationTracksSchema,
  type ObservationKind,
} from "@/lib/api/store-observations";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { serviceOrigin } from "./account-proxy";
import { hasTrustedSameOriginWrite } from "./same-origin-write";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
const safeID = (v: string | null): v is string =>
  typeof v === "string" && /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(v);
const fail = (status: number, code: string) =>
  Response.json(
    { code },
    {
      status,
      headers: {
        "Cache-Control": "private, no-store",
        "X-Content-Type-Options": "nosniff",
      },
    },
  );
type Endpoint = {
  path: string;
  kind: ObservationKind;
  schema: z.ZodType;
  write: boolean;
  begin: boolean;
  store?: string;
  sync?: string;
  record?: string;
  command?: string;
};
function observationEndpoint(url: URL, method: string): Endpoint | null {
  const prefix = "/api/workbench/store-observations/";
  if (!url.pathname.startsWith(prefix) || url.pathname.length > 2048)
    return null;
  const path = url.pathname.slice(prefix.length);
  const segments = path.split("/");
  const parsed = observationKindSchema.safeParse(segments[0]);
  if (!parsed.success) return null;
  const kind = parsed.data;
  const e: Endpoint = {
    path,
    kind,
    schema: observationListSchema,
    write: false,
    begin: false,
  };
  if (segments.length === 1 && method === "GET") {
    if (url.search.length > 8192) return null;
    for (const k of url.searchParams.keys())
      if (
        !["storeId", "syncId", "keyword", "status", "after", "limit"].includes(
          k,
        ) ||
        url.searchParams.getAll(k).length !== 1
      )
        return null;
    for (const k of ["storeId", "syncId"])
      if (
        url.searchParams.has(k) &&
        !observationID.safeParse(url.searchParams.get(k)).success
      )
        return null;
    const limit = url.searchParams.get("limit") ?? "20";
    if (!/^[1-9][0-9]?$/.test(limit) || Number(limit) > 50) return null;
    const keyword = url.searchParams.get("keyword") ?? "";
    if (
      Array.from(keyword).length > 200 ||
      keyword.trim() !== keyword ||
      /\p{Cc}/u.test(keyword) ||
      (url.searchParams.get("after") ?? "").length > 4096
    )
      return null;
    const status = url.searchParams.get("status") ?? "";
    if (
      !(
        kind === "products"
          ? ["", "active", "off", "unknown"]
          : [
              "",
              "1",
              "2",
              "3",
              "4",
              "5",
              "6",
              "7",
              "8",
              "9",
              "transit",
              "exceptional",
              "unknown",
            ]
      ).includes(status)
    )
      return null;
    return e;
  }
  if (url.search || url.href.endsWith("?")) return null;
  if (
    segments.length === 2 &&
    segments[1] === "capabilities" &&
    method === "GET"
  ) {
    e.schema = observationCapabilitiesSchema;
    return e;
  }
  if (segments.length === 2 && segments[1] === "syncs" && method === "POST") {
    e.schema = observationCommandSchema;
    e.write = e.begin = true;
    return e;
  }
  if (
    segments.length === 3 &&
    segments[1] === "commands" &&
    observationID.safeParse(segments[2]).success &&
    method === "GET"
  ) {
    e.schema = observationCommandSchema;
    e.command = segments[2];
    return e;
  }
  if (
    ((segments.length === 3 && method === "GET") ||
      (segments.length === 4 &&
        segments[3] === "ensure" &&
        method === "POST")) &&
    segments[1] === "syncs" &&
    observationID.safeParse(segments[2]).success
  ) {
    e.schema = observationSyncSchema;
    e.sync = segments[2];
    e.write = method === "POST";
    return e;
  }
  if (
    method === "GET" &&
    segments[1] === "stores" &&
    segments[3] === "syncs" &&
    segments[5] === "records" &&
    observationID.safeParse(segments[2]).success &&
    observationID.safeParse(segments[4]).success
  ) {
    let record: string;
    try {
      record = decodeURIComponent(segments[6] ?? "");
    } catch {
      return null;
    }
    if (
      !record ||
      Array.from(record).length > 128 ||
      record.trim() !== record ||
      /[\p{Cc}/\\]/u.test(record)
    )
      return null;
    e.store = segments[2];
    e.sync = segments[4];
    e.record = record;
    if (segments.length === 7) {
      e.schema = observationRecordSchema;
      return e;
    }
    if (
      kind === "orders" &&
      segments.length === 10 &&
      segments[7] === "packages" &&
      segments[9] === "track"
    ) {
      let pkg: string;
      try {
        pkg = decodeURIComponent(segments[8]);
      } catch {
        return null;
      }
      if (
        !pkg ||
        Array.from(pkg).length > 128 ||
        pkg.trim() !== pkg ||
        /[\p{Cc}/\\]/u.test(pkg)
      )
        return null;
      e.schema = observationTracksSchema;
      return e;
    }
  }
  return null;
}
function matches(e: Endpoint, data: unknown): boolean {
  if (e.schema === observationRecordSchema) {
    const v = observationRecordSchema.parse(data);
    return (
      v.id === e.record &&
      v.storeId === e.store &&
      v.syncId === e.sync &&
      (e.kind === "products" ? Boolean(v.product) : Boolean(v.order))
    );
  }
  if (e.schema === observationSyncSchema) {
    const v = observationSyncSchema.parse(data);
    return v.id === e.sync && v.kind === e.kind;
  }
  if (e.schema === observationCapabilitiesSchema)
    return observationCapabilitiesSchema.parse(data).kind === e.kind;
  if (e.schema === observationCommandSchema) {
    const v = observationCommandSchema.parse(data);
    return (
      v.input.kind === e.kind &&
      v.syncs.every(
        (s) =>
          s.kind === e.kind &&
          s.commandId === v.id &&
          v.input.stores.includes(s.storeId),
      ) &&
      new Set(v.syncs.map((s) => s.storeId)).size === v.syncs.length &&
      v.syncs.length === v.input.stores.length
    );
  }
  if (e.schema === observationListSchema) {
    const v = observationListSchema.parse(data);
    return (
      v.items.every((r) =>
        e.kind === "products" ? Boolean(r.product) : Boolean(r.order),
      ) &&
      v.syncs.every((s) => s.kind === e.kind) &&
      v.latest.every((s) => s.kind === e.kind)
    );
  }
  return true;
}
export async function proxyStoreObservations(
  request: Request,
  token: string,
  user: string,
): Promise<Response> {
  if (!token || !safeID(user)) return fail(401, "AUTHENTICATION_REQUIRED");
  if (request.headers.get("X-Expected-User-ID") !== user)
    return fail(409, "IDENTITY_CONTEXT_CHANGED");
  const cookies = (request.headers.get("cookie") ?? "")
    .split(";")
    .map((s) => s.trim())
    .filter((s) => s.startsWith(`${WORKBENCH_COOKIE_NAME}=`));
  let org: string | null = null;
  try {
    if (cookies.length === 1)
      org = decodeURIComponent(
        cookies[0].slice(WORKBENCH_COOKIE_NAME.length + 1),
      );
  } catch {}
  if (!safeID(org) || request.headers.get("X-Expected-Organization-ID") !== org)
    return fail(409, "ORGANIZATION_CONTEXT_CHANGED");
  const url = new URL(request.url);
  const endpoint = observationEndpoint(url, request.method);
  if (!endpoint) return fail(400, "INVALID_REQUEST");
  if (endpoint.write && !hasTrustedSameOriginWrite(request))
    return fail(403, "PERMISSION_DENIED");
  if (
    !endpoint.begin &&
    (request.body !== null ||
      request.headers.has("transfer-encoding") ||
      request.headers.has("idempotency-key") ||
      (request.headers.has("content-length") &&
        request.headers.get("content-length") !== "0"))
  ) {
    void request.body?.cancel().catch(() => undefined);
    return fail(400, "INVALID_REQUEST");
  }
  const origin = serviceOrigin();
  if (!origin) return fail(503, "DEPENDENCY_UNAVAILABLE");
  const controller = new AbortController();
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 20000);
  try {
    const upstream = new URL(
      `/api/v1/workbench/store-observations/${endpoint.path}`,
      origin,
    );
    upstream.search = url.search;
    const headers = new Headers({
      Authorization: `Bearer ${token}`,
      Accept: "application/json",
      "X-Requested-Organization-ID": org,
    });
    let body: string | undefined;
    if (endpoint.begin) {
      const key = request.headers.get("Idempotency-Key");
      if (
        !observationID.safeParse(key).success ||
        !/^application\/json(?:\s*;|$)/i.test(
          request.headers.get("content-type") ?? "",
        )
      )
        return fail(400, "INVALID_REQUEST");
      let raw: unknown;
      try {
        raw = await readBoundedStrictJSON(
          new Response(request.body, {
            headers: { "Content-Type": "application/json" },
          }),
          65536,
          controller.signal,
        );
      } catch {
        return fail(400, "INVALID_REQUEST");
      }
      const parsed = observationBeginSchema.safeParse(raw);
      if (!parsed.success || parsed.data.kind !== endpoint.kind)
        return fail(400, "INVALID_REQUEST");
      body = JSON.stringify(parsed.data);
      headers.set("Content-Type", "application/json");
      headers.set("Idempotency-Key", key!);
    }
    controller.signal.throwIfAborted();
    const response = await fetch(upstream, {
      method: request.method,
      headers,
      body,
      cache: "no-store",
      redirect: "manual",
      signal: controller.signal,
    });
    const payload = await readBoundedStrictJSON(
      response,
      2 << 20,
      controller.signal,
    );
    controller.signal.throwIfAborted();
    if (response.status !== 200) {
      const error = z
        .object({
          code: z.enum([
            "INVALID_REQUEST",
            "PERMISSION_DENIED",
            "NOT_FOUND",
            "REVISION_CONFLICT",
            "UNSUPPORTED_APPLICATION",
            "DEPENDENCY_UNAVAILABLE",
            "DEADLINE_EXCEEDED",
            "AUTHENTICATION_REQUIRED",
            "ORGANIZATION_ACCESS_DENIED",
            "ORGANIZATION_ACCESS_REVOKED",
            "ORGANIZATION_SUSPENDED",
            "ORGANIZATION_CONTEXT_CHANGED",
          ]),
        })
        .passthrough()
        .safeParse(payload);
      if (
        !error.success ||
        ![400, 401, 403, 404, 409, 503, 504].includes(response.status)
      )
        return fail(502, "INVALID_UPSTREAM_RESPONSE");
      return fail(response.status, error.data.code);
    }
    const parsed = observationEnvelopeSchema(endpoint.schema).safeParse(
      payload,
    );
    if (!parsed.success || !matches(endpoint, parsed.data.data))
      return fail(502, "INVALID_UPSTREAM_RESPONSE");
    if (parsed.data.organizationId !== org || parsed.data.userId !== user)
      return fail(409, "ORGANIZATION_CONTEXT_CHANGED");
    return Response.json(parsed.data, {
      headers: {
        "Cache-Control": "private, no-store",
        "X-Content-Type-Options": "nosniff",
      },
    });
  } catch {
    return fail(
      controller.signal.aborted ? 504 : 502,
      controller.signal.aborted
        ? "DEADLINE_EXCEEDED"
        : "DEPENDENCY_UNAVAILABLE",
    );
  } finally {
    clearTimeout(timer);
    request.signal.removeEventListener("abort", abort);
  }
}

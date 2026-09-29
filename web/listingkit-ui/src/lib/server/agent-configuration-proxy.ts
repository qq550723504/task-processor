import { z } from "zod";
import {
  catalogEntrySchema,
  catalogPageSchema,
  configReceiptSchema,
  configVersion,
  recentRunsSchema,
  templateInputSchema,
  templateSchema,
  templatesPageSchema,
  templateRefSchema,
} from "@/lib/contracts/agent-configuration";
import { knowledgeId } from "@/lib/api/knowledge";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { hasTrustedSameOriginWrite } from "./same-origin-write";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
import { hasEmptyBody } from "./members-proxy";
const safeHeaders = {
  "Cache-Control": "private, no-store",
  "X-Content-Type-Options": "nosniff",
};
export const configFailure = (status: number, code: string) =>
  Response.json({ code }, { status, headers: safeHeaders });
function endpoint(url: URL, method: string) {
  const p = url.pathname.slice("/api/workbench/agents/".length).split("/");
  let output: z.ZodType;
  let input: z.ZodType | undefined;
  if (p.length === 1 && ["market", "mine"].includes(p[0]) && method === "GET")
    output = catalogPageSchema;
  else if (p[0] === "product.title.agent") {
    if (p.length === 1 && method === "GET") output = catalogEntrySchema;
    else if (
      p.length === 2 &&
      ["enable", "disable"].includes(p[1]) &&
      method === "POST"
    ) {
      output = configReceiptSchema;
      input = z.strictObject({});
    } else if (
      p.length === 2 &&
      p[1] === "default-template" &&
      method === "PUT"
    ) {
      output = configReceiptSchema;
      input = z.union([
        templateRefSchema,
        z.strictObject({ templateId: z.null(), revision: z.null() }),
      ]);
    } else if (
      p[1] === "templates" &&
      p.length === 2 &&
      ["GET", "POST"].includes(method)
    ) {
      output = method === "GET" ? templatesPageSchema : configReceiptSchema;
      input = method === "POST" ? templateInputSchema : undefined;
    } else if (p[1] === "templates" && knowledgeId.safeParse(p[2]).success) {
      if (p.length === 3 && ["GET", "PUT"].includes(method)) {
        output = method === "GET" ? templateSchema : configReceiptSchema;
        input = method === "PUT" ? templateInputSchema : undefined;
      } else if (p.length === 4 && p[3] === "archive" && method === "POST") {
        output = configReceiptSchema;
        input = z.strictObject({});
      } else if (
        p.length === 5 &&
        p[3] === "revisions" &&
        configVersion.safeParse(p[4]).success &&
        method === "GET"
      )
        output = templateSchema;
      else return null;
    } else if (p.length === 2 && p[1] === "recent-runs" && method === "GET")
      output = recentRunsSchema;
    else return null;
  } else return null;
  const list =
    output === catalogPageSchema ||
    output === templatesPageSchema ||
    output === recentRunsSchema;
  if (url.search.length > 512 || (url.search && !list)) return null;
  for (const [k, v] of url.searchParams) {
    if (url.searchParams.getAll(k).length !== 1) return null;
    if (k === "pageSize") {
      if (
        !/^[1-9][0-9]*$/.test(v) ||
        Number(v) > (output === recentRunsSchema ? 20 : 100)
      )
        return null;
    } else if (k === "cursor") {
      if (!/^[A-Za-z0-9_-]{1,180}$/.test(v)) return null;
    } else if (k === "activation" && p[0] === "mine") {
      if (!["ENABLED", "DISABLED"].includes(v)) return null;
    } else if (k === "lifecycle" && output === templatesPageSchema) {
      if (!["ACTIVE", "ARCHIVED"].includes(v)) return null;
    } else return null;
  }
  return {
    path: p.join("/"),
    output,
    input,
    firstEnable: p[1] === "enable",
    create: p[1] === "templates" && p.length === 2 && method === "POST",
  };
}
export async function proxyAgentConfiguration(
  request: Request,
  token: string,
  userId: string,
): Promise<Response> {
  if (!token || !userId) return configFailure(401, "AUTHENTICATION_REQUIRED");
  if (request.headers.get("X-Expected-User-ID") !== userId)
    return configFailure(409, "IDENTITY_CONTEXT_CHANGED");
  const url = new URL(request.url),
    route = endpoint(url, request.method);
  if (
    !route ||
    request.url.endsWith("?") ||
    request.headers.has("content-encoding")
  )
    return configFailure(400, "INVALID_REQUEST");
  const write = !!route.input;
  if (write && !hasTrustedSameOriginWrite(request))
    return configFailure(403, "FORBIDDEN");
  const cookies = (request.headers.get("cookie") ?? "")
    .split(";")
    .map((v) => v.trim())
    .filter((v) => v.startsWith(WORKBENCH_COOKIE_NAME + "="));
  let org = "";
  try {
    if (cookies.length === 1)
      org = decodeURIComponent(
        cookies[0].slice(WORKBENCH_COOKIE_NAME.length + 1),
      );
  } catch {}
  if (
    !/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(org) ||
    org !== request.headers.get("X-Expected-Organization-ID")
  )
    return configFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
  const controller = new AbortController(),
    abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 12000);
  let dispatched = false;
  try {
    const headers = new Headers({
      Accept: "application/json",
      Authorization: "Bearer " + token,
      "X-Requested-Organization-ID": org,
    });
    let body: string | undefined;
    if (write) {
      const key = request.headers.get("Idempotency-Key");
      if (!knowledgeId.safeParse(key).success)
        return configFailure(400, "INVALID_REQUEST");
      headers.set("Idempotency-Key", key!);
      const match = request.headers.get("If-Match"),
        absent = request.headers.get("If-None-Match");
      if (absent) {
        if (!route.firstEnable || absent !== "*" || match)
          return configFailure(400, "INVALID_REQUEST");
        headers.set("If-None-Match", "*");
      } else if (match) {
        if (
          route.create ||
          !/^"[1-9][0-9]*"$/.test(match) ||
          !configVersion.safeParse(match.slice(1, -1)).success
        )
          return configFailure(400, "INVALID_REQUEST");
        headers.set("If-Match", match);
      } else if (!route.create)
        return configFailure(428, "PRECONDITION_REQUIRED");
      const payload = await readBoundedStrictJSON(
        new Response(request.body, {
          headers: {
            "Content-Type": request.headers.get("Content-Type") ?? "",
          },
        }),
        8192,
        controller.signal,
      );
      const parsed = route.input!.safeParse(payload);
      if (!parsed.success) return configFailure(400, "INVALID_REQUEST");
      body = JSON.stringify(parsed.data);
      headers.set("Content-Type", "application/json");
    } else if (!(await hasEmptyBody(request, controller.signal)))
      return configFailure(400, "INVALID_REQUEST");
    let origin: string;
    try {
      const u = new URL(process.env.LISTINGKIT_SERVICE_API_BASE ?? "");
      if (
        !["http:", "https:"].includes(u.protocol) ||
        u.username ||
        u.password ||
        u.search ||
        u.hash ||
        !["/api/v1", "/api/v1/"].includes(u.pathname)
      )
        throw new Error();
      origin = u.origin;
    } catch {
      return configFailure(503, "DEPENDENCY_UNAVAILABLE");
    }
    controller.signal.throwIfAborted();
    dispatched = true;
    const response = await fetch(
      origin + "/api/v1/workbench/agents/" + route.path + url.search,
      {
        method: request.method,
        headers,
        body,
        signal: controller.signal,
        cache: "no-store",
        redirect: "manual",
      },
    );
    const payload = await readBoundedStrictJSON(
      response,
      128 * 1024,
      controller.signal,
    );
    controller.signal.throwIfAborted();
    if (response.status !== 200) {
      const failure = z
        .strictObject({ code: z.string().regex(/^[A-Z_]{1,80}$/) })
        .safeParse(payload);
      return configFailure(
        response.status >= 400 && response.status < 600 ? response.status : 502,
        write && response.status >= 500
          ? "OUTCOME_UNKNOWN"
          : failure.success
            ? failure.data.code
            : write
              ? "OUTCOME_UNKNOWN"
              : "INVALID_UPSTREAM_RESPONSE",
      );
    }
    const parsed = route.output.safeParse(payload);
    if (!parsed.success) throw new Error();
    const parts = route.path.split("/");
    const value = parsed.data as Record<string, unknown>;
    if (
      route.output === templateSchema &&
      (value.templateId !== parts[2] ||
        (parts[3] === "revisions" && value.version !== parts[4]))
    )
      throw new Error();
    if (route.output === configReceiptSchema) {
      const operation =
        parts[1] === "default-template"
          ? "default"
          : parts[1] === "templates"
            ? parts.length === 2
              ? "create-template"
              : parts[3] === "archive"
                ? "archive-template"
                : "update-template"
            : parts[1];
      if (
        value.operation !== operation ||
        (parts[2] && value.templateId !== parts[2])
      )
        throw new Error();
    }
    const outgoing = new Headers(safeHeaders);
    const etag = response.headers.get("ETag");
    if (
      etag &&
      /^"[1-9][0-9]*"$/.test(etag) &&
      configVersion.safeParse(etag.slice(1, -1)).success
    )
      outgoing.set("ETag", etag);
    return Response.json(parsed.data, { status: 200, headers: outgoing });
  } catch {
    return configFailure(
      controller.signal.aborted ? 504 : 502,
      write && dispatched
        ? "OUTCOME_UNKNOWN"
        : dispatched
          ? "DEPENDENCY_UNAVAILABLE"
          : "INVALID_REQUEST",
    );
  } finally {
    clearTimeout(timer);
    request.signal.removeEventListener("abort", abort);
  }
}

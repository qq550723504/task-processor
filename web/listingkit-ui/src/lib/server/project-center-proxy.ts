import { z } from "zod";
import { projectEndpoint } from "@/lib/contracts/project-center";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { hasTrustedSameOriginWrite } from "./same-origin-write";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
import { hasEmptyBody } from "./members-proxy";
const safe = {
  "Cache-Control": "private, no-store",
  "X-Content-Type-Options": "nosniff",
};
export const projectFailure = (status: number, code: string) =>
  Response.json({ code }, { status, headers: safe });
export async function proxyProjectCenter(
  request: Request,
  token: string,
  userId: string,
): Promise<Response> {
  if (!token || !userId) return projectFailure(401, "AUTHENTICATION_REQUIRED");
  if (request.headers.get("X-Expected-User-ID") !== userId)
    return projectFailure(409, "IDENTITY_CONTEXT_CHANGED");
  const url = new URL(request.url),
    route = projectEndpoint(url, request.method);
  if (
    !route ||
    request.url.endsWith("?") ||
    request.headers.has("content-encoding")
  )
    return projectFailure(400, "INVALID_REQUEST");
  const write = !!route.input;
  if (write && !hasTrustedSameOriginWrite(request))
    return projectFailure(403, "FORBIDDEN");
  let org = "";
  {
    const cookies = (request.headers.get("cookie") ?? "")
      .split(";")
      .map((v) => v.trim())
      .filter((v) => v.startsWith(WORKBENCH_COOKIE_NAME + "="));
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
      return projectFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
  }
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
    });
    headers.set("X-Requested-Organization-ID", org);
    let body: string | undefined;
    if (write) {
      const key = request.headers.get("Idempotency-Key");
      if (!z.uuid().safeParse(key).success)
        return projectFailure(400, "INVALID_REQUEST");
      headers.set("Idempotency-Key", key!);
      const match=request.headers.get("If-Match");
      if(route.expected){if(!match || !/^[1-9][0-9]{0,15}$/.test(match) || !Number.isSafeInteger(Number(match)))return projectFailure(400,"INVALID_REQUEST");headers.set("If-Match",match);}
      else if(match)return projectFailure(400,"INVALID_REQUEST");
      if(request.headers.has("If-None-Match"))return projectFailure(400,"INVALID_REQUEST");
      const payload = await readBoundedStrictJSON(
        new Response(request.body, {
          headers: {
            "Content-Type": request.headers.get("Content-Type") ?? "",
          },
        }),
        16384,
        controller.signal,
      );
      const parsed = route.input!.safeParse(payload);
      if (!parsed.success) return projectFailure(400, "INVALID_REQUEST");
      body = JSON.stringify(parsed.data);
      headers.set("Content-Type", "application/json");
    } else if (!(await hasEmptyBody(request, controller.signal)))
      return projectFailure(400, "INVALID_REQUEST");
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
      return projectFailure(503, "DEPENDENCY_UNAVAILABLE");
    }
    controller.signal.throwIfAborted();
    dispatched = true;
    const response = await fetch(
      origin + "/api/v1/workbench/projects" + route.path + url.search,
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
      256 << 10,
      controller.signal,
    );
    controller.signal.throwIfAborted();
    if (response.status !== 200) {
      const failure = z
        .strictObject({ code: z.string().regex(/^[A-Z_]{1,80}$/) })
        .safeParse(payload);
      return projectFailure(
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
    const v=parsed.data as {id?:string};
    const entity=route.path.split("/")[1];
    if(entity && entity!=="templates" && v.id && v.id!==entity)throw new Error();
    return Response.json(parsed.data, { status: 200, headers: safe });
  } catch {
    return projectFailure(
      controller.signal.aborted ? 504 : dispatched ? 502 : 400,
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

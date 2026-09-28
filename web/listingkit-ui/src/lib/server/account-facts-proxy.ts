import {
  accountFactsSchema,
  accountPreferencesSchema,
  accountFactSubject,
  regionInputSchema,
} from "@/lib/api/account-facts";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { accountFailure, serviceOrigin } from "./account-proxy";
import { hasTrustedSameOriginWrite } from "./same-origin-write";

export async function proxyAccountFacts(
  request: Request,
  token: string,
  subject: string,
): Promise<Response> {
  if (!token || !accountFactSubject.safeParse(subject).success)
    return accountFailure(401, "AUTHENTICATION_REQUIRED");
  if (request.headers.get("X-Expected-User-ID") !== subject)
    return accountFailure(409, "IDENTITY_CONTEXT_CHANGED");
  const url = new URL(request.url);
  const preferences = url.pathname === "/api/account/preferences";
  const write = request.method === "PUT";
  if (
    (url.pathname !== "/api/account/facts" && !preferences) ||
    url.search ||
    request.url.endsWith("?") ||
    !(request.method === "GET" || (preferences && write))
  )
    return accountFailure(400, "INVALID_REQUEST");
  if (write && !hasTrustedSameOriginWrite(request))
    return accountFailure(403, "PERMISSION_DENIED");
  if (
    !write &&
    (request.body ||
      request.headers.has("transfer-encoding") ||
      (request.headers.has("content-length") &&
        request.headers.get("content-length") !== "0"))
  )
    return accountFailure(400, "INVALID_REQUEST");
  const origin = serviceOrigin();
  if (!origin) return accountFailure(503, "ACCOUNT_NOT_CONFIGURED");
  const controller = new AbortController();
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 15000);
  let dispatched = false;
  try {
    const headers = new Headers({
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
    });
    let body: string | undefined;
    if (write) {
      if (
        request.headers
          .get("content-type")
          ?.split(";", 1)[0]
          .trim()
          .toLowerCase() !== "application/json"
      )
        return accountFailure(400, "INVALID_REQUEST");
      let value: unknown;
      try {
        value = await readBoundedStrictJSON(
          new Response(request.body, {
            headers: { "Content-Type": "application/json" },
          }),
          16384,
          controller.signal,
        );
      } catch {
        return accountFailure(400, "INVALID_REQUEST");
      }
      const input = regionInputSchema.safeParse(value);
      if (!input.success) return accountFailure(400, "INVALID_REQUEST");
      body = JSON.stringify(input.data);
      headers.set("Content-Type", "application/json");
    }
    controller.signal.throwIfAborted();
    dispatched = true;
    const response = await fetch(
      `${origin}/api/v1/account/${preferences ? "preferences" : "identity/facts"}`,
      {
        method: request.method,
        headers,
        body,
        cache: "no-store",
        redirect: "manual",
        signal: controller.signal,
      },
    );
    const payload = await readBoundedStrictJSON(
      response,
      16384,
      controller.signal,
    );
    controller.signal.throwIfAborted();
    if (response.status !== 200) {
      const value = payload as { code?: unknown };
      return accountFailure(
        response.status,
        typeof value?.code === "string" ? value.code : "DEPENDENCY_UNAVAILABLE",
      );
    }
    const parsed = (
      preferences ? accountPreferencesSchema : accountFactsSchema
    ).safeParse(payload);
    if (!parsed.success)
      return accountFailure(502, "INVALID_UPSTREAM_RESPONSE");
    if (parsed.data.userId !== subject)
      return accountFailure(409, "IDENTITY_CONTEXT_CHANGED");
    return Response.json(parsed.data, {
      headers: {
        "Cache-Control": "private, no-store",
        "X-Content-Type-Options": "nosniff",
      },
    });
  } catch {
    if (write && dispatched)
      return accountFailure(
        controller.signal.aborted ? 504 : 502,
        "RESULT_UNVERIFIED",
        "unknown",
      );
    return accountFailure(
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

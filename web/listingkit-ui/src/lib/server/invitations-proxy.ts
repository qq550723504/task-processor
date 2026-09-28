import { memberId, MemberError, memberErrorCode } from "@/lib/api/members";
import { parseInvitation } from "@/lib/api/invitations";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { serviceOrigin, accountFailure } from "./account-proxy";
import { hasEmptyBody } from "./members-proxy";
import { hasTrustedSameOriginWrite } from "./same-origin-write";
export async function proxyRecipientInvitation(
  request: Request,
  token: string,
  userId: string,
) {
  if (!token || !memberId.safeParse(userId).success)
    return accountFailure(401, "AUTHENTICATION_REQUIRED");
  if (request.headers.get("X-Expected-User-ID") !== userId)
    return accountFailure(409, "IDENTITY_CONTEXT_CHANGED");
  const url = new URL(request.url),
    match =
      /^\/api\/account\/invitations\/([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})(?:\/(accept|decline))?$/.exec(
        url.pathname,
      );
  if (
    !match ||
    url.search ||
    request.url.endsWith("?") ||
    (match[2] && request.method !== "POST") ||
    (!match[2] && request.method !== "GET")
  )
    return accountFailure(400, "INVALID_REQUEST");
  if (request.method === "POST" && !hasTrustedSameOriginWrite(request))
    return accountFailure(403, "PERMISSION_DENIED");
  const origin = serviceOrigin();
  if (!origin) return accountFailure(503, "ACCOUNT_NOT_CONFIGURED");
  const controller = new AbortController(),
    abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 15000);
  let dispatched = false;
  try {
    if (!(await hasEmptyBody(request, controller.signal)))
      return accountFailure(400, "INVALID_REQUEST");
    controller.signal.throwIfAborted();
    dispatched = true;
    const response = await fetch(
      `${origin}/api/v1/account/invitations/${match[1]}${match[2] ? `/${match[2]}` : ""}`,
      {
        method: request.method,
        headers: {
          Accept: "application/json",
          Authorization: `Bearer ${token}`,
        },
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
    if (response.status !== 200)
      return accountFailure(
        response.status,
        memberErrorCode(response.status, payload),
      );
    const result = parseInvitation(payload, match[1]);
    if (result.userId !== userId)
      return accountFailure(409, "IDENTITY_CONTEXT_CHANGED");
    return Response.json(result, {
      headers: { "Cache-Control": "private, no-store" },
    });
  } catch (error) {
    if (error instanceof MemberError)
      return accountFailure(error.status, error.code);
    return accountFailure(
      controller.signal.aborted ? 504 : 502,
      dispatched && request.method === "POST"
        ? "RESULT_UNVERIFIED"
        : "DEPENDENCY_UNAVAILABLE",
    );
  } finally {
    clearTimeout(timer);
    request.signal.removeEventListener("abort", abort);
  }
}

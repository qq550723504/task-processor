import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession, readZitadelSessionError } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { notificationFailure, proxyNotifications } from "./notification-proxy";
import { hasEmptyBody } from "./members-proxy";

export async function handleNotificationRoute(request: NextRequest): Promise<Response> {
  const dispatch = { forwarded: false }, controller = new AbortController();
  const abort = () => controller.abort(); request.signal.addEventListener("abort", abort, { once: true });
  const timer = setTimeout(abort, 22000);
  const failure = () => notificationFailure(504, dispatch.forwarded && request.method === "POST" ? "OUTCOME_UNKNOWN" : "NOTIFICATION_UNAVAILABLE");
  let finish = () => {};
  const ended = new Promise<Response>(resolve => { finish = () => resolve(failure()); controller.signal.addEventListener("abort", finish, { once: true }); });
  try {
    if (request.signal.aborted) abort(); if (controller.signal.aborted) return failure();
    const authenticated = serverAuth(async (req: NextRequest & { auth?: unknown }) => {
      const identity = readZitadelIdentityFromSession(req.auth as never), token = readZitadelServerAccessToken(req.auth as never);
      if (!identity?.userId || !token || readZitadelSessionError(req.auth as never)) return notificationFailure(401, "AUTHENTICATION_REQUIRED");
      if (new URL(req.url).pathname === "/api/notifications/identity") {
        if (req.method !== "GET" || new URL(req.url).search || req.url.endsWith("?") || !(await hasEmptyBody(req, controller.signal))) return notificationFailure(400, "INVALID_REQUEST");
        return Response.json({ userId: String(identity.userId) }, { headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" } });
      }
      return proxyNotifications(req, token, String(identity.userId), dispatch);
    });
    return await Promise.race([authenticated(new NextRequest(request, { signal: controller.signal }), { params: Promise.resolve({}) }), ended]) ?? notificationFailure(503, "NOTIFICATION_UNAVAILABLE");
  } catch { return notificationFailure(503, dispatch.forwarded && request.method === "POST" ? "OUTCOME_UNKNOWN" : "NOTIFICATION_UNAVAILABLE"); }
  finally { clearTimeout(timer); request.signal.removeEventListener("abort", abort); controller.signal.removeEventListener("abort", finish); }
}

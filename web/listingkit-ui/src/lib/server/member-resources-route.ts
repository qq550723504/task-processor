import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { accountFailure } from "./account-proxy";
import { proxyMemberResources } from "./member-resources-proxy";

const authenticated = serverAuth(
  async (request: NextRequest & { auth?: unknown }) =>
    proxyMemberResources(
      request,
      readZitadelServerAccessToken(request.auth as never),
      String(
        readZitadelIdentityFromSession(request.auth as never)?.userId ?? "",
      ),
    ),
);
export async function handleMemberResources(request: NextRequest) {
  const write = request.method !== "GET";
  const timeout = () =>
    accountFailure(
      504,
      write ? "RESULT_UNVERIFIED" : "DEADLINE_EXCEEDED",
      write ? "unknown" : undefined,
    );
  if (request.signal.aborted) return timeout();
  const controller = new AbortController();
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  const timer = setTimeout(abort, 15000);
  let finish = () => {};
  const ended = new Promise<Response>((resolve) => {
    finish = () => resolve(timeout());
    controller.signal.addEventListener("abort", finish, { once: true });
  });
  try {
    return (
      (await Promise.race([
        authenticated(new NextRequest(request, { signal: controller.signal }), {
          params: Promise.resolve({}),
        }),
        ended,
      ])) ?? accountFailure(503, "DEPENDENCY_UNAVAILABLE")
    );
  } catch {
    return controller.signal.aborted
      ? timeout()
      : accountFailure(
          503,
          write ? "RESULT_UNVERIFIED" : "DEPENDENCY_UNAVAILABLE",
          write ? "unknown" : undefined,
        );
  } finally {
    clearTimeout(timer);
    request.signal.removeEventListener("abort", abort);
    controller.signal.removeEventListener("abort", finish);
  }
}

import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { accountFailure } from "./account-proxy";
import { proxyAccountFacts } from "./account-facts-proxy";

const authenticated = serverAuth(
  async (request: NextRequest & { auth?: unknown }) => {
    const identity = readZitadelIdentityFromSession(request.auth as never);
    return proxyAccountFacts(
      request,
      readZitadelServerAccessToken(request.auth as never),
      String(identity?.userId ?? ""),
    );
  },
);
export async function handleAccountFacts(request: NextRequest) {
  const controller = new AbortController();
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  if (request.signal.aborted) abort();
  const failure = () =>
    accountFailure(
      504,
      request.method === "PUT" ? "RESULT_UNVERIFIED" : "DEADLINE_EXCEEDED",
      request.method === "PUT" ? "unknown" : undefined,
    );
  let finish = () => {};
  const ended = new Promise<Response>((resolve) => {
    finish = () => resolve(failure());
    controller.signal.addEventListener("abort", finish, { once: true });
  });
  const timer = setTimeout(abort, 15000);
  try {
    if (controller.signal.aborted) return failure();
    const result = await Promise.race([
      authenticated(new NextRequest(request, { signal: controller.signal }), {
        params: Promise.resolve({}),
      }),
      ended,
    ]);
    return controller.signal.aborted
      ? failure()
      : (result ?? accountFailure(503, "DEPENDENCY_UNAVAILABLE"));
  } catch {
    return controller.signal.aborted
      ? failure()
      : accountFailure(503, "DEPENDENCY_UNAVAILABLE");
  } finally {
    clearTimeout(timer);
    request.signal.removeEventListener("abort", abort);
    controller.signal.removeEventListener("abort", finish);
  }
}
export const rejectAccountFacts = () => accountFailure(405, "INVALID_REQUEST");

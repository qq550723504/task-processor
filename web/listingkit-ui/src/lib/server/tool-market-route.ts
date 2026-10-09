import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { proxyToolMarket, toolFailure } from "./tool-market-proxy";
const authenticated = serverAuth(
  async (request: NextRequest & { auth?: unknown }) =>
    proxyToolMarket(
      request,
      readZitadelServerAccessToken(request.auth as never),
      String(
        readZitadelIdentityFromSession(request.auth as never)?.userId ?? "",
      ),
    ),
);
export async function handleToolMarket(request: NextRequest) {
  const controller = new AbortController(),
    abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 15000);
  const failure = () =>
    toolFailure(
      504,
      request.method === "GET" ? "DEPENDENCY_UNAVAILABLE" : "OUTCOME_UNKNOWN",
    );
  let finish = () => {};
  const ended = new Promise<Response>((resolve) => {
    finish = () => resolve(failure());
    controller.signal.addEventListener("abort", finish, { once: true });
    if (controller.signal.aborted) finish();
  });
  try {
    return (
      (await Promise.race([
        authenticated(new NextRequest(request, { signal: controller.signal }), {
          params: Promise.resolve({}),
        }),
        ended,
      ])) ?? toolFailure(503, "DEPENDENCY_UNAVAILABLE")
    );
  } catch {
    return toolFailure(
      503,
      request.method === "GET" ? "DEPENDENCY_UNAVAILABLE" : "OUTCOME_UNKNOWN",
    );
  } finally {
    clearTimeout(timer);
    request.signal.removeEventListener("abort", abort);
    controller.signal.removeEventListener("abort", finish);
  }
}
export const rejectToolMarket = () => toolFailure(405, "INVALID_REQUEST");

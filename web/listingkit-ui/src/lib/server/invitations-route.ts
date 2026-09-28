import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { proxyRecipientInvitation } from "./invitations-proxy";
import { membersFailure } from "./members-proxy";
const authenticated = serverAuth(
  async (request: NextRequest & { auth?: unknown }) =>
    proxyRecipientInvitation(
      request,
      readZitadelServerAccessToken(request.auth as never),
      String(
        readZitadelIdentityFromSession(request.auth as never)?.userId ?? "",
      ),
    ),
);
export async function handleInvitation(request: NextRequest) {
  const controller = new AbortController(),
    abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 15000);
  let finish = () => {};
  const ended = new Promise<Response>((resolve) => {
    finish = () => resolve(membersFailure(504, "DEADLINE_EXCEEDED"));
    controller.signal.addEventListener("abort", finish, { once: true });
    if (controller.signal.aborted) finish();
  });
  try {
    const result = await Promise.race([
      authenticated(new NextRequest(request, { signal: controller.signal }), {
        params: Promise.resolve({}),
      }),
      ended,
    ]);
    return result ?? membersFailure(503, "DEPENDENCY_UNAVAILABLE");
  } catch {
    return membersFailure(503, "DEPENDENCY_UNAVAILABLE");
  } finally {
    clearTimeout(timer);
    request.signal.removeEventListener("abort", abort);
    controller.signal.removeEventListener("abort", finish);
  }
}

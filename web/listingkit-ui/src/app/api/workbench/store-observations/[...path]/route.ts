import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "@/lib/server/zitadel-auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";
import { proxyStoreObservations } from "@/lib/server/store-observations-proxy";
export const dynamic = "force-dynamic";
const failure = (status: number, code: string) =>
  Response.json(
    { code },
    { status, headers: { "Cache-Control": "private, no-store" } },
  );
const authenticated = serverAuth(
  async (request: NextRequest & { auth?: unknown }) =>
    proxyStoreObservations(
      request,
      readZitadelServerAccessToken(request.auth as never),
      String(
        readZitadelIdentityFromSession(request.auth as never)?.userId ?? "",
      ),
    ),
);
async function handle(request: NextRequest): Promise<Response> {
  const controller = new AbortController();
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 20000);
  let ended = () => {};
  const timeout = new Promise<Response>((resolve) => {
    ended = () => resolve(failure(504, "DEADLINE_EXCEEDED"));
    if (controller.signal.aborted) ended();
    else controller.signal.addEventListener("abort", ended, { once: true });
  });
  try {
    return (
      (await Promise.race([
        Promise.resolve(
          authenticated(
            new NextRequest(request, { signal: controller.signal }),
            { params: Promise.resolve({}) },
          ),
        ),
        timeout,
      ])) ?? failure(503, "DEPENDENCY_UNAVAILABLE")
    );
  } catch {
    return failure(
      controller.signal.aborted ? 504 : 503,
      controller.signal.aborted
        ? "DEADLINE_EXCEEDED"
        : "DEPENDENCY_UNAVAILABLE",
    );
  } finally {
    clearTimeout(timer);
    request.signal.removeEventListener("abort", abort);
    controller.signal.removeEventListener("abort", ended);
  }
}
export const GET = handle;
export const POST = handle;
const reject = () => failure(405, "INVALID_REQUEST");
export const PUT = reject;
export const PATCH = reject;
export const DELETE = reject;
export const HEAD = reject;
export const OPTIONS = reject;

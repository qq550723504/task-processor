import { NextRequest } from "next/server";

import { serverAuth } from "@/auth";
import {
  buildWorkbenchBrowserResponse,
  buildWorkbenchUpstreamRequest,
  workbenchProtocolError,
} from "@/lib/server/workbench-proxy";
import { readZitadelIdentityFromSession } from "@/lib/server/zitadel-auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";

export const dynamic = "force-dynamic";

const UPSTREAM_TIMEOUT_MS = 15_000;

type AuthenticatedWorkbenchRequest = NextRequest & { auth?: unknown };
type WorkbenchDispatchState = {
  dispatched: boolean;
  requestId: string;
  sourceRequest: boolean;
  sourceMutation: boolean;
  acquisitionRequest: boolean;
};
type WorkbenchRouteContext = {
  params: Promise<{ path: string[] }>;
  dispatchState: WorkbenchDispatchState;
};

async function proxyWorkbenchRequest(
  request: AuthenticatedWorkbenchRequest,
  { params, dispatchState }: WorkbenchRouteContext,
) {
  if (request.signal.aborted) return deadlineFailure(dispatchState);
  const { path } = await params;
  if (request.signal.aborted) return deadlineFailure(dispatchState);
  const session = request.auth as never;
  const identity = readZitadelIdentityFromSession(session);
  const actorSubject =
    typeof identity?.userId === "string" ? identity.userId : "";
  const accessToken = readZitadelServerAccessToken(request.auth as never);
  if (!accessToken || (isSourceMutation(request.method, path) && !actorSubject)) {
    return workbenchProtocolError(
      401,
      "AUTHENTICATION_REQUIRED",
      "Authentication is required",
    );
  }

  const upstreamRequest = await buildWorkbenchUpstreamRequest(
    request,
    path,
    accessToken,
    actorSubject,
  );
  if (upstreamRequest instanceof Response) {
    return request.signal.aborted
      ? deadlineFailure(dispatchState)
      : upstreamRequest;
  }
  dispatchState.requestId = upstreamRequest.requestId;
  dispatchState.acquisitionRequest = upstreamRequest.responseContract === "product-acquisition";
  dispatchState.sourceRequest = dispatchState.acquisitionRequest || upstreamRequest.responseContract.startsWith("source-account-");
  dispatchState.sourceMutation = upstreamRequest.sourceMutation;
  if (request.signal.aborted) return deadlineFailure(dispatchState);
  try {
    dispatchState.dispatched = true;
    const upstream = await fetch(upstreamRequest.url, {
      ...upstreamRequest.init,
      redirect: "manual",
      signal: request.signal,
    });
    const response = await buildWorkbenchBrowserResponse(
      upstream,
      upstreamRequest.responseContract,
      upstreamRequest.expectedStoreId,
      {
        sourceMutation: upstreamRequest.sourceMutation,
        requestId: upstreamRequest.requestId,
        signal: request.signal,
      },
    );
    return request.signal.aborted ? deadlineFailure(dispatchState) : response;
  } catch {
    if (request.signal.aborted) return deadlineFailure(dispatchState);
    if (dispatchState.sourceMutation && dispatchState.dispatched) {
      return unknownMutationFailure(dispatchState.requestId, dispatchState.acquisitionRequest);
    }
    return workbenchProtocolError(
      502,
      "DEPENDENCY_UNAVAILABLE",
      "Workbench upstream is unavailable",
      dispatchState.requestId,
    );
  }
}

const authenticatedProxyRequest = serverAuth(async (request, context) =>
  proxyWorkbenchRequest(
    request as AuthenticatedWorkbenchRequest,
    context as unknown as WorkbenchRouteContext,
  ),
) as (
  request: NextRequest,
  context: WorkbenchRouteContext,
) => Promise<Response | void> | Response | void;

async function handleWorkbenchRequest(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
) {
  const dispatchState: WorkbenchDispatchState = {
    dispatched: false,
    requestId: "",
    sourceRequest: isSourceRequestURL(request.url),
    sourceMutation: isSourceMutationURL(request.method, request.url),
    acquisitionRequest: new URL(request.url).pathname.startsWith("/api/workbench/sourcing/1688/acquisitions"),
  };
  const controller = new AbortController();
  let resolveAbort = () => {};
  const aborted = new Promise<Response>((resolve) => {
    resolveAbort = () => resolve(deadlineFailure(dispatchState));
    controller.signal.addEventListener("abort", resolveAbort, { once: true });
  });
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  if (request.signal.aborted) abort();
  const timeout = setTimeout(abort, UPSTREAM_TIMEOUT_MS);
  try {
    const scopedRequest = new NextRequest(request, { signal: controller.signal });
    const result = await Promise.race([
      authenticatedProxyRequest(scopedRequest, {
        ...context,
        dispatchState,
      }),
      aborted,
    ]);
    return result ?? workbenchProtocolError(
      503,
      "DEPENDENCY_UNAVAILABLE",
      "Workbench upstream is unavailable",
      dispatchState.requestId,
    );
  } catch {
    return controller.signal.aborted
      ? deadlineFailure(dispatchState)
      : workbenchProtocolError(
          502,
          "DEPENDENCY_UNAVAILABLE",
          "Workbench upstream is unavailable",
          dispatchState.requestId,
        );
  } finally {
    clearTimeout(timeout);
    request.signal.removeEventListener("abort", abort);
    controller.signal.removeEventListener("abort", resolveAbort);
  }
}

export const GET = handleWorkbenchRequest;
export const PUT = handleWorkbenchRequest;
export const POST = handleWorkbenchRequest;
export const DELETE = handleWorkbenchRequest;

function isSourceMutation(method: string, path: string[]) {
  return (
    method.toUpperCase() === "POST" &&
    ((path[0] === "sourcing" && path[1] === "1688" && path[2] === "acquisitions" &&
      (path.length === 3 || (path.length === 4 && path[3] === "verify"))) ||
    (path[0] === "source-accounts" &&
    (path.length === 1 ||
      (path.length === 3 && ["enable", "disable"].includes(path[2] ?? "")))))
  );
}

function isSourceRequestURL(rawURL: string) {
  const path = new URL(rawURL).pathname.split("/").filter(Boolean);
  return (
    path[0] === "api" &&
    path[1] === "workbench" &&
    (path[2] === "source-accounts" || (path[2] === "sourcing" && path[3] === "1688" && path[4] === "acquisitions"))
  );
}

function isSourceMutationURL(method: string, rawURL: string) {
  if (method.toUpperCase() !== "POST") return false;
  const path = new URL(rawURL).pathname.split("/").filter(Boolean).slice(2);
  return isSourceMutation(method, path);
}

function deadlineFailure(state: WorkbenchDispatchState) {
  if (state.sourceMutation && state.dispatched) {
    return unknownMutationFailure(state.requestId, state.acquisitionRequest);
  }
  if (state.sourceRequest) {
    return workbenchProtocolError(
      504,
      "DEADLINE_EXCEEDED",
      "Workbench request deadline exceeded",
      state.requestId,
    );
  }
  return workbenchProtocolError(
    502,
    "DEPENDENCY_UNAVAILABLE",
    "Workbench upstream is unavailable",
    state.requestId,
  );
}

function unknownMutationFailure(requestId: string, acquisition = false) {
  return workbenchProtocolError(
    503,
    "OUTCOME_UNKNOWN",
    acquisition ? "Product acquisition outcome is unknown" : "Source Account mutation outcome is unknown",
    requestId,
  );
}

function rejectUnsupportedRequest() {
  return workbenchProtocolError(
    405,
    "INVALID_REQUEST",
    "Workbench method is not allowed",
  );
}

export const HEAD = rejectUnsupportedRequest;
export const PATCH = rejectUnsupportedRequest;
export const OPTIONS = rejectUnsupportedRequest;

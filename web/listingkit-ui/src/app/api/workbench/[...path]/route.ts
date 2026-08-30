import type { NextRequest } from "next/server";

import { serverAuth } from "@/auth";
import {
  buildWorkbenchBrowserResponse,
  buildWorkbenchUpstreamRequest,
  workbenchProtocolError,
} from "@/lib/server/workbench-proxy";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";

export const dynamic = "force-dynamic";

const UPSTREAM_TIMEOUT_MS = 15_000;

type AuthenticatedWorkbenchRequest = NextRequest & { auth?: unknown };

async function proxyWorkbenchRequest(
  request: AuthenticatedWorkbenchRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const accessToken = readZitadelServerAccessToken(request.auth as never);
  if (!accessToken) {
    return workbenchProtocolError(
      401,
      "AUTHENTICATION_REQUIRED",
      "Authentication is required",
    );
  }

  const { path } = await params;
  const upstreamRequest = await buildWorkbenchUpstreamRequest(
    request,
    path,
    accessToken,
  );
  if (upstreamRequest instanceof Response) {
    return upstreamRequest;
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), UPSTREAM_TIMEOUT_MS);
  try {
    const upstream = await fetch(upstreamRequest.url, {
      ...upstreamRequest.init,
      redirect: "manual",
      signal: controller.signal,
    });
    return await buildWorkbenchBrowserResponse(upstream);
  } catch {
    return workbenchProtocolError(
      502,
      "DEPENDENCY_UNAVAILABLE",
      "Workbench upstream is unavailable",
    );
  } finally {
    clearTimeout(timeout);
  }
}

const authenticatedProxyRequest = serverAuth(proxyWorkbenchRequest);

export const GET = authenticatedProxyRequest;
export const PUT = authenticatedProxyRequest;

function rejectUnsupportedRequest() {
  return workbenchProtocolError(
    405,
    "INVALID_REQUEST",
    "Workbench method is not allowed",
  );
}

export const HEAD = rejectUnsupportedRequest;
export const POST = rejectUnsupportedRequest;
export const PATCH = rejectUnsupportedRequest;
export const DELETE = rejectUnsupportedRequest;
export const OPTIONS = rejectUnsupportedRequest;

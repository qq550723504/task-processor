import { NextRequest, NextResponse } from "next/server";

import { buildSheinLoginURL } from "@/app/api/shein-login/shared";
import { getZitadelBearerToken } from "@/lib/server/zitadel-auth";

export const dynamic = "force-dynamic";

function copyHeader(source: Headers, target: Headers, sourceName: string, targetName?: string) {
  const value = source.get(sourceName);
  if (value) {
    target.set(targetName ?? sourceName, value);
  }
}

export function buildSheinLoginUpstreamHeaders(requestHeaders: Headers) {
  const headers = new Headers();
  copyHeader(requestHeaders, headers, "accept", "Accept");
  copyHeader(requestHeaders, headers, "content-type", "Content-Type");
  copyHeader(requestHeaders, headers, "authorization", "Authorization");
  return headers;
}

async function proxyRequest(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  const url = buildSheinLoginURL(`/${path.join("/")}`);
  const headers = buildSheinLoginUpstreamHeaders(request.headers);
  const zitadelToken = getZitadelBearerToken(request);
  if (zitadelToken && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${zitadelToken}`);
  }

  const response = await fetch(url, {
    method: request.method,
    headers,
    body:
      request.method === "GET" || request.method === "HEAD"
        ? undefined
        : await request.text(),
    cache: "no-store",
  });

  const responseHeaders = new Headers();
  const contentType = response.headers.get("content-type");
  if (contentType) {
    responseHeaders.set("Content-Type", contentType);
  }

  return new NextResponse(await response.text(), {
    status: response.status,
    headers: responseHeaders,
  });
}

export const GET = proxyRequest;
export const POST = proxyRequest;
export const DELETE = proxyRequest;

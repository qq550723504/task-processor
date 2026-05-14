import { NextRequest, NextResponse } from "next/server";

import { buildSheinLoginURL } from "@/app/api/shein-login/shared";

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
  copyHeader(requestHeaders, headers, "tenant-id", "tenant-id");
  copyHeader(requestHeaders, headers, "visit-tenant-id", "visit-tenant-id");
  copyHeader(requestHeaders, headers, "login-user", "login-user");
  return headers;
}

async function proxyRequest(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  const url = buildSheinLoginURL(`/${path.join("/")}`);
  const headers = buildSheinLoginUpstreamHeaders(request.headers);

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

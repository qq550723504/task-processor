import { NextRequest, NextResponse } from "next/server";

import {
  buildListingKitProxyUrl,
  getListingKitUpstreamBase,
} from "@/app/api/listing-kits/proxy-url";
import { buildListingKitMockResponse } from "@/app/api/listing-kits/mock-responses";

export const dynamic = "force-dynamic";

async function proxyRequest(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  const mock = await buildListingKitMockResponse(request, path);
  if (mock) {
    if (request.method === "POST" && path.length === 1 && path[0] === "generate") {
      return NextResponse.json(mock.createTask, {
        headers: {
          ETag: "mock-create-task",
        },
      });
    }

    const endpoint = path[path.length - 1];
    const payload =
      request.method === "POST" && endpoint === "execute"
        ? mock.action
        : request.method === "POST"
          ? mock.dispatch
          : endpoint === "generation-queue"
            ? mock.queue
            : endpoint === "generation-review-session"
              ? mock.reviewSession
              : endpoint === "generation-review-preview"
                ? mock.reviewPreview
                : path.length === 2 && path[0] === "tasks"
                  ? mock.taskResult
                  : mock.preview;

    return NextResponse.json(payload, {
      headers: {
        ETag:
          ("conditional" in payload && payload.conditional?.etag) ||
          ("delta_token" in payload && payload.delta_token) ||
          "mock-token",
      },
    });
  }

  const url = buildListingKitProxyUrl(
    getListingKitUpstreamBase(),
    path,
    request.nextUrl.searchParams.toString(),
  );

  const headers = new Headers();
  headers.set("Accept", request.headers.get("accept") ?? "application/json");

  const ifNoneMatch = request.headers.get("if-none-match");
  if (ifNoneMatch) {
    headers.set("If-None-Match", ifNoneMatch);
  }

  const contentType = request.headers.get("content-type");
  if (contentType) {
    headers.set("Content-Type", contentType);
  }

  const upstream = await fetch(url, {
    method: request.method,
    headers,
    body:
      request.method === "GET" || request.method === "HEAD"
        ? undefined
        : await readProxyRequestBody(request),
    cache: "no-store",
  });

  const responseHeaders = new Headers();
  const etag = upstream.headers.get("etag");
  const contentTypeHeader = upstream.headers.get("content-type");

  if (etag) {
    responseHeaders.set("ETag", etag);
  }
  if (contentTypeHeader) {
    responseHeaders.set("Content-Type", contentTypeHeader);
  }

  if (upstream.status === 304) {
    return new NextResponse(null, {
      status: 304,
      headers: responseHeaders,
    });
  }

  const body = await upstream.text();
  return new NextResponse(body, {
    status: upstream.status,
    headers: responseHeaders,
  });
}

export async function GET(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
) {
  return proxyRequest(request, context);
}

export async function POST(
  request: NextRequest,
  context: { params: Promise<{ path: string[] }> },
) {
  return proxyRequest(request, context);
}

async function readProxyRequestBody(request: NextRequest) {
  const buffer = await request.arrayBuffer();
  if (buffer.byteLength === 0) {
    return undefined;
  }
  return buffer;
}

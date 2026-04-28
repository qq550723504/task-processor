import { lookup } from "node:dns/promises";
import { isIP } from "node:net";

import { NextRequest, NextResponse } from "next/server";

import {
  logRequestWarn,
  newRequestLogId,
} from "@/lib/server/request-log";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

const MAX_IMAGE_BYTES = 25 * 1024 * 1024;
const MAX_REDIRECTS = 3;
const DEFAULT_ALLOWED_HOSTS = [
  "cdn.sdspod.com",
  "e.sdspod.com",
  "img.sdspod.com",
  "sdspod.com",
  "oss.shuomiai.com",
];
const REQUEST_HEADERS = {
  Accept: "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
  Referer: "https://www.sdsdiy.com/",
  "User-Agent":
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147 Safari/537.36",
};

function allowedHosts() {
  const configured = process.env.IMAGE_PROXY_ALLOWED_HOSTS?.split(",") ?? [];
  const privateHosts = allowPrivateHosts()
    ? ["127.0.0.1", "localhost", "::1"]
    : [];
  return [...DEFAULT_ALLOWED_HOSTS, ...privateHosts, ...configured]
    .map((host) => host.trim().toLowerCase())
    .filter(Boolean);
}

function allowPrivateHosts() {
  return (
    process.env.IMAGE_PROXY_ALLOW_PRIVATE === "1" ||
    process.env.NODE_ENV !== "production"
  );
}

function hostMatches(hostname: string, allowedHost: string) {
  return hostname === allowedHost || hostname.endsWith(`.${allowedHost}`);
}

function isPrivateIPv4(address: string) {
  const parts = address.split(".").map((part) => Number(part));
  if (parts.length !== 4 || parts.some((part) => !Number.isInteger(part) || part < 0 || part > 255)) {
    return true;
  }
  const [a, b] = parts;
  return (
    a === 0 ||
    a === 10 ||
    a === 127 ||
    (a === 100 && b >= 64 && b <= 127) ||
    (a === 169 && b === 254) ||
    (a === 172 && b >= 16 && b <= 31) ||
    (a === 192 && b === 168) ||
    (a === 198 && (b === 18 || b === 19)) ||
    a >= 224
  );
}

function isPrivateIPv6(address: string) {
  const normalized = address.toLowerCase();
  return (
    normalized === "::1" ||
    normalized === "::" ||
    normalized.startsWith("fc") ||
    normalized.startsWith("fd") ||
    normalized.startsWith("fe80:") ||
    normalized.startsWith("::ffff:127.") ||
    normalized.startsWith("::ffff:10.") ||
    normalized.startsWith("::ffff:192.168.")
  );
}

function isBlockedAddress(address: string) {
  const family = isIP(address);
  if (family === 4) {
    return isPrivateIPv4(address);
  }
  if (family === 6) {
    return isPrivateIPv6(address);
  }
  return true;
}

async function validateImageURL(url: URL) {
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    return "unsupported_protocol";
  }

  const hostname = url.hostname.toLowerCase();
  if (!allowedHosts().some((allowedHost) => hostMatches(hostname, allowedHost))) {
    return "host_not_allowed";
  }

  const addresses = await lookup(hostname, { all: true, verbatim: true });
  if (
    addresses.length === 0 ||
    (!allowPrivateHosts() &&
      addresses.some((address) => isBlockedAddress(address.address)))
  ) {
    return "host_not_allowed";
  }

  return "";
}

async function fetchValidatedImage(url: URL) {
  let current = url;
  for (let redirect = 0; redirect <= MAX_REDIRECTS; redirect += 1) {
    const validationError = await validateImageURL(current);
    if (validationError) {
      return { error: validationError as string, response: null };
    }

    let response: Response;
    try {
      response = await fetch(current.toString(), {
        headers: REQUEST_HEADERS,
        cache: "no-store",
        redirect: "manual",
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : "image fetch failed";
      return { error: `fetch_failed:${message}`, response: null };
    }

    if (response.status >= 300 && response.status < 400) {
      const location = response.headers.get("location");
      if (!location) {
        return { error: "invalid_redirect", response: null };
      }
      current = new URL(location, current);
      continue;
    }

    return { error: "", response };
  }

  return { error: "too_many_redirects", response: null };
}

export async function GET(request: NextRequest) {
  const requestId = newRequestLogId();
  const startedAt = Date.now();
  const rawUrl = request.nextUrl.searchParams.get("url")?.trim();
  if (!rawUrl) {
    return NextResponse.json(
      { error: "missing_url", message: "Image URL is required." },
      { status: 400 },
    );
  }

  let url: URL;
  try {
    url = new URL(rawUrl);
  } catch {
    return NextResponse.json(
      { error: "invalid_url", message: "Image URL is invalid." },
      { status: 400 },
    );
  }

  const { error: proxyError, response: upstream } = await fetchValidatedImage(url);
  if (proxyError || !upstream) {
    logRequestWarn("image proxy rejected request", {
      requestId,
      host: url.hostname,
      status: 400,
      durationMs: Date.now() - startedAt,
      error: proxyError || "image_fetch_failed",
    });
    return NextResponse.json(
      { error: proxyError || "image_fetch_failed", message: "Image URL is not allowed." },
      { status: 400 },
    );
  }

  if (!upstream.ok) {
    logRequestWarn("image proxy upstream failed", {
      requestId,
      host: url.hostname,
      status: upstream.status,
      durationMs: Date.now() - startedAt,
    });
    return NextResponse.json(
      {
        error: "image_fetch_failed",
        message: `Image fetch failed: ${upstream.status}`,
        url: url.toString(),
      },
      { status: upstream.status },
    );
  }

  const contentLength = Number(upstream.headers.get("content-length") ?? 0);
  if (contentLength > MAX_IMAGE_BYTES) {
    return NextResponse.json(
      { error: "image_too_large", message: "Image is too large for preview." },
      { status: 413 },
    );
  }

  const contentType = upstream.headers.get("content-type") ?? "image/jpeg";
  let body: ArrayBuffer;
  try {
    body = await upstream.arrayBuffer();
  } catch (error) {
    const message = error instanceof Error ? error.message : "image body read failed";
    logRequestWarn("image proxy body read failed", {
      requestId,
      host: url.hostname,
      status: 502,
      durationMs: Date.now() - startedAt,
      error: message,
    });
    return NextResponse.json(
      { error: "image_body_unavailable", message },
      { status: 502 },
    );
  }
  if (body.byteLength > MAX_IMAGE_BYTES) {
    return NextResponse.json(
      { error: "image_too_large", message: "Image is too large for preview." },
      { status: 413 },
    );
  }

  const durationMs = Date.now() - startedAt;
  if (durationMs > 5_000) {
    logRequestWarn("image proxy slow response", {
      requestId,
      host: url.hostname,
      status: upstream.status,
      durationMs,
      bytes: body.byteLength,
    });
  }

  return new NextResponse(body, {
    headers: {
      "Cache-Control": "no-store",
      "Content-Type": contentType,
    },
  });
}

import { NextRequest, NextResponse } from "next/server";

import {
  buildListingKitUpstreamHeaders,
  verifyListingKitRequestIdentity,
} from "@/app/api/listing-kits/proxy-auth";

const DEFAULT_SERVICE_API_BASE = "http://localhost:8085/api/v1";

export class SDSAPIError extends Error {
  code: string;
  status: number;
  detail?: string;

  constructor(input: { code: string; message: string; status: number; detail?: string }) {
    super(input.message);
    this.name = "SDSAPIError";
    this.code = input.code;
    this.status = input.status;
    this.detail = input.detail;
  }
}

export type SDSJSONResult<T> = {
  payload: T;
};

export function sdsAPIErrorPayload(error: unknown, fallbackCode: string) {
  if (error instanceof SDSAPIError) {
    return {
      body: {
        error: error.code || fallbackCode,
        message: error.message,
        detail: error.detail,
      },
      status: error.status,
    };
  }
  return {
    body: {
      error: fallbackCode,
      message: error instanceof Error ? error.message : "unknown SDS error",
    },
    status: 502,
  };
}

function buildSDSAPIBase() {
  if (process.env.SDS_API_BASE) {
    return process.env.SDS_API_BASE;
  }
  const serviceBase = process.env.LISTINGKIT_SERVICE_API_BASE ?? DEFAULT_SERVICE_API_BASE;
  return `${serviceBase.replace(/\/+$/, "")}/sds`;
}

export function buildSDSURL(pathname: string, query?: URLSearchParams) {
  const normalizedBase = buildSDSAPIBase().replace(/\/+$/, "");
  const normalizedPath = pathname.startsWith("/") ? pathname : `/${pathname}`;
  const suffix = query && query.toString() ? `?${query.toString()}` : "";
  return `${normalizedBase}${normalizedPath}${suffix}`;
}

export async function fetchSDSJSON<T>(
  request: NextRequest,
  pathname: string,
  query?: URLSearchParams,
): Promise<SDSJSONResult<T>> {
  const auth = await verifyListingKitRequestIdentity(request);
  if (auth.response) {
    throw new SDSAPIError({
      code: "zitadel_proxy_auth_failed",
      message: await readErrorMessage(auth.response, "ZITADEL authentication failed"),
      status: auth.response.status,
    });
  }

  const headers = buildListingKitUpstreamHeaders(request.headers, auth.identity);
  if (auth.token && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${auth.token}`);
  }

  const response = await fetch(buildSDSURL(pathname, query), {
    method: "GET",
    headers,
    cache: "no-store",
  });

  const text = await response.text();
  const payload = text ? (JSON.parse(text) as unknown) : undefined;

  if (!response.ok) {
    const code =
      payload && typeof payload === "object" && "error" in payload
        ? String((payload as { error?: unknown }).error)
        : "sds_request_failed";
    const message =
      payload && typeof payload === "object" && "message" in payload
        ? String((payload as { message?: unknown }).message)
        : `SDS request failed: ${response.status}`;
    const detail =
      payload && typeof payload === "object" && "detail" in payload
        ? String((payload as { detail?: unknown }).detail)
        : undefined;
    throw new SDSAPIError({ code, message, status: response.status, detail });
  }

  return {
    payload: payload as T,
  };
}

async function readErrorMessage(response: NextResponse, fallback: string) {
  const payload = (await response.json().catch(() => null)) as
    | { message?: unknown }
    | null;
  if (payload?.message && typeof payload.message === "string") {
    return payload.message;
  }
  return fallback;
}

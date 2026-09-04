import { buildQueryString } from "@/lib/api/query-string";
import { ApiError } from "@/lib/api/api-error";
import {
  parseJsonResponse,
  ResponseJsonParseError,
} from "@/lib/api/response-json";
import type { ConditionalState, QueueQuery } from "@/lib/types/listingkit";

const API_BASE =
  process.env.NEXT_PUBLIC_LISTINGKIT_API_BASE ?? "/api/listing-kits";

type RequestOptions = {
  method?: "GET" | "POST" | "PATCH" | "PUT" | "DELETE";
  query?: QueueQuery;
  body?: unknown;
  conditional?: ConditionalState | null;
  timeoutMs?: number;
  signal?: AbortSignal;
};

type FormRequestOptions = {
  method?: "POST";
  formData: FormData;
  query?: QueueQuery;
};

export { ApiError };

function buildHeaders(conditional?: ConditionalState | null, source?: HeadersInit) {
  const headers = new Headers(source);

  headers.set("Accept", "application/json");

  if (conditional?.etag) {
    headers.set("If-None-Match", conditional.etag);
  } else if (conditional?.delta_token) {
    headers.set("If-None-Match", conditional.delta_token);
  }

  return headers;
}

function buildApiUrl(path: string, query?: QueueQuery) {
  const queryString = query ? buildQueryString(query) : "";
  return `${API_BASE}${path}${queryString ? `?${queryString}` : ""}`;
}

function apiErrorMessage(status: number, payload: unknown) {
  const serverMessage =
    payload &&
    typeof payload === "object" &&
    "message" in payload &&
    typeof payload.message === "string"
      ? payload.message.trim()
      : "";
  return serverMessage
    ? `ListingKit API request failed: ${status}: ${serverMessage}`
    : `ListingKit API request failed: ${status}`;
}

export async function apiRequest<T>(
  path: string,
  { method = "GET", query, body, conditional, timeoutMs, signal }: RequestOptions = {},
): Promise<T> {
  const url = buildApiUrl(path, query);
  const headers = buildHeaders(conditional);
  const controller = timeoutMs && !signal ? new AbortController() : undefined;
  const activeSignal = signal ?? controller?.signal;
  const timeout =
    timeoutMs && controller
      ? setTimeout(() => controller.abort(), timeoutMs)
      : undefined;

  if (body !== undefined) {
    headers.set("Content-Type", "application/json");
  }

  let response: Response;
  try {
    response = await fetch(url, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: activeSignal,
    });
  } catch (error) {
    if (controller?.signal.aborted) {
      throw new ApiError(
        `ListingKit API request timed out after ${timeoutMs}ms`,
        408,
      );
    }
    if (signal?.aborted) {
      throw error;
    }
    throw error;
  } finally {
    if (timeout) {
      clearTimeout(timeout);
    }
  }

  if (response.status === 304) {
    return {
      not_modified: true,
      conditional: {
        delta_token: response.headers.get("etag") ?? undefined,
        etag: response.headers.get("etag") ?? undefined,
        not_modified: true,
      },
    } as T;
  }

  const payload = await parseApiJsonResponse(response);

  if (!response.ok) {
    throw new ApiError(
      apiErrorMessage(response.status, payload),
      response.status,
      payload,
    );
  }

  return payload as T;
}

export async function apiFormRequest<T>(
  path: string,
  { method = "POST", formData, query }: FormRequestOptions,
): Promise<T> {
  const response = await fetch(buildApiUrl(path, query), {
    method,
    headers: new Headers(),
    body: formData,
  });

  const payload = await parseApiJsonResponse(response);

  if (!response.ok) {
    throw new ApiError(
      apiErrorMessage(response.status, payload),
      response.status,
      payload,
    );
  }

  return payload as T;
}

async function parseApiJsonResponse(response: Response) {
  try {
    return await parseJsonResponse<unknown>(response);
  } catch (error) {
    if (error instanceof ResponseJsonParseError) {
      throw new ApiError("ListingKit API returned invalid JSON", response.status, {
        message: error.message,
      });
    }
    throw error;
  }
}

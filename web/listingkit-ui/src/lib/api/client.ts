import { buildQueryString } from "@/lib/api/query-string";
import type { ConditionalState, QueueQuery } from "@/lib/types/listingkit";

const API_BASE =
  process.env.NEXT_PUBLIC_LISTINGKIT_API_BASE ?? "/api/listing-kits";

type RequestOptions = {
  method?: "GET" | "POST" | "PATCH" | "PUT" | "DELETE";
  query?: QueueQuery;
  body?: unknown;
  conditional?: ConditionalState | null;
  timeoutMs?: number;
};

type FormRequestOptions = {
  method?: "POST";
  formData: FormData;
};

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly payload?: unknown,
  ) {
    super(message);
  }
}

function buildHeaders(conditional?: ConditionalState | null) {
  const headers = new Headers({
    Accept: "application/json",
  });

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

export async function apiRequest<T>(
  path: string,
  { method = "GET", query, body, conditional, timeoutMs }: RequestOptions = {},
): Promise<T> {
  const url = buildApiUrl(path, query);
  const headers = buildHeaders(conditional);
  const controller = timeoutMs ? new AbortController() : undefined;
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
      signal: controller?.signal,
    });
  } catch (error) {
    if (controller?.signal.aborted) {
      throw new ApiError(
        `ListingKit API request timed out after ${timeoutMs}ms`,
        408,
      );
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

  const text = await response.text();
  const payload = text ? (JSON.parse(text) as unknown) : undefined;

  if (!response.ok) {
    throw new ApiError(
      `ListingKit API request failed: ${response.status}`,
      response.status,
      payload,
    );
  }

  return payload as T;
}

export async function apiFormRequest<T>(
  path: string,
  { method = "POST", formData }: FormRequestOptions,
): Promise<T> {
  const response = await fetch(buildApiUrl(path), {
    method,
    body: formData,
  });

  const text = await response.text();
  const payload = text ? (JSON.parse(text) as unknown) : undefined;

  if (!response.ok) {
    throw new ApiError(
      `ListingKit API request failed: ${response.status}`,
      response.status,
      payload,
    );
  }

  return payload as T;
}

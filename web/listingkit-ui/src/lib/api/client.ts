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

type AsyncJobResponse<T> = {
  job_id: string;
  status: "running" | "succeeded" | "failed";
  result?: T;
  error?: string;
  upstream_status?: number;
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

export async function apiAsyncRequest<T>(
  path: string,
  { body, timeoutMs = 3600000 }: Pick<RequestOptions, "body" | "timeoutMs"> = {},
): Promise<T> {
  const started = await fetch("/api/listing-kits/async-jobs", {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ path, body }),
  });
  const startedPayload = (await started.json()) as AsyncJobResponse<T> & {
    message?: string;
  };
  if (!started.ok || !startedPayload.job_id) {
    throw new ApiError(
      startedPayload.message ?? `ListingKit async job start failed: ${started.status}`,
      started.status,
      startedPayload,
    );
  }

  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    await sleep(2000);
    const response = await fetch(
      `/api/listing-kits/async-jobs?id=${encodeURIComponent(startedPayload.job_id)}`,
      {
        headers: { Accept: "application/json" },
        cache: "no-store",
      },
    );
    const payload = (await response.json()) as AsyncJobResponse<T> & {
      message?: string;
    };
    if (!response.ok) {
      throw new ApiError(
        payload.message ?? `ListingKit async job poll failed: ${response.status}`,
        response.status,
        payload,
      );
    }
    if (payload.status === "succeeded") {
      return payload.result as T;
    }
    if (payload.status === "failed") {
      throw new ApiError(
        payload.error ?? "ListingKit async job failed",
        payload.upstream_status ?? 500,
        payload,
      );
    }
  }
  throw new ApiError(
    `ListingKit async job timed out after ${timeoutMs}ms`,
    408,
  );
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

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

import { buildQueryString } from "@/lib/api/query-string";
import {
  buildAsyncJobResumeKey,
  clearAsyncJobResumeEntry,
  loadAsyncJobResumeEntry,
  saveAsyncJobResumeEntry,
} from "@/lib/api/async-job-resume";
import { stageAsyncJobRequestIfNeeded } from "@/lib/api/async-job-staging";
import { fetchWithRetry } from "@/lib/api/fetch-retry";
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
  onJobStarted?: (jobId: string) => void;
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

type AsyncJobSource = "backend" | "next";

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
      `ListingKit API request failed: ${response.status}`,
      response.status,
      payload,
    );
  }

  return payload as T;
}

function buildEmptyJsonError(status: number, fallbackMessage: string) {
  return new ApiError(fallbackMessage, status, {
    message: "Response body was empty",
  });
}

export async function apiAsyncRequest<T>(
  path: string,
  {
    body,
    timeoutMs = 3600000,
    onJobStarted,
  }: Pick<RequestOptions, "body" | "timeoutMs" | "onJobStarted"> = {},
): Promise<T> {
  const resumeKey = buildAsyncJobResumeKey(path, body ?? {});
  const resumed = loadAsyncJobResumeEntry(resumeKey);
  let startedJobId = resumed?.jobId ?? "";
  let startedJobSource: AsyncJobSource = resumed?.source ?? "next";
  let resumedFromStorage = Boolean(startedJobId);

  if (!startedJobId) {
    const backendJob = await startBackendAsyncJob<T>(path, body);
    if (backendJob) {
      startedJobId = backendJob.jobId;
      startedJobSource = "backend";
    } else {
      const localJob = await startNextAsyncJob<T>(path, body);
      startedJobId = localJob.jobId;
      startedJobSource = "next";
    }
    onJobStarted?.(startedJobId);
    saveAsyncJobResumeEntry(resumeKey, startedJobId, startedJobSource);
  }

  const deadline = Date.now() + timeoutMs;
  let lastPollError: ApiError | Error | undefined;
  while (Date.now() < deadline) {
    await sleep(2000);
    try {
      const response = await fetchWithRetry(
        buildAsyncJobPollUrl(startedJobId, startedJobSource),
        {
          headers: new Headers({
            Accept: "application/json",
          }),
          cache: "no-store",
        },
        { retries: 1, retryDelayMs: 1200 },
      );
      let payload: (AsyncJobResponse<T> & { message?: string }) | undefined;
      try {
        payload = await parseJsonResponse<AsyncJobResponse<T> & {
          message?: string;
        }>(response);
      } catch (error) {
        if (error instanceof ResponseJsonParseError) {
          lastPollError = new ApiError(
            "ListingKit async job poll returned invalid JSON",
            response.status,
            { message: error.message },
          );
          continue;
        }
        lastPollError = error instanceof Error ? error : new Error(String(error));
        continue;
      }

      if (!payload) {
        lastPollError = buildEmptyJsonError(
          response.status,
          `ListingKit async job poll returned empty response: ${response.status}`,
        );
        continue;
      }
      if (response.status === 404 && resumedFromStorage) {
        clearAsyncJobResumeEntry(resumeKey);
        resumedFromStorage = false;
        startedJobId = "";
        break;
      }
      if (!response.ok) {
        lastPollError = new ApiError(
          payload.message ?? `ListingKit async job poll failed: ${response.status}`,
          response.status,
          payload,
        );
        continue;
      }
      if (payload.status === "succeeded") {
        clearAsyncJobResumeEntry(resumeKey);
        return payload.result as T;
      }
      if (payload.status === "failed") {
        clearAsyncJobResumeEntry(resumeKey);
        const jobError = new ApiError(
          payload.error ?? "ListingKit async job failed",
          payload.upstream_status ?? 500,
          payload,
        );
        throw jobError;
      }
      lastPollError = undefined;
    } catch (error) {
      if (error instanceof ApiError) {
        throw error;
      }
      lastPollError = error instanceof Error ? error : new Error(String(error));
    }
  }
  if (!startedJobId) {
    return apiAsyncRequest<T>(path, { body, timeoutMs });
  }
  if (lastPollError instanceof ApiError) {
    throw lastPollError;
  }
  if (lastPollError) {
    throw lastPollError;
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
    headers: new Headers(),
    body: formData,
  });

  const payload = await parseApiJsonResponse(response);

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

async function startBackendAsyncJob<T>(path: string, body: unknown) {
  let response: Response;
  try {
    response = await fetchWithRetry(
      buildApiUrl("/studio/async-jobs"),
      {
        method: "POST",
        headers: new Headers({
          Accept: "application/json",
          "Content-Type": "application/json",
        }),
        body: JSON.stringify({ path, body }),
      },
      { retries: 0 },
    );
  } catch {
    return null;
  }

  if (isBackendAsyncJobUnavailable(response.status)) {
    return null;
  }

  let payload: (AsyncJobResponse<T> & { message?: string }) | undefined;
  try {
    payload = await parseJsonResponse<AsyncJobResponse<T> & {
      message?: string;
    }>(response);
  } catch (error) {
    if (error instanceof ResponseJsonParseError) {
      throw new ApiError(
        "ListingKit backend async job start returned invalid JSON",
        response.status,
        { message: error.message },
      );
    }
    throw error;
  }

  if (!response.ok || !payload?.job_id) {
    throw new ApiError(
      payload?.message ?? `ListingKit backend async job start failed: ${response.status}`,
      response.status,
      payload,
    );
  }

  return { jobId: payload.job_id };
}

async function startNextAsyncJob<T>(path: string, body: unknown) {
  const staged = await stageAsyncJobRequestIfNeeded({ path, body });
  const response = await fetchWithRetry(
    staged.staged
      ? "/api/listing-kits/async-jobs/staged"
      : "/api/listing-kits/async-jobs",
    {
      method: staged.staged ? "PATCH" : "POST",
      headers: new Headers({
        Accept: "application/json",
        "Content-Type": "application/json",
      }),
      body: staged.staged
        ? JSON.stringify({ stage_id: staged.stageId })
        : JSON.stringify({ path, body: JSON.parse(staged.bodyText) }),
    },
  );
  let payload: (AsyncJobResponse<T> & { message?: string }) | undefined;
  try {
    payload = await parseJsonResponse<AsyncJobResponse<T> & {
      message?: string;
    }>(response);
  } catch (error) {
    if (error instanceof ResponseJsonParseError) {
      throw new ApiError(
        "ListingKit async job start returned invalid JSON",
        response.status,
        { message: error.message },
      );
    }
    throw error;
  }
  if (!response.ok || !payload?.job_id) {
    throw new ApiError(
      payload?.message ?? `ListingKit async job start failed: ${response.status}`,
      response.status,
      payload,
    );
  }
  return { jobId: payload.job_id };
}

function buildAsyncJobPollUrl(jobId: string, source: AsyncJobSource) {
  if (source === "backend") {
    return buildApiUrl(`/studio/async-jobs/${encodeURIComponent(jobId)}`);
  }
  return `/api/listing-kits/async-jobs?id=${encodeURIComponent(jobId)}`;
}

function isBackendAsyncJobUnavailable(status: number) {
  return status === 404 || status === 405 || status === 501;
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

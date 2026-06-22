import { ApiError } from "@/lib/api/api-error";
import { fetchWithRetry } from "@/lib/api/fetch-retry";
import {
  parseJsonResponse,
  ResponseJsonParseError,
} from "@/lib/api/response-json";

export type AsyncJobResponse<T> = {
  job_id: string;
  status: "running" | "succeeded" | "failed";
  result?: T;
  error?: string;
  upstream_status?: number;
};

type AsyncJobPollRequest = {
  url: string;
  init: RequestInit;
};

type PollAsyncJobOptions = {
  timeoutMs: number;
  signal?: AbortSignal;
  intervalMs?: number;
  buildPollRequest: (jobId: string) => AsyncJobPollRequest;
  shouldStopOnNotFound?: boolean;
};

export class AsyncJobNotFound extends Error {
  constructor(
    public readonly jobId: string,
    public readonly payload?: unknown,
  ) {
    super(`ListingKit async job was not found: ${jobId}`);
  }
}

export async function pollAsyncJob<T>(
  jobId: string,
  {
    timeoutMs,
    signal,
    intervalMs = 2000,
    buildPollRequest,
    shouldStopOnNotFound = false,
  }: PollAsyncJobOptions,
): Promise<T> {
  throwIfAborted(signal);

  const deadline = Date.now() + timeoutMs;
  let lastPollError: ApiError | Error | undefined;

  while (Date.now() < deadline) {
    await sleep(intervalMs, signal);
    throwIfAborted(signal);

    try {
      const { url, init } = buildPollRequest(jobId);
      const response = await fetchWithRetry(
        url,
        {
          ...init,
          signal,
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
        lastPollError = new ApiError(
          `ListingKit async job poll returned empty response: ${response.status}`,
          response.status,
          { message: "Response body was empty" },
        );
        continue;
      }
      if (response.status === 404 && shouldStopOnNotFound) {
        throw new AsyncJobNotFound(jobId, payload);
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
        return payload.result as T;
      }
      if (payload.status === "failed") {
        throw new ApiError(
          payload.error ?? "ListingKit async job failed",
          payload.upstream_status ?? 500,
          payload,
        );
      }
      lastPollError = undefined;
    } catch (error) {
      throwIfAborted(signal);
      if (error instanceof AsyncJobNotFound) {
        throw error;
      }
      if (error instanceof ApiError) {
        throw error;
      }
      lastPollError = error instanceof Error ? error : new Error(String(error));
    }
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

function sleep(ms: number, signal?: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    if (signal?.aborted) {
      reject(signal.reason ?? new DOMException("Aborted", "AbortError"));
      return;
    }
    const timeout = setTimeout(() => {
      signal?.removeEventListener("abort", handleAbort);
      resolve();
    }, ms);
    const handleAbort = () => {
      clearTimeout(timeout);
      reject(signal?.reason ?? new DOMException("Aborted", "AbortError"));
    };
    signal?.addEventListener("abort", handleAbort, { once: true });
  });
}

function throwIfAborted(signal?: AbortSignal) {
  if (signal?.aborted) {
    throw signal.reason ?? new DOMException("Aborted", "AbortError");
  }
}

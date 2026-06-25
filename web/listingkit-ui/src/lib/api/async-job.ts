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

export type AsyncJobStartResult = {
  jobId: string;
};

type AsyncJobStartRequest = {
  url: string;
  init: RequestInit;
};

type AsyncJobPollRequest = {
  url: string;
  init: RequestInit;
};

type StartAsyncJobOptions<TInput> = {
  input: TInput;
  signal?: AbortSignal;
  buildStartRequest: (input: TInput) => AsyncJobStartRequest;
};

type PollAsyncJobOptions = {
  timeoutMs: number;
  signal?: AbortSignal;
  intervalMs?: number;
  terminalPollStatuses?: number[];
  buildPollRequest: (jobId: string) => AsyncJobPollRequest;
};

type ResumeOrRestartAsyncJobOptions<TInput> = Omit<
  StartAsyncJobOptions<TInput>,
  "input"
> &
  PollAsyncJobOptions & {
    jobId?: string;
    onJobStarted?: (jobId: string) => void;
  };

export async function startAsyncJob<TInput>({
  input,
  signal,
  buildStartRequest,
}: StartAsyncJobOptions<TInput>): Promise<AsyncJobStartResult> {
  const { url, init } = buildStartRequest(input);
  const response = await fetchWithRetry(
    url,
    {
      ...init,
      signal,
    },
    { retries: 0 },
  );

  let payload: (AsyncJobResponse<unknown> & { message?: string }) | undefined;
  try {
    payload = await parseJsonResponse<AsyncJobResponse<unknown> & {
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

export async function pollAsyncJob<T>(
  jobId: string,
  {
    timeoutMs,
    signal,
    intervalMs = 2000,
    terminalPollStatuses = [],
    buildPollRequest,
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
      if (!response.ok) {
        lastPollError = new ApiError(
          payload.message ?? `ListingKit async job poll failed: ${response.status}`,
          response.status,
          payload,
        );
        if (terminalPollStatuses.includes(response.status)) {
          throw lastPollError;
        }
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

export async function resumeAsyncJob<T>(
  jobId: string,
  options: PollAsyncJobOptions,
): Promise<T> {
  return pollAsyncJob<T>(jobId, options);
}

export async function resumeOrRestartAsyncJob<T, TInput>(
  input: TInput,
  {
    jobId,
    onJobStarted,
    timeoutMs,
    signal,
    intervalMs,
    buildPollRequest,
    buildStartRequest,
  }: ResumeOrRestartAsyncJobOptions<TInput>,
): Promise<T> {
  const normalizedJobId = jobId?.trim();
  if (normalizedJobId) {
    try {
      return await resumeAsyncJob<T>(normalizedJobId, {
        timeoutMs,
        signal,
        intervalMs,
        terminalPollStatuses: [404],
        buildPollRequest,
      });
    } catch (error) {
      if (!(error instanceof ApiError) || error.status !== 404) {
        throw error;
      }
    }
  }

  const started = await startAsyncJob({
    input,
    signal,
    buildStartRequest,
  });
  onJobStarted?.(started.jobId);
  return pollAsyncJob<T>(started.jobId, {
    timeoutMs,
    signal,
    intervalMs,
    buildPollRequest,
  });
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

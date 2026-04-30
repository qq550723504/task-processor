type FetchRetryOptions = {
  retries?: number;
  retryDelayMs?: number;
  retryStatuses?: number[];
};

const DEFAULT_RETRY_STATUSES = [408, 429, 502, 503, 504];

export async function fetchWithRetry(
  input: RequestInfo | URL,
  init?: RequestInit,
  options?: FetchRetryOptions,
) {
  const retries = options?.retries ?? 2;
  const retryDelayMs = options?.retryDelayMs ?? 800;
  const retryStatuses = options?.retryStatuses ?? DEFAULT_RETRY_STATUSES;

  let attempt = 0;
  let lastError: unknown;

  while (attempt <= retries) {
    try {
      const response = await fetch(input, init);
      if (attempt < retries && retryStatuses.includes(response.status)) {
        await sleep(retryDelayMs * (attempt + 1));
        attempt += 1;
        continue;
      }
      return response;
    } catch (error) {
      lastError = error;
      if (attempt >= retries) {
        throw error;
      }
      await sleep(retryDelayMs * (attempt + 1));
      attempt += 1;
    }
  }

  throw lastError instanceof Error ? lastError : new Error("fetch retry failed");
}

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

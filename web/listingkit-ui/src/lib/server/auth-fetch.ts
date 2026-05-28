const DEFAULT_TIMEOUT_MS = 15_000;
const DEFAULT_RETRIES = 2;
const DEFAULT_RETRY_DELAY_MS = 250;

type ResilientOidcFetchOptions = {
  fetchImpl?: typeof fetch;
  retries?: number;
  retryDelayMs?: number;
  timeoutMs?: number;
  onRetry?: (input: RequestInfo | URL, attempt: number, error: unknown) => void;
};

export function createResilientOidcFetch(
  options: ResilientOidcFetchOptions = {},
) {
  const fetchImpl = options.fetchImpl ?? fetch;
  const retries = options.retries ?? DEFAULT_RETRIES;
  const retryDelayMs = options.retryDelayMs ?? DEFAULT_RETRY_DELAY_MS;
  const timeoutMs = options.timeoutMs ?? DEFAULT_TIMEOUT_MS;

  return async function resilientOidcFetch(
    input: RequestInfo | URL,
    init?: RequestInit,
  ) {
    let lastError: unknown;

    for (let attempt = 0; attempt <= retries; attempt += 1) {
      try {
        const response = await fetchImpl(input, {
          ...init,
          signal: createTimeoutSignal(init?.signal, timeoutMs),
        });
        return response;
      } catch (error) {
        lastError = error;
        if (attempt >= retries || !isRetryableOidcError(error)) {
          throw error;
        }
        options.onRetry?.(input, attempt + 1, error);
        if (retryDelayMs > 0) {
          await delay(retryDelayMs);
        }
      }
    }

    throw lastError instanceof Error
      ? lastError
      : new Error("OIDC fetch failed");
  };
}

function createTimeoutSignal(signal: AbortSignal | null | undefined, timeoutMs: number) {
  const timeoutSignal = AbortSignal.timeout(timeoutMs);
  if (!signal) {
    return timeoutSignal;
  }
  return AbortSignal.any([signal, timeoutSignal]);
}

function isRetryableOidcError(error: unknown) {
  const code =
    error instanceof Error && "cause" in error
      ? readErrorCode((error as Error & { cause?: unknown }).cause)
      : readErrorCode(error);

  return (
    code === "UND_ERR_CONNECT_TIMEOUT" ||
    code === "UND_ERR_HEADERS_TIMEOUT" ||
    code === "ECONNRESET" ||
    code === "ETIMEDOUT"
  );
}

function readErrorCode(error: unknown) {
  if (!error || typeof error !== "object") {
    return "";
  }
  const code = Reflect.get(error, "code");
  return typeof code === "string" ? code : "";
}

function delay(ms: number) {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

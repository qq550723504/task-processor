export const LISTINGKIT_TRACE_STORAGE_KEY =
  "listingkit:shein-studio:trace-context";

export const LISTINGKIT_TRACE_HEADER_NAMES = {
  batchRunId: "X-ListingKit-Batch-Run-Id",
  batchId: "X-ListingKit-Batch-Id",
  sessionId: "X-ListingKit-Studio-Session-Id",
  queueMode: "X-ListingKit-Queue-Mode",
  queueIndex: "X-ListingKit-Queue-Index",
  queueTotal: "X-ListingKit-Queue-Total",
} as const;

export type ListingKitTraceContext = {
  batchRunId?: string;
  batchId?: string;
  sessionId?: string;
  queueMode?: string;
  queueIndex?: number;
  queueTotal?: number;
};

export function isListingKitStudioPath(path: string) {
  const normalized = path.trim();
  return normalized === "/studio" || normalized.startsWith("/studio/");
}

export function beginListingKitTraceRun(
  context: Partial<ListingKitTraceContext> = {},
) {
  return writeListingKitTraceContext(
    {
      ...context,
      batchRunId: newListingKitBatchRunId(),
      sessionId: undefined,
    },
    { replace: true },
  );
}

export function readListingKitTraceContext(): ListingKitTraceContext {
  if (!canUseSessionStorage()) {
    return {};
  }
  const raw = window.sessionStorage.getItem(LISTINGKIT_TRACE_STORAGE_KEY);
  if (!raw) {
    return {};
  }
  try {
    return normalizeListingKitTraceContext(
      JSON.parse(raw) as Partial<ListingKitTraceContext>,
    );
  } catch {
    return {};
  }
}

export function writeListingKitTraceContext(
  next: Partial<ListingKitTraceContext>,
  options?: { replace?: boolean },
) {
  const merged = normalizeListingKitTraceContext(
    options?.replace ? next : { ...readListingKitTraceContext(), ...next },
  );
  if (!canUseSessionStorage()) {
    return merged;
  }
  if (isListingKitTraceContextEmpty(merged)) {
    window.sessionStorage.removeItem(LISTINGKIT_TRACE_STORAGE_KEY);
    return {};
  }
  window.sessionStorage.setItem(
    LISTINGKIT_TRACE_STORAGE_KEY,
    JSON.stringify(merged),
  );
  return merged;
}

export function clearListingKitTraceContext() {
  if (!canUseSessionStorage()) {
    return;
  }
  window.sessionStorage.removeItem(LISTINGKIT_TRACE_STORAGE_KEY);
}

export function buildListingKitTraceHeaders(
  source?: HeadersInit,
  context?: Partial<ListingKitTraceContext>,
) {
  const headers = new Headers(source);
  applyListingKitTraceHeaders(headers, context);
  return headers;
}

export function applyListingKitTraceHeaders(
  headers: Headers,
  context?: Partial<ListingKitTraceContext>,
) {
  const trace = normalizeListingKitTraceContext(
    context ?? readListingKitTraceContext(),
  );
  setTraceHeader(headers, LISTINGKIT_TRACE_HEADER_NAMES.batchRunId, trace.batchRunId);
  setTraceHeader(headers, LISTINGKIT_TRACE_HEADER_NAMES.batchId, trace.batchId);
  setTraceHeader(headers, LISTINGKIT_TRACE_HEADER_NAMES.sessionId, trace.sessionId);
  setTraceHeader(headers, LISTINGKIT_TRACE_HEADER_NAMES.queueMode, trace.queueMode);
  setTraceHeader(
    headers,
    LISTINGKIT_TRACE_HEADER_NAMES.queueIndex,
    trace.queueIndex,
  );
  setTraceHeader(
    headers,
    LISTINGKIT_TRACE_HEADER_NAMES.queueTotal,
    trace.queueTotal,
  );
  return headers;
}

export function readListingKitTraceContextFromHeaders(headers: Headers) {
  const queueIndexHeader = headers.get(LISTINGKIT_TRACE_HEADER_NAMES.queueIndex);
  const queueTotalHeader = headers.get(LISTINGKIT_TRACE_HEADER_NAMES.queueTotal);
  return normalizeListingKitTraceContext({
    batchRunId: headers.get(LISTINGKIT_TRACE_HEADER_NAMES.batchRunId) ?? undefined,
    batchId: headers.get(LISTINGKIT_TRACE_HEADER_NAMES.batchId) ?? undefined,
    sessionId: headers.get(LISTINGKIT_TRACE_HEADER_NAMES.sessionId) ?? undefined,
    queueMode: headers.get(LISTINGKIT_TRACE_HEADER_NAMES.queueMode) ?? undefined,
    queueIndex: queueIndexHeader ? Number(queueIndexHeader) : undefined,
    queueTotal: queueTotalHeader ? Number(queueTotalHeader) : undefined,
  });
}

export function buildListingKitTraceLogFields(
  source: Headers | Partial<ListingKitTraceContext>,
) {
  const trace =
    source instanceof Headers
      ? readListingKitTraceContextFromHeaders(source)
      : normalizeListingKitTraceContext(source);
  return {
    batchRunId: trace.batchRunId,
    batchId: trace.batchId,
    sessionId: trace.sessionId,
    queueMode: trace.queueMode,
    queueIndex: trace.queueIndex,
    queueTotal: trace.queueTotal,
  };
}

export function logListingKitTraceEvent(
  level: "info" | "warn",
  message: string,
  fields?: Record<string, unknown>,
) {
  const logger =
    level === "warn"
      ? console.warn.bind(console)
      : console.info.bind(console);
  logger(`[shein-studio-trace] ${message}`, {
    ...readListingKitTraceContext(),
    ...fields,
  });
}

function canUseSessionStorage() {
  return typeof window !== "undefined" && typeof window.sessionStorage !== "undefined";
}

function newListingKitBatchRunId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `run-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

function normalizeListingKitTraceContext(
  context?: Partial<ListingKitTraceContext> | null,
): ListingKitTraceContext {
  if (!context) {
    return {};
  }
  const queueIndex = normalizePositiveInt(context.queueIndex);
  const queueTotal = normalizePositiveInt(context.queueTotal);
  return {
    batchRunId: normalizeTraceString(context.batchRunId),
    batchId: normalizeTraceString(context.batchId),
    sessionId: normalizeTraceString(context.sessionId),
    queueMode: normalizeTraceString(context.queueMode),
    queueIndex,
    queueTotal,
  };
}

function normalizeTraceString(value: unknown) {
  return typeof value === "string" && value.trim() ? value.trim() : undefined;
}

function normalizePositiveInt(value: unknown) {
  if (typeof value === "number" && Number.isFinite(value) && value > 0) {
    return Math.trunc(value);
  }
  if (typeof value === "string" && value.trim()) {
    const parsed = Number(value);
    if (Number.isFinite(parsed) && parsed > 0) {
      return Math.trunc(parsed);
    }
  }
  return undefined;
}

function isListingKitTraceContextEmpty(context: ListingKitTraceContext) {
  return !context.batchRunId &&
    !context.batchId &&
    !context.sessionId &&
    !context.queueMode &&
    !context.queueIndex &&
    !context.queueTotal;
}

function setTraceHeader(headers: Headers, name: string, value: string | number | undefined) {
  if (value === undefined || value === "") {
    headers.delete(name);
    return;
  }
  headers.set(name, String(value));
}

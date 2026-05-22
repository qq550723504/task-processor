export const PROXY_UPSTREAM_TIMEOUT_MS = 15_000;
export const PROXY_ADMIN_COLLECTION_UPSTREAM_TIMEOUT_MS = 30_000;
export const PROXY_SUBMIT_UPSTREAM_TIMEOUT_MS = 180_000;
export const PROXY_CHILD_TASK_RETRY_UPSTREAM_TIMEOUT_MS = 180_000;
export const PROXY_SHEIN_CATEGORY_SEARCH_UPSTREAM_TIMEOUT_MS = 60_000;

export function resolveListingKitProxyTimeoutMs(
  method: string,
  path: string[],
) {
  if (
    method.toUpperCase() === "POST" &&
    path.length === 3 &&
    path[0] === "tasks" &&
    path[2] === "submit"
  ) {
    return PROXY_SUBMIT_UPSTREAM_TIMEOUT_MS;
  }
  if (
    method.toUpperCase() === "POST" &&
    path.length === 4 &&
    path[0] === "tasks" &&
    path[2] === "child-tasks" &&
    path[3] === "retry"
  ) {
    return PROXY_CHILD_TASK_RETRY_UPSTREAM_TIMEOUT_MS;
  }
  if (
    method.toUpperCase() === "GET" &&
    path.length === 4 &&
    path[0] === "tasks" &&
    path[2] === "shein" &&
    path[3] === "categories"
  ) {
    return PROXY_SHEIN_CATEGORY_SEARCH_UPSTREAM_TIMEOUT_MS;
  }
  if (
    method.toUpperCase() === "GET" &&
    path.length === 2 &&
    path[0] === "admin" &&
    (path[1] === "product-import-mappings" || path[1] === "product-data")
  ) {
    return PROXY_ADMIN_COLLECTION_UPSTREAM_TIMEOUT_MS;
  }
  return PROXY_UPSTREAM_TIMEOUT_MS;
}

export function shouldProxyListingKitResponseAsBinary(
  contentType: string | null,
  path: string[],
) {
  const normalized = (contentType ?? "").toLowerCase();
  if (path.length >= 3 && path[0] === "uploads" && path[1] === "files") {
    return true;
  }
  return (
    normalized.startsWith("image/") ||
    normalized.startsWith("audio/") ||
    normalized.startsWith("video/") ||
    normalized === "application/octet-stream" ||
    normalized.startsWith("application/pdf")
  );
}

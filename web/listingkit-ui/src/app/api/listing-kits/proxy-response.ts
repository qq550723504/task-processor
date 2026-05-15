export const PROXY_UPSTREAM_TIMEOUT_MS = 15_000;
export const PROXY_SUBMIT_UPSTREAM_TIMEOUT_MS = 180_000;

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

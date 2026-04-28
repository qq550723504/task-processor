export function toImageProxyUrl(url?: string | null) {
  const trimmed = typeof url === "string" ? url.trim() : "";
  if (!trimmed) {
    return "";
  }
  if (trimmed.startsWith("data:") || trimmed.startsWith("/api/image-proxy")) {
    return trimmed;
  }
  try {
    const parsed = new URL(trimmed);
    if (parsed.hostname.toLowerCase() === "oss.shuomiai.com") {
      return trimmed;
    }
  } catch {
    // Fall through to the proxy for relative or malformed-but-displayable values.
  }
  return `/api/image-proxy?url=${encodeURIComponent(trimmed)}`;
}

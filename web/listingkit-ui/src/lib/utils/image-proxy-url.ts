export function toImageProxyUrl(url?: string | null) {
  const trimmed = typeof url === "string" ? url.trim() : "";
  if (!trimmed) {
    return "";
  }
  if (trimmed.startsWith("data:") || trimmed.startsWith("/api/image-proxy")) {
    return trimmed;
  }
  return `/api/image-proxy?url=${encodeURIComponent(trimmed)}`;
}

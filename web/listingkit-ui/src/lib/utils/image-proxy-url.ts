export function toImageProxyUrl(url?: string | null) {
  const trimmed = typeof url === "string" ? url.trim() : "";
  if (!trimmed) {
    return "";
  }
  if (trimmed.startsWith("data:") || trimmed.startsWith("/api/image-proxy")) {
    return trimmed;
  }
  const listingKitUploadPrefix = "/api/v1/listing-kits/uploads/files/";
  if (trimmed.startsWith(listingKitUploadPrefix)) {
    return trimmed.replace(
      listingKitUploadPrefix,
      "/api/listing-kits/uploads/files/",
    );
  }
  try {
    const parsed = new URL(trimmed);
    if (parsed.pathname.startsWith(listingKitUploadPrefix)) {
      return `${parsed.pathname.replace(listingKitUploadPrefix, "/api/listing-kits/uploads/files/")}${parsed.search}`;
    }
    if (isDirectPublicImageHost(parsed.hostname)) {
      return trimmed;
    }
  } catch {
    // Fall through to the proxy for relative or malformed-but-displayable values.
  }
  return `/api/image-proxy?url=${encodeURIComponent(trimmed)}`;
}

function isDirectPublicImageHost(hostname: string) {
  const normalized = hostname.toLowerCase();
  return (
    normalized === "oss.shuomiai.com" ||
    normalized === "cos-1303159911.cos.na-ashburn.myqcloud.com" ||
    normalized === "shuomi-1303159911.cos.ap-hongkong.myqcloud.com"
  );
}

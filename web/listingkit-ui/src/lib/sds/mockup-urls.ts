export function isSDSPreviewImageUrl(url: string) {
  try {
    const parsed = new URL(url);
    return parsed.hostname.toLowerCase().endsWith("sdspod.com") && parsed.pathname.includes("/images/");
  } catch {
    return false;
  }
}

export function isSDSUnavailableImageUrl(url: string) {
  const value = url.trim().toLowerCase();
  if (!value) {
    return true;
  }
  return (
    value.includes("shengchengzhong") ||
    value.includes("/output/generating") ||
    value.includes("/output/loading") ||
    value.includes("/output/placeholder")
  );
}

export function isUsableSDSMockupImageUrl(url: string) {
  const trimmed = url.trim();
  if (!trimmed) {
    return false;
  }
  return !isSDSPreviewImageUrl(trimmed) && !isSDSUnavailableImageUrl(trimmed);
}

export function sanitizeSDSMockupImageUrls(urls: string[] | undefined) {
  const sanitized: string[] = [];
  const seen = new Set<string>();
  for (const rawUrl of urls ?? []) {
    const trimmed = rawUrl.trim();
    if (!isUsableSDSMockupImageUrl(trimmed) || seen.has(trimmed)) {
      continue;
    }
    seen.add(trimmed);
    sanitized.push(trimmed);
  }
  return sanitized;
}

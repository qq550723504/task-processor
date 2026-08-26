export function getListingKitUpstreamBase() {
  return (
    process.env.LISTINGKIT_API_BASE ??
    process.env.NEXT_PUBLIC_LISTINGKIT_API_BASE ??
    "http://localhost:8085/api/v1/listing-kits"
  );
}

export function buildListingKitProxyUrl(
  upstreamBase: string,
  pathParts: string[],
  search: string,
) {
  for (const part of pathParts) {
    if (
      !part ||
      part === "." ||
      part === ".." ||
      part.includes("/") ||
      part.includes("\\") ||
      part.includes("\0")
    ) {
      throw new Error("invalid proxy path segment");
    }
  }
  let normalizedBase = upstreamBase.replace(/\/+$/, "");
  let routedParts = pathParts;
  if (pathParts[0] === "image-agent") {
    const upstream = new URL(normalizedBase);
    const listingKitSuffix = "/api/v1/listing-kits";
    if (!upstream.pathname.endsWith(listingKitSuffix)) {
      throw new Error("image-agent proxy requires a ListingKit API base");
    }
    upstream.pathname = `${upstream.pathname.slice(0, -listingKitSuffix.length)}/api/v1/image-agent`;
    normalizedBase = upstream.toString().replace(/\/+$/, "");
    routedParts = pathParts.slice(1);
  }
  const path = routedParts.map(encodeURIComponent).join("/");
  return `${normalizedBase}/${path}${search ? `?${search}` : ""}`;
}

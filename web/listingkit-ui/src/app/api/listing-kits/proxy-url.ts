export function getListingKitUpstreamBase() {
  return (
    process.env.LISTINGKIT_API_BASE ??
    process.env.NEXT_PUBLIC_LISTINGKIT_API_BASE ??
    "http://localhost:8080/api/v1/listing-kits"
  );
}

export function buildListingKitProxyUrl(
  upstreamBase: string,
  pathParts: string[],
  search: string,
) {
  const normalizedBase = upstreamBase.replace(/\/+$/, "");
  const path = pathParts.map(encodeURIComponent).join("/");
  return `${normalizedBase}/${path}${search ? `?${search}` : ""}`;
}

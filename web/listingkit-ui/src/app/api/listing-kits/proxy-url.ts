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
  method: string,
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
    if (!isAllowedImageAgentRoute(method, pathParts.slice(1))) {
      throw new Error("image-agent proxy route is not allowed");
    }
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

const safeID = /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/;

function isAllowedImageAgentRoute(method: string, route: string[]) {
  const verb = method.toUpperCase();
  if (verb === "POST" && route.length === 1 && route[0] === "runs") return true;
  if (route[0] !== "runs" || !safeID.test(route[1] ?? "")) return false;
  if (verb === "GET" && route.length === 2) return true;
  if (verb === "PUT" && route.length === 3 && route[2] === "plan") return true;
  if (verb === "POST" && route.length === 3 && route[2] === "cancel") return true;
  if (verb === "GET" && route.length === 3 && route[2] === "events") return true;
  if (verb === "POST" && route.length === 4 && route[2] === "results" && route[3] === "approve") return true;
  if (verb === "POST" && route.length === 5 && route[2] === "slots" && safeID.test(route[3] ?? "") && route[4] === "retry") return true;
  return verb === "POST" && route.length === 5 && route[2] === "commands" && safeID.test(route[3] ?? "") && route[4] === "resume";
}

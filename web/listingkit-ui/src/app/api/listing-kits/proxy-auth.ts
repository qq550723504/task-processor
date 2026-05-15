import type { ZitadelVerifiedIdentity } from "@/lib/server/zitadel-auth";

export type VerifiedIdentity = ZitadelVerifiedIdentity;

export function shouldBypassListingKitProxyAuth() {
  return (
    process.env.NODE_ENV !== "production" &&
    process.env.LISTINGKIT_UI_BYPASS_AUTH_GATE === "1"
  );
}

export function buildListingKitUpstreamHeaders(
  requestHeaders: Headers,
  verifiedIdentity?: VerifiedIdentity,
) {
  const headers = new Headers();
  headers.set("Accept", requestHeaders.get("accept") ?? "application/json");

  copyHeader(requestHeaders, headers, "if-none-match", "If-None-Match");
  copyHeader(requestHeaders, headers, "content-type", "Content-Type");
  copyHeader(requestHeaders, headers, "authorization", "Authorization");

  const tenantID = stringifyIdentityValue(
    verifiedIdentity?.tenantId ?? requestHeaders.get("tenant-id"),
  );
  const userID = stringifyIdentityValue(verifiedIdentity?.userId);
  const userType = stringifyIdentityValue(verifiedIdentity?.userType);

  if (tenantID) {
    headers.set("tenant-id", tenantID);
    headers.set("X-Tenant-ID", tenantID);
  }
  if (userID) {
    headers.set("X-User-ID", userID);
  }
  if (userType) {
    headers.set("X-User-Type", userType);
  }

  return headers;
}

function copyHeader(
  source: Headers,
  target: Headers,
  sourceName: string,
  targetName: string,
) {
  const value = source.get(sourceName);
  if (value) {
    target.set(targetName, value);
  }
}

function stringifyIdentityValue(value: unknown) {
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value);
  }
  if (typeof value === "string" && value.trim()) {
    return value.trim();
  }
  return "";
}

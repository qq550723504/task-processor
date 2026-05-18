import { NextRequest, NextResponse } from "next/server";

import { auth } from "@/auth";
import {
  authorizeZitadelIdentity,
  fetchZitadelDiscovery,
  getZitadelAuthOptions,
  readZitadelAccessTokenFromSession,
  readZitadelIdentityFromSession,
  readZitadelSessionError,
  verifyZitadelAccessToken,
  type ZitadelVerifiedIdentity,
} from "@/lib/server/zitadel-auth";

export type VerifiedIdentity = ZitadelVerifiedIdentity;
export type VerifiedIdentityResult =
  | {
      identity: VerifiedIdentity;
      token: string;
      response?: undefined;
    }
  | { identity?: undefined; token?: undefined; response: NextResponse };

export function shouldBypassListingKitProxyAuth() {
  return (
    process.env.NODE_ENV !== "production" &&
    process.env.LISTINGKIT_UI_BYPASS_AUTH_GATE === "1"
  );
}

function buildLocalBypassIdentity(): VerifiedIdentity {
  const tenantId =
    process.env.LISTINGKIT_UI_LOCAL_TENANT_ID?.trim() || "1";
  const userId = process.env.LISTINGKIT_UI_LOCAL_USER_ID?.trim() || undefined;
  const userType =
    process.env.LISTINGKIT_UI_LOCAL_USER_TYPE?.trim() || "local";
  const roles = (
    process.env.LISTINGKIT_UI_LOCAL_ROLES?.trim() ||
    "platform_admin,listingkit_admin"
  )
    .split(",")
    .map((role) => role.trim())
    .filter(Boolean);

  return {
    tenantId,
    userId,
    userType,
    roles,
  };
}

export async function verifyListingKitRequestIdentity(
  request: NextRequest,
): Promise<VerifiedIdentityResult> {
  const zitadelOptions = getZitadelAuthOptions();
  if (!zitadelOptions) {
    if (shouldBypassListingKitProxyAuth()) {
      return { identity: buildLocalBypassIdentity(), token: "" };
    }
    return {
      response: NextResponse.json(
        {
          error: "zitadel_auth_not_configured",
          message: "ZITADEL authentication is not configured",
        },
        { status: 503 },
      ),
    };
  }

  try {
    const headerToken = extractBearerToken(request.headers.get("authorization"));
    let identity: VerifiedIdentity | null = null;
    let zitadelToken = headerToken;

    if (headerToken) {
      const discovery = await fetchZitadelDiscovery(zitadelOptions);
      identity = await verifyZitadelAccessToken(
        headerToken,
        zitadelOptions,
        discovery,
      );
    } else {
      const session = await auth();
      const sessionError = readZitadelSessionError(session);
      if (sessionError) {
        throw new Error(sessionError);
      }
      zitadelToken = readZitadelAccessTokenFromSession(session);
      identity = readZitadelIdentityFromSession(session);
    }

    if (!zitadelToken || !identity) {
      throw new Error("Missing ZITADEL session");
    }

    const authorization = authorizeZitadelIdentity(identity);
    if (!authorization.authorized) {
      return {
        response: NextResponse.json(
          {
            error: "zitadel_access_denied",
            message: authorization.reason ?? "ZITADEL access denied",
          },
          { status: 403 },
        ),
      };
    }
    return {
      identity,
      token: zitadelToken,
    };
  } catch (error) {
    return {
      response: NextResponse.json(
        {
          error: "zitadel_token_invalid",
          message:
            error instanceof Error
              ? error.message
              : "ZITADEL token verification failed",
        },
        { status: 401 },
      ),
    };
  }
}

export function hasStoredListingKitSession(request: NextRequest) {
  return Boolean(request.cookies.get("authjs.session-token") || request.cookies.get("__Secure-authjs.session-token"));
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
  const userRoles = verifiedIdentity?.roles
    ?.map((role) => stringifyIdentityValue(role))
    .filter(Boolean)
    .join(",");

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
  if (userRoles) {
    headers.set("X-User-Roles", userRoles);
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

function extractBearerToken(authorization: string | null) {
  const match = authorization?.match(/^Bearer\s+(.+)$/i);
  return match?.[1]?.trim() ?? "";
}

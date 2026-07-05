import { NextRequest, NextResponse } from "next/server";

import { auth } from "@/auth";
import { LISTINGKIT_TRACE_HEADER_NAMES } from "@/lib/listingkit/request-trace";
import {
  getZitadelAuthOptions,
  getListingKitLocalDebugIdentity,
  isListingKitLocalAuthBypassed,
  readZitadelAccessTokenFromSession,
  readZitadelSessionClientID,
  readZitadelIdentityFromSession,
  readZitadelSessionError,
  readZitadelSessionIssuerURL,
  type ZitadelVerifiedIdentity,
} from "@/lib/server/zitadel-auth";
import { logRequestInfo, logRequestWarn } from "@/lib/server/request-log";

export type VerifiedIdentity = ZitadelVerifiedIdentity;
export type VerifiedIdentityResult =
  | {
      identity?: VerifiedIdentity;
      token: string;
      response?: undefined;
    }
  | { identity?: undefined; token?: undefined; response: NextResponse };

export async function verifyListingKitRequestIdentity(
  request: NextRequest,
): Promise<VerifiedIdentityResult> {
  if (isListingKitLocalAuthBypassed()) {
    return {
      identity: getListingKitLocalDebugIdentity(),
      token: "",
    };
  }

  const zitadelOptions = getZitadelAuthOptions();
  if (!zitadelOptions) {
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
    if (headerToken) {
      logRequestInfo("listingkit proxy auth forwarding bearer token", {
        path: request.nextUrl.pathname,
      });
      return {
        token: headerToken,
      };
    }

    const session = await auth();
    const sessionError = readZitadelSessionError(session);
    if (sessionError) {
      logRequestWarn("listingkit proxy auth session error", {
        path: request.nextUrl.pathname,
        error: sessionError,
      });
      throw new Error(sessionError);
    }
    const zitadelToken = readZitadelAccessTokenFromSession(session);
    const storedIssuerURL = readZitadelSessionIssuerURL(session);
    const storedClientID = readZitadelSessionClientID(session);
    const identity = readZitadelIdentityFromSession(session) ?? undefined;

    logRequestInfo("listingkit proxy auth forwarding stored session", {
      path: request.nextUrl.pathname,
      hasToken: Boolean(zitadelToken),
      hasIdentity: Boolean(identity),
      storedIssuerURL,
      storedClientID,
      configuredIssuerURL: zitadelOptions.issuerUrl,
      configuredClientID: zitadelOptions.clientId,
    });

    if (!zitadelToken) {
      throw new Error("Missing ZITADEL session");
    }

    if (
      (storedIssuerURL && storedIssuerURL !== zitadelOptions.issuerUrl) ||
      (storedClientID && storedClientID !== zitadelOptions.clientId)
    ) {
      throw new Error(
        "Stored ZITADEL session was created for a different issuer/client; please sign in again",
      );
    }

    return {
      identity,
      token: zitadelToken,
    };
  } catch (error) {
    logRequestWarn("listingkit proxy auth rejected request", {
      path: request.nextUrl.pathname,
      error:
        error instanceof Error
          ? error.message
          : "ZITADEL token verification failed",
    });
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
  for (const headerName of Object.values(LISTINGKIT_TRACE_HEADER_NAMES)) {
    copyHeader(requestHeaders, headers, headerName, headerName);
  }

  const scopedIdentity = isLocalDebugIdentity(verifiedIdentity)
    ? undefined
    : verifiedIdentity;
  const tenantID = stringifyIdentityValue(
    scopedIdentity?.tenantId ?? requestHeaders.get("tenant-id"),
  );
  const userID = stringifyIdentityValue(scopedIdentity?.userId);
  const userType = stringifyIdentityValue(scopedIdentity?.userType);
  const userRoles = scopedIdentity?.roles
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

function isLocalDebugIdentity(identity?: VerifiedIdentity) {
  return (
    identity?.userType === "local_debug" &&
    identity.tenantId === "local-debug" &&
    identity.userId === "local-debug"
  );
}

function extractBearerToken(authorization: string | null) {
  const match = authorization?.match(/^Bearer\s+(.+)$/i);
  return match?.[1]?.trim() ?? "";
}

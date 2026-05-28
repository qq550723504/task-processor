import { NextRequest, NextResponse } from "next/server";

import { auth } from "@/auth";
import {
  authorizeZitadelIdentity,
  fetchZitadelDiscovery,
  getZitadelAuthOptions,
  readZitadelAccessTokenFromSession,
  readZitadelSessionClientID,
  readZitadelIdentityFromSession,
  readZitadelSessionError,
  readZitadelSessionIssuerURL,
  verifyZitadelAccessToken,
  type ZitadelVerifiedIdentity,
} from "@/lib/server/zitadel-auth";
import { logRequestInfo, logRequestWarn } from "@/lib/server/request-log";

export type VerifiedIdentity = ZitadelVerifiedIdentity;
export type VerifiedIdentityResult =
  | {
      identity: VerifiedIdentity;
      token: string;
      response?: undefined;
    }
  | { identity?: undefined; token?: undefined; response: NextResponse };

export async function verifyListingKitRequestIdentity(
  request: NextRequest,
): Promise<VerifiedIdentityResult> {
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
    let identity: VerifiedIdentity | null = null;
    let zitadelToken = headerToken;

    if (headerToken) {
      logRequestInfo("listingkit proxy auth using bearer token", {
        path: request.nextUrl.pathname,
      });
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
        logRequestWarn("listingkit proxy auth session error", {
          path: request.nextUrl.pathname,
          error: sessionError,
        });
        throw new Error(sessionError);
      }
      zitadelToken = readZitadelAccessTokenFromSession(session);
      const storedIssuerURL = readZitadelSessionIssuerURL(session);
      const storedClientID = readZitadelSessionClientID(session);
      identity = readZitadelIdentityFromSession(session);

      logRequestInfo("listingkit proxy auth inspecting stored session", {
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

      if (!identity || !storedIssuerURL || !storedClientID) {
        logRequestInfo("listingkit proxy auth revalidating stored session", {
          path: request.nextUrl.pathname,
          reason: !identity
            ? "missing_identity"
            : !storedIssuerURL
              ? "missing_issuer"
              : "missing_client",
        });
        const discovery = await fetchZitadelDiscovery(zitadelOptions);
        identity = await verifyZitadelAccessToken(
          zitadelToken,
          zitadelOptions,
          discovery,
        );
      }
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

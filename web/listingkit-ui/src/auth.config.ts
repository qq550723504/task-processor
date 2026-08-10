import { customFetch } from "@auth/core";
import type { NextAuthConfig } from "next-auth";
import type { JWT } from "next-auth/jwt";
import ZITADEL from "next-auth/providers/zitadel";

import { createResilientOidcFetch } from "@/lib/server/auth-fetch";
import {
  extractZitadelIdentityFromClaims,
  normalizeClaim,
  type ListingKitSessionIdentity,
  type ZitadelTokenPayload,
  ZITADEL_IDENTITY_VERSION,
} from "@/lib/server/zitadel-identity";

export type { ListingKitSessionIdentity };

declare module "next-auth" {
  interface Session {
    accessToken?: string;
    idToken?: string;
    expiresAt?: number;
    error?: string;
    issuerUrl?: string;
    clientId?: string;
    identity?: ListingKitSessionIdentity | null;
    identityVersion?: number;
  }
}

declare module "next-auth/jwt" {
  interface JWT {
    accessToken?: string;
    refreshToken?: string;
    idToken?: string;
    expiresAt?: number;
    error?: string;
    issuerUrl?: string;
    clientId?: string;
    identity?: ListingKitSessionIdentity | null;
    identityVersion?: number;
  }
}

export type ZitadelAuthOptions = {
  issuerUrl: string;
  clientId: string;
  clientSecret?: string;
  redirectUri?: string;
  postLogoutRedirectUri?: string;
  scopes: string;
};

type ZitadelDiscovery = {
  token_endpoint: string;
};

type ZitadelTokenResponse = {
  access_token?: string;
  id_token?: string;
  refresh_token?: string;
  expires_in?: number;
  error?: string;
  error_description?: string;
};

const DEFAULT_ZITADEL_SCOPES = [
  "openid",
  "profile",
  "email",
  "urn:zitadel:iam:user:resourceowner",
  "urn:zitadel:iam:org:project:id:zitadel:aud",
  "urn:zitadel:iam:org:project:role:listingkit_viewer",
  "urn:zitadel:iam:org:project:role:listingkit_operator",
  "urn:zitadel:iam:org:project:role:listingkit_admin",
  "urn:zitadel:iam:org:project:role:platform_admin",
].join(" ");

export function getZitadelAuthOptions(): ZitadelAuthOptions | undefined {
  const issuerUrl = process.env.ZITADEL_ISSUER_URL?.trim().replace(/\/+$/, "");
  const clientId = process.env.ZITADEL_CLIENT_ID?.trim();
  if (!issuerUrl || !clientId) {
    return undefined;
  }

  return {
    issuerUrl,
    clientId,
    clientSecret: process.env.ZITADEL_CLIENT_SECRET?.trim() || undefined,
    redirectUri: process.env.ZITADEL_REDIRECT_URI?.trim() || undefined,
    postLogoutRedirectUri:
      process.env.ZITADEL_POST_LOGOUT_REDIRECT_URI?.trim() || undefined,
    scopes:
      process.env.ZITADEL_SCOPES?.trim() ||
      DEFAULT_ZITADEL_SCOPES,
  };
}

export function isZitadelAuthConfigured() {
  return Boolean(getZitadelAuthOptions());
}

export function getAuthJsSecret() {
  return (
    process.env.AUTH_SECRET?.trim() ||
    process.env.NEXTAUTH_SECRET?.trim() ||
    process.env.ZITADEL_CLIENT_SECRET?.trim() ||
    process.env.ZITADEL_CLIENT_ID?.trim() ||
    undefined
  );
}

export function buildAuthConfig(): NextAuthConfig {
  const zitadel = getZitadelAuthOptions();
  const oidcFetch = createResilientOidcFetch({
    onRetry(input, attempt, error) {
      console.warn("[listingkit-ui] retrying OIDC request", {
        url: String(input),
        attempt,
        error:
          error instanceof Error
            ? {
                message: error.message,
                cause:
                  error instanceof Error &&
                  "cause" in error &&
                  error.cause &&
                  typeof error.cause === "object"
                    ? {
                        code: Reflect.get(error.cause, "code"),
                      }
                    : undefined,
              }
            : undefined,
      });
    },
  });
  const publicOrigin =
    process.env.LISTINGKIT_PUBLIC_BASE_URL?.trim() ||
    process.env.TASK_PROCESSOR_LISTINGKIT_PUBLIC_BASE_URL?.trim() ||
    process.env.NEXT_PUBLIC_APP_URL?.trim() ||
    process.env.APP_URL?.trim() ||
    "";
  const normalizedPublicOrigin = publicOrigin.replace(/\/+$/, "");
  const postLogoutRedirect =
    zitadel?.postLogoutRedirectUri || normalizedPublicOrigin || "/";
  const zitadelProvider = zitadel
    ? ZITADEL({
        issuer: zitadel.issuerUrl,
        clientId: zitadel.clientId,
        clientSecret: zitadel.clientSecret,
        authorization: { params: { scope: zitadel.scopes } },
      })
    : undefined;

  // Auth.js exposes customFetch on the resolved provider configuration, while
  // the ZITADEL factory input type does not include the symbol yet.
  if (zitadelProvider) {
    zitadelProvider[customFetch] = oidcFetch;
  }

  return {
    secret: getAuthJsSecret(),
    trustHost: true,
    session: { strategy: "jwt" },
    providers: zitadelProvider ? [zitadelProvider] : [],
    callbacks: {
      async jwt({ token, account, profile }) {
        if (account?.provider === "zitadel") {
          const identity = profile
            ? extractZitadelIdentityFromClaims(profile as ZitadelTokenPayload)
            : null;
          return {
            ...token,
            accessToken: account.access_token,
            refreshToken: account.refresh_token,
            idToken: account.id_token,
            expiresAt:
              typeof account.expires_at === "number"
                ? account.expires_at
                : typeof account.expires_in === "number"
                  ? Math.floor(Date.now() / 1000) + account.expires_in
                  : undefined,
            issuerUrl: zitadel?.issuerUrl,
            clientId: zitadel?.clientId,
            error: undefined,
            identity,
            identityVersion: identity ? ZITADEL_IDENTITY_VERSION : undefined,
          } satisfies JWT;
        }

        if (!token.accessToken || !token.expiresAt) {
          return token;
        }

        if (token.expiresAt > Math.floor(Date.now() / 1000) + 30) {
          return token;
        }

        if (!zitadel) {
          return { ...token, error: "ZITADEL auth is not configured" };
        }

        try {
          const refreshed = await refreshZitadelToken(token, zitadel, oidcFetch);
          return {
            ...token,
            ...refreshed,
            issuerUrl: zitadel.issuerUrl,
            clientId: zitadel.clientId,
            error: undefined,
          } satisfies JWT;
        } catch (error) {
          return {
            ...token,
            identity: null,
            identityVersion: undefined,
            error:
              error instanceof Error
                ? error.message
                : "Failed to refresh ZITADEL session",
          } satisfies JWT;
        }
      },
      async session({ session, token }) {
        session.accessToken = token.accessToken;
        session.idToken = token.idToken;
        session.expiresAt = token.expiresAt;
        session.error = token.error;
        session.issuerUrl = token.issuerUrl;
        session.clientId = token.clientId;
        session.identity = hasCurrentZitadelIdentity(token)
          ? token.identity
          : null;
        session.identityVersion = hasCurrentZitadelIdentity(token)
          ? ZITADEL_IDENTITY_VERSION
          : undefined;
        return session;
      },
      async redirect({ url, baseUrl }) {
        if (url.startsWith("/")) {
          return `${baseUrl}${url}`;
        }
        try {
          const target = new URL(url);
          if (target.origin === baseUrl) {
            return target.toString();
          }
          const issuer = zitadel?.issuerUrl;
          if (issuer && target.origin === new URL(issuer).origin) {
            return target.toString();
          }
        } catch {
          return postLogoutRedirect;
        }
        return postLogoutRedirect;
      },
    },
  };
}

async function refreshZitadelToken(
  token: JWT,
  options: ZitadelAuthOptions,
  fetchImpl: typeof fetch,
) {
  if (!token.refreshToken) {
    throw new Error("Missing ZITADEL refresh token");
  }

  const discovery = await fetchZitadelDiscovery(options, fetchImpl);
  const response = await fetchImpl(discovery.token_endpoint, {
    method: "POST",
    headers: buildZitadelClientAuthHeaders(options),
    body: new URLSearchParams({
      grant_type: "refresh_token",
      refresh_token: token.refreshToken,
      client_id: options.clientId,
    }).toString(),
    cache: "no-store",
  });
  const payload = (await response.json().catch(() => undefined)) as
    | ZitadelTokenResponse
    | undefined;

  if (!response.ok || !payload?.access_token) {
    throw new Error(
      payload?.error_description ??
        payload?.error ??
        `ZITADEL refresh token exchange failed: ${response.status}`,
    );
  }

  const includesIDToken = Object.hasOwn(payload, "id_token");
  const identity = includesIDToken
    ? extractIdentityFromTokenPayload(payload.id_token)
    : hasCurrentZitadelIdentity(token)
      ? token.identity
      : null;
  if (includesIDToken && !identity) {
    throw new Error("Refreshed ZITADEL ID token is missing a canonical subject");
  }

  return {
    accessToken: payload.access_token,
    refreshToken: payload.refresh_token ?? token.refreshToken,
    idToken: payload.id_token ?? token.idToken,
    expiresAt: payload.expires_in
      ? Math.floor(Date.now() / 1000) + payload.expires_in
      : token.expiresAt,
    identity,
    identityVersion: identity ? ZITADEL_IDENTITY_VERSION : undefined,
  } satisfies Partial<JWT>;
}

function hasCurrentZitadelIdentity(
  token: Pick<JWT, "identity" | "identityVersion">,
): token is JWT & { identity: ListingKitSessionIdentity } {
  return Boolean(
    token.identityVersion === ZITADEL_IDENTITY_VERSION &&
      normalizeClaim(token.identity?.tenantId) &&
      normalizeClaim(token.identity?.userId),
  );
}

async function fetchZitadelDiscovery(
  options: ZitadelAuthOptions,
  fetchImpl: typeof fetch,
) {
  const response = await fetchImpl(
    `${options.issuerUrl}/.well-known/openid-configuration`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw new Error(`ZITADEL discovery failed: ${response.status}`);
  }
  return (await response.json()) as ZitadelDiscovery;
}

function buildZitadelClientAuthHeaders(options: ZitadelAuthOptions) {
  const headers = new Headers({
    "Content-Type": "application/x-www-form-urlencoded",
  });
  if (options.clientSecret) {
    headers.set(
      "Authorization",
      `Basic ${Buffer.from(`${options.clientId}:${options.clientSecret}`).toString("base64")}`,
    );
  }
  return headers;
}

function extractIdentityFromTokenPayload(
  rawToken: string | undefined,
): ListingKitSessionIdentity | null {
  if (!rawToken) {
    return null;
  }
  const payload = parseJWTClaims(rawToken);
  if (!payload) {
    return null;
  }
  return extractZitadelIdentityFromClaims(payload);
}

function parseJWTClaims(token: string): ZitadelTokenPayload | null {
  const [, payload = ""] = token.split(".");
  if (!payload) {
    return null;
  }
  try {
    return JSON.parse(base64UrlDecode(payload)) as ZitadelTokenPayload;
  } catch {
    return null;
  }
}

function base64UrlDecode(value: string) {
  const padded = value.replace(/-/g, "+").replace(/_/g, "/");
  return Buffer.from(padded, "base64").toString("utf8");
}

import type { NextAuthConfig } from "next-auth";
import type { JWT } from "next-auth/jwt";
import ZITADEL from "next-auth/providers/zitadel";

export type ListingKitSessionIdentity = {
  tenantId?: string | number;
  userId?: string | number;
  username?: string;
  userType?: string | number;
  roles?: string[];
};

declare module "next-auth" {
  interface Session {
    accessToken?: string;
    idToken?: string;
    expiresAt?: number;
    error?: string;
    identity?: ListingKitSessionIdentity | null;
  }
}

declare module "next-auth/jwt" {
  interface JWT {
    accessToken?: string;
    refreshToken?: string;
    idToken?: string;
    expiresAt?: number;
    error?: string;
    identity?: ListingKitSessionIdentity | null;
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
      "openid profile email urn:zitadel:iam:user:resourceowner urn:zitadel:iam:org:project:id:zitadel:aud",
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
  const publicOrigin =
    process.env.LISTINGKIT_PUBLIC_BASE_URL?.trim() ||
    process.env.TASK_PROCESSOR_LISTINGKIT_PUBLIC_BASE_URL?.trim() ||
    process.env.NEXT_PUBLIC_APP_URL?.trim() ||
    process.env.APP_URL?.trim() ||
    "";
  const normalizedPublicOrigin = publicOrigin.replace(/\/+$/, "");
  const postLogoutRedirect =
    zitadel?.postLogoutRedirectUri || normalizedPublicOrigin || "/";

  return {
    secret: getAuthJsSecret(),
    trustHost: true,
    session: { strategy: "jwt" },
    providers: zitadel
      ? [
          ZITADEL({
            issuer: zitadel.issuerUrl,
            clientId: zitadel.clientId,
            clientSecret: zitadel.clientSecret,
            authorization: { params: { scope: zitadel.scopes } },
          }),
        ]
      : [],
    callbacks: {
      async jwt({ token, account, profile }) {
        if (account?.provider === "zitadel") {
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
            error: undefined,
            identity: extractIdentity({
              preferredUsername:
                asString(profile?.preferred_username) ||
                asString(profile?.username),
              subject: asString(profile?.sub),
              resourceOwnerId: asString(
                profile?.["urn:zitadel:iam:user:resourceowner:id"],
              ),
              roles: extractRoles(profile),
            }),
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
          const refreshed = await refreshZitadelToken(token, zitadel);
          return {
            ...token,
            ...refreshed,
            error: undefined,
            identity:
              refreshed.identity ??
              token.identity ??
              extractIdentityFromTokenPayload(refreshed.idToken),
          } satisfies JWT;
        } catch (error) {
          return {
            ...token,
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
        session.identity = token.identity ?? null;
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

async function refreshZitadelToken(token: JWT, options: ZitadelAuthOptions) {
  if (!token.refreshToken) {
    throw new Error("Missing ZITADEL refresh token");
  }

  const discovery = await fetchZitadelDiscovery(options);
  const response = await fetch(discovery.token_endpoint, {
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

  return {
    accessToken: payload.access_token,
    refreshToken: payload.refresh_token ?? token.refreshToken,
    idToken: payload.id_token ?? token.idToken,
    expiresAt: payload.expires_in
      ? Math.floor(Date.now() / 1000) + payload.expires_in
      : token.expiresAt,
    identity:
      extractIdentityFromTokenPayload(payload.id_token) ??
      token.identity ??
      null,
  } satisfies Partial<JWT>;
}

async function fetchZitadelDiscovery(options: ZitadelAuthOptions) {
  const response = await fetch(
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
  return extractIdentity({
    preferredUsername:
      asString(payload.preferred_username) || asString(payload.username),
    subject: asString(payload.sub),
    resourceOwnerId: asString(
      payload["urn:zitadel:iam:user:resourceowner:id"],
    ),
    roles: extractRoles(payload),
  });
}

function extractIdentity(input: {
  preferredUsername?: string;
  subject?: string;
  resourceOwnerId?: string;
  roles?: string[];
}) {
  return {
    tenantId: input.resourceOwnerId,
    userId: input.subject ?? input.preferredUsername,
    username: input.preferredUsername,
    userType: "zitadel",
    roles: input.roles ?? [],
  } satisfies ListingKitSessionIdentity;
}

function extractRoles(payload: Record<string, unknown> | null | undefined) {
  if (!payload) {
    return [];
  }
  const seen = new Set<string>();
  const roles: string[] = [];
  const addRole = (value: unknown) => {
    if (typeof value !== "string") {
      return;
    }
    const normalized = value.trim();
    if (!normalized || seen.has(normalized)) {
      return;
    }
    seen.add(normalized);
    roles.push(normalized);
  };

  for (const raw of [
    payload["urn:zitadel:iam:org:project:roles"],
    payload.roles,
    payload.role,
  ]) {
    if (!raw) {
      continue;
    }
    if (Array.isArray(raw)) {
      raw.forEach(addRole);
      continue;
    }
    if (typeof raw === "string") {
      raw.split(",").forEach(addRole);
      continue;
    }
    if (typeof raw === "object") {
      Object.keys(raw).forEach(addRole);
    }
  }

  return roles;
}

function parseJWTClaims(token: string) {
  const [, payload = ""] = token.split(".");
  if (!payload) {
    return null;
  }
  try {
    return JSON.parse(base64UrlDecode(payload)) as Record<string, unknown>;
  } catch {
    return null;
  }
}

function base64UrlDecode(value: string) {
  const padded = value.replace(/-/g, "+").replace(/_/g, "/");
  return Buffer.from(padded, "base64").toString("utf8");
}

function asString(value: unknown) {
  if (typeof value === "string" && value.trim()) {
    return value.trim();
  }
  return undefined;
}

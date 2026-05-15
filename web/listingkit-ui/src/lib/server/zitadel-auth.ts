import { NextRequest, NextResponse } from "next/server";
import { createHash, randomBytes } from "node:crypto";

const SESSION_COOKIE = "listingkit_zitadel_session";
const STATE_COOKIE = "listingkit_zitadel_state";
const VERIFIER_COOKIE = "listingkit_zitadel_verifier";
const RETURN_TO_COOKIE = "listingkit_zitadel_return_to";
const COOKIE_MAX_AGE_SECONDS = 60 * 60 * 8;
const TRANSIENT_COOKIE_MAX_AGE_SECONDS = 10 * 60;

export type ZitadelAuthOptions = {
  issuerUrl: string;
  clientId: string;
  clientSecret?: string;
  redirectUri?: string;
  postLogoutRedirectUri?: string;
  scopes: string;
};

export type ZitadelDiscovery = {
  authorization_endpoint: string;
  token_endpoint: string;
  introspection_endpoint?: string;
  userinfo_endpoint?: string;
  end_session_endpoint?: string;
};

export type ZitadelSession = {
  accessToken: string;
  idToken?: string;
  refreshToken?: string;
  expiresAt?: number;
};

export type ZitadelVerifiedIdentity = {
  tenantId?: string | number;
  userId?: string | number;
  userType?: string | number;
};

type ZitadelTokenResponse = {
  access_token?: string;
  id_token?: string;
  refresh_token?: string;
  expires_in?: number;
  error?: string;
  error_description?: string;
};

type ZitadelIntrospectionResponse = {
  active?: boolean;
  sub?: string;
  username?: string;
  user_id?: string;
  client_id?: string;
  aud?: string | string[];
  exp?: number;
  "urn:zitadel:iam:user:resourceowner:id"?: string;
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

export function resolveZitadelRedirectUri(
  request: NextRequest,
  options: ZitadelAuthOptions,
) {
  if (options.redirectUri) {
    return options.redirectUri;
  }
  return new URL("/api/zitadel-auth/callback", request.nextUrl.origin).toString();
}

export function resolveZitadelPostLogoutRedirectUri(
  request: NextRequest,
  options: ZitadelAuthOptions,
) {
  if (options.postLogoutRedirectUri) {
    return options.postLogoutRedirectUri;
  }
  return request.nextUrl.origin;
}

export async function fetchZitadelDiscovery(
  options: ZitadelAuthOptions,
): Promise<ZitadelDiscovery> {
  const response = await fetch(
    `${options.issuerUrl}/.well-known/openid-configuration`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw new Error(`ZITADEL discovery failed: ${response.status}`);
  }
  return (await response.json()) as ZitadelDiscovery;
}

export function getZitadelSession(request: NextRequest): ZitadelSession | undefined {
  const raw = request.cookies.get(SESSION_COOKIE)?.value;
  if (!raw) {
    return undefined;
  }
  try {
    const session = JSON.parse(base64UrlDecode(raw)) as ZitadelSession;
    if (!session.accessToken) {
      return undefined;
    }
    if (session.expiresAt && session.expiresAt <= Math.floor(Date.now() / 1000)) {
      return undefined;
    }
    return session;
  } catch {
    return undefined;
  }
}

export function setZitadelSessionCookie(
  response: NextResponse,
  session: ZitadelSession,
) {
  response.cookies.set(SESSION_COOKIE, base64UrlEncode(JSON.stringify(session)), {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: COOKIE_MAX_AGE_SECONDS,
  });
}

export function clearZitadelCookies(response: NextResponse) {
  for (const name of [
    SESSION_COOKIE,
    STATE_COOKIE,
    VERIFIER_COOKIE,
    RETURN_TO_COOKIE,
  ]) {
    response.cookies.set(name, "", { path: "/", maxAge: 0 });
  }
}

export function getZitadelBearerToken(request: NextRequest) {
  const headerToken = extractBearerToken(request.headers.get("authorization"));
  if (headerToken) {
    return headerToken;
  }
  return getZitadelSession(request)?.accessToken ?? "";
}

export async function verifyZitadelAccessToken(
  token: string,
  options: ZitadelAuthOptions,
  discovery?: ZitadelDiscovery,
): Promise<ZitadelVerifiedIdentity> {
  if (!token) {
    throw new Error("Missing ZITADEL bearer token");
  }

  const metadata = discovery ?? (await fetchZitadelDiscovery(options));
  if (!metadata.introspection_endpoint) {
    throw new Error("ZITADEL introspection endpoint is not available");
  }

  const response = await fetch(metadata.introspection_endpoint, {
    method: "POST",
    headers: buildZitadelClientAuthHeaders(options),
    body: new URLSearchParams({
      token,
      token_type_hint: "access_token",
    }).toString(),
    cache: "no-store",
  });
  const payload = (await response.json().catch(() => undefined)) as
    | ZitadelIntrospectionResponse
    | undefined;

  if (!response.ok || !payload?.active) {
    throw new Error(`ZITADEL token introspection failed: ${response.status}`);
  }

  return {
    tenantId: payload["urn:zitadel:iam:user:resourceowner:id"],
    userId: payload.sub ?? payload.user_id ?? payload.username,
    userType: "zitadel",
  };
}

export function createZitadelAuthRequest(
  request: NextRequest,
  options: ZitadelAuthOptions,
  discovery: ZitadelDiscovery,
) {
  const state = randomBase64Url(32);
  const verifier = randomBase64Url(64);
  const redirectUri = resolveZitadelRedirectUri(request, options);
  const returnTo = normalizeReturnTo(request.nextUrl.searchParams.get("returnTo"));
  const params = new URLSearchParams({
    client_id: options.clientId,
    redirect_uri: redirectUri,
    response_type: "code",
    scope: options.scopes,
    state,
    code_challenge: base64UrlEncode(sha256(verifier)),
    code_challenge_method: "S256",
  });

  const url = `${discovery.authorization_endpoint}?${params.toString()}`;
  const response = NextResponse.redirect(url);
  response.cookies.set(STATE_COOKIE, state, transientCookieOptions());
  response.cookies.set(VERIFIER_COOKIE, verifier, transientCookieOptions());
  response.cookies.set(RETURN_TO_COOKIE, returnTo, transientCookieOptions());
  return response;
}

export async function exchangeZitadelAuthorizationCode(
  request: NextRequest,
  options: ZitadelAuthOptions,
  discovery: ZitadelDiscovery,
) {
  const code = request.nextUrl.searchParams.get("code") ?? "";
  const state = request.nextUrl.searchParams.get("state") ?? "";
  const expectedState = request.cookies.get(STATE_COOKIE)?.value ?? "";
  const verifier = request.cookies.get(VERIFIER_COOKIE)?.value ?? "";
  if (!code || !state || !expectedState || state !== expectedState || !verifier) {
    throw new Error("Invalid ZITADEL authorization callback");
  }

  const response = await fetch(discovery.token_endpoint, {
    method: "POST",
    headers: buildZitadelClientAuthHeaders(options),
    body: new URLSearchParams({
      grant_type: "authorization_code",
      code,
      redirect_uri: resolveZitadelRedirectUri(request, options),
      code_verifier: verifier,
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
        `ZITADEL token exchange failed: ${response.status}`,
    );
  }

  return {
    session: {
      accessToken: payload.access_token,
      idToken: payload.id_token,
      refreshToken: payload.refresh_token,
      expiresAt: payload.expires_in
        ? Math.floor(Date.now() / 1000) + payload.expires_in
        : undefined,
    },
    returnTo: request.cookies.get(RETURN_TO_COOKIE)?.value || "/",
  };
}

export function buildZitadelLogoutResponse(
  request: NextRequest,
  options: ZitadelAuthOptions,
  discovery: ZitadelDiscovery,
) {
  const session = getZitadelSession(request);
  const fallbackUrl = resolveZitadelPostLogoutRedirectUri(request, options);
  const logoutUrl = discovery.end_session_endpoint
    ? new URL(discovery.end_session_endpoint)
    : new URL(fallbackUrl);
  if (discovery.end_session_endpoint) {
    logoutUrl.searchParams.set("client_id", options.clientId);
    logoutUrl.searchParams.set("post_logout_redirect_uri", fallbackUrl);
    if (session?.idToken) {
      logoutUrl.searchParams.set("id_token_hint", session.idToken);
    }
  }
  const response = NextResponse.redirect(logoutUrl);
  clearZitadelCookies(response);
  return response;
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

function transientCookieOptions() {
  return {
    httpOnly: true,
    sameSite: "lax" as const,
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: TRANSIENT_COOKIE_MAX_AGE_SECONDS,
  };
}

function normalizeReturnTo(value: string | null) {
  if (!value || !value.startsWith("/") || value.startsWith("//")) {
    return "/";
  }
  return value;
}

function extractBearerToken(authorization: string | null) {
  const match = authorization?.match(/^Bearer\s+(.+)$/i);
  return match?.[1]?.trim() ?? "";
}

function randomBase64Url(byteLength: number) {
  return base64UrlEncode(randomBytes(byteLength));
}

function sha256(value: string) {
  return createHash("sha256").update(value).digest();
}

function base64UrlEncode(value: string | ArrayBuffer | Uint8Array | Buffer) {
  const buffer =
    typeof value === "string"
      ? Buffer.from(value, "utf8")
      : Buffer.from(value as Uint8Array);
  return buffer
    .toString("base64")
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/g, "");
}

function base64UrlDecode(value: string) {
  const padded = value.replace(/-/g, "+").replace(/_/g, "/");
  return Buffer.from(padded, "base64").toString("utf8");
}

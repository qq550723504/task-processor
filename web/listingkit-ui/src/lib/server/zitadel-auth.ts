import type { Session } from "next-auth";

import {
  getZitadelAuthOptions,
  type ListingKitSessionIdentity,
  type ZitadelAuthOptions,
} from "@/auth.config";

export type ZitadelDiscovery = {
  authorization_endpoint: string;
  token_endpoint: string;
  introspection_endpoint?: string;
  userinfo_endpoint?: string;
  end_session_endpoint?: string;
};

export type ZitadelVerifiedIdentity = ListingKitSessionIdentity;

export type ZitadelAuthorizationResult = {
  authorized: boolean;
  required: boolean;
  reason?: string;
};

const LOCAL_DEBUG_DEFAULT_ROLES = [
  "platform_admin",
  "listingkit_admin",
  "listingkit_operator",
];

const LOCAL_DEBUG_IDENTITY: ZitadelVerifiedIdentity = {
  tenantId: "default",
  userId: "local-debug",
  username: "local-debug",
  userType: "local_debug",
  roles: LOCAL_DEBUG_DEFAULT_ROLES,
};

type ZitadelIntrospectionResponse = {
  active?: boolean;
  sub?: string;
  username?: string;
  user_id?: string;
  "urn:zitadel:iam:user:resourceowner:id"?: string;
  "urn:zitadel:iam:org:project:roles"?: Record<string, unknown> | string[] | string;
  roles?: string[] | string;
  role?: string;
};

export { getZitadelAuthOptions };

export function isZitadelAuthConfigured() {
  return Boolean(getZitadelAuthOptions());
}

export function isListingKitLocalAuthBypassed() {
  const value = process.env.LISTINGKIT_UI_BYPASS_AUTH_GATE?.trim().toLowerCase();
  return value === "1" || value === "true" || value === "yes" || value === "on";
}

export function getListingKitLocalDebugIdentity(): ZitadelVerifiedIdentity {
  return {
    ...LOCAL_DEBUG_IDENTITY,
    tenantId:
      readOptionalEnv("LISTINGKIT_UI_LOCAL_DEBUG_TENANT_ID") ??
      LOCAL_DEBUG_IDENTITY.tenantId,
    userId:
      readOptionalEnv("LISTINGKIT_UI_LOCAL_DEBUG_USER_ID") ??
      LOCAL_DEBUG_IDENTITY.userId,
    username:
      readOptionalEnv("LISTINGKIT_UI_LOCAL_DEBUG_USERNAME") ??
      LOCAL_DEBUG_IDENTITY.username,
    roles:
      readDelimitedEnvList("LISTINGKIT_UI_LOCAL_DEBUG_ROLES") ??
      [...LOCAL_DEBUG_DEFAULT_ROLES],
  };
}

export function resolvePublicAppOrigin() {
  const configuredOrigin =
    process.env.LISTINGKIT_PUBLIC_BASE_URL?.trim() ||
    process.env.TASK_PROCESSOR_LISTINGKIT_PUBLIC_BASE_URL?.trim() ||
    process.env.NEXT_PUBLIC_APP_URL?.trim() ||
    process.env.APP_URL?.trim();

  if (configuredOrigin) {
    return configuredOrigin.replace(/\/+$/, "");
  }

  return "http://localhost:3000";
}

export function normalizeReturnTo(value: string | null) {
  if (!value || !value.startsWith("/") || value.startsWith("//")) {
    return "/";
  }
  if (!isAllowedReturnToPath(value)) {
    return "/";
  }
  return value;
}

function isAllowedReturnToPath(value: string) {
  let pathname: string;
  try {
    pathname = new URL(value, "http://localhost").pathname;
  } catch {
    return false;
  }
  return pathname === "/" || pathname === "/listing-kits" || pathname.startsWith("/listing-kits/");
}

export function readZitadelIdentityFromSession(
  session: Session | null | undefined,
): ZitadelVerifiedIdentity | null {
  const identity = session?.identity;
  if (!identity) {
    return null;
  }
  return {
    tenantId: identity.tenantId,
    userId: identity.userId,
    username: identity.username,
    userType: identity.userType ?? "zitadel",
    roles: identity.roles ?? [],
  };
}

export function readZitadelAccessTokenFromSession(
  session: Session | null | undefined,
) {
  return typeof session?.accessToken === "string" ? session.accessToken : "";
}

export function readZitadelIDTokenFromSession(
  session: Session | null | undefined,
) {
  return typeof session?.idToken === "string" ? session.idToken : "";
}

export function readZitadelSessionError(session: Session | null | undefined) {
  return typeof session?.error === "string" ? session.error : "";
}

export function readZitadelSessionIssuerURL(
  session: Session | null | undefined,
) {
  return typeof session?.issuerUrl === "string" ? session.issuerUrl : "";
}

export function readZitadelSessionClientID(
  session: Session | null | undefined,
) {
  return typeof session?.clientId === "string" ? session.clientId : "";
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
    userId: payload.user_id ?? payload.sub ?? payload.username,
    username: payload.username,
    userType: "zitadel",
    roles: parseZitadelRoles(payload),
  };
}

export function authorizeZitadelIdentity(
  identity: ZitadelVerifiedIdentity,
): ZitadelAuthorizationResult {
  const config = readZitadelAuthorizationConfig();
  if (!config.required) {
    return { authorized: true, required: false };
  }

  if (
    config.allowedTenantIds.size === 0 &&
    config.allowedUserIds.size === 0 &&
    config.allowedUsernames.size === 0 &&
    config.allowedRoles.size === 0
  ) {
    return {
      authorized: false,
      required: true,
      reason: "ZITADEL authorization is required but no allowlist is configured",
    };
  }

  const tenantId = stringifyIdentityValue(identity.tenantId);
  if (tenantId && config.allowedTenantIds.has(tenantId)) {
    return { authorized: true, required: true };
  }

  const userId = stringifyIdentityValue(identity.userId);
  if (userId && config.allowedUserIds.has(userId)) {
    return { authorized: true, required: true };
  }

  const username = stringifyIdentityValue(identity.username);
  if (username && config.allowedUsernames.has(username)) {
    return { authorized: true, required: true };
  }

  for (const role of identity.roles ?? []) {
    const normalizedRole = stringifyIdentityValue(role);
    if (normalizedRole && config.allowedRoles.has(normalizedRole)) {
      return { authorized: true, required: true };
    }
  }

  return {
    authorized: false,
    required: true,
    reason: "ZITADEL identity is not allowed to access ListingKit",
  };
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

function parseZitadelRoles(payload: ZitadelIntrospectionResponse) {
  const seen = new Set<string>();
  const roles: string[] = [];
  const add = (value: string) => {
    const role = value.trim();
    if (!role || seen.has(role)) {
      return;
    }
    seen.add(role);
    roles.push(role);
  };
  for (const value of [
    payload["urn:zitadel:iam:org:project:roles"],
    payload.roles,
    payload.role,
  ]) {
    if (!value) {
      continue;
    }
    if (Array.isArray(value)) {
      value.forEach(add);
      continue;
    }
    if (typeof value === "string") {
      value.split(",").forEach(add);
      continue;
    }
    Object.keys(value).forEach(add);
  }
  return roles;
}

function readZitadelAuthorizationConfig() {
  const allowedTenantIds = readDelimitedEnvSet(
    "LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS",
    "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS",
  );
  const allowedUserIds = readDelimitedEnvSet(
    "LISTINGKIT_ZITADEL_ALLOWED_USER_IDS",
    "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USER_IDS",
  );
  const allowedUsernames = readDelimitedEnvSet(
    "LISTINGKIT_ZITADEL_ALLOWED_USERNAMES",
    "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES",
  );
  const allowedRoles = readDelimitedEnvSet(
    "LISTINGKIT_ZITADEL_ALLOWED_ROLES",
    "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES",
  );

  return {
    required:
      allowedTenantIds.size > 0 ||
      allowedUserIds.size > 0 ||
      allowedUsernames.size > 0 ||
      allowedRoles.size > 0,
    allowedTenantIds,
    allowedUserIds,
    allowedUsernames,
    allowedRoles,
  };
}

function readDelimitedEnvSet(...names: string[]) {
  const values = new Set<string>();
  for (const name of names) {
    const raw = process.env[name];
    if (!raw) {
      continue;
    }
    for (const item of raw.split(",")) {
      const normalized = item.trim();
      if (normalized) {
        values.add(normalized);
      }
    }
  }
  return values;
}

function readDelimitedEnvList(...names: string[]) {
  const values = [...readDelimitedEnvSet(...names)];
  return values.length > 0 ? values : undefined;
}

function readOptionalEnv(...names: string[]) {
  for (const name of names) {
    const value = process.env[name]?.trim();
    if (value) {
      return value;
    }
  }
  return undefined;
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

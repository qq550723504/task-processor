 const RESOURCE_OWNER_CLAIM =
  "urn:zitadel:iam:user:resourceowner:id";
// Bump when the OIDC authorization contract changes so existing Auth.js
// sessions are forced through a fresh authorization request.
export const ZITADEL_IDENTITY_VERSION = 3;

export type ListingKitSessionIdentity = {
  tenantId?: string | number;
  userId?: string | number;
  username?: string;
  userType?: string | number;
  roles?: string[];
};

export type ZitadelTokenPayload = Record<string, unknown> & {
  sub?: unknown;
  preferred_username?: unknown;
  username?: unknown;
  user_id?: unknown;
};

export function extractZitadelIdentityFromClaims(
  payload: ZitadelTokenPayload,
  projectId?: string,
): ListingKitSessionIdentity | null {
  const tenantId = normalizeClaim(payload[RESOURCE_OWNER_CLAIM]);
  const userId = normalizeClaim(payload.sub);
  if (!tenantId || !userId) {
    return null;
  }

  return {
    tenantId,
    userId,
    username: normalizeClaim(payload.preferred_username ?? payload.username),
    userType: "zitadel",
    roles: extractProjectRoles(payload, projectId),
  };
}

export function normalizeClaim(value: unknown) {
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value);
  }
  if (typeof value === "string" && value.trim()) {
    return value.trim();
  }
  return undefined;
}

function extractProjectRoles(payload: ZitadelTokenPayload, projectId?: string) {
  const seen = new Set<string>();
  const roles: string[] = [];
  const add = (value: unknown) => {
    const role = normalizeClaim(value);
    if (!role || seen.has(role)) {
      return;
    }
    seen.add(role);
    roles.push(role);
  };

  const addClaimValue = (value: unknown) => {
    if (!value) {
      return;
    }
    if (Array.isArray(value)) {
      value.forEach(addClaimValue);
      return;
    }
    if (typeof value === "string") {
      value.split(",").forEach(add);
      return;
    }
    if (typeof value === "object") {
      Object.keys(value).forEach(add);
    }
  };

  for (const value of [
    projectId
      ? payload[`urn:zitadel:iam:org:project:${projectId}:roles`]
      : undefined,
    payload["urn:zitadel:iam:org:project:roles"],
    payload.roles,
    payload.role,
  ]) {
    addClaimValue(value);
  }

  return roles;
}

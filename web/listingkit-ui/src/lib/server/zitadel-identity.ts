export const RESOURCE_OWNER_CLAIM =
  "urn:zitadel:iam:user:resourceowner:id";
export const ZITADEL_IDENTITY_VERSION = 1;

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
    roles: extractProjectRoles(payload),
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

function extractProjectRoles(payload: ZitadelTokenPayload) {
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
    if (typeof value === "object") {
      Object.keys(value).forEach(add);
    }
  }

  return roles;
}

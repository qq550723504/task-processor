export type YudaoAuthPayload = {
  accessToken?: null | string;
  tenantId?: null | number | string;
  visitTenantId?: null | number | string;
};

const STORAGE_KEY = "listingkit:yudao-auth";
export const YUDAO_AUTH_CHANGED_EVENT = "listingkit:yudao-auth-changed";

function normalizeString(value: unknown) {
  if (value === null || value === undefined) {
    return "";
  }
  return String(value).trim();
}

function getSessionStorage() {
  if (typeof window === "undefined") {
    return undefined;
  }
  return window.sessionStorage;
}

export function rememberYudaoAuth(payload: YudaoAuthPayload) {
  const accessToken = normalizeString(payload.accessToken);
  const tenantId = normalizeString(payload.tenantId);
  const visitTenantId = normalizeString(payload.visitTenantId);

  getSessionStorage()?.setItem(
    STORAGE_KEY,
    JSON.stringify({ accessToken, tenantId, visitTenantId }),
  );
  notifyYudaoAuthChanged();
}

export function readYudaoAuth(): YudaoAuthPayload {
  const raw = getSessionStorage()?.getItem(STORAGE_KEY);
  if (!raw) {
    return {};
  }

  try {
    const parsed = JSON.parse(raw) as YudaoAuthPayload;
    return {
      accessToken: normalizeString(parsed.accessToken),
      tenantId: normalizeString(parsed.tenantId),
      visitTenantId: normalizeString(parsed.visitTenantId),
    };
  } catch {
    return {};
  }
}

export function clearYudaoAuth() {
  getSessionStorage()?.removeItem(STORAGE_KEY);
  notifyYudaoAuthChanged();
}

export function hasRequiredYudaoAuth(payload: YudaoAuthPayload) {
  return Boolean(
    normalizeString(payload.accessToken) && normalizeString(payload.tenantId),
  );
}

export function applyYudaoAuthHeaders(headers: Headers) {
  const auth = readYudaoAuth();
  const accessToken = normalizeString(auth.accessToken);
  const tenantId = normalizeString(auth.tenantId);
  const visitTenantId = normalizeString(auth.visitTenantId);

  if (accessToken) {
    headers.set("Authorization", `Bearer ${accessToken}`);
  }
  if (tenantId) {
    headers.set("tenant-id", tenantId);
  }
  if (visitTenantId) {
    headers.set("visit-tenant-id", visitTenantId);
  }

  return headers;
}

function notifyYudaoAuthChanged() {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new Event(YUDAO_AUTH_CHANGED_EVENT));
}

export function isYudaoAuthMessage(data: unknown): data is {
  payload: YudaoAuthPayload;
  type: "listingkit:yudao-auth";
} {
  return (
    Boolean(data) &&
    typeof data === "object" &&
    (data as { type?: unknown }).type === "listingkit:yudao-auth" &&
    typeof (data as { payload?: unknown }).payload === "object"
  );
}

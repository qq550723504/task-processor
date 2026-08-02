import type {
  SheinLoginAccountStatus,
  SheinLoginFailureDetail,
  SheinLoginWarehouse,
} from "@/lib/types/shein-login";

async function readJSON<T>(response: Response): Promise<T> {
  const text = await response.text();
  return text ? (JSON.parse(text) as T) : ({} as T);
}

function withTenantScope(path: string, tenantID?: string) {
  const tenant = tenantID?.trim();
  if (!tenant) {
    return path;
  }
  const separator = path.includes("?") ? "&" : "?";
  return `${path}${separator}tenant_id=${encodeURIComponent(tenant)}`;
}

async function request<T>(path: string, init?: RequestInit, tenantID?: string): Promise<T> {
  const response = await fetch(`/api/shein-login${withTenantScope(path, tenantID)}`, {
    ...init,
    headers: new Headers({
      Accept: "application/json",
      ...(init?.headers instanceof Headers
        ? Object.fromEntries(init.headers.entries())
        : (init?.headers as Record<string, string> | undefined)),
    }),
    cache: "no-store",
  });
  const payload = await readJSON<{ success?: boolean; data?: T; message?: string }>(response);
  if (!response.ok || payload.success === false) {
    throw new Error(payload.message ?? `SHEIN login request failed: ${response.status}`);
  }
  return (payload.data ?? (payload as unknown as T)) as T;
}

export function listSheinLoginAccounts(tenantID?: string) {
  return request<SheinLoginAccountStatus[]>("/accounts", undefined, tenantID);
}

export function loginSheinAccount(storeID: number, tenantID?: string) {
  return request(`/accounts/${storeID}/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ force_login: true, headless: true }),
  }, tenantID);
}

export function submitSheinVerifyCode(storeID: number, code: string, tenantID?: string, attemptID?: string) {
  return request(`/accounts/${storeID}/verify-code`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code, ...(attemptID ? { attempt_id: attemptID } : {}) }),
  }, tenantID);
}

export function clearSheinCookie(storeID: number, tenantID?: string) {
  return request(`/accounts/${storeID}/cookie`, {
    method: "DELETE",
  }, tenantID);
}

export function getSheinLastFailure(storeID: number, tenantID?: string) {
  return request<SheinLoginFailureDetail | undefined>(`/accounts/${storeID}/last-failure`, undefined, tenantID);
}

export function listSheinStoreWarehouses(storeID: number) {
  return request<SheinLoginWarehouse[]>(`/accounts/${storeID}/warehouses`);
}

export function clearSheinLastFailure(storeID: number, tenantID?: string) {
  return request(`/accounts/${storeID}/last-failure`, {
    method: "DELETE",
  }, tenantID);
}

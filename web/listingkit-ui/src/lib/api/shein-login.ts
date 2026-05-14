import { applyYudaoAuthHeaders } from "@/lib/api/yudao-auth";
import type {
  SheinLoginAccountStatus,
  SheinLoginFailureDetail,
} from "@/lib/types/shein-login";

async function readJSON<T>(response: Response): Promise<T> {
  const text = await response.text();
  return text ? (JSON.parse(text) as T) : ({} as T);
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api/shein-login${path}`, {
    ...init,
    headers: applyYudaoAuthHeaders(
      new Headers({
        Accept: "application/json",
        ...(init?.headers instanceof Headers
          ? Object.fromEntries(init.headers.entries())
          : (init?.headers as Record<string, string> | undefined)),
      }),
    ),
    cache: "no-store",
  });
  const payload = await readJSON<{ success?: boolean; data?: T; message?: string }>(response);
  if (!response.ok || payload.success === false) {
    throw new Error(payload.message ?? `SHEIN login request failed: ${response.status}`);
  }
  return (payload.data ?? (payload as unknown as T)) as T;
}

export function listSheinLoginAccounts() {
  return request<SheinLoginAccountStatus[]>("/accounts");
}

export function loginSheinAccount(storeID: number) {
  return request(`/accounts/${storeID}/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ force_login: true }),
  });
}

export function submitSheinVerifyCode(storeID: number, code: string) {
  return request(`/accounts/${storeID}/verify-code`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),
  });
}

export function clearSheinCookie(storeID: number) {
  return request(`/accounts/${storeID}/cookie`, {
    method: "DELETE",
  });
}

export function getSheinLastFailure(storeID: number) {
  return request<SheinLoginFailureDetail | undefined>(`/accounts/${storeID}/last-failure`);
}

export function clearSheinLastFailure(storeID: number) {
  return request(`/accounts/${storeID}/last-failure`, {
    method: "DELETE",
  });
}

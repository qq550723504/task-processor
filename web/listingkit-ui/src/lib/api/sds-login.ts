type SDSLoginResponseEnvelope<T> = {
  success?: boolean;
  data?: T;
  message?: string;
};

export type SDSLoginStatus = {
  tenant_id?: string;
  identifier?: string;
  merchant_name?: string;
  username?: string;
  has_cookie?: boolean;
  has_access_token?: boolean;
  merchant_id?: number;
  issued_at?: string;
  source?: string;
  waiting_for_verify_code?: boolean;
  login_in_progress?: boolean;
  last_error?: string;
};

export type SDSLoginAuthState = {
  tenant_id?: string;
  shop_id?: string;
  identifier?: string;
  username?: string;
  merchant_name?: string;
  access_token?: string;
  out_token?: string;
  merchant_id?: number;
  user_id?: number;
  cookies?: Array<{
    name: string;
    value: string;
    domain?: string;
    path?: string;
    expires?: string;
    secure?: boolean;
    httpOnly?: boolean;
  }>;
  browser_state?: Record<string, unknown>;
  issued_at?: string;
  source?: string;
  current_url?: string;
};

export type SDSManualLoginInput = {
  tenantID: string;
  identifier: string;
  merchantName: string;
  username: string;
  password: string;
};

async function readJSON<T>(response: Response): Promise<T> {
  const text = await response.text();
  return text ? (JSON.parse(text) as T) : ({} as T);
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api/sds-login${path}`, {
    ...init,
    headers: new Headers({
      Accept: "application/json",
      ...(init?.headers instanceof Headers
        ? Object.fromEntries(init.headers.entries())
        : (init?.headers as Record<string, string> | undefined)),
    }),
    cache: "no-store",
  });
  const payload = await readJSON<SDSLoginResponseEnvelope<T>>(response);
  if (!response.ok || payload.success === false) {
    throw new Error(payload.message ?? `SDS login request failed: ${response.status}`);
  }
  return (payload.data ?? (payload as unknown as T)) as T;
}

export function getSDSLoginStatus() {
  return request<SDSLoginStatus>("/status");
}

export function triggerSDSLogin() {
  return request("/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ force_login: true }),
  });
}

export function manualSDSLogin(input: SDSManualLoginInput) {
  return request("/manual-login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      tenant_id: input.tenantID,
      identifier: input.identifier,
      merchant_name: input.merchantName,
      username: input.username,
      password: input.password,
      force_login: true,
    }),
  });
}

export function getSDSLoginAuthState() {
  return request<SDSLoginAuthState>("/auth-state");
}

export function clearSDSLoginState() {
  return request("/state", {
    method: "DELETE",
  });
}

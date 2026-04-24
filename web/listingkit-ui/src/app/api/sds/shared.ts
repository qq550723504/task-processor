import { readFile } from "node:fs/promises";
import path from "node:path";

const SDS_API_BASE = "https://mapi.sdspod.com";

type AuthState = {
  accessToken?: string;
};

function resolveRepoRoot() {
  return path.resolve(process.cwd(), "..", "..");
}

async function loadAuthState() {
  const authFile = path.join(resolveRepoRoot(), "data", "sds", "auth_state.json");
  const raw = await readFile(authFile, "utf8");
  return JSON.parse(raw) as AuthState;
}

export async function createSDSHeaders() {
  const auth = await loadAuthState();
  if (!auth.accessToken) {
    throw new Error("SDS access token is missing");
  }

  return new Headers({
    Accept: "application/json",
    "access-token": auth.accessToken,
  });
}

export function buildSDSURL(pathname: string, query?: URLSearchParams) {
  const normalized = pathname.startsWith("/") ? pathname : `/${pathname}`;
  const suffix = query && query.toString() ? `?${query.toString()}` : "";
  return `${SDS_API_BASE}${normalized}${suffix}`;
}

export async function fetchSDSJSON<T>(pathname: string, query?: URLSearchParams) {
  const response = await fetch(buildSDSURL(pathname, query), {
    method: "GET",
    headers: await createSDSHeaders(),
    cache: "no-store",
  });

  const text = await response.text();
  const payload = text ? (JSON.parse(text) as unknown) : undefined;

  if (!response.ok) {
    throw new Error(`SDS request failed: ${response.status}`);
  }

  return payload as T;
}

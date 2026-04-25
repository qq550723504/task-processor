const DEFAULT_SERVICE_API_BASE = "http://localhost:8085/api/v1";

function buildSDSAPIBase() {
  if (process.env.SDS_API_BASE) {
    return process.env.SDS_API_BASE;
  }
  const serviceBase = process.env.LISTINGKIT_SERVICE_API_BASE ?? DEFAULT_SERVICE_API_BASE;
  return `${serviceBase.replace(/\/+$/, "")}/sds`;
}

export function buildSDSURL(pathname: string, query?: URLSearchParams) {
  const normalizedBase = buildSDSAPIBase().replace(/\/+$/, "");
  const normalizedPath = pathname.startsWith("/") ? pathname : `/${pathname}`;
  const suffix = query && query.toString() ? `?${query.toString()}` : "";
  return `${normalizedBase}${normalizedPath}${suffix}`;
}

export async function fetchSDSJSON<T>(pathname: string, query?: URLSearchParams) {
  const response = await fetch(buildSDSURL(pathname, query), {
    method: "GET",
    headers: { Accept: "application/json" },
    cache: "no-store",
  });

  const text = await response.text();
  const payload = text ? (JSON.parse(text) as unknown) : undefined;

  if (!response.ok) {
    const message =
      payload && typeof payload === "object" && "message" in payload
        ? String((payload as { message?: unknown }).message)
        : `SDS request failed: ${response.status}`;
    throw new Error(message);
  }

  return payload as T;
}

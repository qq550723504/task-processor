const DEFAULT_SERVICE_API_BASE = "http://localhost:8085/api/v1";

function buildSDSLoginAPIBase() {
  if (process.env.SDS_LOGIN_API_BASE) {
    return process.env.SDS_LOGIN_API_BASE;
  }
  const serviceBase = process.env.LISTINGKIT_SERVICE_API_BASE ?? DEFAULT_SERVICE_API_BASE;
  return `${serviceBase.replace(/\/+$/, "")}/sds-login`;
}

export function buildSDSLoginURL(pathname: string) {
  const normalizedBase = buildSDSLoginAPIBase().replace(/\/+$/, "");
  const normalizedPath = pathname.startsWith("/") ? pathname : `/${pathname}`;
  return `${normalizedBase}${normalizedPath}`;
}

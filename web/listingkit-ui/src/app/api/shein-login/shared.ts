const DEFAULT_SERVICE_API_BASE = "http://localhost:8085/api/v1";

function buildSheinLoginAPIBase() {
  if (process.env.SHEIN_LOGIN_API_BASE) {
    return process.env.SHEIN_LOGIN_API_BASE;
  }
  const serviceBase = process.env.LISTINGKIT_SERVICE_API_BASE ?? DEFAULT_SERVICE_API_BASE;
  return `${serviceBase.replace(/\/+$/, "")}/shein-login`;
}

export function buildSheinLoginURL(pathname: string) {
  const normalizedBase = buildSheinLoginAPIBase().replace(/\/+$/, "");
  const normalizedPath = pathname.startsWith("/") ? pathname : `/${pathname}`;
  return `${normalizedBase}${normalizedPath}`;
}

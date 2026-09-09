import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
import { hasTrustedSameOriginWrite } from "./same-origin-write";

function exactOrigin(raw: string | undefined): string | null {
  if (!raw) return null;
  try {
    const url = new URL(raw);
    return ["https:", "http:"].includes(url.protocol) && !url.username && !url.password &&
      (raw === url.origin || raw === `${url.origin}/`) ? url.origin : null;
  } catch { return null; }
}

export function configuredProductReviewOrigin(): string | null {
  return exactOrigin(process.env.PRODUCT_REVIEW_API_ORIGIN);
}

export function hasTrustedReviewWriteOrigin(request: Request): boolean {
  return hasTrustedSameOriginWrite(request);
}

export function reviewSelectedOrganization(request: Request): string | null {
  const values = (request.headers.get("cookie") ?? "").split(";").map((part) => part.trim())
    .filter((part) => part.startsWith(`${WORKBENCH_COOKIE_NAME}=`));
  if (values.length !== 1) return null;
  try {
    const value = decodeURIComponent(values[0].slice(WORKBENCH_COOKIE_NAME.length + 1));
    return /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(value) ? value : null;
  } catch { return null; }
}

import { resolvePublicAppOrigin } from "./zitadel-auth";

function exactOrigin(raw: string | undefined): string | null {
  if (!raw) return null;
  try {
    const url = new URL(raw);
    return ["https:", "http:"].includes(url.protocol) &&
      !url.username &&
      !url.password &&
      (raw === url.origin || raw === `${url.origin}/`)
      ? url.origin
      : null;
  } catch {
    return null;
  }
}

export function hasTrustedSameOriginWrite(request: Request): boolean {
  const origin = trustedPublicOrigin();
  const site = request.headers.get("sec-fetch-site");
  return Boolean(
    origin &&
      request.headers.get("origin") === origin &&
      (site === null || site === "same-origin"),
  );
}

export function hasTrustedSameOriginRecovery(request: Request): boolean {
  const origin = trustedPublicOrigin();
  const requestOrigin = request.headers.get("origin");
  const site = request.headers.get("sec-fetch-site");
  return Boolean(
    origin &&
      ((requestOrigin === origin && (site === null || site === "same-origin")) ||
        (requestOrigin === null && site === "same-origin")),
  );
}

function trustedPublicOrigin(): string | null {
  const configured =
    process.env.LISTINGKIT_PUBLIC_BASE_URL?.trim() ||
    process.env.TASK_PROCESSOR_LISTINGKIT_PUBLIC_BASE_URL?.trim() ||
    process.env.NEXT_PUBLIC_APP_URL?.trim() ||
    process.env.APP_URL?.trim();
  return configured ? exactOrigin(resolvePublicAppOrigin()) : null;
}

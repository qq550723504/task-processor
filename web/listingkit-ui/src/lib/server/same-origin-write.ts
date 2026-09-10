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
  const configured =
    process.env.LISTINGKIT_PUBLIC_BASE_URL?.trim() ||
    process.env.TASK_PROCESSOR_LISTINGKIT_PUBLIC_BASE_URL?.trim() ||
    process.env.NEXT_PUBLIC_APP_URL?.trim() ||
    process.env.APP_URL?.trim();
  const origin = configured && exactOrigin(resolvePublicAppOrigin());
  const site = request.headers.get("sec-fetch-site");
  return Boolean(
    origin &&
      request.headers.get("origin") === origin &&
      (site === null || site === "same-origin"),
  );
}

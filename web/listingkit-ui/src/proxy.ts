import { NextRequest, NextResponse } from "next/server";

const SESSION_COOKIE = "listingkit_zitadel_session";

type ZitadelSessionCookie = {
  accessToken?: unknown;
  expiresAt?: unknown;
};

export function proxy(request: NextRequest) {
  if (!isListingKitPagePath(request.nextUrl.pathname)) {
    return NextResponse.next();
  }

  if (shouldBypassListingKitAuth()) {
    return NextResponse.next();
  }

  if (!isZitadelConfigured()) {
    return NextResponse.json(
      { error: "ZITADEL auth is not configured" },
      { status: 503 },
    );
  }

  if (hasValidZitadelSession(request)) {
    return NextResponse.next();
  }

  return redirectToZitadelLogin(request);
}

function isListingKitPagePath(pathname: string) {
  return pathname === "/" || pathname === "/listing-kits" || pathname.startsWith("/listing-kits/");
}

function shouldBypassListingKitAuth() {
  return (
    process.env.NODE_ENV !== "production" &&
    process.env.LISTINGKIT_UI_BYPASS_AUTH_GATE === "1"
  );
}

function isZitadelConfigured() {
  return Boolean(
    process.env.ZITADEL_ISSUER_URL?.trim() && process.env.ZITADEL_CLIENT_ID?.trim(),
  );
}

function hasValidZitadelSession(request: NextRequest) {
  const raw = request.cookies.get(SESSION_COOKIE)?.value;
  if (!raw) {
    return false;
  }

  try {
    const session = JSON.parse(decodeBase64Url(raw)) as ZitadelSessionCookie;
    if (typeof session.accessToken !== "string" || session.accessToken.length === 0) {
      return false;
    }
    if (
      typeof session.expiresAt === "number" &&
      session.expiresAt <= Math.floor(Date.now() / 1000)
    ) {
      return false;
    }
    return true;
  } catch {
    return false;
  }
}

function redirectToZitadelLogin(request: NextRequest) {
  const loginUrl = request.nextUrl.clone();
  loginUrl.pathname = "/api/zitadel-auth/login";
  loginUrl.search = "";
  loginUrl.searchParams.set("returnTo", buildReturnTo(request));
  return NextResponse.redirect(loginUrl);
}

function buildReturnTo(request: NextRequest) {
  const returnTo = `${request.nextUrl.pathname}${request.nextUrl.search}`;
  if (!returnTo.startsWith("/") || returnTo.startsWith("//")) {
    return "/";
  }
  return returnTo;
}

function decodeBase64Url(value: string) {
  const base64 = value.replace(/-/g, "+").replace(/_/g, "/");
  const padded = `${base64}${"=".repeat((4 - (base64.length % 4)) % 4)}`;
  return globalThis.atob(padded);
}

export const config = {
  matcher: ["/", "/listing-kits/:path*"],
};

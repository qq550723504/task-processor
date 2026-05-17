import { NextRequest, NextResponse } from "next/server";
import type { Session } from "next-auth";

import { auth } from "@/auth";
import {
  authorizeZitadelIdentity,
  isZitadelAuthConfigured,
  readZitadelAccessTokenFromSession,
  readZitadelIdentityFromSession,
  readZitadelSessionError,
} from "@/lib/server/zitadel-auth";

type AuthenticatedProxyRequest = NextRequest & {
  auth?: unknown;
};

const authenticatedProxy = auth(async (request) =>
  handleProxy(request as AuthenticatedProxyRequest),
);

export { authenticatedProxy as proxy };
export default authenticatedProxy;

async function handleProxy(request: AuthenticatedProxyRequest) {
  if (!isListingKitPagePath(request.nextUrl.pathname)) {
    return NextResponse.next();
  }

  if (shouldBypassListingKitAuth()) {
    return NextResponse.next();
  }

  if (!isZitadelAuthConfigured()) {
    return NextResponse.json(
      { error: "ZITADEL auth is not configured" },
      { status: 503 },
    );
  }

  const session = (request.auth ?? null) as Session | null;
  const accessToken = readZitadelAccessTokenFromSession(session);
  const sessionError = readZitadelSessionError(session);
  if (!accessToken || sessionError) {
    return redirectToZitadelLogin(request);
  }

  const identity = readZitadelIdentityFromSession(session);
  if (!identity) {
    return redirectToZitadelLogin(request);
  }

  const authorization = authorizeZitadelIdentity(identity);
  if (!authorization.authorized) {
    return NextResponse.redirect(new URL("/unauthorized", request.nextUrl));
  }

  return NextResponse.next();
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

function redirectToZitadelLogin(request: NextRequest) {
  const loginUrl = request.nextUrl.clone();
  loginUrl.pathname = "/login";
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

export const config = {
  matcher: ["/", "/listing-kits/:path*"],
};

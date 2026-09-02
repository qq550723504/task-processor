import { NextRequest, NextResponse } from "next/server";
import type { Session } from "next-auth";

import { serverAuth } from "@/auth";
import {
  authorizeZitadelIdentity,
  isZitadelAuthConfigured,
  normalizeReturnTo,
  readZitadelIdentityFromSession,
  readZitadelSessionError,
} from "@/lib/server/zitadel-auth";
import { hasPlatformAdminRole } from "@/lib/listingkit-permissions";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";

type AuthenticatedProxyRequest = NextRequest & {
  auth?: unknown;
};

const authenticatedProxy = serverAuth(async (request) =>
  handleProxy(request as AuthenticatedProxyRequest),
);

export { authenticatedProxy as proxy };
export default authenticatedProxy;

async function handleProxy(request: AuthenticatedProxyRequest) {
  if (!isProtectedPagePath(request.nextUrl.pathname)) {
    return NextResponse.next();
  }

  if (!isZitadelAuthConfigured()) {
    return NextResponse.json(
      { error: "ZITADEL auth is not configured" },
      { status: 503 },
    );
  }

  const session = (request.auth ?? null) as Session | null;
  const accessToken = readZitadelServerAccessToken(session);
  const sessionError = readZitadelSessionError(session);
  if (!accessToken || sessionError) {
    return redirectToZitadelLogin(request);
  }

  const identity = readZitadelIdentityFromSession(session);
  if (!identity) {
    return redirectToZitadelLogin(request);
  }

  if (isWorkbenchPagePath(request.nextUrl.pathname)) {
    return NextResponse.next();
  }

  const authorization = authorizeZitadelIdentity(identity);
  if (!authorization.authorized) {
    return NextResponse.redirect(new URL("/unauthorized", request.nextUrl));
  }

  if (
    isSDSLoginPagePath(request.nextUrl.pathname) &&
    !hasPlatformAdminRole(identity.roles)
  ) {
    return NextResponse.redirect(new URL("/unauthorized", request.nextUrl));
  }

  return NextResponse.next();
}

function isSDSLoginPagePath(pathname: string) {
  return pathname === "/listing-kits/sds-login";
}

function isListingKitPagePath(pathname: string) {
  return pathname === "/listing-kits" || pathname.startsWith("/listing-kits/");
}

function isWorkbenchPagePath(pathname: string) {
  return pathname === "/workbench" || pathname.startsWith("/workbench/");
}

function isProtectedPagePath(pathname: string) {
  return isListingKitPagePath(pathname) || isWorkbenchPagePath(pathname);
}

function redirectToZitadelLogin(request: NextRequest) {
  const loginUrl = request.nextUrl.clone();
  loginUrl.pathname = "/login";
  loginUrl.search = "";
  loginUrl.searchParams.set("returnTo", buildReturnTo(request));
  return NextResponse.redirect(loginUrl);
}

function buildReturnTo(request: NextRequest) {
  return normalizeReturnTo(
    `${request.nextUrl.pathname}${request.nextUrl.search}`,
  );
}

export const config = {
  matcher: ["/listing-kits/:path*", "/workbench/:path*"],
};

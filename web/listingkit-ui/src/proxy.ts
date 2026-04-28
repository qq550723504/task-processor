import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";

const AUTH_HEADER = "Basic realm=\"ListingKit Workbench\"";

export function proxy(request: NextRequest) {
  if (isPublicPath(request.nextUrl.pathname)) {
    return NextResponse.next();
  }

  const username = process.env.LISTINGKIT_BASIC_AUTH_USERNAME;
  const password = process.env.LISTINGKIT_BASIC_AUTH_PASSWORD;
  if (!username || !password) {
    return NextResponse.next();
  }

  if (isAuthorized(request.headers.get("authorization"), username, password)) {
    return NextResponse.next();
  }

  return new NextResponse("Authentication required", {
    status: 401,
    headers: {
      "WWW-Authenticate": AUTH_HEADER,
      "Cache-Control": "no-store",
    },
  });
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico|robots.txt|file.svg|globe.svg|next.svg|vercel.svg|window.svg).*)"],
};

function isPublicPath(pathname: string) {
  return pathname === "/healthz";
}

function isAuthorized(header: string | null, username: string, password: string) {
  if (!header?.startsWith("Basic ")) {
    return false;
  }

  const decoded = decodeBasicCredentials(header.slice("Basic ".length));
  if (!decoded) {
    return false;
  }

  const separator = decoded.indexOf(":");
  if (separator < 0) {
    return false;
  }

  const suppliedUsername = decoded.slice(0, separator);
  const suppliedPassword = decoded.slice(separator + 1);
  return (
    timingSafeEqual(suppliedUsername, username) &&
    timingSafeEqual(suppliedPassword, password)
  );
}

function decodeBasicCredentials(encoded: string) {
  try {
    return atob(encoded);
  } catch {
    return "";
  }
}

function timingSafeEqual(left: string, right: string) {
  const encoder = new TextEncoder();
  const leftBytes = encoder.encode(left);
  const rightBytes = encoder.encode(right);
  const length = Math.max(leftBytes.length, rightBytes.length);
  let diff = leftBytes.length ^ rightBytes.length;

  for (let index = 0; index < length; index += 1) {
    diff |= (leftBytes[index] ?? 0) ^ (rightBytes[index] ?? 0);
  }

  return diff === 0;
}

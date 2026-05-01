import { NextResponse } from "next/server";
export function proxy() {
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico|robots.txt|file.svg|globe.svg|next.svg|vercel.svg|window.svg).*)"],
};

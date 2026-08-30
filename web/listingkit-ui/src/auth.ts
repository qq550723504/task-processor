import NextAuth from "next-auth";

import { buildAuthConfig, buildServerAuthConfig } from "@/auth.config";

const publicAuth = NextAuth(buildAuthConfig());
const serverOnlyAuth = NextAuth(buildServerAuthConfig());

export const { handlers, signIn, signOut } = publicAuth;
export const { auth: serverAuth } = serverOnlyAuth;

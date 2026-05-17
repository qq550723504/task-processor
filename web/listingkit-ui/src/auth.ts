import NextAuth from "next-auth";

import { buildAuthConfig } from "@/auth.config";

export const { handlers, auth, signIn, signOut } = NextAuth(buildAuthConfig());

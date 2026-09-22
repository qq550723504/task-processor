import { redirect } from "next/navigation";

import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession, readZitadelSessionError } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";

const AUTH_DEADLINE_MS = 15_000;

export async function requireReferralUserId(returnTo: string) {
  const session = await boundedServerAuth();
  const identity = readZitadelIdentityFromSession(session);
  if (!identity || !readZitadelServerAccessToken(session) || readZitadelSessionError(session)) {
    redirect(`/login?returnTo=${encodeURIComponent(returnTo)}`);
  }
  return String(identity.userId);
}

async function boundedServerAuth() {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const deadline = new Promise<null>((resolve) => { timer = setTimeout(() => resolve(null), AUTH_DEADLINE_MS); });
  try {
    return await Promise.race([serverAuth(), deadline]);
  } catch {
    return null;
  } finally {
    if (timer) clearTimeout(timer);
  }
}

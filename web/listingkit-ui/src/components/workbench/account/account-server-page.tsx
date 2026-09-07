import { redirect } from "next/navigation";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession, readZitadelSessionError } from "@/lib/server/zitadel-auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";
import { AccountPage } from "./account-page";

export async function AccountServerPage({ page }: { page: "profile" | "organization" }) {
  const session = await serverAuth();
  const identity = readZitadelIdentityFromSession(session);
  if (!identity || !readZitadelServerAccessToken(session) || readZitadelSessionError(session)) redirect(`/login?returnTo=${encodeURIComponent(`/workbench/account/${page}`)}`);
  return <AccountPage page={page} expectedUserId={String(identity.userId)} />;
}

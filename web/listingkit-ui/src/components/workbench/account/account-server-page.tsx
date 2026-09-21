import { redirect } from "next/navigation";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession, readZitadelSessionError } from "@/lib/server/zitadel-auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";
import { AccountPage, accountPagePath, type AccountPageKind } from "./account-page";

export async function AccountServerPage({ page }: { page: AccountPageKind }) {
  const session = await serverAuth();
  const identity = readZitadelIdentityFromSession(session);
  if (!identity || !readZitadelServerAccessToken(session) || readZitadelSessionError(session)) redirect(`/login?returnTo=${encodeURIComponent(accountPagePath(page))}`);
  return <AccountPage page={page} expectedUserId={String(identity.userId)} />;
}

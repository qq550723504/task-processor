import { redirect } from "next/navigation";

import { serverAuth } from "@/auth";
import { ReferralsPage } from "@/components/workbench/referrals/referrals-page";
import { readZitadelIdentityFromSession, readZitadelSessionError } from "@/lib/server/zitadel-auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";

export default async function AccountReferralCompletionPage() {
  const session = await serverAuth();
  const identity = readZitadelIdentityFromSession(session);
  if (!identity || !readZitadelServerAccessToken(session) || readZitadelSessionError(session)) redirect("/login?returnTo=%2Fworkbench%2Faccount%2Freferrals%2Fcomplete");
  return <ReferralsPage mode="complete" expectedUserId={String(identity.userId)} />;
}

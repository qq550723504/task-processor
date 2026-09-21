import { redirect } from "next/navigation";
import { serverAuth } from "@/auth";
import { ReferralsPage } from "@/components/workbench/referrals/referrals-page";
import { readZitadelIdentityFromSession, readZitadelSessionError } from "@/lib/server/zitadel-auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";

export default async function ReferralRulesPage() {
  const session = await serverAuth();
  const identity = readZitadelIdentityFromSession(session);
  if (!identity || !readZitadelServerAccessToken(session) || readZitadelSessionError(session)) redirect("/login?returnTo=%2Fworkbench%2Faccount%2Freferrals%2Frules");
  return <ReferralsPage mode="overview" view="rules" expectedUserId={String(identity.userId)} />;
}

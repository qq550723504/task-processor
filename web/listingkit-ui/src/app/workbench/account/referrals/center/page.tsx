import { redirect } from "next/navigation";
import { serverAuth } from "@/auth";
import { ReferralsPage } from "@/components/workbench/referrals/referrals-page";
import { readZitadelIdentityFromSession, readZitadelSessionError } from "@/lib/server/zitadel-auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";
import { isReferralRegistrationAvailable } from "@/lib/server/referral-registration-route";

export default async function ReferralCenterPage() {
  const session = await serverAuth();
  const identity = readZitadelIdentityFromSession(session);
  if (!identity || !readZitadelServerAccessToken(session) || readZitadelSessionError(session)) redirect("/login?returnTo=%2Fworkbench%2Faccount%2Freferrals%2Fcenter");
  return <ReferralsPage mode="overview" view="center" expectedUserId={String(identity.userId)} registrationAvailable={await isReferralRegistrationAvailable()} />;
}

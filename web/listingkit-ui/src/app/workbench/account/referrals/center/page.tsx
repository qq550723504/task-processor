import { ReferralsPage } from "@/components/workbench/referrals/referrals-page";
import { isReferralRegistrationAvailable } from "@/lib/server/referral-registration-route";
import { requireReferralUserId } from "@/lib/server/referral-page-auth";

export default async function ReferralCenterPage() {
  const expectedUserId = await requireReferralUserId("/workbench/account/referrals/center");
  return <ReferralsPage mode="overview" view="center" expectedUserId={expectedUserId} registrationAvailable={await isReferralRegistrationAvailable()} />;
}

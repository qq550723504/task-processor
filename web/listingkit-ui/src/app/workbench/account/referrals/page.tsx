import { ReferralsPage } from "@/components/workbench/referrals/referrals-page";
import { isReferralRegistrationAvailable } from "@/lib/server/referral-registration-route";
import { requireReferralUserId } from "@/lib/server/referral-page-auth";

export default async function AccountReferralsPage() {
  const expectedUserId = await requireReferralUserId("/workbench/account/referrals");
  return <ReferralsPage mode="overview" expectedUserId={expectedUserId} registrationAvailable={await isReferralRegistrationAvailable()} />;
}

import { ReferralsPage } from "@/components/workbench/referrals/referrals-page";
import { requireReferralUserId } from "@/lib/server/referral-page-auth";

export default async function ReferralEarningsPage() {
  const expectedUserId = await requireReferralUserId("/workbench/account/referrals/earnings");
  return <ReferralsPage mode="overview" view="earnings" expectedUserId={expectedUserId} />;
}

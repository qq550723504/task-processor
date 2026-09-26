import { ReferralsPage } from "@/components/workbench/referrals/referrals-page";
import { requireReferralUserId } from "@/lib/server/referral-page-auth";

export default async function ReferralWithdrawalsPage() {
  const expectedUserId = await requireReferralUserId("/workbench/account/referrals/withdrawals");
  return <ReferralsPage mode="overview" view="withdrawals" expectedUserId={expectedUserId} />;
}

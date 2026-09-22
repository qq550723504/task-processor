import { ReferralsPage } from "@/components/workbench/referrals/referrals-page";
import { requireReferralUserId } from "@/lib/server/referral-page-auth";

export default async function ReferralRulesPage() {
  const expectedUserId = await requireReferralUserId("/workbench/account/referrals/rules");
  return <ReferralsPage mode="overview" view="rules" expectedUserId={expectedUserId} />;
}

import { handleAccountReferralEconomics, rejectAccountReferralEconomicsMethod } from "@/lib/server/account-referral-economics-route";

export const dynamic = "force-dynamic";
export const GET = handleAccountReferralEconomics;
export const POST = rejectAccountReferralEconomicsMethod;
export const PUT = rejectAccountReferralEconomicsMethod;
export const PATCH = rejectAccountReferralEconomicsMethod;
export const DELETE = rejectAccountReferralEconomicsMethod;

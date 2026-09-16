import { handleAccountReferrals, rejectAccountReferralMethod } from "@/lib/server/account-referrals-route";

export const dynamic = "force-dynamic";
export const POST = handleAccountReferrals;
export const GET = rejectAccountReferralMethod;
export const PUT = rejectAccountReferralMethod;
export const PATCH = rejectAccountReferralMethod;
export const DELETE = rejectAccountReferralMethod;
export const HEAD = rejectAccountReferralMethod;
export const OPTIONS = rejectAccountReferralMethod;

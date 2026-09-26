import { handleReferralRegistrationPOST, referralMethodNotAllowed } from "@/lib/server/referral-registration-route";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";
export const POST = (request: Parameters<typeof handleReferralRegistrationPOST>[0]) => handleReferralRegistrationPOST(request, "start");
export const GET = referralMethodNotAllowed;
export const PUT = referralMethodNotAllowed;
export const PATCH = referralMethodNotAllowed;
export const DELETE = referralMethodNotAllowed;
export const HEAD = referralMethodNotAllowed;
export const OPTIONS = referralMethodNotAllowed;

import { z } from "zod";
import { AccountReadError } from "./account";
import { readBoundedStrictJSON } from "./strict-json-response";

function safeVerificationURL(value: string): boolean {
  try { const u = new URL(value); return value.length <= 8192 && u.protocol === "https:" && !u.username && !u.password && !u.port && !u.hash && ["qian.tencent.cn", "qian.tencent.com", "ess.tencent.cn", "essurl.cn"].some(h => u.hostname === h || u.hostname.endsWith(`.${h}`)); } catch { return false; }
}
const id = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const schema = z.object({state:z.enum(["NOT_STARTED","PENDING","VERIFIED","OUTCOME_UNKNOWN","EXPIRED"]),userId:id,organizationId:id,canStart:z.boolean(),maskedPhone:z.string().regex(/^(?:1[3-9][0-9]\*{4}[0-9]{4})?$/),applicationId:id.optional(),companyName:z.string().max(256).optional(),creditCode:z.string().regex(/^[A-Z0-9]{18}$/).optional(),verificationUrl:z.string().refine(safeVerificationURL).optional(),expiresAt:z.string().datetime().optional(),verifiedAt:z.string().datetime().optional(),isApplicant:z.boolean().optional()}).strict().refine(v =>
  (v.state === "NOT_STARTED" ? !v.applicationId && !v.verificationUrl : !!v.applicationId && !!v.companyName && !!v.creditCode && !v.canStart) &&
  (!v.verificationUrl || v.state === "PENDING" && v.isApplicant === true && !!v.expiresAt) && (v.state !== "VERIFIED" || !!v.verifiedAt));
export type VerificationState = z.infer<typeof schema>;
export const verificationInputSchema = z.object({companyName:z.string().trim().min(1).max(256),creditCode:z.string().regex(/^[0-9A-Z]{18}$/),legalName:z.string().max(128).optional(),consent:z.literal(true),idempotencyKey:z.string().uuid()}).strict();
export type VerificationInput = z.infer<typeof verificationInputSchema>;
export function parseVerification(value: unknown, user: string, org: string): VerificationState {
  const result=schema.safeParse(value);if(!result.success) throw new AccountReadError(502,"INVALID_UPSTREAM_RESPONSE");
  if(result.data.userId!==user) throw new AccountReadError(409,"IDENTITY_CONTEXT_CHANGED");
  if(result.data.organizationId!==org) throw new AccountReadError(409,"ORGANIZATION_CONTEXT_CHANGED");return result.data;
}
const allowedCodes = new Set(["AUTHENTICATION_REQUIRED","PERMISSION_DENIED","ORGANIZATION_ACCESS_DENIED","ORGANIZATION_ACCESS_REVOKED","ORGANIZATION_SUSPENDED","IDENTITY_CONTEXT_CHANGED","ORGANIZATION_CONTEXT_CHANGED","ORGANIZATION_SELECTION_REQUIRED","VERIFIED_PHONE_REQUIRED","VERIFICATION_CONFLICT","VERIFICATION_UNAVAILABLE","RESULT_UNVERIFIED","INVALID_REQUEST"]);
export function verificationErrorCode(payload: unknown) { const code = payload && typeof payload === "object" && "code" in payload ? payload.code : null; return typeof code === "string" && allowedCodes.has(code) ? code : "VERIFICATION_UNAVAILABLE"; }
export async function verificationRequest(user: string, org: string, input?: VerificationInput, signal?: AbortSignal): Promise<VerificationState> {
  const combined=AbortSignal.any([AbortSignal.timeout(16000),...(signal?[signal]:[])]);
  try {
    const response=await fetch(`/api/account/verification${input?"/applications":""}`,{method:input?"POST":"GET",headers:{"X-Expected-User-ID":user,"X-Expected-Organization-ID":org,...(input?{"Content-Type":"application/json"}:{})},...(input?{body:JSON.stringify(verificationInputSchema.parse(input))}:{}),cache:"no-store",redirect:"error",signal:combined});
    const payload=await readBoundedStrictJSON(response,16384,combined);
    if(!response.ok) throw new AccountReadError(response.status,verificationErrorCode(payload));
    return parseVerification(payload,user,org);
  } catch(error) {if(error instanceof AccountReadError) throw error;throw new AccountReadError(502,input?"RESULT_UNVERIFIED":"VERIFICATION_UNAVAILABLE");}
}

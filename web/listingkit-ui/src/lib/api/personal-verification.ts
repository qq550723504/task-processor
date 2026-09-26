import { z } from "zod";
import { AccountReadError } from "./account";
import { readBoundedStrictJSON } from "./strict-json-response";
import { verificationErrorCode } from "./subject-verification";

function safePersonalURL(value:string):boolean {try{const u=new URL(value);return value.length<=8192&&u.protocol==="https:"&&!!u.hostname&&!u.username&&!u.password&&!u.port&&!u.hash;}catch{return false;}}
const quota=z.object({totalLimit:z.number().int().min(1).max(100),totalUsed:z.number().int().nonnegative(),totalRemaining:z.number().int().nonnegative(),dailyLimit:z.number().int().min(1).max(100),dailyUsed:z.number().int().nonnegative(),dailyRemaining:z.number().int().nonnegative(),serverTime:z.string().datetime(),resetAt:z.string().datetime(),nextAllowedAt:z.string().datetime()}).strict().refine(q=>q.totalRemaining===Math.max(0,q.totalLimit-q.totalUsed)&&q.dailyRemaining===Math.max(0,q.dailyLimit-q.dailyUsed)&&q.dailyUsed<=q.totalUsed&&q.dailyLimit<=q.totalLimit);
const schema=z.object({userId:z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/),state:z.enum(["NOT_STARTED","PENDING","OUTCOME_UNKNOWN","REJECTED","EXPIRED","VERIFIED"]),applicationId:z.string().uuid().optional(),maskedPhone:z.string().regex(/^(?:1[3-9][0-9]\*{4}[0-9]{4})?$/),canStart:z.boolean(),canRefresh:z.boolean(),phoneReady:z.boolean(),verificationUrl:z.string().refine(safePersonalURL).optional(),expiresAt:z.string().datetime().optional(),verifiedAt:z.string().datetime().optional(),quota}).strict().refine(v=>
 (v.state==="NOT_STARTED"?!v.applicationId:!!v.applicationId)&&
 (!v.canStart||v.phoneReady&&v.quota.totalRemaining>0&&v.quota.dailyRemaining>0&&["NOT_STARTED","REJECTED","EXPIRED"].includes(v.state))&&
 (!v.verificationUrl||v.state==="PENDING"&&v.canRefresh&&!!v.expiresAt)&&
 (!v.canRefresh||["PENDING","EXPIRED"].includes(v.state))&&
 (v.state!=="VERIFIED"||!!v.verifiedAt&&!v.canStart&&!v.canRefresh));
export type PersonalVerificationState=z.infer<typeof schema>;
export const personalInputSchema=z.object({name:z.string().trim().min(1).max(128),idNumber:z.string().regex(/^[0-9]{17}[0-9X]$/),metaInfo:z.string().min(2).max(8192),consent:z.literal(true),idempotencyKey:z.string().uuid()}).strict();
export const personalRefreshSchema=z.object({applicationId:z.string().uuid()}).strict();
export type PersonalInput=z.infer<typeof personalInputSchema>;
export function parsePersonalVerification(value:unknown,user:string):PersonalVerificationState {const r=schema.safeParse(value);if(!r.success)throw new AccountReadError(502,"INVALID_UPSTREAM_RESPONSE");if(r.data.userId!==user)throw new AccountReadError(409,"IDENTITY_CONTEXT_CHANGED");return r.data;}
const limitCodes=new Set(["VERIFICATION_TOTAL_LIMIT","VERIFICATION_DAILY_LIMIT","VERIFICATION_COOLDOWN","VERIFICATION_REFRESH_BUSY"]);
export function personalErrorCode(value:unknown):string{const code=value&&typeof value==="object"&&"code" in value?value.code:undefined;return typeof code==="string"&&limitCodes.has(code)?code:verificationErrorCode(value);}
export async function personalVerificationRequest(user:string,action:"read"|"start"|"refresh"="read",input?:PersonalInput|{applicationId:string},signal?:AbortSignal):Promise<PersonalVerificationState>{
 const write=action!=="read",combined=AbortSignal.any([AbortSignal.timeout(23000),...(signal?[signal]:[])]);
 try{
 const body=write?JSON.stringify((action==="start"?personalInputSchema:personalRefreshSchema).parse(input)):undefined;
 const r=await fetch(`/api/account/personal-verification${action==="start"?"/applications":action==="refresh"?"/refresh":""}`,{method:write?"POST":"GET",headers:{"X-Expected-User-ID":user,...(write?{"Content-Type":"application/json"}:{})},...(body?{body}:{}),cache:"no-store",redirect:"error",signal:combined});
 const payload=await readBoundedStrictJSON(r,16384,combined);if(!r.ok)throw new AccountReadError(r.status,personalErrorCode(payload));return parsePersonalVerification(payload,user);
 }catch(e){if(e instanceof AccountReadError)throw e;throw new AccountReadError(502,action==="start"?"RESULT_UNVERIFIED":"VERIFICATION_UNAVAILABLE");}
}

import { z } from "zod";
import { readBoundedStrictJSON } from "./strict-json-response";

export const ecoId=z.string().uuid().refine(v=>v===v.toLowerCase()&&v!=="00000000-0000-0000-0000-000000000000");
const ecoMinor=z.string().regex(/^(0|[1-9][0-9]{0,18})$/).pipe(z.string().refine(v=>BigInt(v)<=BigInt("9223372036854775807")));
const ecoVersion=ecoMinor.pipe(z.string().refine(v=>BigInt(v)>BigInt(0)));
export const ecoFinancialSchema=z.object({grossMinor:ecoMinor,refundedMinor:ecoMinor,chargedBackMinor:ecoMinor,platformMinor:ecoMinor,providerMinor:ecoMinor,sharedMinor:ecoMinor,returnedMinor:ecoMinor,releasedMinor:ecoMinor,channelFeeMinor:ecoMinor,channelFeeObserved:z.boolean(),reconciliationReason:z.string().max(128)}).strict();
const ecoCategory=z.enum(["COMPANY_REGISTRATION","TRADEMARK_REGISTRATION","STORE_OPENING","STORE_OPERATION"]);
const timestamp=z.string().datetime({offset:true});
export const ecoPolicy="ecoservices-v1-10-platform-fee-manual-expiry";
const ecoListingSchema=z.object({id:ecoId,providerName:z.string().max(256),category:ecoCategory,title:z.string().max(128),description:z.string().max(2000),items:z.array(z.string().max(512)).max(10),regions:z.array(z.string().max(128)).max(30),platforms:z.array(z.string().max(128)).max(20).nullable(),priceMinor:ecoVersion,deliveryDays:z.number().int().min(1).max(180),state:z.enum(["DRAFT","PUBLISHED"]),version:ecoVersion}).strict();
const ecoApplicationSchema=z.object({currentMerchantRevisionId:ecoId.optional(),currentMerchantRevisionVersion:ecoVersion.optional(),id:ecoId,companyName:z.string().max(256),registrationNumber:z.string().max(128),categories:z.array(ecoCategory).max(4),regions:z.array(z.string().max(128)).max(30),fileIds:z.array(ecoId).max(10),state:z.enum(["SUBMITTED","APPROVED","REJECTED","ACTIVE"]),version:ecoVersion,agreementVersion:z.string().max(128),agreementAccepted:z.boolean(),onboardingState:z.string().max(64),reviewReason:z.string().max(2000),updatedAt:timestamp}).strict();
const quoteSchema=z.object({commissionBps:z.number().int().min(0).max(10000),allocationBasis:z.literal("CUMULATIVE_NET_FLOOR_V1"),policyVersion:z.string().min(1).max(128),amountMinor:ecoVersion,scope:z.string().max(10000),acceptanceCriteria:z.string().max(5000),deliveryDays:z.number().int().min(1).max(180),version:ecoVersion}).strict();
const deliverySchema=z.object({version:ecoVersion,content:z.string().max(10000),fileIds:z.array(ecoId).max(10).nullable(),submittedAt:timestamp,rejection:z.object({deliveryVersion:ecoVersion,reason:z.string().max(2000),actorId:z.string().max(256),rejectedAt:timestamp}).strict().optional()}).strict();
const refundSchema=z.object({version:ecoVersion,amountMinor:ecoVersion,reason:z.string().max(2000),buyerConfirmed:z.boolean(),providerConfirmed:z.boolean(),state:z.enum(["NEGOTIATING","APPROVED","REJECTED","REFUNDED","FAILED"]),review:z.object({reason:z.string().min(1).max(2000),actorId:z.string().min(1).max(256),reviewedAt:timestamp}).strict().optional()}).strict();
const ecoRequestSchema=z.object({id:ecoId,listingId:ecoId,listingVersion:ecoVersion,title:z.string().max(128),category:ecoCategory,description:z.string().max(10000),fileIds:z.array(ecoId).max(10).nullable(),state:z.enum(["REQUESTED","QUOTED","ORDER_PENDING","PAID_READY","SERVICING","AWAITING_ACCEPTANCE","ACCEPTED","CANCEL_REQUESTED","CANCELLED"]),version:ecoVersion,quote:quoteSchema.optional(),delivery:deliverySchema.optional(),refund:refundSchema.optional(),orderId:ecoId.optional(),acceptanceId:ecoId.optional(),acceptedDeliveryVersion:ecoMinor,financialHold:z.boolean(),financialState:z.string().max(80),financialReason:z.string().max(256),fundsExpireAt:timestamp.optional(),createdAt:timestamp,updatedAt:timestamp,side:z.enum(["buyer","provider",""])}).strict();
export const ecoPageSchema=z.object({providerQualified:z.boolean().optional(),applications:z.array(ecoApplicationSchema).max(100).optional(),listings:z.array(ecoListingSchema).max(100).optional(),requests:z.array(ecoRequestSchema).max(100).optional(),total:ecoMinor,counts:z.record(z.string(),z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER)).optional()}).strict();
export const ecoResultSchema=z.object({application:ecoApplicationSchema.optional(),listing:ecoListingSchema.optional(),request:ecoRequestSchema.optional()}).strict().refine(v=>!!v.application||!!v.listing||!!v.request);
export const ecoFileSchema=z.object({id:ecoId,parentId:z.union([ecoId,z.literal("")]),parentKind:z.enum(["APPLICATION","REQUEST"]),filename:z.string().max(256),contentType:z.enum(["application/pdf","image/png","image/jpeg","text/plain; charset=utf-8"]),sizeBytes:ecoVersion,state:z.enum(["PENDING","CONFIRMED"])}).strict();
export const ecoCheckoutSchema=z.object({orderId:ecoId,codeUrl:z.string().max(8192).refine(v=>{try{const u=new URL(v);return u.protocol==="weixin:"&&u.host==="wxpay"&&(u.pathname==="/bizpayurl"||u.pathname==="/bizpayurl/up")&&!u.username&&!u.password&&!u.hash&&!!u.searchParams.get("pr")}catch{return false}})}).strict();
const merchantURL=z.string().max(2048).refine(v=>{if(!v)return true;try{const u=new URL(v);return u.protocol==="https:"&&u.host==="pay.weixin.qq.com"&&!u.username&&!u.password&&!u.hash&&u.pathname.startsWith("/public/")}catch{return false}});
export const ecoMerchantSchema=z.object({revisionVersion:ecoVersion,canCorrect:z.boolean(),verificationPending:z.boolean(),id:ecoId,applicationId:ecoId,state:z.enum(["PREPARING","CHECKING","ACCOUNT_NEED_VERIFY","AUDITING","REJECTED","NEED_SIGN","FINISH","FROZEN","CANCELED"]),signState:z.string().max(32),signUrl:merchantURL,legalValidationUrl:merchantURL,reason:z.string().max(21000),bank:z.object({accountName:z.string().max(256),accountNumber:z.string().max(128),payAmountMinor:ecoVersion,destinationNumber:z.string().max(128),destinationName:z.string().max(256),destinationBank:z.string().max(256),city:z.string().max(256),remark:z.string().max(512),deadline:z.string().max(128)}).strict().optional(),updatedAt:timestamp}).strict();
export const ecoDocumentTypes=["IDENTIFICATION_TYPE_MAINLAND_IDCARD","IDENTIFICATION_TYPE_OVERSEA_PASSPORT","IDENTIFICATION_TYPE_HONGKONG","IDENTIFICATION_TYPE_MACAO","IDENTIFICATION_TYPE_TAIWAN","IDENTIFICATION_TYPE_FOREIGN_RESIDENT","IDENTIFICATION_TYPE_HONGKONG_MACAO_RESIDENT","IDENTIFICATION_TYPE_TAIWAN_RESIDENT"] as const;
const ecoIdentitySchema=z.object({type:z.enum(ecoDocumentTypes),name:z.string().min(1).max(128),number:z.string().min(1).max(64),address:z.string().max(512),frontFileId:ecoId,backFileId:z.union([ecoId,z.literal("")]),validFrom:z.string().regex(/^\d{4}-\d{2}-\d{2}$/),validUntil:z.string().regex(/^(\d{4}-\d{2}-\d{2}|长期)$/)}).strict();
export const ecoMerchantInput=z.object({expectedRevisionVersion:ecoMinor,licenseFileId:ecoId,legal:ecoIdentitySchema,soleLegalBeneficiary:z.boolean(),beneficiaries:z.array(ecoIdentitySchema).max(4),contactMobile:z.string().min(1).max(32),accountBank:z.string().min(1).max(128),accountNumber:z.string().min(1).max(64),bankBranchName:z.string().max(128),merchantShortName:z.string().min(1).max(64),storeName:z.string().min(1).max(128),storeUrl:z.string().url().max(256)}).strict();

export type EcoIdentity=z.infer<typeof ecoIdentitySchema>;
export type EcoMerchantInput=z.infer<typeof ecoMerchantInput>;
export const ecoInputs={
 application:z.object({companyName:z.string().min(1).max(256),registrationNumber:z.string().min(1).max(128),categories:z.array(ecoCategory).min(1).max(4),regions:z.array(z.string().min(1).max(128)).min(1).max(30),fileIds:z.array(ecoId).min(1).max(10)}).strict(),
 agreement:z.object({agreementVersion:z.literal(ecoPolicy)}).strict(),
 listing:z.object({category:ecoCategory,title:z.string().min(1).max(128),description:z.string().min(1).max(2000),items:z.array(z.string().min(1).max(512)).min(1).max(10),regions:z.array(z.string().min(1).max(128)).min(1).max(30),platforms:z.array(z.string().min(1).max(128)).max(20),priceMinor:ecoVersion,deliveryDays:z.number().int().min(1).max(180)}).strict(),
 request:z.object({description:z.string().min(1).max(10000),fileIds:z.array(ecoId).max(10)}).strict(),
 quote:z.object({amountMinor:ecoVersion,scope:z.string().min(1).max(10000),acceptanceCriteria:z.string().min(1).max(5000),deliveryDays:z.number().int().min(1).max(180)}).strict(),
 confirm:z.object({quoteVersion:ecoVersion,policyAccepted:z.literal(ecoPolicy)}).strict(),
 delivery:z.object({content:z.string().min(1).max(10000),fileIds:z.array(ecoId).max(10)}).strict(),
 accept:z.object({deliveryVersion:ecoVersion}).strict(),
 reject:z.object({deliveryVersion:ecoVersion,reason:z.string().min(1).max(2000)}).strict(),
 refund:z.object({amountMinor:ecoVersion,reason:z.string().min(1).max(2000)}).strict(),
 refundConfirm:z.object({refundVersion:ecoVersion}).strict(),
 refundReview:z.object({refundVersion:ecoVersion,reason:z.string().min(1).max(2000)}).strict(),
 review:z.object({reason:z.string().min(1).max(2000)}).strict(),empty:z.object({}).strict(),
};
export type EcoScope={userId:string;organizationId:string};
export type EcoListing=z.infer<typeof ecoListingSchema>;
export type EcoApplication=z.infer<typeof ecoApplicationSchema>;
export type EcoRequest=z.infer<typeof ecoRequestSchema>;
export class EcoservicesError extends Error{constructor(public code:string,public status=500){super(code)}}
export async function ecoRequest<T>(scope:EcoScope,path:string,schema:z.ZodType<T>,init:RequestInit={},admin=false):Promise<T>{
 const headers=new Headers(init.headers);headers.set("X-Expected-User-ID",scope.userId);if(!admin)headers.set("X-Expected-Organization-ID",scope.organizationId);headers.set("Accept","application/json");
 let response:Response;try{response=await fetch((admin?"/api/admin/ecoservices/":"/api/ecoservices/")+path,{...init,headers,cache:"no-store",credentials:"same-origin"})}catch{throw new EcoservicesError(init.method&&init.method!=="GET"?"OUTCOME_UNKNOWN":"ECOSERVICES_UNAVAILABLE")}
 const payload=await readBoundedStrictJSON(response,1024*1024,init.signal??undefined).catch(()=>{throw new EcoservicesError(init.method&&init.method!=="GET"?"OUTCOME_UNKNOWN":"INVALID_UPSTREAM_RESPONSE",502)});
 if(!response.ok){const error=z.object({code:z.string()}).passthrough().safeParse(payload);throw new EcoservicesError(error.success?error.data.code:"ECOSERVICES_UNAVAILABLE",response.status)}
 const result=schema.safeParse(payload);if(!result.success)throw new EcoservicesError("INVALID_UPSTREAM_RESPONSE",502);return result.data;
}

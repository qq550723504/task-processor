import { z } from "zod";
import { readBoundedStrictJSON } from "./strict-json-response";
import { parseWorkbenchErrorEnvelopePayload } from "./workbench-context";

export const COMMERCIAL_MAX_BYTES = 64 * 1024;
const id = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const boundedText = (maximum: number) => z.string().min(1).refine(v => v.trim() === v && new TextEncoder().encode(v).length <= maximum && !/[\u0000-\u001f\u007f-\u009f]/.test(v));
const text = boundedText(256);
const timestamp = z.string().max(40).regex(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,9})?Z$/).refine(v => Number.isFinite(Date.parse(v)));
// Zod can run later refinements after an earlier refinement fails. Every
// predicate must therefore handle malformed decimal text without throwing.
function readInt64(value: string): bigint | null {
  if (value.length > 20 || !/^(0|-?[1-9][0-9]*)$/.test(value)) return null;
  const parsed = BigInt(value);
  return parsed >= BigInt("-9223372036854775808") && parsed <= BigInt("9223372036854775807") ? parsed : null;
}
const integer = z.string().refine(v => readInt64(v) !== null);
const nonnegative = integer.refine(v => { const parsed = readInt64(v); return parsed !== null && parsed >= BigInt(0); });
const status = z.enum(["active","trialing","expired","disabled"]);
const effective = z.enum(["active","trialing","expired","disabled","not_started"]);
const metric = z.enum(["listingkit_generations_succeeded","product_image_jobs_succeeded","shein_drafts_succeeded","shein_publishes_succeeded","storage_bytes_current"]);
const modules = z.enum(["store_management","task_import","rules","operation_strategy","listingkit","oss_storage"]);
const plan = z.object({code:z.enum(["base_payg","paid_pilot"]),name:text,source:z.literal("approved_product_description"),availability:z.enum(["not_for_sale","invitation_only"]),price:z.null(),currency:z.null()}).strict();
const subscription = z.object({plan_code:boundedText(128),plan_name:text.nullable(),status,effective_status:effective,starts_at:timestamp.nullable(),expires_at:timestamp.nullable(),updated_at:timestamp}).strict();
const limit = z.object({metric:z.enum(["store_count",...metric.options]),source_key:id,unit:z.enum(["store","operation","byte"]),kind:z.enum(["finite","unlimited"]),raw_value:nonnegative,value:nonnegative.nullable()}).strict().refine(v => v.kind==="unlimited" ? v.metric!=="store_count" && v.raw_value==="0" && v.value===null : v.value===v.raw_value && (v.metric==="store_count" || v.raw_value!=="0"));
const entitlement = z.object({module_code:modules,status,effective_status:effective,starts_at:timestamp.nullable(),expires_at:timestamp.nullable(),updated_at:timestamp,limits_scope:z.literal("explicit_grant_only"),limits:z.array(limit).max(6),uninterpreted_limit_count:z.number().int().min(0).max(64)}).strict();
const usage = z.object({module_code:z.enum(["listingkit","oss_storage"]),metric,source:z.literal("subscription_usage_ledger"),unit:z.enum(["operation","byte"]),period_key:z.string().max(16),window_start:timestamp.nullable(),window_end:timestamp.nullable(),state:z.enum(["known","unknown"]),committed:integer.nullable(),reserved:integer.nullable(),updated_at:timestamp.nullable()}).strict().refine(v => {
  const storage=v.metric==="storage_bytes_current";
  if (v.module_code!==(storage?"oss_storage":"listingkit") || v.unit!==(storage?"byte":"operation")) return false;
  if (storage ? v.period_key!=="__current__" || v.window_start!==null || v.window_end!==null : !/^\d{4}-\d{2}$/.test(v.period_key) || v.window_start===null || v.window_end===null) return false;
  if(v.state==="unknown") return v.committed===null && v.reserved===null && v.updated_at===null;
  if(v.committed===null || v.reserved===null || v.updated_at===null) return false;
  const committed=readInt64(v.committed),reserved=readInt64(v.reserved);
  if(committed===null || reserved===null) return false;
  const total=committed+reserved;
  return total>=BigInt(0) && total<=BigInt("9223372036854775807") && (storage || (committed>=BigInt(0) && reserved>=BigInt(0)));
});
const unsupported = z.object({state:z.literal("unsupported"),value:z.null()}).strict();
const overview = z.object({organization_id:id,observed_at:timestamp,plans:z.array(plan).min(1).max(2),subscription:subscription.nullable(),entitlements:z.array(entitlement).max(6),usage:z.array(usage).length(5),resource_balance:unsupported,cash_balance:unsupported}).strict().refine(v => {
  if (new Set(v.entitlements.map(e=>e.module_code)).size!==v.entitlements.length || new Set(v.usage.map(u=>u.metric)).size!==5 || new Set(v.plans.map(p=>p.code)).size!==v.plans.length) return false;
  if(!v.plans.some(p=>p.code==="base_payg") || v.plans.some(p=>p.availability!==(p.code==="base_payg"?"not_for_sale":"invitation_only") || (p.code==="paid_pilot" && v.subscription?.plan_code!=="paid_pilot"))) return false;
  const month=v.observed_at.slice(0,7),start=`${month}-01T00:00:00Z`,end=new Date(start); end.setUTCMonth(end.getUTCMonth()+1);
  if(v.usage.some(u=>u.metric!=="storage_bytes_current" && (u.period_key!==month || Date.parse(u.window_start!)!==Date.parse(start) || Date.parse(u.window_end!)!==end.getTime()))) return false;
  for(const row of [v.subscription,...v.entitlements]) {
    if(!row) continue;
    if("limits" in row) {
      if(new Set(row.limits.map(l=>l.metric)).size!==row.limits.length) return false;
      for(const value of row.limits) {
        const moduleCode=value.metric==="store_count"?"store_management":value.metric==="storage_bytes_current"?"oss_storage":"listingkit";
        const unit=value.metric==="store_count"?"store":value.metric==="storage_bytes_current"?"byte":"operation";
        const aliases=value.metric==="storage_bytes_current"?[value.metric,"storage_bytes"]:value.metric==="product_image_jobs_succeeded"?[value.metric,"product_image_jobs"]:[value.metric];
        if(row.module_code!==moduleCode || value.unit!==unit || !aliases.includes(value.source_key)) return false;
      }
    }
    if(row.starts_at && row.expires_at && Date.parse(row.starts_at)>=Date.parse(row.expires_at)) return false;
    const expected=row.status==="active" || row.status==="trialing" ? row.starts_at && Date.parse(row.starts_at)>Date.parse(v.observed_at)?"not_started":row.expires_at && Date.parse(row.expires_at)<=Date.parse(v.observed_at)?"expired":row.status:row.status;
    if(row.effective_status!==expected) return false;
  }
  return true;
});

export type CommercialOverview = z.infer<typeof overview>;
export type CommercialPlanOption = z.infer<typeof plan>;
export type CommercialSubscription = z.infer<typeof subscription>;
export type CommercialEntitlement = z.infer<typeof entitlement>;
export type CommercialLimit = z.infer<typeof limit>;
export type CommercialUsage = z.infer<typeof usage>;
export type UnsupportedCommercialValue = z.infer<typeof unsupported>;
export function parseCommercialOverview(payload:unknown):CommercialOverview|null { const result=overview.safeParse(payload); return result.success?result.data:null; }

const errorStatuses:Readonly<Record<string,number>>={INVALID_REQUEST:400,AUTHENTICATION_REQUIRED:401,PERMISSION_DENIED:403,ORGANIZATION_ACCESS_DENIED:403,ORGANIZATION_ACCESS_REVOKED:403,ORGANIZATION_SUSPENDED:403,ORGANIZATION_SELECTION_REQUIRED:409,ORGANIZATION_CONTEXT_CHANGED:409,DEPENDENCY_UNAVAILABLE:503,DEADLINE_EXCEEDED:504,INVALID_UPSTREAM_RESPONSE:502};
export function parseCommercialReadFailure(payload:unknown,status:number) {
  const parsed=parseWorkbenchErrorEnvelopePayload(payload);
  return parsed.success && errorStatuses[parsed.data.code]===status ? {...parsed.data,message:"Commercial read could not be completed",fieldErrors:[]}:null;
}
export class CommercialReadError extends Error {
  constructor(public readonly status:number,public readonly code:string,public readonly requestId="") { super("Commercial read could not be completed"); }
}

export async function getCommercialOverview(expectedOrganizationId:string,signal?:AbortSignal):Promise<CommercialOverview> {
  if(!id.safeParse(expectedOrganizationId).success) throw new CommercialReadError(400,"INVALID_REQUEST");
  const controller=new AbortController(); const abort=()=>controller.abort();
  signal?.addEventListener("abort",abort,{once:true}); if(signal?.aborted) abort();
  const timeout=setTimeout(abort,15000);
  try {
    controller.signal.throwIfAborted();
    const response=await fetch("/api/workbench/commercial/overview",{method:"GET",headers:{Accept:"application/json","X-Expected-Organization-ID":expectedOrganizationId},cache:"no-store",redirect:"manual",signal:controller.signal});
    const payload=await readBoundedStrictJSON(response,response.status===200?COMMERCIAL_MAX_BYTES:8192,controller.signal);
    controller.signal.throwIfAborted();
    if(response.status===200) { const result=parseCommercialOverview(payload); if(result?.organization_id===expectedOrganizationId) return result; }
    const failure=parseCommercialReadFailure(payload,response.status);
    throw failure?new CommercialReadError(response.status,failure.code,failure.requestId):new CommercialReadError(502,"INVALID_UPSTREAM_RESPONSE");
  } catch(error) {
    if(controller.signal.aborted) throw new CommercialReadError(504,"DEADLINE_EXCEEDED");
    if(error instanceof CommercialReadError) throw error;
    throw new CommercialReadError(502,"INVALID_UPSTREAM_RESPONSE");
  } finally { clearTimeout(timeout); signal?.removeEventListener("abort",abort); }
}

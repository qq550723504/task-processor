import {z} from "zod";
import {readBoundedStrictJSON} from "./strict-json-response";
import {isAcquisitionUUID} from "../contracts/product-acquisition";
import {imageSetMethods,imageSetRequestSchema,imageSetSuccessStatus,imageSetResponseMatches,type ImageSetRoute} from "../contracts/product-image-set";

export type ImageSetScope={userId:string;organizationId:string;kind:"acquisition"|"supply";contextId:string};
export class ImageSetError extends Error { constructor(public code:string,public status=500){super(code)} }
function imageSetBase(scope:ImageSetScope){
 if(!isAcquisitionUUID(scope.contextId)||!scope.userId||!scope.organizationId)throw new ImageSetError("INVALID_IMAGE_REQUEST",400);
 return scope.kind==="acquisition"?`/api/workbench/sourcing/1688/acquisitions/${scope.contextId}/images`:`/api/workbench/supply-preparations/sources/${scope.contextId}/images`;
}
export async function imageSetRequest<T>(scope:ImageSetScope,action:ImageSetRoute,schema:z.ZodType<T>,options:{runId?:string;requestKey?:string;body?:unknown;signal?:AbortSignal;query?:string;approvalId?:string}={}):Promise<T>{
 const method=imageSetMethods[action],mutation=method==="POST"&&action!=="preview"&&action!=="requirements";
 let path=imageSetBase(scope);
 if(action==="sources"||action==="prepare"||action==="requirements")path+="/"+action;
 else if(action==="verify-prepare"){
  if(!isAcquisitionUUID(options.requestKey??""))throw new ImageSetError("INVALID_IMAGE_REQUEST",400);path+="/by-key/"+options.requestKey;
 }else if(action==="recent")path+="/runs";
 else {
  if(!isAcquisitionUUID(options.runId??""))throw new ImageSetError("INVALID_IMAGE_REQUEST",400);
  path+="/runs/"+options.runId+(action==="read"?"":"/"+action);
 }
 if(action==="approval"){if(!isAcquisitionUUID(options.approvalId??""))throw new ImageSetError("INVALID_IMAGE_REQUEST",400);path=path.replace(/\/approval$/,"/approvals/"+options.approvalId)}
 if(options.query)path+="?"+options.query;
 const body=method==="POST"?imageSetRequestSchema(action).safeParse(options.body):null;
 if(method==="POST"&&!body?.success)throw new ImageSetError("INVALID_IMAGE_REQUEST",400);
 const controller=new AbortController(),abort=()=>controller.abort();
 if(options.signal?.aborted)throw new ImageSetError("CONTEXT_CHANGED",409);
 // Allow the BFF's 32s image window to finish before the browser deadline.
 options.signal?.addEventListener("abort",abort,{once:true});const deadline=setTimeout(abort,35_000);
 try{
  const headers=new Headers({Accept:"application/json","X-Expected-User-ID":scope.userId,"X-Expected-Organization-ID":scope.organizationId});
  if(method==="POST")headers.set("Content-Type","application/json");
  if(action==="prepare"||action==="regenerate"){
   if(!isAcquisitionUUID(options.requestKey??""))throw new ImageSetError("INVALID_IMAGE_REQUEST",400);headers.set("Idempotency-Key",options.requestKey!);
  }
  const response=await fetch(path,{method,headers,body:body?.success?JSON.stringify(body.data):undefined,credentials:"same-origin",cache:"no-store",redirect:"error",signal:controller.signal});
  const payload=await readBoundedStrictJSON(response,2*1024*1024,controller.signal);
  if(!response.ok){
   const error=z.object({code:z.string().optional(),error:z.object({code:z.string()}).optional()}).safeParse(payload);
   throw new ImageSetError(error.success?(error.data.code??error.data.error?.code??"IMAGE_UNAVAILABLE"):"IMAGE_UNAVAILABLE",response.status);
  }
  const parsed=schema.safeParse(payload);
  if(response.status!==imageSetSuccessStatus(action)||!parsed.success)throw new ImageSetError(mutation?"OUTCOME_UNKNOWN":"INVALID_UPSTREAM_RESPONSE",mutation?503:502);
  const data=parsed.data;
  if(!imageSetResponseMatches(action,data,{kind:scope.kind,contextId:scope.contextId,runId:options.runId,approvalId:options.approvalId}))throw new ImageSetError(mutation?"OUTCOME_UNKNOWN":"CONTEXT_CHANGED",mutation?503:409);
  controller.signal.throwIfAborted();return data;
 }catch(error){if(error instanceof ImageSetError)throw error;throw new ImageSetError(mutation?"OUTCOME_UNKNOWN":"IMAGE_UNAVAILABLE",mutation?503:502)}
 finally{clearTimeout(deadline);options.signal?.removeEventListener("abort",abort)}
}

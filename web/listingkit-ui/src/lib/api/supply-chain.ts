import { z } from "zod";
import * as c from "../contracts/supply-chain";
import { collectionID } from "../contracts/product-collection";
import type { CollectionScope } from "./product-collection";
import { readBoundedStrictJSON } from "./strict-json-response";
import { supplyReviewCommand,supplyReviewApplyCommandSchema,supplyReviewDecisionCommandSchema } from "./supply-review-command";
import { SupplyAPIError } from "./supply-error";

export type SupplyScope=CollectionScope;
export type SupplyIntent=Readonly<SupplyScope & { key:string; route:"transfer"|"save-target"|"approve"|"create-operation"|"review-decision"|"review-apply"; command:unknown }>;
export function supplyIntentRequestSchema(route:SupplyIntent["route"]){
 if(route==="review-decision")return supplyReviewDecisionCommandSchema;
 if(route==="review-apply")return supplyReviewApplyCommandSchema;
 return c.supplyRequestSchema(route);
}
export {SupplyAPIError} from "./supply-error";
const base="/api/workbench/supply-preparations";
type Query={after?:string;keyword?:string;limit?:number};
const query=(q:Query)=>{const p=new URLSearchParams({limit:String(q.limit??50)});if(q.after)p.set("after",q.after);if(q.keyword)p.set("keyword",q.keyword);return `?${p}`};
export const listSupplyPreparations=(scope:SupplyScope,q:Query={},signal?:AbortSignal)=>send(base+query(q),scope,c.preparationPageSchema,signal);
export const readSupplyPreparation=async(scope:SupplyScope,id:string,signal?:AbortSignal)=>{
 const value=await send(`${base}/${collectionID.parse(id)}`,scope,c.preparationSchema,signal);if(value.id!==id)throw new SupplyAPIError("DEPENDENCY_UNAVAILABLE",502);return value;
};
export const listSupplyOperations=async(scope:SupplyScope,preparation:string,store:string,q:Query={},signal?:AbortSignal)=>{
 const value=await send(`${base}/${collectionID.parse(preparation)}/operations${query(q)}&storeId=${collectionID.parse(store)}`,scope,c.operationPageSchema,signal);
 if(value.items.some(o=>o.input.preparationId!==preparation||o.input.storeId!==store))throw new SupplyAPIError("DEPENDENCY_UNAVAILABLE",502);return value;
};
export const listSupplySources=(scope:SupplyScope,id:string,q:Query={},signal?:AbortSignal)=>send(`${base}/${collectionID.parse(id)}/sources${query(q)}`,scope,c.sourcePageSchema,signal);
export const readSupplySource=async(scope:SupplyScope,id:string,signal?:AbortSignal)=>{
 const value=await send(`${base}/sources/${collectionID.parse(id)}`,scope,c.sourceDetailSchema,signal);if(value.source.id!==id)throw new SupplyAPIError("DEPENDENCY_UNAVAILABLE",502);return value;
};
export const readSupplyTarget=async(scope:SupplyScope,source:string,store:string,signal?:AbortSignal)=>{
 const value=await send(`${base}/sources/${collectionID.parse(source)}/targets/${collectionID.parse(store)}`,scope,c.targetRecordSchema,signal);if(value.source.id!==source||value.merchant.store_id!==store)throw new SupplyAPIError("DEPENDENCY_UNAVAILABLE",502);return value;
};
export const readSupplyPublication=async(scope:SupplyScope,source:string,store:string,signal?:AbortSignal)=>{
 const value=await send(`${base}/sources/${collectionID.parse(source)}/publications/${collectionID.parse(store)}`,scope,c.publicationSchema,signal);if(value.sourceId!==source||value.storeId!==store)throw new SupplyAPIError("DEPENDENCY_UNAVAILABLE",502);return value;
};
export const readSupplyRecord=async(scope:SupplyScope,id:string,signal?:AbortSignal)=>{
 const value=await send(`${base}/records/${collectionID.parse(id)}`,scope,c.targetRecordSchema,signal);if(value.id!==id)throw new SupplyAPIError("DEPENDENCY_UNAVAILABLE",502);return value;
};
export const supplyRules=(scope:SupplyScope,input:c.SupplyTargetInput,signal?:AbortSignal)=>send(`${base}/target-rules`,scope,c.rulesSchema,signal,{method:"POST",command:c.targetInputSchema.parse(input)});
export const supplyInventory=(scope:SupplyScope,input:z.infer<typeof c.sourceSelectionSchema>,signal?:AbortSignal)=>send(`${base}/images/inventory`,scope,c.inventorySchema,signal,{method:"POST",command:c.sourceSelectionSchema.parse(input)});
export async function supplyCommand(intent:SupplyIntent,signal?:AbortSignal){
	if(intent.route==="review-decision"||intent.route==="review-apply")return supplyReviewCommand(intent,signal);
 const schema=c.supplyRequestSchema(intent.route);if(!schema)throw new SupplyAPIError("INVALID_REQUEST",400);
 const suffix=intent.route==="transfer"?"transfer":intent.route==="save-target"?"targets":intent.route==="approve"?"images/approve":"operations";
 const value=await send(`${base}/${suffix}`,intent,c.supplyResponseSchema(intent.route),signal,{method:"POST",command:schema.parse(intent.command),key:collectionID.parse(intent.key),mutates:true});
 return commandResult(value,intent,true);
}
export async function readSupplyCommand(intent:SupplyIntent,signal?:AbortSignal){
 const key=collectionID.parse(intent.key);
 // Approval is a safely repeatable local fact commit. Replaying the exact full
 // set with the same key reads its immutable receipt without a platform send.
 if(intent.route==="approve"||intent.route==="review-decision"||intent.route==="review-apply")return supplyCommand(intent,signal);
 const suffix=intent.route==="transfer"?`transfers/by-key/${key}`:intent.route==="save-target"?`target-commands/${key}`:`operations/by-key/${key}`;
 const route=intent.route==="transfer"?"transfer-read":intent.route==="save-target"?"target-command":"operation-key";
 const value=await send(`${base}/${suffix}`,intent,c.supplyResponseSchema(route),signal);
 return commandResult(value,intent,false);
}
export const readSupplyOperation=async(scope:SupplyScope,id:string,signal?:AbortSignal)=>{
 const value=await send(`${base}/operations/${collectionID.parse(id)}`,scope,c.operationSchema,signal);if(value.id!==id)throw new SupplyAPIError("DEPENDENCY_UNAVAILABLE",502);return value;
};
export const listSupplyOperationItems=(scope:SupplyScope,id:string,q:Query={},signal?:AbortSignal)=>send(`${base}/operations/${collectionID.parse(id)}/items${query(q)}`,scope,c.operationItemPageSchema,signal);
export const controlSupplyOperation=async(scope:SupplyScope,id:string,action:"ensure"|"cancel",signal?:AbortSignal)=>{
 const value=await send(`${base}/operations/${collectionID.parse(id)}/${action}`,scope,c.operationSchema,signal,{method:"POST",mutates:true});if(value.id!==id)throw new SupplyAPIError("OUTCOME_UNKNOWN",503);return value;
};
async function send<T extends z.ZodType>(path:string,scope:SupplyScope,schema:T,signal?:AbortSignal,input?:{method:"POST";command?:unknown;key?:string;mutates?:boolean}):Promise<z.output<T>>{
 const bounded=/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/;
 if(!bounded.test(scope.userId)||!bounded.test(scope.organizationId))throw new SupplyAPIError("INVALID_REQUEST",400);
 const controller=new AbortController();const abort=()=>controller.abort();signal?.addEventListener("abort",abort,{once:true});if(signal?.aborted)abort();
 const timeout=setTimeout(abort,27_000);const unavailable=()=>new SupplyAPIError(input?.mutates?"OUTCOME_UNKNOWN":"DEPENDENCY_UNAVAILABLE",input?.mutates?503:502);
 try{
  const headers=new Headers({Accept:"application/json","X-Expected-Organization-ID":scope.organizationId,"X-Expected-User-ID":scope.userId});
  if(input?.command!==undefined)headers.set("Content-Type","application/json");if(input?.key)headers.set("Idempotency-Key",input.key);
  const response=await fetch(path,{method:input?.method??"GET",body:input?.command!==undefined?JSON.stringify(input.command):undefined,headers,credentials:"same-origin",cache:"no-store",redirect:"error",signal:controller.signal});
  const payload=await readBoundedStrictJSON(response,c.SUPPLY_MAX_BYTES,controller.signal);
  if(!response.ok){const error=z.object({code:z.string().regex(/^[A-Z][A-Z0-9_]{0,79}$/)}).safeParse(payload);if(error.success)throw new SupplyAPIError(error.data.code,response.status);throw unavailable()}
  const value=schema.safeParse(payload);if(response.status!==200 || !value.success)throw unavailable();return value.data;
 }catch(error){if(error instanceof SupplyAPIError)throw error;throw unavailable()}finally{clearTimeout(timeout);signal?.removeEventListener("abort",abort)}
}

function commandResult(value:unknown,intent:SupplyIntent,mutates:boolean){
 let valid=false;
 if(intent.route==="approve")valid=c.imageApprovalReceiptSchema.parse(value).action_id===intent.key;
 if(intent.route==="transfer"){
  const input=c.transferInputSchema.parse(intent.command),result=c.transferReceiptSchema.parse(value);
  valid=result.preparation.sourceBatchId===input.batchId&&result.preparation.sourceRevision===input.expectedRevision&&(!input.itemIds?.length||result.preparation.count===input.itemIds.length);
 }
 if(intent.route==="save-target"){
  const input=c.targetInputSchema.parse(intent.command),result=c.targetReceiptSchema.parse(value).record;
  valid=result.source.id===input.sourceId&&result.merchant.store_id===input.storeId&&result.input.expectedRevision===input.expectedRevision&&result.effectiveVersion===input.effectiveVersion&&(result.applyReceiptId??"")===(input.applyReceiptId??"");
 }
 if(intent.route==="create-operation"){
  const input=c.operationInputSchema.parse(intent.command),receipt=c.operationReceiptSchema.safeParse(value);
  const output=(receipt.success?receipt.data.operation:c.operationSchema.parse(value)).input;
  const canonical=(v:z.infer<typeof c.operationInputSchema>)=>JSON.stringify([v.preparationId,v.expectedRevision,v.action,v.storeId,[...(v.sourceIds??[])].sort(),v.categoryId??0,v.titleTemplateId??"",v.titleTemplateRevision??"",v.titleQuoteHash??"",v.imageTemplateId??""]);
  valid=canonical(input)===canonical(output);
 }
 if(!valid)throw new SupplyAPIError(mutates?"OUTCOME_UNKNOWN":"DEPENDENCY_UNAVAILABLE",mutates?503:502);
 return value;
}

export const readSupplyOptimizationOptions=(scope:SupplyScope,q:Query={},signal?:AbortSignal)=>send(`${base}/optimization-options${query(q)}`,scope,c.optimizationOptionsSchema,signal);
